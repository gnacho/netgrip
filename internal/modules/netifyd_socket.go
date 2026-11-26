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
	netifydSocketPath      = "/var/run/netifyd/netifyd.sock"
	defaultMaxApps         = 256
	defaultMaxFlows        = 4096
	timelineBucketDuration = 5 * time.Minute
	maxTimelineBuckets     = 288 // 24h at 5-minute buckets
)

// NetifydBucket holds aggregated bytes for one application in one time bucket.
type NetifydBucket struct {
	Local int64 `json:"local"`
	Other int64 `json:"other"`
	Total int64 `json:"total"`
}

// NetifydTimelineResponse is the pre-aggregated response for the timeline API.
type NetifydTimelineResponse struct {
	Buckets []NetifydTimelineBucket `json:"buckets"`
	Top     []NetifydApp            `json:"top"`
	Totals  NetifydBucket           `json:"totals"`
}

// NetifydTimelineBucket is one time slot with per-app counters.
type NetifydTimelineBucket struct {
	Time string                   `json:"time"`
	Apps map[string]NetifydBucket `json:"apps"`
}

// NetifydApp is one aggregated application entry from netifyd flow data.
type NetifydApp struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	Bytes      int64  `json:"bytes"`
	LocalBytes int64  `json:"local_bytes"`
	OtherBytes int64  `json:"other_bytes"`
	Packets    int64  `json:"packets"`
	Flows      int    `json:"flows"`
}

// netifydFlow is what the table knows about one live flow.
//
// Netifyd reports byte counters CUMULATIVELY per flow (every flow_stats and
// the final flow_purge repeat the running total), so only the delta between
// messages is real traffic. And nDPI usually names the protocol first and
// the application a moment later, so the deltas seen before the application
// is known are held per bucket and moved to the application when it appears:
// crediting them straight away is what filled the table with protocols
// (#620).
type netifydFlow struct {
	appName   string
	protoName string
	hostName  string
	catName   string
	lastLocal int64
	lastOther int64
	lastTotal int64
	lastPkts  int64
	// pending holds deltas keyed by timeline bucket while appName is empty.
	pending   map[int64]*NetifydBucket
	pendingAt time.Time
	sawBytes  bool
	countedAs string
}

// label is what a flow's bytes are credited to right now. When nDPI cannot
// name the application, the domain netifyd saw is far more useful than the
// protocol, and the protocol beats an anonymous Unknown (#711).
func (f *netifydFlow) label() string {
	if f.appName != "" {
		return f.appName
	}
	if f.hostName != "" {
		return f.hostName
	}
	if f.protoName != "" {
		return f.protoName
	}
	return "Unknown"
}

// pendingFlushAfter bounds how long bytes can wait for an application name:
// a flow that never gets one (local traffic, unknown protocols) lands on its
// protocol label instead of staying invisible.
const pendingFlushAfter = 90 * time.Second

// netifydAppTable holds the live aggregation. It is safe for concurrent use.
type netifydAppTable struct {
	mu   sync.Mutex
	apps map[string]*NetifydApp
	// cats remembers the category per label: nDPI names the application
	// before the first byte arrives, so the category has to wait for the app
	// entry to exist (#687).
	cats       map[string]string
	flows      map[string]*netifydFlow
	buckets    map[int64]map[string]*NetifydBucket
	maxApps    int
	maxFlows   int
	maxBuckets int
}

func newNetifydTable(maxApps, maxFlows int) *netifydAppTable {
	if maxApps <= 0 {
		maxApps = defaultMaxApps
	}
	if maxFlows <= 0 {
		maxFlows = defaultMaxFlows
	}
	return &netifydAppTable{
		apps:       make(map[string]*NetifydApp),
		cats:       make(map[string]string),
		flows:      make(map[string]*netifydFlow),
		buckets:    make(map[int64]map[string]*NetifydBucket),
		maxApps:    maxApps,
		maxFlows:   maxFlows,
		maxBuckets: maxTimelineBuckets,
	}
}

