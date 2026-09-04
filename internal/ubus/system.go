package ubus

import (
	"bufio"
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

	// On ubifs/overlay routers MemAvailable is pessimistic (it does not
	// discount the dentry cache), so (total-available)/total overreports
	// the used percentage (e.g. 75% when the real usage is ~20%). Recompute
	// available as free + buffers + cached + SReclaimable (/proc/meminfo),
	// which is what can actually be reclaimed, and expose it.
	if mem := readMeminfo(); len(mem) > 0 {
		total := mem["MemTotal"] * 1024
		available := (mem["MemFree"] + mem["Buffers"] + mem["Cached"] + mem["SReclaimable"]) * 1024
		if total > 0 && available > 0 {
			info.Memory.Available = available
			if info.Memory.Total == 0 {
				info.Memory.Total = total
			}
		}
	}
	return info, nil
}

// readMeminfo returns /proc/meminfo values in KiB (per-line key -> value).
func readMeminfo() map[string]int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil
	}
	return parseMeminfo(string(data))
}

// parseMeminfo parses the /proc/meminfo text format (key: value KiB).
func parseMeminfo(text string) map[string]int64 {
	out := map[string]int64{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := sc.Text()
		sep := strings.IndexByte(line, ':')
		if sep < 0 {
			continue
		}
		key := strings.TrimSpace(line[:sep])
		fields := strings.Fields(line[sep+1:])
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.ParseInt(fields[0], 10, 64)
		if err == nil {
			out[key] = v
		}
	}
	return out
}
