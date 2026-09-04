package ubus

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// SystemInfo mirrors `ubus call system info`. Load values are fixed-point
// (load * 65536), root/tmp are in KiB, memory in bytes.
type SystemInfo struct {
	Uptime int64     `json:"uptime"`
	Load   []float64 `json:"load"` // normalized to /proc/loadavg units
	Memory struct {
		Total     int64 `json:"total"`
		Free      int64 `json:"free"`
		Available int64 `json:"available"`
		Cached    int64 `json:"cached"`
		Buffered  int64 `json:"buffered"`
	} `json:"memory"`
	Root struct {
		Total int64 `json:"total"`
		Free  int64 `json:"free"`
	} `json:"root"`
}

func GetSystemInfo() (*SystemInfo, error) {
	raw, err := Call("system", "info")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Uptime int64   `json:"uptime"`
		Load   []int64 `json:"load"`
		Memory struct {
			Total     int64 `json:"total"`
			Free      int64 `json:"free"`
			Available int64 `json:"available"`
			Cached    int64 `json:"cached"`
			Buffered  int64 `json:"buffered"`
		} `json:"memory"`
		Root struct {
			Total int64 `json:"total"`
			Free  int64 `json:"free"`
		} `json:"root"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	info := &SystemInfo{Uptime: payload.Uptime}
	for _, l := range payload.Load {
		info.Load = append(info.Load, float64(l)/65536.0)
	}
	info.Memory = payload.Memory
	info.Root = payload.Root

	// /proc/meminfo is misleading on ubifs/overlay routers: MemAvailable is
	// pessimistic and the squashfs page cache is not counted as free, so
	// (total-available)/total overreports (75% when most is reclaimable
	// cache, still ~59% after discounting cached/sreclaimable). What a panel
	// user really wants is the memory held by processes. Report the used as
	// the sum of process VmRSS, so an AP with ~70 MB of processes shows ~17%
	// instead of an inflated figure.
	if rss := sumProcRSS(); rss > 0 && info.Memory.Total > rss {
		info.Memory.Available = info.Memory.Total - rss
	}
	return info, nil
}

// sumProcRSS sums the VmRSS of every user-space process (/proc/<pid>/status),
// i.e. the memory actually held by processes (kernel page cache excluded).
func sumProcRSS() int64 {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		pid := e.Name()
		if !isDigits(pid) {
			continue
		}
		b, err := os.ReadFile("/proc/" + pid + "/status")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						total += v * 1024
					}
				}
				break
			}
		}
	}
	return total
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
