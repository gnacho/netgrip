# Spike: native Go ubus client (issue #461)

Status: spike complete, recommendation below. Code lives in `internal/ubusconn`
(provisional name) plus a throwaway benchmark in `cmd/ubusspike`. Nothing in
production paths was touched; `internal/ubus` (the CLI-fork wrapper) is
untouched.

## Why

Every probe in NetGrip currently forks `ubus call` / `uci` CLI processes. On
mipsle routers the fork+exec+JSON re-parse cost dominates small calls, and the
probes run in tight loops. A pure-Go client speaking the ubus wire protocol
over the unix socket removes the process fork entirely.

Reference read for the protocol: we-are-mono/verso (`internal/ubus`,
`docs/ubus-protocol.md`). Note the license conflict: verso is GPL-2.0-only,
NetGrip is AGPL-3.0, so no code was copied. The facts of the wire protocol
(blob/blobmsg framing, message types) are not copyrightable; the
implementation here is written from scratch against the protocol description.

## Wire protocol (summary, as implemented)

Transport: unix stream socket, default `/var/run/ubus/ubus.sock`. On connect
the server (ubusd) sends a HELLO first; its header `peer` field is our
assigned client id.

Message framing: every message is an 8-byte header (version u8 = 0, type u8,
seq u16 big-endian, peer u32 big-endian) followed by exactly one blob
container. The body length is carried inside the container's own `id_len`;
there is no separate length prefix.

blob_attr: one big-endian u32 `id_len`:

- bit 31 (0x80000000): extended (blobmsg: payload begins with a name)
- bits 30-24: attribute id / blobmsg type
- bits 23-0: length INCLUDING the 4-byte header

Payload is padded to a 4-byte boundary; a container's children are packed
back-to-back each padded to 4 bytes. Integers are big-endian.

blobmsg (extended) attribute payload: u16 big-endian namelen (excludes the
trailing NUL), name bytes + one NUL, padded to 4 (hdrlen = pad4(2+namelen+1)),
then the value. blobmsg types: 1 ARRAY, 2 TABLE, 3 STRING, 4 INT64, 5 INT32,
6 INT16, 7 INT8/BOOL, 8 DOUBLE. Array elements carry an empty name.

Message types (enum ubus_msg_type): 0 HELLO, 1 STATUS, 2 DATA, 3 PING,
4 LOOKUP, 5 INVOKE, 6 ADD_OBJECT, 7 REMOVE_OBJECT, 8 SUBSCRIBE,
9 UNSUBSCRIBE, 10 NOTIFY, 11 MONITOR.

Attribute ids (enum ubus_msg_attr): 0 UNSPEC, 1 STATUS, 2 OBJPATH, 3 OBJID,
4 METHOD, 5 OBJTYPE, 6 SIGNATURE, 7 DATA, 8 TARGET, 9 ACTIVE, 10 NO_REPLY,
11 SUBSCRIBERS, 12 USER, 13 GROUP.

Status codes (enum ubus_msg_status): 0 OK, 1 INVALID_COMMAND,
2 INVALID_ARGUMENT, 3 METHOD_NOT_FOUND, 4 NOT_FOUND, 5 NO_DATA,
6 PERMISSION_DENIED, 7 TIMEOUT, 8 NOT_SUPPORTED, 9 UNKNOWN_ERROR.

Call flow:

1. Connect, read the server HELLO.
2. LOOKUP with body `{OBJPATH: name}`; the server answers one or more DATA
   messages carrying OBJID, then a STATUS. Cache the OBJID per object path.
3. INVOKE with body `{OBJID: id, METHOD: name, DATA: <blobmsg table>}`.
   The DATA attribute must be present even for no-argument methods, as an
   empty table, or ubusd answers status 2 (INVALID_ARGUMENT).
4. The server replies DATA carrying the result in UBUS_ATTR_DATA (a blobmsg
   table), then a STATUS with the return code.

Gotchas confirmed during the spike:

- seq and peer in the header are big-endian (easy to miss on little-endian
  targets; a wrong-endian seq desynchronizes the whole connection).
- The 24-bit length includes the 4-byte attribute header; when iterating a
  container's children, advance by pad4(length), not by length.
- The sessionless AUTH exchange (INVOKE on the internal "ubus" object with
  method `sessionless` and `{"guid": "ssssssss"}`) is only needed when ubusd
  runs with ACLs (`ubusd -a`). Default OpenWrt runs without ACLs; the spike
  client does not implement it and everything on rt-lab answered without it.

## Package API (internal/ubusconn)

```go
c, err := ubusconn.Dial("")            // "" = /var/run/ubus/ubus.sock
defer c.Close()
res, err := c.Call("system", "board", nil) // or map[string]any{...}
// res is map[string]any: string -> string/int64/float64/map[string]any/[]any
```

