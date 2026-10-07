// Package ubusconn is a spike: a minimal pure-Go client for OpenWrt's ubus,
// speaking the native blob/blobmsg protocol over the ubus unix socket
// (/var/run/ubus/ubus.sock). It exists to measure whether replacing forked
// `ubus call` CLI invocations with an in-process client is worth it on
// low-end mipsle routers.
//
// Wire format (documented from libubox blob.h/blobmsg.h and ubus ubusmsg.h):
//
//   - Every message is an 8-byte header (version u8, type u8, seq u16 BE,
//     peer u32 BE) followed by one blob container.
//   - A blob_attr is a big-endian u32 id_len: bit 31 extended (blobmsg:
//     payload starts with a name), bits 30-24 id/type, bits 23-0 length
//     INCLUDING the 4-byte header. Payload is padded to 4 bytes.
//   - blobmsg attribute: u16 BE namelen, name bytes, NUL, padded to 4
//     (hdrlen = pad4(2+namelen+1)), then the value. Types: 1 ARRAY, 2 TABLE,
//     3 STRING, 4 INT64, 5 INT32, 6 INT16, 7 INT8/BOOL, 8 DOUBLE.
//
// Call flow: connect (server sends HELLO, hdr.peer = our client id), LOOKUP
// {OBJPATH: name} -> DATA (OBJID) + STATUS, INVOKE {OBJID, METHOD, DATA:
// blobmsg table} -> DATA (UBUS_ATTR_DATA blobmsg table) + STATUS(0).
package ubusconn

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ubus message types (ubusmsg.h enum ubus_msg_type).
const (
	msgHello  = 0
	msgStatus = 1
	msgData   = 2
	msgLookup = 4
	msgInvoke = 5
)

// ubus message attribute ids (ubusmsg.h enum ubus_msg_attr).
const (
	attrStatus  = 1
	attrObjPath = 2
	attrObjID   = 3
	attrMethod  = 4
	attrData    = 7
)

// blobmsg value types (blobmsg.h enum blobmsg_type).
const (
	bmArray  = 1
	bmTable  = 2
	bmString = 3
	bmInt64  = 4
	bmInt32  = 5
	bmInt16  = 6
	bmInt8   = 7
	bmDouble = 8
)

const (
	flagExtended = 0x80000000
	idShift      = 24
	idMask       = 0x7f
	lenMask      = 0x00ffffff
	align        = 4
)

func pad4(n int) int { return (n + align - 1) &^ (align - 1) }

func idLen(id int, extended bool, rawLen int) uint32 {
	v := uint32(rawLen) & lenMask
	v |= uint32(id&idMask) << idShift
	if extended {
		v |= flagExtended
	}
	return v
}

// attr is a decoded blob attribute.
type attr struct {
	id       int
	extended bool
	payload  []byte
}

// parseAttrs splits a packed sequence of blob attributes (a container's
// payload). Each attribute occupies pad4(rawLen) bytes.
func parseAttrs(b []byte) ([]attr, error) {
	var out []attr
	for len(b) >= align {
		raw := int(binary.BigEndian.Uint32(b[:4]) & lenMask)
		if raw < align || raw > len(b) {
			return nil, fmt.Errorf("ubusconn: attr length %d out of bounds (%d available)", raw, len(b))
		}
		out = append(out, attr{
			id:       int((binary.BigEndian.Uint32(b[:4]) >> idShift) & idMask),
			extended: binary.BigEndian.Uint32(b[:4])&flagExtended != 0,
			payload:  b[4:raw],
		})
		adv := pad4(raw)
		if adv > len(b) {
			adv = len(b)
		}
		b = b[adv:]
	}
	return out, nil
}

// parseContainer parses a full blob container (u32 header + children) as
// received in a ubus message body.
func parseContainer(b []byte) ([]attr, error) {
	if len(b) < align {
		return nil, errors.New("ubusconn: short message body")
	}
	raw := int(binary.BigEndian.Uint32(b[:4]) & lenMask)
	if raw < align || raw > len(b) {
		return nil, fmt.Errorf("ubusconn: container length %d out of bounds (%d available)", raw, len(b))
	}
	return parseAttrs(b[4:raw])
}

