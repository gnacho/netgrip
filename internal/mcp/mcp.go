// Package mcp exposes an embedded MCP (Model Context Protocol) server so AI
// assistants can query the panel's probes without browser automation (#439).
// It runs in the same binary and process, mounted at /mcp with the
// streamable-HTTP transport from mcp-go. Auth: ONLY a dedicated bearer
// token read from UCI (netgrip.mcp.token), never the session cookie.
// Activation: netgrip.mcp.enabled=1; disabled by default and the endpoint
// answers 404 while disabled, so an inactive install reveals nothing.
//
// First batch of tools: READ-ONLY, backed by the same probes and caches the
// panel uses. They never write UCI, never restart services and never shell
// out beyond what the panel already does. All declare readOnlyHint from day
// one; future write tools must declare destructiveHint.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gnacho/netgrip/internal/modules"
	"github.com/gnacho/netgrip/internal/ubus"
)

const (
	// leasesPath mirrors the panel's DHCP lease file location.
	leasesPath = "/tmp/dhcp.leases"
)

// probeTTL mirrors the panel's probe cache TTL: MCP reads share the same
// caches instead of forking ubus calls per assistant request.
const probeTTL = 15 * time.Second

// Deps carries the panel identity the tools report. The data sources are the
// package-level probes in internal/modules and internal/ubus, the same ones
// the HTTP handlers use, so MCP results never drift from what the UI shows.
type Deps struct {
	Version string
	Started time.Time
	// RatePerMin caps tool calls per client IP per minute. Zero means the
	// default (60), matching the sibling apps' MCP rate limit.
	RatePerMin int
}

// Server wraps the mcp-go server with NetGrip's probes and the per-call
// audit log.
type Server struct {
	version    string
	started    time.Time
	ratePerMin int
	http       *server.StreamableHTTPServer
}

// New builds the MCP server with the first read-only tool batch.
func New(d Deps) *Server {
	s := &Server{
		version:    d.Version,
		started:    d.Started,
		ratePerMin: d.RatePerMin,
	}
	if s.ratePerMin <= 0 {
		s.ratePerMin = 60
	}
	mcpServer := server.NewMCPServer(
		"netgrip",
		d.Version,
		server.WithToolCapabilities(false),
	)
	s.addTools(mcpServer)
	s.http = server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true))
	return s
}

// readOnly is the shared annotation of the whole first batch: AI clients can
// run these freely without asking the user for confirmation. mcp-go defaults
// destructiveHint/openWorldHint to TRUE, so read-only tools must turn them
// off explicitly.
var readOnly = []mcp.ToolOption{
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithOpenWorldHintAnnotation(false),
}

// tool builds a tool with description + own options + the shared read-only
// annotations. Helper because Go cannot mix plain and spread variadic args
// in one call.
func tool(name, description string, opts ...mcp.ToolOption) mcp.Tool {
	all := make([]mcp.ToolOption, 0, len(opts)+len(readOnly)+1)
	all = append(all, mcp.WithDescription(description))
	all = append(all, opts...)
	all = append(all, readOnly...)
	return mcp.NewTool(name, all...)
}

// addTools registers the seven read-only tools.
func (s *Server) addTools(mcpServer *server.MCPServer) {
	mcpServer.AddTool(tool("server_status",
		"NetGrip service status: version, uptime, system info (load, memory, board), client counts by link type and Wi-Fi band, and uplink health. Read-only.",
	), s.run("server_status", s.toolServerStatus))

	mcpServer.AddTool(tool("list_clients",
		"Clients seen by the router (DHCP, ARP and wireless association), with IP, MAC, hostname, vendor guess, signal and band when wireless, traffic counters when known, and whether each client is blocked. Read-only.",
	), s.run("list_clients", s.toolListClients))

	mcpServer.AddTool(tool("wireless_status",
		"Wi-Fi radios and their SSIDs with channel, bandwidth, transmit power and associated stations. Read-only.",
	), s.run("wireless_status", s.toolWirelessStatus))

	mcpServer.AddTool(tool("wan_status",
		"Status of the primary WAN uplink: protocol, IPv4/IPv6 addresses, gateway, DNS, uptime and errors. Read-only.",
	), s.run("wan_status", s.toolWanStatus))

	mcpServer.AddTool(tool("multiwan_status",
		"Internet uplinks and, when mwan3 is installed, what it is doing with them: per-interface online/tracking state, active policy and interface shares. Read-only.",
	), s.run("multiwan_status", s.toolMultiWanStatus))

	mcpServer.AddTool(tool("list_leases",
		"Active DHCP leases with MAC, IP, hostname and expiry. Read-only.",
	), s.run("list_leases", s.toolListLeases))

	mcpServer.AddTool(tool("list_devices",
		"Network devices and their traffic counters (netdev view), plus the Ethernet switch ports with link state and speed when the switch exposes them. Read-only.",
	), s.run("list_devices", s.toolListDevices))

	mcpServer.AddTool(tool("channel_recommendation",
		"Wi-Fi channel recommendation per radio from live RF data (survey utilization and neighboring BSSs): current channel, utilization, neighbor count, and a deterministic suggestion with reason and confidence. Read-only; it never applies anything.",
	), s.run("channel_recommendation", s.toolChannelRecommendation))

	mcpServer.AddTool(tool("wifi_schedule_status",
		"Per-SSID weekly off-schedule state: configured windows, whether each SSID is off by schedule right now, and manual pause state. Read-only.",
	), s.run("wifi_schedule_status", s.toolWifiScheduleStatus))

	mcpServer.AddTool(tool("snmp_status",
		"SNMP agent state: package installed, daemon running/enabled, managed-vs-external config, location/contact/listen and the configured communities. Never returns community values unless they are configured in UCI; read-only.",
	), s.run("snmp_status", s.toolSNMPStatus))
}

