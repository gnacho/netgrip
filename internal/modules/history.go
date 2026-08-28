package modules

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
)

const (
	historyPath  = "/etc/netgrip/history.json"
	historySize  = 288
	historyEvery = 5 * time.Minute
)

type HistoryEntry struct {
	Timestamp int64   `json:"ts"`
	RxBytes   int64   `json:"rx"`
	TxBytes   int64   `json:"tx"`
	Load      float64 `json:"load"`
	Clients   int     `json:"clients"`
}

var (
	historyMu   sync.Mutex
	historyRing []HistoryEntry
	historyOnce sync.Once
)

func StartHistoryCollector() {
	historyOnce.Do(func() {
		loadHistory()
		go collectLoop()
	})
}

func collectLoop() {
	ticker := time.NewTicker(historyEvery)
	defer ticker.Stop()
	sample()
	for range ticker.C {
		sample()
	}
}

func sample() {
	counters := NetDevCounters()
	var rx, tx int64
	for _, c := range counters {
		if c.Name == "br-lan" {
			rx = c.RxBytes
			tx = c.TxBytes
			break
		}
	}
	info, _ := ubus.GetSystemInfo()
	var load float64
	if info != nil && len(info.Load) > 0 {
		load = info.Load[0]
	}
	clients := len(ListClients(""))

	entry := HistoryEntry{
		Timestamp: time.Now().Unix(),
		RxBytes:   rx,
		TxBytes:   tx,
		Load:      load,
		Clients:   clients,
	}

	historyMu.Lock()
	historyRing = append(historyRing, entry)
	if len(historyRing) > historySize {
		historyRing = historyRing[len(historyRing)-historySize:]
	}
	ringCopy := make([]HistoryEntry, len(historyRing))
	copy(ringCopy, historyRing)
	historyMu.Unlock()

	saveHistory(ringCopy)
}

func GetHistory() []HistoryEntry {
	historyMu.Lock()
	defer historyMu.Unlock()
	out := make([]HistoryEntry, len(historyRing))
	copy(out, historyRing)
	return out
}

func loadHistory() {
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	historyMu.Lock()
	historyRing = entries
	historyMu.Unlock()
}

func saveHistory(entries []HistoryEntry) {
	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	os.WriteFile(historyPath, data, 0644)
}