// Reset clears all accumulated data.
func (t *netifydAppTable) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.apps = make(map[string]*NetifydApp)
	t.flows = make(map[string]*netifydFlow)
	t.buckets = make(map[int64]map[string]*NetifydBucket)
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

// flowLocked returns the flow entry for digest, creating it if needed.
func (t *netifydAppTable) flowLocked(digest string) *netifydFlow {
	f, ok := t.flows[digest]
	if !ok {
		f = &netifydFlow{}
		t.flows[digest] = f
		if len(t.flows) > t.maxFlows {
			t.evictOldestFlow()
		}
	}
	return f
}

// bucketNow is the current 5-minute bucket key.
func bucketNow() int64 {
	step := int64(timelineBucketDuration.Seconds())
	return time.Now().Unix() / step * step
}

// creditLocked writes counters to one label and one bucket.
func (t *netifydAppTable) creditLocked(label string, bucket, local, other, total, packets int64) {
	app, ok := t.apps[label]
	if !ok {
		app = &NetifydApp{Name: label, Category: t.cats[label]}
		t.apps[label] = app
		if len(t.apps) > t.maxApps {
			t.evictSmallestApp()
		}
	}
	app.Bytes += total
	app.LocalBytes += local
	app.OtherBytes += other
	app.Packets += packets

	b, ok := t.buckets[bucket]
	if !ok {
		b = make(map[string]*NetifydBucket)
		t.buckets[bucket] = b
	}
	entry, ok := b[label]
	if !ok {
		entry = &NetifydBucket{}
		b[label] = entry
	}
	entry.Local += local
	entry.Other += other
	entry.Total += total

	if len(t.buckets) > t.maxBuckets {
		t.evictOldestBucket()
	}
}

// countFlowLocked counts the flow once under label, moving the count if the
// label improves later (protocol first, application a moment after).
func (t *netifydAppTable) countFlowLocked(f *netifydFlow, label string) {
	if f.countedAs == label {
		return
	}
	if f.countedAs != "" {
		if old, ok := t.apps[f.countedAs]; ok && old.Flows > 0 {
			old.Flows--
		}
	}
	app, ok := t.apps[label]
	if !ok {
		app = &NetifydApp{Name: label, Category: t.cats[label]}
		t.apps[label] = app
		if len(t.apps) > t.maxApps {
			t.evictSmallestApp()
		}
	}
	app.Flows++
	f.countedAs = label
}

// flushLocked credits whatever is pending for the flow and counts it once.
func (t *netifydAppTable) flushLocked(f *netifydFlow) {
	label := f.label()
	for bucket, b := range f.pending {
		t.creditLocked(label, bucket, b.Local, b.Other, b.Total, 0)
	}
	f.pending = nil
	if f.sawBytes {
		t.countFlowLocked(f, label)
	}
	if f.catName != "" {
		if app, ok := t.apps[label]; ok && app.Category == "" {
			app.Category = f.catName
		}
	}
}

// setFlowApp records what nDPI knows about a digest. The application is kept
// apart from the protocol so the protocol is only used as a fallback, and the
// bytes held while waiting are moved to the application as soon as it is
// named.
func (t *netifydAppTable) setFlowApp(digest, appName, protoName, category string) {
	if digest == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	f := t.flowLocked(digest)
	if protoName != "" {
		f.protoName = protoName
	}
	if category != "" {
		f.catName = category
		t.cats[appName] = category
	}
	if appName == "" || appName == "Unknown" || appName == f.appName {
		return
	}
	f.appName = appName
	t.flushLocked(f)
}

// setFlowHost remembers the domain netifyd saw for a digest. It is the label
// used while nDPI has not named the application (#711).
func (t *netifydAppTable) setFlowHost(digest, host string) {
	if digest == "" || host == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	f := t.flowLocked(digest)
	if f.appName != "" || f.hostName == host {
		return
	}
	f.hostName = host
}

