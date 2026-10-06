package modules

import (
	"strconv"
	"strings"
	"testing"
)

const surveyFixture = `Survey data from phy0-ap0
	frequency:			2412 MHz [in use]
	noise:				-87 dBm
	channel active time:		79249139 ms
	channel busy time:		42716927 ms
	channel receive time:		37057520 ms
	channel BSS receive time:	0 ms
	channel transmit time:		595839 ms
Survey data from phy0-ap0
	frequency:			2417 MHz
	noise:				-87 dBm
	channel active time:		0 ms
	channel busy time:		0 ms
`

const scanFixture = `BSS 32:4e:8e:ac:c6:16(on phy0-ap0)
	last seen: 79283.884s [boottime]
	freq: 2412.0
	signal: -56.00 dBm
	SSID: temiscira
	HT capabilities:
BSS aa:bb:cc:dd:ee:ff(on phy0-ap0)
	last seen: 10.2s [boottime]
	freq: 2417.0
	channel width: 40 MHz
	signal: -70.00 dBm
	SSID: vecino2
BSS 11:22:33:44:55:66(on phy0-ap0)
	last seen: 5.1s [boottime]
	freq: 2437.0
	signal: -80.00 dBm
	SSID: vecino6
`

func TestParseSurveyInUse(t *testing.T) {
	freq, noise, util, ok := parseSurveyInUse(surveyFixture)
	if !ok {
		t.Fatal("expected in-use survey block")
	}
	if freq != 2412 {
		t.Errorf("freq = %d, want 2412", freq)
	}
	if noise != -87 {
		t.Errorf("noise = %v, want -87", noise)
	}
	if util < 53.8 || util > 54.0 {
		t.Errorf("util = %v, want ~53.9", util)
	}
}

func TestParseSurveyInUseMissing(t *testing.T) {
	if _, _, _, ok := parseSurveyInUse(""); ok {
		t.Fatal("empty output must not parse")
	}
	if _, _, _, ok := parseSurveyInUse("Survey data from phy0\n\tfrequency: 2412 MHz\n"); ok {
		t.Fatal("block without [in use] must not parse")
	}
}

func TestParseScanNeighbors(t *testing.T) {
	ns, ok := parseScanNeighbors(scanFixture)
	if !ok {
		t.Fatal("expected neighbors")
	}
	if len(ns) != 3 {
		t.Fatalf("got %d neighbors, want 3", len(ns))
	}
	if ns[0].BSSID != "32:4e:8e:ac:c6:16" || ns[0].Channel != 1 || ns[0].SSID != "temiscira" {
		t.Errorf("n0 = %+v", ns[0])
	}
	if ns[1].Channel != 2 || ns[1].Width != 40 {
		t.Errorf("n1 = %+v, want ch2 width40", ns[1])
	}
	if ns[2].Channel != 6 || ns[2].Width != 20 {
		t.Errorf("n2 = %+v, want ch6 width20 (default)", ns[2])
	}
	if _, ok := parseScanNeighbors(""); ok {
		t.Fatal("empty scan must report ok=false")
	}
}

func TestFreqToChannel(t *testing.T) {
	cases := map[int]int{2412: 1, 2437: 6, 2462: 11, 2484: 14, 5180: 36, 5200: 40, 5240: 48, 5745: 149}
	for freq, want := range cases {
		if got := freqToChannel(freq); got != want {
			t.Errorf("freqToChannel(%d) = %d, want %d", freq, got, want)
		}
	}
	if got := freqToChannel(1000); got != 0 {
		t.Errorf("unknown freq = %d, want 0", got)
	}
}

