package modules

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
)

// WifiChanNeighbor is one foreign BSS seen by a background scan: the channel
// it occupies, how loud it is and how wide it transmits.
type WifiChanNeighbor struct {
	BSSID   string  `json:"bssid"`
	SSID    string  `json:"ssid"`
	Freq    int     `json:"freq"`
	Channel int     `json:"channel"`
	Signal  float64 `json:"signal"`
	Width   int     `json:"width"`
}

// ChanLoad is the scoring input for one channel: airtime utilization of the
// channel itself (0-100, from the survey dump) plus how many foreign BSS
// overlap it.
type ChanLoad struct {
	Channel     int
	Utilization float64
	Neighbors   int
}

// WifiChanSuggestion is the deterministic recommendation for one radio.
// Reason is an i18n key rendered by the panel. Confidence is high/medium/low.
type WifiChanSuggestion struct {
	Channel    int    `json:"channel"`
	Width      int    `json:"width"`
	Htmode     string `json:"htmode"`
	Reason     string `json:"reason"`
	Confidence string `json:"confidence"`
}

// WifiChanRadio is the RF survey probe for one radio.
type WifiChanRadio struct {
	Radio        string               `json:"radio"`
	Band         string               `json:"band"`
	Ifname       string               `json:"ifname"`
	Channel      int                  `json:"channel"`
	Width        int                  `json:"width"`
	Htmode       string               `json:"htmode"`
	Utilization  float64              `json:"utilization"`
	Noise        float64              `json:"noise"`
	Neighbors    []WifiChanNeighbor   `json:"neighbors"`
	ScanComplete bool                 `json:"scan_complete"`
	Suggestion   *WifiChanSuggestion  `json:"suggestion,omitempty"`
}

// dfsChannels is the 5 GHz DFS range (channels 52-64 and 100-140).
var dfsChannels = map[int]bool{}

func init() {
	for _, r := range [][2]int{{52, 64}, {100, 140}} {
		for c := r[0]; c <= r[1]; c++ {
			dfsChannels[c] = true
		}
	}
}

// ProbeWifiChan surveys every up radio: airtime of the current channel from
// `iw survey dump` and neighbor BSS from `iw dev <if> scan`. A radio whose
// driver refuses the background scan still gets a recommendation, but it is
// built only from the survey and flagged with scan_complete=false.
func ProbeWifiChan() []WifiChanRadio {
	if _, err := exec.LookPath("iw"); err != nil {
		return nil
	}
	radios, err := ubus.GetWirelessStatus()
	if err != nil {
		return nil
	}
	var out []WifiChanRadio
	for _, r := range radios {
		if !r.Up {
			continue
		}
		ifname := ""
		for _, iface := range r.Interfaces {
			if !iface.Disabled && iface.Ifname != "" {
				ifname = iface.Ifname
				break
			}
		}
		if ifname == "" {
			continue
		}
		cur, err := strconv.Atoi(r.Channel)
		if err != nil {
			continue
		}
		rec := WifiChanRadio{
			Radio:     r.Name,
			Band:      r.Band,
			Ifname:    ifname,
			Channel:   cur,
			Width:     htmodeWidth(r.Htmode),
			Htmode:    r.Htmode,
			Neighbors: []WifiChanNeighbor{},
		}
		if _, noise, util, ok := parseSurveyInUse(iwRead(ifname, "survey", "dump")); ok {
			rec.Noise = noise
			rec.Utilization = util
		}
		neighbors, ok := parseScanNeighbors(iwRead(ifname, "scan"))
		rec.ScanComplete = ok
		for _, n := range neighbors {
			if n.Channel > 0 {
				rec.Neighbors = append(rec.Neighbors, n)
			}
		}
		sug, _ := recommendChannel(buildChanLoads(rec), cur, rec.Width, r.Band, r.Htmode, rec.ScanComplete)
		rec.Suggestion = &sug
		out = append(out, rec)
	}
	return out
}