- `Dial(socket string) (*Client, error)` - connects and completes the HELLO
  handshake.
- `(*Client).Call(object, method string, args map[string]any) (map[string]any, error)`
  - LOOKUP (cached per object path) + INVOKE, fully synchronous.
- Argument encoding supports strings, bools, all signed int widths, float64,
  nested `map[string]any` / `map[string]string`, and `[]any` / `[]string`.
  Unsupported types error before anything hits the wire.
- Decoding covers all blobmsg types, including DOUBLE and INT8/16/64.
- The client is not safe for concurrent use; guard with a mutex if probes are
  parallelized.

## Measurement

Device: rt-lab (MediaTek MT7621, mipsle, OpenWrt 25.12.5, ubusd without
ACLs). Method: `ubusspike` runs 10 iterations per combination and reports
min/median/max of a full call (native: Dial + Lookup + Invoke per iteration,
no connection reuse; cli: fork+exec `ubus call` + JSON parse). Two tandas
run back to back, results consistent.

### system board (small result table)

| mode   | tanda | min    | median  | max     |
|--------|-------|--------|---------|---------|
| native | 1     | 3.67ms | 3.86ms  | 5.20ms  |
| cli    | 1     | 11.10ms| 12.09ms | 14.62ms |
| native | 2     | 3.27ms | 3.47ms  | 5.22ms  |
| cli    | 2     | 11.81ms| 12.59ms | 15.10ms |

### network.wireless status (bigger result, real radios)

| mode   | tanda | min    | median  | max     |
|--------|-------|--------|---------|---------|
| native | 1     | 5.14ms | 5.91ms  | 6.81ms  |
| cli    | 1     | 16.40ms| 17.27ms | 19.48ms |
| native | 2     | 5.10ms | 5.50ms  | 6.38ms  |
| cli    | 2     | 15.04ms| 17.71ms | 21.26ms |

Median summary (tanda 1 / tanda 2):

- system.board: native 3.86 / 3.47 ms vs cli 12.09 / 12.59 ms -> ~71-72%
  lower latency.
- network.wireless status: native 5.91 / 5.50 ms vs cli 17.27 / 17.71 ms ->
  ~66-69% lower latency.

Note these numbers include one Dial (connect + HELLO) and one LOOKUP per call;
a production client keeping one connection and the object-id cache warm would
be faster still. The forked CLI pays process spawn, ubusd session setup,
JSON text encode/decode on both sides, and pipe I/O.

## Test coverage

`internal/ubusconn/ubusconn_test.go` runs against a fake ubus server on a
temp unix socket that builds frames by hand:

- TestLoginHelloAndCallNestedTable: HELLO + LOOKUP + INVOKE end-to-end with
  a nested-table payload (mirrors `system board` shape: nested release table,
  string array, INT64, DOUBLE, BOOL), plus a check that the INVOKE carries
  OBJID, METHOD and a decodable empty args table.
- TestLookupCachedSecondCall: object id cache is reused.
- TestRoundTripAllArgTypes: encode/decode round trip for every supported
  argument type.
- TestEncodeRejectsUnsupportedType, TestDoubleRoundTrip,
  TestContainerRejectsGarbage.

`go build ./...`, `go vet ./...` and `go test -count=1 ./internal/ubusconn/`
are green.

## Recommendation: GO

The protocol subset needed for probes (connect + lookup + invoke + blobmsg
decode) is small, well-documented, and works on the first implementation
against a real ubusd on the target architecture. Measured median latency is
~3x lower than the forked CLI on both representative probes, well above the
30% success criterion. Complexity is contained: ~600 lines including tests,
zero external dependencies, and the encoding/decoding code is deterministic
and unit-testable without a device.

Suggested rollout, in order:

1. Land `internal/ubusconn` (this spike, cleaned up) with the fake-server
   tests as the contract.
2. Migrate read-only probes one module at a time behind a per-module or
   global switch, starting with the hottest paths (wireless status, system
   info, network data). Keep the CLI fallback for a release or two.
3. Measure again in production probes (per-probe latency is already logged)
   and only then remove the CLI path per module.
4. Event subscription (SUBSCRIBE/NOTIFY) is explicitly out of scope; the
   current synchronous client does not need it and adding it later is a
   separate decision.
5. Write paths (uci set equivalents, rpc session login for rpcd) are not
   covered by this spike; evaluate separately if needed.

Risks: ubusd with ACLs requires the sessionless AUTH exchange (one extra
INVOKE on object "ubus" at connect); detect the status code and implement it
only if a target shows PERMISSION_DENIED. Concurrent use needs a mutex or a
connection pool; probes are currently sequential, so this is not blocking.