func TestRecommendPrefersFree24g(t *testing.T) {
	// Current ch6 is crowded (high util + neighbors), ch1 and ch11 empty.
	loads := []ChanLoad{
		{Channel: 6, Utilization: 80, Neighbors: 4},
		{Channel: 1, Utilization: 0, Neighbors: 0},
	}
	sug, reason := recommendChannel(loads, 6, 20, "2g", "HE20", true)
	if sug.Channel != 1 && sug.Channel != 11 {
		t.Errorf("suggestion = ch%d, want a free preferred channel", sug.Channel)
	}
	if reason != "less_congested" {
		t.Errorf("reason = %q, want less_congested", reason)
	}
	if sug.Confidence != "high" {
		t.Errorf("confidence = %q, want high", sug.Confidence)
	}
}

func TestRecommendStaysWhenCurrentIsGood(t *testing.T) {
	loads := []ChanLoad{
		{Channel: 6, Utilization: 10, Neighbors: 0},
		{Channel: 1, Utilization: 0, Neighbors: 2},
	}
	sug, reason := recommendChannel(loads, 6, 20, "2g", "HE20", true)
	if sug.Channel != 6 {
		t.Errorf("suggestion = ch%d, want to stay on ch6", sug.Channel)
	}
	if reason != "current_ok" {
		t.Errorf("reason = %q, want current_ok", reason)
	}
}

func TestRecommendAvoidsDfs(t *testing.T) {
	// Current ch52 (DFS) has light load; a non-DFS alternative exists at no
	// extra congestion cost.
	loads := []ChanLoad{
		{Channel: 52, Utilization: 15, Neighbors: 0},
		{Channel: 36, Utilization: 18, Neighbors: 0},
	}
	sug, reason := recommendChannel(loads, 52, 80, "5g", "HE80", true)
	if reason != "avoid_dfs" {
		t.Errorf("reason = %q, want avoid_dfs (suggested ch%d)", reason, sug.Channel)
	}
	if sug.Channel == 52 {
		t.Error("should move off DFS")
	}
}

func TestRecommendKeepsWorkingDfs(t *testing.T) {
	// Current ch100 DFS is nearly idle; every non-DFS candidate carries load
	// (a real scan covers the whole band), so the DFS penalty must not force
	// a worse move.
	loads := []ChanLoad{
		{Channel: 100, Utilization: 5, Neighbors: 0},
	}
	for _, c := range chanCandidates("5g", 80, 36) {
		loads = append(loads, ChanLoad{Channel: c, Utilization: 50, Neighbors: 2})
	}
	sug, _ := recommendChannel(loads, 100, 80, "5g", "HE80", true)
	if sug.Channel != 100 {
		t.Errorf("suggestion = ch%d, want to keep the working DFS channel", sug.Channel)
	}
}

func TestRecommendPartialScan(t *testing.T) {
	// No scan: neighbors unknown, suggestion differs but confidence is low
	// and the reason flags the partial data.
	loads := []ChanLoad{{Channel: 6, Utilization: 90}}
	sug, reason := recommendChannel(loads, 6, 20, "2g", "HE20", false)
	if sug.Channel == 6 {
		t.Fatal("ch6 at 90% airtime should not be kept")
	}
	if reason != "partial" || sug.Confidence != "low" {
		t.Errorf("reason=%q confidence=%q, want partial/low", reason, sug.Confidence)
	}
}

func TestRecommendWidthKeepsSuggestions(t *testing.T) {
	for _, width := range []int{20, 40, 80} {
		sug, _ := recommendChannel(nil, 36, width, "5g", "HE80", true)
		if sug.Width != width {
			t.Errorf("width = %d, want %d", sug.Width, width)
		}
	}
}

func TestChanCandidates24g(t *testing.T) {
	c := chanCandidates("2g", 20, 6)
	if strings.Join(ints(c), ",") != "1,6,11" {
		t.Errorf("2g candidates = %v", c)
	}
}

func TestHtmodeWidth(t *testing.T) {
	if htmodeWidth("HE80") != 80 || htmodeWidth("VHT40") != 40 || htmodeWidth("") != 20 {
		t.Error("htmodeWidth mismatch")
	}
}

func ints(v []int) []string {
	out := make([]string, len(v))
	for i, n := range v {
		out[i] = strconv.Itoa(n)
	}
	return out
}