// ApplyWifiChan applies a channel recommendation reusing SetWifiRadio, so the
// write path (snapshot, wifi_reload, healthcheck, rollback) is the same the
// radio editor uses.
func ApplyWifiChan(radio string, channel, width int) (*ubus.WirelessRadio, bool, error) {
	if radio == "" {
		return nil, false, fmt.Errorf("radio is required")
	}
	if channel <= 0 {
		return nil, false, fmt.Errorf("invalid channel: %d", channel)
	}
	if width != 20 && width != 40 && width != 80 && width != 160 {
		return nil, false, fmt.Errorf("invalid width: %d", width)
	}
	prefix := "HE"
	if r, err := radioFromStatus(radio); err == nil {
		if m := regexp.MustCompile(`^(HE|VHT|HT)`).FindStringSubmatch(r.Htmode); m != nil {
			prefix = m[1]
		}
	}
	return SetWifiRadio(RadioEdit{
		Radio:   radio,
		Channel: strconv.Itoa(channel),
		Htmode:  fmt.Sprintf("%s%d", prefix, width),
	})
}

// htmodeWidth extracts the channel width in MHz from an htmode like HE80.
func htmodeWidth(htmode string) int {
	m := regexp.MustCompile(`(\d+)`).FindStringSubmatch(htmode)
	if m == nil {
		return 20
	}
	w, _ := strconv.Atoi(m[1])
	if w <= 0 {
		return 20
	}
	return w
}

// iwRead runs a read-only iw command with a bounded timeout. Read commands
// never go through the executor (only writes do); a missing iw or a driver
// that rejects the command yields empty output, which the parsers treat as
// "no data".
func iwRead(ifname string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	full := append([]string{"dev", ifname}, args...)
	out, err := exec.CommandContext(ctx, "iw", full...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// parseSurveyInUse extracts frequency, noise and busy/active utilization of
// the channel marked "[in use]" in `iw survey dump` output.
func parseSurveyInUse(out string) (freq int, noise float64, util float64, ok bool) {
	blocks := strings.Split(out, "Survey data from")
	for _, b := range blocks {
		if !strings.Contains(b, "[in use]") {
			continue
		}
		freq = surveyIntField(b, "frequency")
		noise = surveyFloatField(b, "noise")
		active := surveyIntField(b, "channel active time")
		busy := surveyIntField(b, "channel busy time")
		if active > 0 {
			util = math.Min(100, float64(busy)*100/float64(active))
		}
		return freq, noise, util, freq > 0
	}
	return 0, 0, 0, false
}

func surveyField(block, name string) string {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, name+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, name+":"))
		}
	}
	return ""
}

func surveyIntField(block, name string) int {
	fields := strings.Fields(surveyField(block, name))
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(fields[0])
	return n
}

func surveyFloatField(block, name string) float64 {
	f := surveyField(block, name)
	fields := strings.Fields(f)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.ParseFloat(fields[0], 64)
	return n
}

// parseScanNeighbors parses `iw dev <if> scan` output into neighbor BSS. ok
// is false when the scan produced nothing usable (driver without background
// scan support, interface busy): the caller degrades gracefully.
func parseScanNeighbors(out string) ([]WifiChanNeighbor, bool) {
	var outN []WifiChanNeighbor
	for _, b := range strings.Split(out, "BSS ") {
		if !strings.Contains(b, "freq:") {
			continue
		}
		head := strings.TrimSpace(b)
		mac := head
		if i := strings.Index(head, "(on"); i >= 0 {
			mac = head[:i]
		} else if i := strings.Index(head, " "); i >= 0 {
			mac = head[:i]
		}
		n := WifiChanNeighbor{BSSID: strings.TrimSpace(mac)}
		for _, line := range strings.Split(b, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "freq:"):
				f, _ := strconv.ParseFloat(strings.Fields(line)[1], 64)
				freq := int(f)
				n.Freq = freq
				n.Channel = freqToChannel(freq)
			case strings.HasPrefix(line, "signal:"):
				sig, _ := strconv.ParseFloat(strings.Fields(line)[1], 64)
				n.Signal = sig
			case strings.HasPrefix(line, "SSID:"):
				n.SSID = strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
			case strings.HasPrefix(line, "channel width:"):
				w, _ := strconv.Atoi(strings.Fields(line)[2])
				n.Width = w
			}
		}
		if n.Width == 0 {
			n.Width = 20
		}
		if n.Freq > 0 {
			outN = append(outN, n)
		}
	}
	return outN, len(outN) > 0
}

