package modules

import (
	"context"
	"fmt"
	"math"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
)

// Diagnostics (issue #305): guided network tests - self-test, ping,
// traceroute, DNS lookup and TCP port test. Every external command runs via
// exec.Command with separate ARGS (never a shell) and strict input validation;
// each test is bounded by a timeout. Missing tools (traceroute/nslookup/dig on
// OpenWrt) are detected and reported honestly instead of installing packages.

type DiagnosticsTools struct {
	Ping       bool `json:"ping"`
	Traceroute bool `json:"traceroute"`
	Nslookup   bool `json:"nslookup"`
	Dig        bool `json:"dig"`
}

type SelfTestResult struct {
	Gateway bool             `json:"gateway"`
	Wan     bool             `json:"wan"`
	DNS     bool             `json:"dns"`
	NTP     bool             `json:"ntp"`
	AllOk   bool             `json:"all_ok"`
	Tools   DiagnosticsTools `json:"tools"`
}

type PingSample struct {
	Seq    int     `json:"seq"`
	TimeMs float64 `json:"time_ms"`
	Error  bool    `json:"error"`
}

type PingResult struct {
	Host        string       `json:"host"`
	Count       int          `json:"count"`
	Sent        int          `json:"sent"`
	Received    int          `json:"received"`
	LossPct     float64      `json:"loss_pct"`
	MinMs       float64      `json:"min_ms"`
	AvgMs       float64      `json:"avg_ms"`
	MaxMs       float64      `json:"max_ms"`
	Samples     []PingSample `json:"samples"`
	MissingTool string       `json:"missing_tool,omitempty"`
}

type TraceHop struct {
	Hop    int       `json:"hop"`
	Host   string    `json:"host"`
	Rtts   []float64 `json:"rtts"`
	Raw    string    `json:"raw,omitempty"`
	Parsed bool      `json:"parsed"`
}

type TracerouteResult struct {
	Host        string     `json:"host"`
	Hops        []TraceHop `json:"hops"`
	MissingTool string     `json:"missing_tool,omitempty"`
}

type DNSAnswer struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value"`
	TTL   int    `json:"ttl,omitempty"`
}

type DNSResult struct {
	Query       string      `json:"query"`
	Resolver    string      `json:"resolver,omitempty"`
	Answers     []DNSAnswer `json:"answers"`
	Raw         string      `json:"raw,omitempty"`
	Parsed      bool        `json:"parsed"`
	MissingTool string      `json:"missing_tool,omitempty"`
}

type TCPResult struct {
	Host  string `json:"host"`
	Port  int    `json:"port"`
	Open  bool   `json:"open"`
	Error string `json:"error,omitempty"`
}

// DiagnosticsParams is the validated request payload for RunDiagnostics.
type DiagnosticsParams struct {
	Host     string `json:"host"`
	Count    int    `json:"count"`
	Query    string `json:"query"`
	Resolver string `json:"resolver"`
	Port     int    `json:"port"`
}

const (
	diagnosticsPingTimeout        = 15 * time.Second
	diagnosticsTraceTimeout       = 30 * time.Second
	diagnosticsDNSTimeout         = 15 * time.Second
	diagnosticsTCPTimeout         = 5 * time.Second
	diagnosticsSelfTestDNSTimeout = 5 * time.Second
)

// ── Validation ─────────────────────────────────────────────────────────────

var reDiagHostname = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// validHost accepts a hostname, IPv4 or IPv6 literal, and rejects anything
// that could be interpreted as a flag or carry shell metacharacters.
func validHost(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	if strings.ContainsAny(s, " \t;`\\\"'()&|<>$") {
		return false
	}
	if net.ParseIP(s) != nil {
		return true
	}
	return reDiagHostname.MatchString(s)
}

func validTCPPort(p int) bool { return p >= 1 && p <= 65535 }

func validCount(c int) bool { return c >= 1 && c <= 10 }

func toolAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ── Parsers (pure; unit-testable with fixtures) ────────────────────────────

var (
	rePingTime = regexp.MustCompile(`time=([0-9.]+)\s*ms`)
	rePingSent = regexp.MustCompile(`(\d+)\s+packets transmitted`)
	rePingRecv = regexp.MustCompile(`(\d+)\s+(?:packets\s+)?received`)
	rePingLoss = regexp.MustCompile(`([0-9.]+)%\s+packet loss`)
)

// parsePing extracts per-reply latencies and the summary counters from ping
// output. busybox and iputils share both the `time=X ms` reply lines and the
// "N packets transmitted, M received, L% packet loss" summary, so one parser
// covers both. min/avg/max are computed from the samples to avoid diverging
// summary-line formats.
func parsePing(out string) (sent, received int, lossPct float64, samples []PingSample) {
	for _, m := range rePingTime.FindAllStringSubmatch(out, -1) {
		v, _ := strconv.ParseFloat(m[1], 64)
		samples = append(samples, PingSample{Seq: len(samples), TimeMs: v})
	}
	if m := rePingSent.FindStringSubmatch(out); m != nil {
		sent, _ = strconv.Atoi(m[1])
	}
	if m := rePingRecv.FindStringSubmatch(out); m != nil {
		received, _ = strconv.Atoi(m[1])
	}
	if m := rePingLoss.FindStringSubmatch(out); m != nil {
		lossPct, _ = strconv.ParseFloat(m[1], 64)
	}
	if received == 0 && len(samples) > 0 {
		received = len(samples)
	}
	if sent == 0 && len(samples) > 0 {
		sent = len(samples)
	}
	return
}

