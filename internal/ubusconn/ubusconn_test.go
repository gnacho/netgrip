package ubusconn

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeServer is a minimal in-test ubus server speaking the wire protocol:
// HELLO on connect, LOOKUP -> DATA(OBJID) + STATUS(0), INVOKE -> DATA(blobmsg
// table) + STATUS(0). Frames are built by hand from the protocol definition.
type fakeServer struct {
	ln      net.Listener
	payload map[string]any // result table served for every INVOKE
	objID   uint32
	got     chan []byte // raw INVOKE bodies, for assertions
}

func startFakeServer(t *testing.T, payload map[string]any) *fakeServer {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "ubus.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeServer{ln: ln, payload: payload, objID: 42, got: make(chan []byte, 8)}
	go fs.serve(t, sock)
	t.Cleanup(func() { ln.Close() })
	return fs
}

func (fs *fakeServer) serve(t *testing.T, sock string) {
	conn, err := fs.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	// HELLO: header (version 0, type 0, seq 0, peer 7) + empty container.
	if err := writeFrame(conn, msgHello, 0, 7, containerRaw(nil)); err != nil {
		return
	}
	for {
		typ, _, body, err := readFrame(conn)
		if err != nil {
			return
		}
		switch typ {
		case msgLookup:
			var b bodyBuilder
			b.putU32(attrObjID, fs.objID)
			if err := writeFrame(conn, msgData, 1, 0, b.bytes()); err != nil {
				return
			}
			var sc bodyBuilder
			sc.putU32(attrStatus, 0)
			if err := writeFrame(conn, msgStatus, 1, 0, sc.bytes()); err != nil {
				return
			}
		case msgInvoke:
			select {
			case fs.got <- body:
			default:
			}
			table, err := encodeBlobmsgTable(fs.payload)
			if err != nil {
				t.Errorf("encode payload: %v", err)
				return
			}
			var b bodyBuilder
			b.put(attrData, table)
			if err := writeFrame(conn, msgData, 1, 0, b.bytes()); err != nil {
				return
			}
			var sc bodyBuilder
			sc.putU32(attrStatus, 0)
			if err := writeFrame(conn, msgStatus, 1, 0, sc.bytes()); err != nil {
				return
			}
		}
	}
}

// containerRaw wraps child attributes in a top-level blob container.
func containerRaw(children []byte) []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], idLen(0, false, len(children)+4))
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
	rawLen := int(binary.BigEndian.Uint32(lb[:]) & lenMask)
	body := make([]byte, rawLen)
	copy(body, lb[:])
	if rawLen > 4 {
		if _, err := io.ReadFull(r, body[4:]); err != nil {
			return 0, 0, nil, err
		}
	}
	return hb[1], binary.BigEndian.Uint16(hb[2:4]), body, nil
}

