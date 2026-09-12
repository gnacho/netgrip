package modules

import (
	"math"
	"strings"
	"testing"
)

const busyboxPing = `PING 8.8.8.8 (8.8.8.8): 56 data bytes
64 bytes from 8.8.8.8: seq=0 ttl=117 time=20.123 ms
64 bytes from 8.8.8.8: seq=1 ttl=117 time=21.456 ms
64 bytes from 8.8.8.8: seq=2 ttl=117 time=19.876 ms
64 bytes from 8.8.8.8: seq=3 ttl=117 time=20.900 ms

--- 8.8.8.8 ping statistics ---
4 packets transmitted, 4 packets received, 0% packet loss
round-trip min/avg/max = 19.876/20.588/21.456 ms
`

const iputilsPing = `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=117 time=20.1 ms
64 bytes from 8.8.8.8: icmp_seq=2 ttl=117 time=21.5 ms
64 bytes from 8.8.8.8: icmp_seq=3 ttl=117 time=19.9 ms
64 bytes from 8.8.8.8: icmp_seq=4 ttl=117 time=20.9 ms

--- 8.8.8.8 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3004ms
rtt min/avg/max/mdev = 19.876/20.588/21.456/0.650 ms
`

func TestParsePingBusybox(t *testing.T) {
	sent, received, loss, samples := parsePing(busyboxPing)
	if sent != 4 || received != 4 {
		t.Fatalf("sent/received = %d/%d, want 4/4", sent, received)
	}
	if loss != 0 {
		t.Fatalf("loss = %v, want 0", loss)
	}
	if len(samples) != 4 {
		t.Fatalf("samples = %d, want 4", len(samples))
	}
	min, avg, max := pingStats(samples)
	if math.Abs(min-19.876) > 0.001 || math.Abs(avg-20.58875) > 0.001 || math.Abs(max-21.456) > 0.001 {
		t.Fatalf("min/avg/max = %v/%v/%v", min, avg, max)
	}
}

func TestParsePingIputils(t *testing.T) {
	sent, received, loss, samples := parsePing(iputilsPing)
	if sent != 4 || received != 4 || loss != 0 {
		t.Fatalf("sent/received/loss = %d/%d/%v", sent, received, loss)
	}
	if len(samples) != 4 {
		t.Fatalf("samples = %d, want 4", len(samples))
	}
}

func TestParsePingLoss(t *testing.T) {
	out := `PING 10.0.0.9 (10.0.0.9): 56 data bytes

--- 10.0.0.9 ping statistics ---
4 packets transmitted, 0 packets received, 100% packet loss
`
	sent, received, loss, samples := parsePing(out)
	if sent != 4 || received != 0 || loss != 100 {
		t.Fatalf("sent/received/loss = %d/%d/%v", sent, received, loss)
	}
	if len(samples) != 0 {
		t.Fatalf("samples = %d, want 0", len(samples))
	}
}

const busyboxTrace = `traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 38 byte packets
 1  192.168.1.1  0.482 ms  0.440 ms  0.431 ms
 2  10.0.0.1  1.234 ms  1.100 ms  1.300 ms
 3  8.8.8.8  20.123 ms  21.000 ms  19.900 ms
`

const iputilsTrace = `traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 60 byte packets
 1  _gateway (192.168.1.1)  0.500 ms  0.510 ms  0.480 ms
 2  * * *
 3  8.8.8.8 (8.8.8.8)  20.100 ms  20.200 ms  20.300 ms
`

func TestParseTracerouteBusybox(t *testing.T) {
	hops := parseTraceroute(busyboxTrace)
	if len(hops) != 3 {
		t.Fatalf("hops = %d, want 3", len(hops))
	}
	if hops[0].Host != "192.168.1.1" || !hops[0].Parsed || len(hops[0].Rtts) != 3 {
		t.Fatalf("hop 0 = %+v", hops[0])
	}
}