func (s *Server) toolChannelRecommendation(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	return map[string]any{"radios": modules.ProbeWifiChan()}, nil
}

func (s *Server) toolWifiScheduleStatus(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	return map[string]any{"schedules": modules.ProbeWifiSchedule()}, nil
}

func (s *Server) toolSNMPStatus(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	return modules.ProbeSNMP(), nil
}

// toolFunc produces a tool result from the current probes and the request
// arguments.
type toolFunc func(ctx context.Context, req mcp.CallToolRequest) (any, error)

// run wraps every tool with the audit log. Business errors come back as
// ToolResultError (isError), not as a Go error: the protocol treats them as
// a valid result carrying an error inside.
func (s *Server) run(name string, fn toolFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		v, err := fn(ctx, req)
		audit(name, req, err, time.Since(start))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultJSON(v)
	}
}

// audit records every tool call: name, arguments, outcome and duration. It
// is the trail of which assistant read what and when.
func audit(name string, req mcp.CallToolRequest, err error, dur time.Duration) {
	args, _ := json.Marshal(req.GetArguments())
	if err != nil {
		log.Printf("[netgrip] mcp: tool=%s args=%s error=%q dur=%s", name, args, err.Error(), dur)
		return
	}
	log.Printf("[netgrip] mcp: tool=%s args=%s ok dur=%s", name, args, dur)
}

// serverStatus summarizes the router without exposing secrets: no passwords,
// no full config, no session data.
type serverStatus struct {
	Name      string           `json:"name"`
	Version   string           `json:"version"`
	UptimeSec int64            `json:"uptime_sec"`
	System    *ubus.SystemInfo `json:"system,omitempty"`
	Clients   map[string]int   `json:"clients"`
	Wan       *ubus.WanStatus  `json:"wan,omitempty"`
	WanError  string           `json:"wan_error,omitempty"`
}

func (s *Server) toolServerStatus(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	out := serverStatus{
		Name:      "NetGrip",
		Version:   s.version,
		UptimeSec: int64(time.Since(s.started).Seconds()),
		Clients:   clientCounts(),
	}
	if info, err := ubus.GetSystemInfo(); err == nil {
		out.System = info
	}
	if wan, err := ubus.GetWanStatus(); err == nil {
		out.Wan = wan
	} else {
		out.WanError = err.Error()
	}
	return out, nil
}

// clientCounts aggregates the client list the same way the MQTT telemetry
// does, so the numbers match the Home Assistant entities. It shares the
// "clients|mcp" cache entry with the list_clients tool: same compute
// function, so first writer populates the full payload for both.
func clientCounts() map[string]int {
	payload, err := modules.CachedRead("clients|mcp", probeTTL, clientsPayload)
	if err != nil {
		return map[string]int{}
	}
	clients, _ := payload["clients"].([]modules.Client)
	counts := map[string]int{"total": len(clients)}
	for _, c := range clients {
		switch c.Type {
		case "wifi24":
			counts["wifi"]++
			counts["wifi_2_4"]++
		case "wifi5":
			counts["wifi"]++
			counts["wifi_5_6"]++
		case "cable":
			counts["wired"]++
		}
		if c.Type != "cable" && c.Signal < -75 {
			counts["weak_signal"]++
		}
		if c.Blocked {
			counts["blocked"]++
		}
	}
	return counts
}

func (s *Server) toolListClients(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	payload, err := modules.CachedRead("clients|mcp", probeTTL, clientsPayload)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	return payload, nil
}

// clientsPayload is the shared cache compute for the "clients|mcp" entry.
func clientsPayload() (map[string]any, error) {
	return map[string]any{
		"clients": modules.ListClients("mcp"),
		"bands":   modules.AvailableBands(),
		"ts":      time.Now().UnixMilli(),
	}, nil
}

func (s *Server) toolWirelessStatus(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	radios, err := modules.CachedRead("wireless", probeTTL, func() ([]ubus.WirelessRadio, error) {
		return ubus.GetWirelessStatus()
	})
	if err != nil {
		return nil, fmt.Errorf("wireless status: %w", err)
	}
	return map[string]any{"radios": radios}, nil
}

func (s *Server) toolWanStatus(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	status, err := ubus.GetWanStatus()
	if err != nil {
		return nil, fmt.Errorf("wan status: %w", err)
	}
	return status, nil
}

func (s *Server) toolMultiWanStatus(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	return modules.ProbeMultiWAN(), nil
}

func (s *Server) toolListLeases(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	leases, err := ubus.ReadLeases(leasesPath)
	if err != nil {
		return nil, fmt.Errorf("dhcp leases: %w", err)
	}
	return map[string]any{"leases": leases}, nil
}

func (s *Server) toolListDevices(_ context.Context, _ mcp.CallToolRequest) (any, error) {
	return map[string]any{"devices": modules.NetDevCounters()}, nil
}
