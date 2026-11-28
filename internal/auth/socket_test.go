package auth

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// fakeRPCd speaks just enough of the ubus wire protocol to answer one
// session.login: HELLO on connect, LOOKUP answered with an object id, INVOKE
// answered with the configured status and, when the status is 0, a result
// table carrying a session token. The framing is built by hand from
// docs/ubus-client-spike.md, the same way the ubusconn tests do.
type fakeRPCd struct {
	sock   string
	status int
	token  string
	// invoked collects the raw INVOKE bodies, so a test can check what the
	// client actually sent.
	invoked chan []byte
}

func startFakeRPCd(t *testing.T, status int, token string) *fakeRPCd {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "ubus.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeRPCd{sock: sock, status: status, token: token, invoked: make(chan []byte, 8)}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	return f
}

func (f *fakeRPCd) serve(conn net.Conn) {
	defer conn.Close()
	if err := writeFrame(conn, msgHello, 0, 7, ubusContainer(nil)); err != nil {
		return
	}
	for {
		typ, _, body, err := readFrame(conn)
		if err != nil {
			return
		}
		switch typ {
		case msgLookup:
			var d []byte
			if f.status == ubusStatusNotFound {
				// rpcd is not running: nobody serves "session".
				if err := writeFrame(conn, msgStatus, 1, 0, ubusPlainAttr(attrStatus, u32(ubusStatusNotFound))); err != nil {
					return
				}
				continue
			}
			d = ubusPlainAttr(attrObjID, u32(42))
			if err := writeFrame(conn, msgData, 1, 0, ubusContainer(d)); err != nil {
				return
			}
			if err := writeFrame(conn, msgStatus, 1, 0, ubusContainer(ubusPlainAttr(attrStatus, u32(0)))); err != nil {
				return
			}
		case msgInvoke:
			select {
			case f.invoked <- body:
			default:
			}
			if f.status != 0 {
				if err := writeFrame(conn, msgStatus, 1, 0, ubusContainer(ubusPlainAttr(attrStatus, u32(uint32(f.status))))); err != nil {
					return
				}
				continue
			}
			table := ubusBlobmsg(3, "ubus_rpc_session", append([]byte(f.token), 0))
			if err := writeFrame(conn, msgData, 1, 0, ubusContainer(ubusPlainAttr(attrData, table))); err != nil {
				return
			}
			if err := writeFrame(conn, msgStatus, 1, 0, ubusContainer(ubusPlainAttr(attrStatus, u32(0)))); err != nil {
				return
			}
		}
	}
}

// statuses used by the fake, mirroring enum ubus_msg_status.
const (
	ubusStatusPermissionDenied = 6
	ubusStatusNotFound         = 4
)

// message types and attribute ids used by the fake (ubusmsg.h).
const (
	msgHello   = 0
	msgStatus  = 1
	msgData    = 2
	msgLookup  = 4
	msgInvoke  = 5
	attrStatus = 1
	attrObjID  = 3
	attrData   = 7
)

func u32(v uint32) []byte {
	var p [4]byte
	binary.BigEndian.PutUint32(p[:], v)
	return p[:]
}

// ubusPlainAttr encodes one non-extended blob attribute (the message
// envelope: OBJID, METHOD, DATA...).
func ubusPlainAttr(id int, payload []byte) []byte {
	raw := len(payload) + 4
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(raw)|uint32(id&0x7f)<<24)
	out := append(hdr[:], payload...)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}

// ubusBlobmsg encodes one blobmsg attribute: padded name header plus value.
func ubusBlobmsg(typ int, name string, value []byte) []byte {
	hdrLen := (2 + len(name) + 1 + 3) &^ 3
	p := make([]byte, hdrLen+len(value))
	binary.BigEndian.PutUint16(p[0:2], uint16(len(name)))
	copy(p[2:], name)
	copy(p[hdrLen:], value)
	raw := len(p) + 4
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 0x80000000|uint32(raw)|uint32(typ&0x7f)<<24)
	out := append(hdr[:], p...)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}

// ubusContainer wraps children in a top-level blob container, which is what
// follows the 8-byte message header.
func ubusContainer(children []byte) []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(children)+4))
	return append(hdr[:], children...)
}