// --- encoding ---

// bodyBuilder accumulates plain (non-extended) blob attributes, the envelope
// of a ubus message (OBJPATH, OBJID, METHOD, DATA).
type bodyBuilder struct{ buf []byte }

func (b *bodyBuilder) put(id int, payload []byte) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], idLen(id, false, len(payload)+4))
	b.buf = append(b.buf, hdr[:]...)
	b.buf = append(b.buf, payload...)
	for len(b.buf)%align != 0 {
		b.buf = append(b.buf, 0)
	}
}

func (b *bodyBuilder) putString(id int, s string) {
	b.put(id, append([]byte(s), 0)) // NUL-terminated like blob_put_string
}

func (b *bodyBuilder) putU32(id int, v uint32) {
	var p [4]byte
	binary.BigEndian.PutUint32(p[:], v)
	b.put(id, p[:])
}

// bytes returns the complete message body: a container (id 0) wrapping the
// accumulated attributes, exactly what follows the 8-byte message header.
func (b *bodyBuilder) bytes() []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], idLen(0, false, len(b.buf)+4))
	return append(hdr[:], b.buf...)
}

// appendBlobmsg appends one extended attribute (blobmsg): a padded name
// header followed by a pre-encoded value payload, padded to 4 bytes.
func appendBlobmsg(dst []byte, typ int, name string, value []byte) []byte {
	hdrLen := pad4(2 + len(name) + 1)
	p := make([]byte, hdrLen+len(value))
	binary.BigEndian.PutUint16(p[0:2], uint16(len(name)))
	copy(p[2:], name)
	copy(p[hdrLen:], value)
	rawLen := 4 + len(p)
	a := make([]byte, pad4(rawLen))
	binary.BigEndian.PutUint32(a[:], idLen(typ, true, rawLen))
	copy(a[4:], p)
	return append(dst, a...)
}

// encodeBlobmsgTable encodes a Go map as a blobmsg table body. Supported
// values: string, bool, any signed integer up to int64, float64,
// map[string]any, map[string]string, []any, []string. Anything else errors
// before anything is sent.
func encodeBlobmsgTable(t map[string]any) ([]byte, error) {
	var body []byte
	for name, val := range t {
		var err error
		if body, err = appendValue(body, name, val); err != nil {
			return nil, err
		}
	}
	return body, nil
}

func appendValue(body []byte, name string, val any) ([]byte, error) {
	switch v := val.(type) {
	case string:
		return appendBlobmsg(body, bmString, name, append([]byte(v), 0)), nil
	case bool:
		b := byte(0)
		if v {
			b = 1
		}
		return appendBlobmsg(body, bmInt8, name, []byte{b}), nil
	case int:
		return appendInt(body, name, int64(v))
	case int8:
		return appendBlobmsg(body, bmInt8, name, []byte{byte(v)}), nil
	case int16:
		var p [2]byte
		binary.BigEndian.PutUint16(p[:], uint16(v))
		return appendBlobmsg(body, bmInt16, name, p[:]), nil
	case int32:
		var p [4]byte
		binary.BigEndian.PutUint32(p[:], uint32(v))
		return appendBlobmsg(body, bmInt32, name, p[:]), nil
	case int64:
		var p [8]byte
		binary.BigEndian.PutUint64(p[:], uint64(v))
		return appendBlobmsg(body, bmInt64, name, p[:]), nil
	case float64:
		var p [8]byte
		binary.BigEndian.PutUint64(p[:], math.Float64bits(v))
		return appendBlobmsg(body, bmDouble, name, p[:]), nil
	case map[string]string:
		var sub []byte
		for k, s := range v {
			sub = appendBlobmsg(sub, bmString, k, append([]byte(s), 0))
		}
		return appendBlobmsg(body, bmTable, name, sub), nil
	case map[string]any:
		sub, err := encodeBlobmsgTable(v)
		if err != nil {
			return nil, err
		}
		return appendBlobmsg(body, bmTable, name, sub), nil
	case []string:
		var sub []byte
		for _, s := range v {
			sub = appendBlobmsg(sub, bmString, "", append([]byte(s), 0))
		}
		return appendBlobmsg(body, bmArray, name, sub), nil
	case []any:
		var sub []byte
		for _, el := range v {
			var err error
			if sub, err = appendValue(sub, "", el); err != nil {
				return nil, err
			}
		}
		return appendBlobmsg(body, bmArray, name, sub), nil
	default:
		return nil, fmt.Errorf("ubusconn: cannot encode value %q of type %T", name, val)
	}
}