// freqToChannel maps a frequency in MHz to a Wi-Fi channel number.
func freqToChannel(freq int) int {
	switch {
	case freq >= 2412 && freq <= 2484:
		if freq == 2484 {
			return 14
		}
		return (freq-2407)/5
	case freq >= 5180 && freq <= 5885:
		return (freq - 5000) / 5
	}
	return 0
}

// buildChanLoads aggregates the probe into per-channel scoring inputs: the
// survey utilization lands on the current channel, each neighbor BSS counts
// toward the channel it occupies.
func buildChanLoads(rec WifiChanRadio) []ChanLoad {
	byChan := map[int]*ChanLoad{}
	get := func(ch int) *ChanLoad {
		if l, ok := byChan[ch]; ok {
			return l
		}
		l := &ChanLoad{Channel: ch}
		byChan[ch] = l
		return l
	}
	if rec.Utilization > 0 || rec.Channel > 0 {
		get(rec.Channel).Utilization = rec.Utilization
	}
	for _, n := range rec.Neighbors {
		if n.Channel > 0 {
			get(n.Channel).Neighbors++
		}
	}
	var loads []ChanLoad
	for _, l := range byChan {
		loads = append(loads, *l)
	}
	return loads
}

// chanCandidates lists the channels worth considering for a band/width
// combination. 2.4 GHz stays on the classic 1/6/11; 5 GHz prefers non-DFS,
// but keeps the current channel in the pool when it is DFS and working, so
// the scorer can decide the move is not worth it.
func chanCandidates(band string, width, current int) []int {
	if band == "2g" {
		return []int{1, 6, 11}
	}
	step := width / 20 // channels are 20 MHz apart in numbering
	if step < 1 {
		step = 1
	}
	var nonDFS []int
	for c := 36; c <= 64; c += step {
		if !dfsChannels[c] {
			nonDFS = append(nonDFS, c)
		}
	}
	for c := 149; c <= 165; c += step {
		if !dfsChannels[c] {
			nonDFS = append(nonDFS, c)
		}
	}
	seen := map[int]bool{}
	var out []int
	for _, c := range nonDFS {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	if dfsChannels[current] && !seen[current] {
		out = append(out, current)
	}
	return out
}

// channelsOverlap reports whether a channel/width pair overlaps another.
func channelsOverlap(band string, a, wa int, b, wb int) bool {
	if band == "2g" {
		span := wa/10 + wb/10 - 1 // 20 MHz spans ~2 channel numbers
		if span < 1 {
			span = 1
		}
		d := a - b
		if d < 0 {
			d = -d
		}
		return d < span
	}
	fa := 5000 + 5*a
	fb := 5000 + 5*b
	d := fa - fb
	if d < 0 {
		d = -d
	}
	return d < (wa+wb)/2
}

// recommendChannel scores candidate channels deterministically and returns
// the best one with an i18n reason key. Weights: airtime utilization of
// overlapping channels, number of foreign BSS, a DFS penalty and a bonus for
// staying put (an unnecessary change drops every client for a moment).
func recommendChannel(loads []ChanLoad, current, width int, band, htmode string, scanComplete bool) (WifiChanSuggestion, string) {
	sug := WifiChanSuggestion{Channel: current, Width: width, Htmode: htmode}
	cands := chanCandidates(band, width, current)
	if len(cands) == 0 {
		sug.Reason = "current_ok"
		sug.Confidence = "low"
		return sug, sug.Reason
	}
	score := func(c int) float64 {
		s := 0.0
		for _, l := range loads {
			if channelsOverlap(band, c, width, l.Channel, 20) {
				s += l.Utilization + float64(l.Neighbors)*12
			}
		}
		if band != "2g" && dfsChannels[c] {
			s += 25
		}
		if c == current {
			s -= 15 // staying is cheaper than moving
		}
		return s
	}
	best, bestScore := current, score(current)
	for _, c := range cands {
		if s := score(c); s < bestScore {
			best, bestScore = c, s
		}
	}
	switch {
	case best == current:
		sug.Reason = "current_ok"
		sug.Confidence = "high"
	case band != "2g" && dfsChannels[current] && !dfsChannels[best]:
		sug.Reason = "avoid_dfs"
		sug.Confidence = "high"
	case !scanComplete:
		sug.Reason = "partial"
		sug.Confidence = "low"
	default:
		sug.Reason = "less_congested"
		sug.Confidence = "high"
	}
	sug.Channel = best
	return sug, sug.Reason
}
