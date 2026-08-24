package ubus

import "encoding/json"

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
	return info, nil
}