// addStats credits the delta since the last message for digest.
func (t *netifydAppTable) addStats(digest string, local, other, total, packets int64) {
	if digest == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.creditDeltaLocked(t.flowLocked(digest), local, other, total, packets)
}

// creditDeltaLocked turns cumulative counters into a delta and credits it.
func (t *netifydAppTable) creditDeltaLocked(f *netifydFlow, local, other, total, packets int64) {
	if local < f.lastLocal || other < f.lastOther || total < f.lastTotal || packets < f.lastPkts {
		// Counters went backwards: the digest is being reused by a new flow,
		// so what arrived is the whole traffic of the new one.
		f.lastLocal, f.lastOther, f.lastTotal, f.lastPkts = 0, 0, 0, 0
	}
	dLocal := local - f.lastLocal
	dOther := other - f.lastOther
	dTotal := total - f.lastTotal
	dPkts := packets - f.lastPkts
	f.lastLocal, f.lastOther, f.lastTotal, f.lastPkts = local, other, total, packets
	if dLocal <= 0 && dOther <= 0 && dTotal <= 0 {
		return
	}
	f.sawBytes = true
	bucket := bucketNow()

	if f.appName != "" {
		t.creditLocked(f.appName, bucket, dLocal, dOther, dTotal, dPkts)
		t.countFlowLocked(f, f.appName)
		return
	}
	if f.pending == nil {
		f.pending = map[int64]*NetifydBucket{}
		f.pendingAt = time.Now()
	}
	b, ok := f.pending[bucket]
	if !ok {
		b = &NetifydBucket{}
		f.pending[bucket] = b
	}
	b.Local += dLocal
	b.Other += dOther
	b.Total += dTotal
	if time.Since(f.pendingAt) > pendingFlushAfter {
		t.flushLocked(f)
	}
}

// purgeFlow applies the final counters of a finished flow, credits whatever
// is still pending and forgets the digest.
func (t *netifydAppTable) purgeFlow(digest string, local, other, total, packets int64) {
	if digest == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	f := t.flowLocked(digest)
	t.creditDeltaLocked(f, local, other, total, packets)
	t.flushLocked(f)
	delete(t.flows, digest)
}

func (t *netifydAppTable) evictOldestBucket() {
	var oldest int64
	for k := range t.buckets {
		if oldest == 0 || k < oldest {
			oldest = k
		}
	}
	if oldest != 0 {
		delete(t.buckets, oldest)
	}
}

