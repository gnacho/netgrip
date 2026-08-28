package modules

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type PortStats struct {
	Name     string `json:"name"`
	RxBytes  int64  `json:"rx_bytes"`
	TxBytes  int64  `json:"tx_bytes"`
	RxErrors int64  `json:"rx_errors"`
	TxErrors int64  `json:"tx_errors"`
	RxDrops  int64  `json:"rx_drops"`
	TxDrops  int64  `json:"tx_drops"`
}

type PortStatsProbe struct {
	Ports []PortStats `json:"ports"`
	TS    int64       `json:"ts"`
}

func ProbePortStats() *PortStatsProbe {
	ports := bridgePortList()
	var result []PortStats
	for _, name := range ports {
		base := "/sys/class/net/" + name + "/statistics/"
		ps := PortStats{Name: name}
		ps.RxBytes = readStatInt(base + "rx_bytes")
		ps.TxBytes = readStatInt(base + "tx_bytes")
		ps.RxErrors = readStatInt(base + "rx_errors")
		ps.TxErrors = readStatInt(base + "tx_errors")
		ps.RxDrops = readStatInt(base + "rx_dropped")
		ps.TxDrops = readStatInt(base + "tx_dropped")
		result = append(result, ps)
	}
	return &PortStatsProbe{
		Ports: result,
		TS:    time.Now().UnixMilli(),
	}
}

func readStatInt(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return v
}