func pingStats(samples []PingSample) (min, avg, max float64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}
	min = samples[0].TimeMs
	max = samples[0].TimeMs
	var sum float64
	for _, s := range samples {
		if s.TimeMs < min {
			min = s.TimeMs
		}
		if s.TimeMs > max {
			max = s.TimeMs
		}
		sum += s.TimeMs
	}
	avg = sum / float64(len(samples))
	return
}

var (
	reTraceLine  = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
	reTraceMs    = regexp.MustCompile(`([0-9.]+)\s+ms`)
	reTraceParen = regexp.MustCompile(`\(([^)]+)\)`)
)

// parseTraceroute parses hop lines from both busybox (`1  192.168.1.1  rtt rtt`)
// and iputils (`1  _gateway (192.168.1.1)  rtt rtt`). Unparseable hops keep
// their raw line with Parsed=false so the UI can show them honestly.
func parseTraceroute(out string) []TraceHop {
	var hops []TraceHop
	for _, line := range strings.Split(out, "\n") {
		m := reTraceLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		hop, _ := strconv.Atoi(m[1])
		rest := m[2]
		var rtts []float64
		for _, r := range reTraceMs.FindAllStringSubmatch(rest, -1) {
			if v, err := strconv.ParseFloat(r[1], 64); err == nil {
				rtts = append(rtts, v)
			}
		}
		host := hopHost(rest)
		hops = append(hops, TraceHop{
			Hop:    hop,
			Host:   host,
			Rtts:   rtts,
			Raw:    strings.TrimSpace(line),
			Parsed: len(rtts) > 0,
		})
	}
	return hops
}

// hopHost prefers the parenthesized IP (iputils), then the first IP literal,
// then the first non-star token. Returns "*" for a fully unanswered hop.
func hopHost(rest string) string {
	if m := reTraceParen.FindStringSubmatch(rest); m != nil {
		return m[1]
	}
	for _, f := range strings.Fields(rest) {
		if net.ParseIP(f) != nil {
			return f
		}
	}
	for _, f := range strings.Fields(rest) {
		if f != "*" && !strings.HasSuffix(f, "ms") {
			return f
		}
	}
	return "*"
}

var (
	reNsName = regexp.MustCompile(`^Name:\s*(.+)$`)
	reNsAddr = regexp.MustCompile(`^Address(?:\s+\d+)?:\s*(\S+)`)
)

// parseNslookup extracts answer addresses from busybox or bind nslookup output.
// Only lines after a "Name:" line count as answers (the leading "Server:" /
// "Address:" pair is the resolver's own address, not an answer). busybox prints
// no TTL or record type, so those stay empty.
func parseNslookup(out string) []DNSAnswer {
	var answers []DNSAnswer
	name := ""
	for _, line := range strings.Split(out, "\n") {
		if m := reNsName.FindStringSubmatch(line); m != nil {
			name = strings.TrimSpace(m[1])
			continue
		}
		if m := reNsAddr.FindStringSubmatch(line); m != nil && name != "" {
			v := strings.Fields(m[1])[0]
			answers = append(answers, DNSAnswer{Name: name, Value: v})
		}
	}
	return answers
}

// parseDig parses `dig +noall +answer` lines (name ttl class type data).
func parseDig(out string) []DNSAnswer {
	var answers []DNSAnswer
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 5 || f[2] != "IN" {
			continue
		}
		ttl, _ := strconv.Atoi(f[1])
		answers = append(answers, DNSAnswer{
			Name:  strings.TrimSuffix(f[0], "."),
			Type:  f[3],
			Value: strings.Join(f[4:], " "),
			TTL:   ttl,
		})
	}
	return answers
}

// ── Runners ────────────────────────────────────────────────────────────────

func runPing(host string, count int) (*PingResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticsPingTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "ping", "-c", strconv.Itoa(count), "-W", "1", host).CombinedOutput()
	sent, received, loss, samples := parsePing(string(out))
	if sent == 0 && received == 0 && len(samples) == 0 {
		return nil, fmt.Errorf("ping: %s", strings.TrimSpace(string(out)))
	}
	min, avg, max := pingStats(samples)
	round2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	return &PingResult{
		Host:     host,
		Count:    count,
		Sent:     sent,
		Received: received,
		LossPct:  loss,
		MinMs:    round2(min),
		AvgMs:    round2(avg),
		MaxMs:    round2(max),
		Samples:  samples,
	}, nil
}