func writeFrame(w io.Writer, typ byte, seq uint16, peer uint32, body []byte) error {
	buf := make([]byte, 8+len(body))
	buf[1] = typ
	binary.BigEndian.PutUint16(buf[2:4], seq)
	binary.BigEndian.PutUint32(buf[4:8], peer)
	copy(buf[8:], body)
	_, err := w.Write(buf)
	return err
}

func readFrame(r io.Reader) (byte, uint16, []byte, error) {
	var hb [8]byte
	if _, err := io.ReadFull(r, hb[:]); err != nil {
		return 0, 0, nil, err
	}
	var lb [4]byte
	if _, err := io.ReadFull(r, lb[:]); err != nil {
		return 0, 0, nil, err
	}
	rawLen := int(binary.BigEndian.Uint32(lb[:]) & 0x00ffffff)
	body := make([]byte, rawLen)
	copy(body, lb[:])
	if rawLen > 4 {
		if _, err := io.ReadFull(r, body[4:]); err != nil {
			return 0, 0, nil, err
		}
	}
	return hb[1], binary.BigEndian.Uint16(hb[2:4]), body, nil
}

func TestSocketLoginAcceptsThePassword(t *testing.T) {
	f := startFakeRPCd(t, 0, "tok123")
	token, err := SocketLogin(f.sock, "root", "s3cret")
	if err != nil {
		t.Fatalf("SocketLogin: %v", err)
	}
	if token != "tok123" {
		t.Fatalf("token = %q, want tok123", token)
	}
	select {
	case body := <-f.invoked:
		for _, want := range []string{"root", "s3cret"} {
			if !bytes.Contains(body, []byte(want)) {
				t.Errorf("INVOKE body does not carry %q", want)
			}
		}
	default:
		t.Fatal("no INVOKE reached the fake rpcd")
	}
}

func TestSocketLoginRefusedPasswordIsNotAnError(t *testing.T) {
	f := startFakeRPCd(t, ubusStatusPermissionDenied, "")
	token, err := SocketLogin(f.sock, "root", "wrong")
	if err != nil {
		t.Fatalf("a refused password must not be an error, got %v", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}

func TestSocketLoginMissingObjectIsAnError(t *testing.T) {
	f := startFakeRPCd(t, ubusStatusNotFound, "")
	if _, err := SocketLogin(f.sock, "root", "pass"); err == nil {
		t.Fatal("want an error when nothing serves session.login (rpcd not running)")
	}
}

func TestSocketLoginUnreachableSocket(t *testing.T) {
	if _, err := SocketLogin(filepath.Join(t.TempDir(), "absent.sock"), "root", "pass"); err == nil {
		t.Fatal("want an error dialing a socket that does not exist")
	}
}

func TestBackendPrefersTheSocket(t *testing.T) {
	f := startFakeRPCd(t, 0, "tok123")
	httpCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[6]}`))
	}))
	defer srv.Close()

	b := Backend{Socket: f.sock, URL: srv.URL}
	ok, err := b.ValidatePassword("root", "pass")
	if err != nil || !ok {
		t.Fatalf("ValidatePassword = %v, %v; want true, nil", ok, err)
	}
	if httpCalled {
		t.Error("the HTTP endpoint was asked about a password the socket already accepted")
	}
}

func TestBackendFallsBackToHTTP(t *testing.T) {
	srv := httptest.NewServer(jsonRPCHandler(t, `{"jsonrpc":"2.0","id":1,"result":[0,{"ubus_rpc_session":"tok123"}]}`))
	defer srv.Close()

	b := Backend{Socket: filepath.Join(t.TempDir(), "absent.sock"), URL: srv.URL}
	ok, err := b.ValidatePassword("root", "pass")
	if err != nil || !ok {
		t.Fatalf("ValidatePassword = %v, %v; want true, nil", ok, err)
	}
}

func TestBackendWithoutAnyEndpointFails(t *testing.T) {
	b := Backend{Socket: filepath.Join(t.TempDir(), "absent.sock")}
	ok, err := b.ValidatePassword("root", "pass")
	if err == nil {
		t.Fatalf("want an error with no socket and no URL, got ok=%v", ok)
	}
}
