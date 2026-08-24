package ubus

import "encoding/json"

// WanStatus is the WAN interface state. Present is false on pure access
// points (no wan interface at all).
type WanStatus struct {
	Present bool     `json:"present"`
	Up      bool     `json:"up"`
	Uptime  int64    `json:"uptime"`
	IPv4    []string `json:"ipv4"`
	Gateway string   `json:"gateway,omitempty"`
	DNS     []string `json:"dns,omitempty"`
}

func GetWanStatus() (*WanStatus, error) {
	raw, err := Call("network.interface.wan", "status")
	if err != nil {
		// Dumb APs have no wan interface: ubus exits non-zero ("Not found").
		return &WanStatus{Present: false, IPv4: []string{}, DNS: []string{}}, nil
	}
	var payload struct {
		Up     bool  `json:"up"`
		Uptime int64 `json:"uptime"`
		IPv4   []struct {
			Address string `json:"address"`
			Mask    int    `json:"mask"`
		} `json:"ipv4-address"`
		Route []struct {
			Target  string `json:"target"`
			Nexthop string `json:"nexthop"`
		} `json:"route"`
		DNS []string `json:"dns-server"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	status := &WanStatus{Present: true, Up: payload.Up, Uptime: payload.Uptime, IPv4: []string{}, DNS: payload.DNS}
	for _, a := range payload.IPv4 {
		status.IPv4 = append(status.IPv4, a.Address)
	}
	for _, r := range payload.Route {
		if r.Target == "0.0.0.0" && r.Nexthop != "" {
			status.Gateway = r.Nexthop
			break
		}
	}
	if status.DNS == nil {
		status.DNS = []string{}
	}
	return status, nil
}
