package modules

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	netifydSocketPath = "/var/run/netifyd/netifyd.sock"
	defaultMaxApps    = 256
	defaultMaxFlows   = 4096
)

// NetifydApp is one aggregated application entry from netifyd flow data.
type NetifydApp struct {
	Name        string `json:"name"`
	Bytes       int64  `json:"bytes"`
	LocalBytes  int64  `json:"local_bytes"`
	OtherBytes  int64  `json:"other_bytes"`
	Packets     int64  `json:"packets"`
	Flows       int    `json:"flows"`
}

// netifydFlow keeps the latest application name seen for a digest.
type netifydFlow struct {
	appName string
}

// netifydAppTable holds the live aggregation. It is safe for concurrent use.
type netifydAppTable struct {
	mu       sync.Mutex
	apps     map[string]*NetifydApp
	flows    map[string]*netifydFlow
	maxApps  int
	maxFlows int
}

func newNetifydTable(maxApps, maxFlows int) *netifydAppTable {
	if maxApps <= 0 {
		maxApps = defaultMaxApps
	}
	if maxFlows <= 0 {
		maxFlows = defaultMaxFlows
	}
	return &netifydAppTable{
		apps:     make(map[string]*NetifydApp),
		flows:    make(map[string]*netifydFlow),
		maxApps:  maxApps,
		maxFlows: maxFlows,
	}
}

// Reset clears all accumulated data.
func (t *netifydAppTable) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.apps = make(map[string]*NetifydApp)
	t.flows = make(map[string]*netifydFlow)
}

// Apps returns a sorted snapshot of the current app table.
func (t *netifydAppTable) Apps() []NetifydApp {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]NetifydApp, 0, len(t.apps))
	for _, a := range t.apps {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return strings.Compare(out[i].Name, out[j].Name) < 0
	})
	return out
}

func (t *netifydAppTable) appNameForDigest(digest string) string {
	if f, ok := t.flows[digest]; ok {
		return f.appName
	}
	return ""
}

// setFlowApp remembers which application a digest belongs to.
func (t *netifydAppTable) setFlowApp(digest, appName string) {
	if digest == "" {
		return
	}
	if appName == "" {
		appName = "Unknown"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flows[digest] = &netifydFlow{appName: appName}
	if len(t.flows) > t.maxFlows {
		t.evictOldestFlow()
	}
}

// addStats increments counters for the application associated with digest.
func (t *netifydAppTable) addStats(digest string, local, other, total, packets int64) {
	if digest == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	appName := "Unknown"
	if f, ok := t.flows[digest]; ok {
		appName = f.appName
	}

	app, ok := t.apps[appName]
	if !ok {
		app = &NetifydApp{Name: appName}
		t.apps[appName] = app
	}
	app.Bytes += total
	app.LocalBytes += local
	app.OtherBytes += other
	app.Packets += packets
	app.Flows++

	if len(t.apps) > t.maxApps {
		t.evictSmallestApp()
	}
}

func (t *netifydAppTable) evictOldestFlow() {
	for k := range t.flows {
		delete(t.flows, k)
		return
	}
}

func (t *netifydAppTable) evictSmallestApp() {
	var victim string
	var minBytes int64 = -1
	for name, app := range t.apps {
		if minBytes < 0 || app.Bytes < minBytes {
			minBytes = app.Bytes
			victim = name
		}
	}
	if victim != "" {
		delete(t.apps, victim)
	}
}

// netifydSocketClient reads netifyd flow events from a UNIX socket and feeds a
// shared table. It reconnects automatically until stopped.
type netifydSocketClient struct {
	socketPath string
	table      *netifydAppTable
	cancel     context.CancelFunc
	done       chan struct{}
	logf       func(format string, v ...any)
}

func newNetifydSocketClient(socketPath string, table *netifydAppTable) *netifydSocketClient {
	if socketPath == "" {
		socketPath = netifydSocketPath
	}
	return &netifydSocketClient{
		socketPath: socketPath,
		table:      table,
		done:       make(chan struct{}),
		logf:       func(format string, v ...any) { log.Printf(format, v...) },
	}
}

func (c *netifydSocketClient) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.run(ctx)
}

func (c *netifydSocketClient) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	<-c.done
}

func (c *netifydSocketClient) run(ctx context.Context) {
	defer close(c.done)
	for {
		err := c.readLoop(ctx)
		if err != nil {
			c.logf("netifyd client: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *netifydSocketClient) readLoop(ctx context.Context) error {
	if _, err := os.Stat(c.socketPath); err != nil {
		return fmt.Errorf("socket not available: %w", err)
	}

	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.socketPath, err)
	}
	defer conn.Close()

	// Closing the connection will unblock the reader when we are stopped.
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	reader := bufio.NewReader(conn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Each message is preceded by a {"length": N} framing line.
		 framing, err := reader.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read framing: %w", err)
		}
		framing = strings.TrimSpace(framing)
		if framing == "" {
			continue
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read message: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		c.handleMessage(line)
	}
}

// messageBase lets us peek at the event type before full decoding.
type messageBase struct {
	Type string `json:"type"`
}

// flowMsg carries application identification for a flow.
type flowMsg struct {
	Type string `json:"type"`
	Flow *struct {
		Digest                  string `json:"digest"`
		DetectedApplicationName string `json:"detected_application_name"`
		DetectedProtocolName    string `json:"detected_protocol_name"`
	} `json:"flow,omitempty"`
}

func (m *flowMsg) digest() string {
	if m.Flow != nil {
		return m.Flow.Digest
	}
	return ""
}

func (m *flowMsg) appName() string {
	if m.Flow != nil {
		return m.Flow.DetectedApplicationName
	}
	return ""
}

// statsMsg carries byte counters inside a nested flow object.
type statsMsg struct {
	Type string `json:"type"`
	Flow *struct {
		Digest       string `json:"digest"`
		LocalBytes   int64  `json:"local_bytes"`
		OtherBytes   int64  `json:"other_bytes"`
		TotalBytes   int64  `json:"total_bytes"`
		TotalPackets int64  `json:"total_packets"`
	} `json:"flow,omitempty"`
}

func (m *statsMsg) digest() string {
	if m.Flow != nil {
		return m.Flow.Digest
	}
	return ""
}

func (m *statsMsg) counters() (local, other, total, packets int64) {
	if m.Flow == nil {
		return 0, 0, 0, 0
	}
	return m.Flow.LocalBytes, m.Flow.OtherBytes, m.Flow.TotalBytes, m.Flow.TotalPackets
}

func (c *netifydSocketClient) handleMessage(line string) {
	var base messageBase
	if err := json.Unmarshal([]byte(line), &base); err != nil {
		return
	}

	switch base.Type {
	case "flow":
		var msg flowMsg
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return
		}
		c.table.setFlowApp(msg.digest(), msg.appName())
	case "flow_stats", "flow_purge":
		var msg statsMsg
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return
		}
		local, other, total, packets := msg.counters()
		c.table.addStats(msg.digest(), local, other, total, packets)
	}
}