func runTraceroute(host string) (*TracerouteResult, error) {
	if !toolAvailable("traceroute") {
		return &TracerouteResult{Host: host, MissingTool: "traceroute"}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticsTraceTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "traceroute", "-n", "-q", "1", "-w", "1", host).CombinedOutput()
	hops := parseTraceroute(string(out))
	if len(hops) == 0 {
		return nil, fmt.Errorf("traceroute: no output (%s)", strings.TrimSpace(string(out)))
	}
	return &TracerouteResult{Host: host, Hops: hops}, nil
}

func runDNS(query, resolver string) (*DNSResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticsDNSTimeout)
	defer cancel()

	var out []byte
	used := ""
	switch {
	case toolAvailable("nslookup"):
		used = "nslookup"
		args := []string{query}
		if resolver != "" {
			args = append(args, resolver)
		}
		out, _ = exec.CommandContext(ctx, "nslookup", args...).CombinedOutput()
	case toolAvailable("dig"):
		used = "dig"
		args := []string{"+noall", "+answer"}
		if resolver != "" {
			args = append(args, "@"+resolver)
		}
		args = append(args, query)
		out, _ = exec.CommandContext(ctx, "dig", args...).CombinedOutput()
	default:
		return &DNSResult{Query: query, Resolver: resolver, MissingTool: "nslookup"}, nil
	}

	var answers []DNSAnswer
	if used == "dig" {
		answers = parseDig(string(out))
	} else {
		answers = parseNslookup(string(out))
	}
	return &DNSResult{
		Query:    query,
		Resolver: resolver,
		Answers:  answers,
		Raw:      strings.TrimSpace(string(out)),
		Parsed:   len(answers) > 0,
	}, nil
}

func runTCP(host string, port int) (*TCPResult, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, diagnosticsTCPTimeout)
	if err != nil {
		return &TCPResult{Host: host, Port: port, Open: false, Error: err.Error()}, nil
	}
	_ = conn.Close()
	return &TCPResult{Host: host, Port: port, Open: true}, nil
}

// ── Self-test ──────────────────────────────────────────────────────────────

func pingOnce(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", host).Run() == nil
}

func dnsResolves(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticsSelfTestDNSTimeout)
	defer cancel()
	_, err := net.DefaultResolver.LookupHost(ctx, host)
	return err == nil
}

// ntpRunning approximates "NTP synced" by checking whether a time-sync daemon
// is running. This is a running check, not a proof the clock is synced.
func ntpRunning() bool {
	for _, svc := range []string{"sysntpd", "ntpd", "chrony", "chronyd"} {
		if exec.Command("/etc/init.d/"+svc, "running").Run() == nil {
			return true
		}
	}
	return false
}

// RunSelfTest reports gateway reachability, WAN up, DNS resolution and NTP in
// one green/red summary, plus tool availability. On AP mode the WAN/gateway
// checks report their honest state (no WAN interface).
func RunSelfTest() *SelfTestResult {
	res := &SelfTestResult{
		Tools: DiagnosticsTools{
			Ping:       toolAvailable("ping"),
			Traceroute: toolAvailable("traceroute"),
			Nslookup:   toolAvailable("nslookup"),
			Dig:        toolAvailable("dig"),
		},
	}
	if wan, err := ubus.GetWanStatus(); err == nil {
		res.Wan = wan.Present && wan.Up
		if wan.Gateway != "" {
			res.Gateway = pingOnce(wan.Gateway)
		}
	}
	res.DNS = dnsResolves("openwrt.org")
	res.NTP = ntpRunning()
	res.AllOk = res.Gateway && res.Wan && res.DNS && res.NTP
	return res
}

// ── Dispatch ───────────────────────────────────────────────────────────────

// RunDiagnostics validates params and runs the requested test.
func RunDiagnostics(test string, p DiagnosticsParams) (any, error) {
	switch test {
	case "ping":
		if !validHost(p.Host) {
			return nil, fmt.Errorf("invalid host")
		}
		if p.Count == 0 {
			p.Count = 4
		}
		if !validCount(p.Count) {
			return nil, fmt.Errorf("count must be between 1 and 10")
		}
		return runPing(p.Host, p.Count)
	case "traceroute":
		if !validHost(p.Host) {
			return nil, fmt.Errorf("invalid host")
		}
		return runTraceroute(p.Host)
	case "dns":
		if !validHost(p.Query) {
			return nil, fmt.Errorf("invalid query")
		}
		if p.Resolver != "" && !validHost(p.Resolver) {
			return nil, fmt.Errorf("invalid resolver")
		}
		return runDNS(p.Query, p.Resolver)
	case "tcp":
		if !validHost(p.Host) {
			return nil, fmt.Errorf("invalid host")
		}
		if !validTCPPort(p.Port) {
			return nil, fmt.Errorf("port must be between 1 and 65535")
		}
		return runTCP(p.Host, p.Port)
	default:
		return nil, fmt.Errorf("unknown test: %s", test)
	}
}