func appendInt(body []byte, name string, v int64) ([]byte, error) {
	switch {
	case v >= math.MinInt8 && v <= math.MaxInt8:
		return appendBlobmsg(body, bmInt8, name, []byte{byte(int8(v))}), nil
	case v >= math.MinInt16 && v <= math.MaxInt16:
		var p [2]byte
		binary.BigEndian.PutUint16(p[:], uint16(int16(v)))
		return appendBlobmsg(body, bmInt16, name, p[:]), nil
	case v >= math.MinInt32 && v <= math.MaxInt32:
		var p [4]byte
		binary.BigEndian.PutUint32(p[:], uint32(int32(v)))
		return appendBlobmsg(body, bmInt32, name, p[:]), nil
	default:
		var p [8]byte
		binary.BigEndian.PutUint64(p[:], uint64(v))
		return appendBlobmsg(body, bmInt64, name, p[:]), nil
	}
}

// --- decoding ---

// decodeBlobmsgValue splits one extended attribute into name and typed
// value. Tables decode to map[string]any, arrays to []any, integers to
// int64, doubles to float64.
func decodeBlobmsgValue(a attr) (string, any, error) {
	p := a.payload
	if len(p) < 2 {
		return "", nil, errors.New("ubusconn: short blobmsg attribute")
	}
	nameLen := int(binary.BigEndian.Uint16(p[:2]))
	hdrLen := pad4(2 + nameLen + 1)
	if hdrLen > len(p) {
		return "", nil, errors.New("ubusconn: blobmsg name out of bounds")
	}
	name := string(p[2 : 2+nameLen])
	data := p[hdrLen:]

	switch a.id {
	case bmTable:
		v, err := decodeTable(data)
		return name, v, err
	case bmArray:
		v, err := decodeArray(data)
		return name, v, err
	case bmString:
		if i := bytes.IndexByte(data, 0); i >= 0 {
			data = data[:i]
		}
		return name, string(data), nil
	case bmInt64:
		if len(data) < 8 {
			return "", nil, errors.New("ubusconn: short INT64")
		}
		return name, int64(binary.BigEndian.Uint64(data[:8])), nil
	case bmInt32:
		if len(data) < 4 {
			return "", nil, errors.New("ubusconn: short INT32")
		}
		return name, int64(int32(binary.BigEndian.Uint32(data[:4]))), nil
	case bmInt16:
		if len(data) < 2 {
			return "", nil, errors.New("ubusconn: short INT16")
		}
		return name, int64(int16(binary.BigEndian.Uint16(data[:2]))), nil
	case bmInt8:
		if len(data) < 1 {
			return "", nil, errors.New("ubusconn: short INT8")
		}
		return name, int64(int8(data[0])), nil
	case bmDouble:
		if len(data) < 8 {
			return "", nil, errors.New("ubusconn: short DOUBLE")
		}
		return name, math.Float64frombits(binary.BigEndian.Uint64(data[:8])), nil
	default:
		return name, nil, nil
	}
}

// decodeTable decodes a blobmsg table body into name -> value pairs.
func decodeTable(body []byte) (map[string]any, error) {
	attrs, err := parseAttrs(body)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(attrs))
	for _, a := range attrs {
		name, val, err := decodeBlobmsgValue(a)
		if err != nil {
			return nil, err
		}
		out[name] = val
	}
	return out, nil
}

// decodeArray decodes a blobmsg array body. Elements carry empty names.
func decodeArray(body []byte) ([]any, error) {
	attrs, err := parseAttrs(body)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(attrs))
	for _, a := range attrs {
		_, val, err := decodeBlobmsgValue(a)
		if err != nil {
			return nil, err
		}
		out = append(out, val)
	}
	return out, nil
}