// Timeline returns a snapshot of the last 24 hours aggregated in 5-minute buckets.
func (t *netifydAppTable) Timeline() NetifydTimelineResponse {
	t.mu.Lock()
	defer t.mu.Unlock()

	for len(t.buckets) > t.maxBuckets {
		t.evictOldestBucket()
	}

	var totals NetifydBucket
	appTotals := make(map[string]*NetifydBucket)
	for _, b := range t.buckets {
		for name, entry := range b {
			totals.Local += entry.Local
			totals.Other += entry.Other
			totals.Total += entry.Total
			agg, ok := appTotals[name]
			if !ok {
				agg = &NetifydBucket{}
				appTotals[name] = agg
			}
			agg.Local += entry.Local
			agg.Other += entry.Other
			agg.Total += entry.Total
		}
	}

	top := make([]NetifydApp, 0, len(appTotals))
	for name, entry := range appTotals {
		category := ""
		if app, ok := t.apps[name]; ok {
			category = app.Category
		}
		top = append(top, NetifydApp{
			Name:       name,
			Category:   category,
			Bytes:      entry.Total,
			LocalBytes: entry.Local,
			OtherBytes: entry.Other,
		})
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].Bytes != top[j].Bytes {
			return top[i].Bytes > top[j].Bytes
		}
		return strings.Compare(top[i].Name, top[j].Name) < 0
	})
	if len(top) > 10 {
		top = top[:10]
	}

	topSet := make(map[string]bool, len(top))
	for _, a := range top {
		topSet[a.Name] = true
	}

	bucketKeys := make([]int64, 0, len(t.buckets))
	for k := range t.buckets {
		bucketKeys = append(bucketKeys, k)
	}
	sort.Slice(bucketKeys, func(i, j int) bool { return bucketKeys[i] < bucketKeys[j] })

	buckets := make([]NetifydTimelineBucket, 0, len(bucketKeys))
	for _, k := range bucketKeys {
		apps := make(map[string]NetifydBucket)
		for name, entry := range t.buckets[k] {
			if topSet[name] {
				apps[name] = *entry
			}
		}
		if len(apps) == 0 {
			continue
		}
		buckets = append(buckets, NetifydTimelineBucket{
			Time: time.Unix(k, 0).UTC().Format(time.RFC3339),
			Apps: apps,
		})
	}

	return NetifydTimelineResponse{
		Buckets: buckets,
		Top:     top,
		Totals:  totals,
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
	// Una conexión viva y sin datos se parece demasiado a una que nunca se
	// abrió: sin esta línea no hay forma de distinguirlas desde el registro.
	c.logf("netifyd client: connected to %s", c.socketPath)

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

// netifydCategory normalises the category field. Older exports sent a plain
// string; netifyd 4.x sends an object with numeric ids per dimension
// ({"application":0,"domain":0,"protocol":14}). Decoding the object into a
// string failed the whole flow message, which left every flow as Unknown with
// no protocol and no domain (#711).
type netifydCategory struct {
	Name string
}

func (c *netifydCategory) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		c.Name = s
		return nil
	}
	var obj struct {
		Application int `json:"application"`
		Domain      int `json:"domain"`
		Protocol    int `json:"protocol"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	return nil
}

// flowMsg carries application identification for a flow.
type flowMsg struct {
	Type string `json:"type"`
	Flow *struct {
		Digest                  string          `json:"digest"`
		DetectedApplicationName string          `json:"detected_application_name"`
		DetectedProtocolName    string          `json:"detected_protocol_name"`
		HostServerName          string          `json:"host_server_name"`
		DNSHostName             string          `json:"dns_host_name"`
		Category                netifydCategory `json:"category"`
	} `json:"flow,omitempty"`
}

func (m *flowMsg) category() string {
	if m.Flow != nil {
		return m.Flow.Category.Name
	}
	return ""
}

func (m *flowMsg) digest() string {
	if m.Flow != nil {
		return m.Flow.Digest
	}
	return ""
}

func (m *flowMsg) protoName() string {
	if m.Flow == nil {
		return ""
	}
	return m.Flow.DetectedProtocolName
}

// hostName is the domain netifyd resolved for the flow, either from TLS SNI
// or from DNS. It is the fallback label when nDPI cannot name the app.
func (m *flowMsg) hostName() string {
	if m.Flow == nil {
		return ""
	}
	if m.Flow.HostServerName != "" {
		return m.Flow.HostServerName
	}
	return m.Flow.DNSHostName
}

func (m *flowMsg) appName() string {
	if m.Flow == nil {
		return ""
	}
	if m.Flow.DetectedApplicationName != "" && m.Flow.DetectedApplicationName != "Unknown" {
		return appLabel(m.Flow.DetectedApplicationName)
	}
	return ""
}

// appLabel hides netify's application namespace: "netify.youtube" reads
// "youtube" in the table.
func appLabel(name string) string {
	if rest, ok := strings.CutPrefix(name, "netify."); ok && rest != "" {
		return rest
	}
	return name
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
		c.table.setFlowHost(msg.digest(), msg.hostName())
		c.table.setFlowApp(msg.digest(), msg.appName(), msg.protoName(), msg.category())
	case "flow_stats":
		var msg statsMsg
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return
		}
		local, other, total, packets := msg.counters()
		c.table.addStats(msg.digest(), local, other, total, packets)
	case "flow_purge":
		var msg statsMsg
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return
		}
		local, other, total, packets := msg.counters()
		c.table.purgeFlow(msg.digest(), local, other, total, packets)
	}
}