func TestParseTracerouteIputils(t *testing.T) {
	hops := parseTraceroute(iputilsTrace)
	if len(hops) != 3 {
		t.Fatalf("hops = %d, want 3", len(hops))
	}
	// iputils parenthesized IP wins over the hostname label.
	if hops[0].Host != "192.168.1.1" {
		t.Fatalf("hop 0 host = %q, want 192.168.1.1", hops[0].Host)
	}
	// Fully unanswered hop: no rtts, not parsed, host "*".
	if hops[1].Parsed || hops[1].Host != "*" || len(hops[1].Rtts) != 0 {
		t.Fatalf("hop 1 = %+v", hops[1])
	}
}

const busyboxNslookup = `Server:    127.0.0.1
Address 1: 127.0.0.1 localhost

Name:      example.com
Address 1: 93.184.216.34
`

const bindNslookup = `Server:		127.0.0.53
Address:	127.0.0.53#53

Non-authoritative answer:
Name:	example.com
Address: 93.184.216.34
Name:	example.com
Address: 2606:2800:220:1:248:1893:25c8:1946
`

func TestParseNslookupBusybox(t *testing.T) {
	answers := parseNslookup(busyboxNslookup)
	if len(answers) != 1 {
		t.Fatalf("answers = %+v, want 1", answers)
	}
	if answers[0].Value != "93.184.216.34" {
		t.Fatalf("answer value = %q", answers[0].Value)
	}
}

func TestParseNslookupBind(t *testing.T) {
	answers := parseNslookup(bindNslookup)
	if len(answers) != 2 {
		t.Fatalf("answers = %+v, want 2", answers)
	}
	// The resolver's own "Address: 127.0.0.53#53" line must not leak in.
	for _, a := range answers {
		if strings.HasPrefix(a.Value, "127.0.0.53") {
			t.Fatalf("resolver address leaked into answers: %+v", a)
		}
	}
}

const digOutput = `
; <<>> DiG 9.18 <<>> +noall +answer example.com
;; global options: +cmd
example.com.		3600	IN	A	93.184.216.34
example.com.		3600	IN	AAAA	2606:2800:220:1:248:1893:25c8:1946
`

func TestParseDig(t *testing.T) {
	answers := parseDig(digOutput)
	if len(answers) != 2 {
		t.Fatalf("answers = %+v, want 2", answers)
	}
	if answers[0].Type != "A" || answers[0].Value != "93.184.216.34" || answers[0].TTL != 3600 {
		t.Fatalf("answer 0 = %+v", answers[0])
	}
}

func TestValidHost(t *testing.T) {
	valid := []string{"example.com", "router", "localhost", "8.8.8.8", "2001:db8::1", "sub.domain.example.com"}
	for _, h := range valid {
		if !validHost(h) {
			t.Errorf("host %q should be valid", h)
		}
	}
	invalid := []string{"", "-c", "bad host", "a;b", "a`b", "a$(x)", "a|b", "a<b", "a>b", "a&b", "a'b", "a\"b", "a(b)"}
	for _, h := range invalid {
		if validHost(h) {
			t.Errorf("host %q should be invalid", h)
		}
	}
}

func TestValidPortAndCount(t *testing.T) {
	if validTCPPort(0) || validTCPPort(65536) || !validTCPPort(443) || !validTCPPort(1) || !validTCPPort(65535) {
		t.Fatal("port validation wrong")
	}
	if validCount(0) || validCount(11) || !validCount(1) || !validCount(10) || !validCount(4) {
		t.Fatal("count validation wrong")
	}
}

func TestRunDiagnosticsUnknownTest(t *testing.T) {
	_, err := RunDiagnostics("bogus", DiagnosticsParams{})
	if err == nil {
		t.Fatal("expected error for unknown test")
	}
}

func TestRunDiagnosticsInvalidHost(t *testing.T) {
	_, err := RunDiagnostics("ping", DiagnosticsParams{Host: "bad host!"})
	if err == nil {
		t.Fatal("expected error for invalid host")
	}
	_, err = RunDiagnostics("tcp", DiagnosticsParams{Host: "example.com", Port: 99999})
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}
