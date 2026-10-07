package ubusconn

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	defaultSocket  = "/var/run/ubus/ubus.sock"
	maxMessageSize = 1 << 24 // the blob length field is 24 bits
	defaultTimeout = 10 * time.Second
)

// Client is a synchronous ubus client over the unix socket. It is not safe
// for concurrent use; guard with a mutex or dial one per caller.
type Client struct {
	conn    net.Conn
	timeout time.Duration
	seq     uint16
	objects map[string]uint32 // object path -> id, filled by Lookup
}

// Dial connects to the ubus socket ("" selects the default path) and
// completes the server HELLO handshake.
func Dial(socket string) (*Client, error) {
	if socket == "" {
		socket = defaultSocket
	}
	conn, err := net.DialTimeout("unix", socket, defaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("ubusconn: dial %s: %w", socket, err)
	}
	c := &Client{conn: conn, timeout: defaultTimeout, objects: map[string]uint32{}}
	hdr, _, err := c.readMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ubusconn: hello: %w", err)
	}
	if hdr.typ != msgHello {
		conn.Close()
		return nil, fmt.Errorf("ubusconn: expected HELLO, got message type %d", hdr.typ)
	}
	return c, nil
}

// Close releases the connection.
func (c *Client) Close() error { return c.conn.Close() }

type header struct {
	typ  byte
	seq  uint16
	peer uint32
}

// send writes one ubus message: 8-byte header + blob container body.
func (c *Client) send(typ byte, body []byte) error {
	c.seq++
	buf := make([]byte, 8+len(body))
	buf[1] = typ
	binary.BigEndian.PutUint16(buf[2:4], c.seq)
	// peer stays 0: requests go to ubusd, replies are matched by seq.
	copy(buf[8:], body)
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return err
	}
	_, err := c.conn.Write(buf)
	return err
}

// readMessage reads one full ubus message. The body length is carried in the
// container's own id_len, so there is no separate length prefix.
func (c *Client) readMessage() (header, []byte, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return header{}, nil, err
	}
	var hb [8]byte
	if _, err := io.ReadFull(c.conn, hb[:]); err != nil {
		return header{}, nil, err
	}
	if hb[0] != 0 {
		return header{}, nil, fmt.Errorf("ubusconn: unsupported version %d", hb[0])
	}
	h := header{typ: hb[1], seq: binary.BigEndian.Uint16(hb[2:4]), peer: binary.BigEndian.Uint32(hb[4:8])}

	var lb [4]byte
	if _, err := io.ReadFull(c.conn, lb[:]); err != nil {
		return h, nil, err
	}
	rawLen := int(binary.BigEndian.Uint32(lb[:]) & lenMask)
	if rawLen < 4 || rawLen > maxMessageSize {
		return h, nil, fmt.Errorf("ubusconn: message length %d out of range", rawLen)
	}
	body := make([]byte, rawLen)
	copy(body, lb[:])
	if rawLen > 4 {
		if _, err := io.ReadFull(c.conn, body[4:]); err != nil {
			return h, nil, err
		}
	}
	return h, body, nil
}

// statusCode extracts UBUS_ATTR_STATUS from a STATUS message body.
func statusCode(body []byte) int {
	attrs, err := parseContainer(body)
	if err != nil {
		return -1
	}
	for _, a := range attrs {
		if a.id == attrStatus && len(a.payload) >= 4 {
			return int(int32(binary.BigEndian.Uint32(a.payload[:4])))
		}
	}
	return -1
}

// Lookup resolves an object path to its numeric id, caching the result.
func (c *Client) Lookup(name string) (uint32, error) {
	if id, ok := c.objects[name]; ok {
		return id, nil
	}
	var b bodyBuilder
	b.putString(attrObjPath, name)
	if err := c.send(msgLookup, b.bytes()); err != nil {
		return 0, err
	}
	var id uint32
	found := false
	for {
		h, body, err := c.readMessage()
		if err != nil {
			return 0, err
		}
		switch h.typ {
		case msgData:
			attrs, err := parseContainer(body)
			if err != nil {
				return 0, err
			}
			for _, a := range attrs {
				if a.id == attrObjID && len(a.payload) >= 4 {
					id = binary.BigEndian.Uint32(a.payload[:4])
					found = true
				}
			}
		case msgStatus:
			if code := statusCode(body); code != 0 {
				return 0, fmt.Errorf("ubusconn: lookup %q: status %d", name, code)
			}
			if !found {
				return 0, fmt.Errorf("ubusconn: object %q not found", name)
			}
			c.objects[name] = id
			return id, nil
		}
	}
}

// Call invokes object.method with args encoded as a blobmsg table (an empty
// table when args is nil, which ubusd requires even for no-argument methods)
// and returns the decoded result table.
func (c *Client) Call(object, method string, args map[string]any) (map[string]any, error) {
	id, err := c.Lookup(object)
	if err != nil {
		return nil, err
	}
	table, err := encodeBlobmsgTable(args)
	if err != nil {
		return nil, err
	}
	var b bodyBuilder
	b.putU32(attrObjID, id)
	b.putString(attrMethod, method)
	b.put(attrData, table)
	if err := c.send(msgInvoke, b.bytes()); err != nil {
		return nil, err
	}
	var result map[string]any
	for {
		h, body, err := c.readMessage()
		if err != nil {
			return nil, err
		}
		switch h.typ {
		case msgData:
			attrs, err := parseContainer(body)
			if err != nil {
				return nil, err
			}
			for _, a := range attrs {
				if a.id == attrData {
					result, err = decodeTable(a.payload)
					if err != nil {
						return nil, err
					}
				}
			}
		case msgStatus:
			if code := statusCode(body); code != 0 {
				return nil, fmt.Errorf("ubusconn: call %s.%s: status %d", object, method, code)
			}
			if result == nil {
				result = map[string]any{}
			}
			return result, nil
		}
	}
}