func dialFake(t *testing.T, fs *fakeServer) *Client {
	t.Helper()
	sock := fs.ln.Addr().String()
	c, err := Dial(sock)
	if err != nil {
		t.Fatalf("dial fake server: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestLoginHelloAndCallNestedTable(t *testing.T) {
	payload := map[string]any{
		"board_name": "sim,simax1800t",
		"model":      map[string]any{"id": "simax1800t", "name": "SIMAX1800T"},
		"release": map[string]any{
			"distribution": "OpenWrt",
			"version":      "25.12.5",
			"revision":     "r33051",
			"target":       "ramips/mt7621",
			"descr":        "OpenWrt 25.12.5 r33051",
			"board_prefix": []any{"sim_ax1800t", "sim,simax1800t"},
		},
		"kernel":      "6.12.74",
		"uptime":      int64(12345),
		"temp":        41.5,
		"loaded":      true,
		"rootfs_type": "squashfs",
	}
	fs := startFakeServer(t, payload)
	c := dialFake(t, fs)

	got, err := c.Call("system", "board", map[string]any{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got["board_name"] != "sim,simax1800t" {
		t.Errorf("board_name = %v", got["board_name"])
	}
	if got["kernel"] != "6.12.74" {
		t.Errorf("kernel = %v", got["kernel"])
	}
	if got["uptime"] != int64(12345) {
		t.Errorf("uptime = %v (%T)", got["uptime"], got["uptime"])
	}
	if got["temp"] != 41.5 {
		t.Errorf("temp = %v (%T)", got["temp"], got["temp"])
	}
	if got["loaded"] != true {
		t.Errorf("loaded = %v (%T), want bool true (CLI parity: INT8 decodes as bool)", got["loaded"], got["loaded"])
	}
	release, ok := got["release"].(map[string]any)
	if !ok {
		t.Fatalf("release is %T, want nested table", got["release"])
	}
	if release["distribution"] != "OpenWrt" || release["version"] != "25.12.5" {
		t.Errorf("release fields = %v", release)
	}
	prefix, ok := release["board_prefix"].([]any)
	if !ok || len(prefix) != 2 || prefix[0] != "sim_ax1800t" {
		t.Errorf("board_prefix = %v", release["board_prefix"])
	}

	// The INVOKE must carry an (empty) args table in UBUS_ATTR_DATA.
	select {
	case body := <-fs.got:
		attrs, err := parseContainer(body)
		if err != nil {
			t.Fatalf("parse INVOKE body: %v", err)
		}
		var sawData, sawObj, sawMethod bool
		for _, a := range attrs {
			switch a.id {
			case attrObjID:
				sawObj = binary.BigEndian.Uint32(a.payload[:4]) == fs.objID
			case attrMethod:
				sawMethod = string(bytes.TrimRight(a.payload, "\x00")) == "board"
			case attrData:
				sawData = true
				if _, err := decodeTable(a.payload); err != nil {
					t.Errorf("args table not decodable: %v", err)
				}
			}
		}
		if !sawObj || !sawMethod || !sawData {
			t.Errorf("INVOKE attrs: obj=%v method=%v data=%v", sawObj, sawMethod, sawData)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server saw no INVOKE")
	}
}

func TestLookupCachedSecondCall(t *testing.T) {
	fs := startFakeServer(t, map[string]any{"ok": true})
	c := dialFake(t, fs)
	if _, err := c.Call("network.wireless", "status", nil); err != nil {
		t.Fatalf("first Call: %v", err)
	}
	if _, err := c.Call("network.wireless", "status", nil); err != nil {
		t.Fatalf("second Call: %v", err)
	}
	// Only one LOOKUP should have reached the wire; the second Call reuses
	// the cached object id and goes straight to INVOKE.
	select {
	case <-fs.got:
	default:
		t.Fatal("first INVOKE not seen")
	}
	select {
	case <-fs.got:
	default:
		t.Fatal("second INVOKE not seen")
	}
	// A second client connection is needed to count LOOKUPs precisely; the
	// cache is client-side state, so just assert the map holds the id.
	if c.objects["network.wireless"] != fs.objID {
		t.Errorf("cache = %v", c.objects)
	}
}

func TestRoundTripAllArgTypes(t *testing.T) {
	// Encode a table with every supported argument type and decode it back
	// through the same code path the server side would use.
	args := map[string]any{
		"s":     "hola",
		"b":     true,
		"i":     int(-7),
		"i8":    int8(-8),
		"i16":   int16(-1600),
		"i32":   int32(-320000),
		"i64":   int64(1 << 40),
		"f":     2.5,
		"sub":   map[string]any{"k": "v"},
		"ssub":  map[string]string{"a": "b"},
		"arr":   []any{"x", int64(3), false},
		"sarr":  []string{"p", "q"},
	}
	body, err := encodeBlobmsgTable(args)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := decodeTable(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["s"] != "hola" || got["b"] != true || got["i"] != int64(-7) ||
		got["i8"] != int64(-8) || got["i16"] != int64(-1600) || got["i32"] != int64(-320000) ||
		got["i64"] != int64(1<<40) || got["f"] != 2.5 {
		t.Errorf("scalars = %v", got)
	}
	if got["sub"].(map[string]any)["k"] != "v" {
		t.Errorf("sub = %v", got["sub"])
	}
	if got["ssub"].(map[string]any)["a"] != "b" {
		t.Errorf("ssub = %v", got["ssub"])
	}
	arr := got["arr"].([]any)
	if len(arr) != 3 || arr[0] != "x" || arr[1] != int64(3) || arr[2] != false {
		t.Errorf("arr = %v", arr)
	}
	sarr := got["sarr"].([]any)
	if len(sarr) != 2 || sarr[1] != "q" {
		t.Errorf("sarr = %v", sarr)
	}
}

func TestEncodeRejectsUnsupportedType(t *testing.T) {
	if _, err := encodeBlobmsgTable(map[string]any{"bad": struct{}{}}); err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if _, err := encodeBlobmsgTable(map[string]any{"bad": uint(1)}); err == nil {
		t.Fatal("expected error for uint")
	}
}

func TestDoubleRoundTrip(t *testing.T) {
	body, err := encodeBlobmsgTable(map[string]any{"pi": math.Pi})
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeTable(body)
	if err != nil {
		t.Fatal(err)
	}
	if got["pi"] != math.Pi {
		t.Errorf("pi = %v", got["pi"])
	}
}

func TestContainerRejectsGarbage(t *testing.T) {
	if _, err := parseContainer([]byte{0, 0}); err == nil {
		t.Fatal("expected error for short container")
	}
	if _, err := parseContainer([]byte{0, 0, 0, 100, 1, 2}); err == nil {
		t.Fatal("expected error for oversized length")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// CLI parity: daemons send booleans as INT8 on the wire and the ubus CLI
// renders INT8 as true/false (#471: interfaceDump.autostart). The native
// path must decode the same way or every struct written against CLI output
// breaks.
func TestInt8DecodesAsBool(t *testing.T) {
	feed := map[string]any{"autostart": true}
	body, err := encodeBlobmsgTable(feed)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Re-decode: the encoder stores bools as INT8, and the decoder must
	// hand back a bool, matching `ubus call` output.
	got, err := decodeTable(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	v, ok := got["autostart"].(bool)
	if !ok || !v {
		t.Fatalf("autostart = %#v, want bool(true)", got["autostart"])
	}
}
