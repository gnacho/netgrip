package modules

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	bufferbloatPath = "/etc/netgrip/bufferbloat.json"
	bufferbloatMax  = 50
	saturateURL     = "http://speedtest.tele2.net/10MB.zip"
)

type BufferbloatResult struct {
	BaselineMs    float64   `json:"baseline_ms"`
	LoadedMs      float64   `json:"loaded_ms"`
	DeltaMs       float64   `json:"delta_ms"`
	Grade         string    `json:"grade"`
	Timestamp     string    `json:"timestamp"`
	SamplesLoaded []float64 `json:"samples_loaded"`
}

var (
	bufferbloatMu      sync.Mutex
	bufferbloatHistory []BufferbloatResult
	bufferbloatOnce    sync.Once
)

func initBufferbloatHistory() {
	bufferbloatOnce.Do(func() {
		data, err := os.ReadFile(bufferbloatPath)
		if err != nil {
			return
		}
		_ = json.Unmarshal(data, &bufferbloatHistory)
	})
}

func appendBufferbloatResult(r BufferbloatResult) {
	bufferbloatMu.Lock()
	defer bufferbloatMu.Unlock()
	initBufferbloatHistory()
	bufferbloatHistory = append(bufferbloatHistory, r)
	if len(bufferbloatHistory) > bufferbloatMax {
		bufferbloatHistory = bufferbloatHistory[len(bufferbloatHistory)-bufferbloatMax:]
	}
	data, err := json.Marshal(bufferbloatHistory)
	if err != nil {
		return
	}
	os.MkdirAll("/etc/netgrip", 0755)
	os.WriteFile(bufferbloatPath, data, 0644)
}

func GetBufferbloatHistory() []BufferbloatResult {
	bufferbloatMu.Lock()
	defer bufferbloatMu.Unlock()
	initBufferbloatHistory()
	out := make([]BufferbloatResult, len(bufferbloatHistory))
	copy(out, bufferbloatHistory)
	return out
}

var rttRe = regexp.MustCompile(`time=([0-9.]+)\s*ms`)

func pingRTTs(gateway string, count int) ([]float64, error) {
	cmd := exec.Command("ping", "-c", fmt.Sprint(count), "-i", "0.2", gateway)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}
	matches := rttRe.FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no ping replies parsed")
	}
	rtts := make([]float64, 0, len(matches))
	for _, m := range matches {
		v := 0.0
		fmt.Sscanf(m[1], "%f", &v)
		if v > 0 {
			rtts = append(rtts, v)
		}
	}
	return rtts, nil
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := make([]float64, len(v))
	copy(s, v)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[n/2]
}

func gradeFor(delta float64) string {
	switch {
	case delta < 5:
		return "A"
	case delta < 15:
		return "B"
	case delta < 40:
		return "C"
	default:
		return "D"
	}
}

func defaultGateway() (string, error) {
	out, err := exec.Command("sh", "-c", "ip route show default 2>/dev/null | awk '{print $3}' | head -1").Output()
	if err != nil {
		return "", fmt.Errorf("cannot resolve default gateway: %w", err)
	}
	gw := strings.TrimSpace(string(out))
	if gw == "" {
		return "", fmt.Errorf("no default gateway found")
	}
	return gw, nil
}

func RunBufferbloatTest() (*BufferbloatResult, error) {
	gw, err := defaultGateway()
	if err != nil {
		return nil, err
	}

	baselineRTTs, err := pingRTTs(gw, 20)
	if err != nil {
		return nil, fmt.Errorf("baseline ping: %w", err)
	}
	baselineMs := median(baselineRTTs)

	wget := exec.Command("wget", "-O", "/dev/null", "--timeout=15", "-q", saturateURL)
	if err := wget.Start(); err != nil {
		return nil, fmt.Errorf("saturation download: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

	loadedRTTs, err := pingRTTs(gw, 30)
	_ = wget.Process.Kill()
	_ = wget.Wait()
	if err != nil {
		return nil, fmt.Errorf("loaded ping: %w", err)
	}
	loadedMs := median(loadedRTTs)

	delta := loadedMs - baselineMs
	if delta < 0 {
		delta = 0
	}

	result := &BufferbloatResult{
		BaselineMs:    math.Round(baselineMs*100) / 100,
		LoadedMs:      math.Round(loadedMs*100) / 100,
		DeltaMs:       math.Round(delta*100) / 100,
		Grade:         gradeFor(delta),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		SamplesLoaded: loadedRTTs,
	}
	appendBufferbloatResult(*result)
	return result, nil
}
