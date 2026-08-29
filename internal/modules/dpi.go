package modules

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
)

type DPIProtocol struct {
	Name     string `json:"name"`
	Bytes    int64  `json:"bytes"`
	Flows    int    `json:"flows"`
	Category string `json:"category"`
}

type DPIProbe struct {
	Applicable bool          `json:"applicable"`
	TotalBytes int64         `json:"total_bytes"`
	TotalFlows int           `json:"total_flows"`
	Protocols  []DPIProtocol `json:"protocols"`
}

var portMap = map[int]struct{ name, category string }{
	20:    {"FTP-Data", "file"},
	21:    {"FTP", "file"},
	22:    {"SSH", "admin"},
	23:    {"Telnet", "admin"},
	25:    {"SMTP", "mail"},
	53:    {"DNS", "dns"},
	80:    {"HTTP", "web"},
	110:   {"POP3", "mail"},
	143:   {"IMAP", "mail"},
	443:   {"HTTPS", "web"},
	465:   {"SMTPS", "mail"},
	587:   {"SMTP-Sub", "mail"},
	993:   {"IMAPS", "mail"},
	995:   {"POP3S", "mail"},
	1935:  {"RTMP", "streaming"},
	3306:  {"MySQL", "database"},
	3389:  {"RDP", "admin"},
	5060:  {"SIP", "voip"},
	5222:  {"XMPP", "chat"},
	5432:  {"PostgreSQL", "database"},
	5900:  {"VNC", "admin"},
	8080:  {"HTTP-Alt", "web"},
	8443:  {"HTTPS-Alt", "web"},
	8883:  {"MQTT-TLS", "iot"},
	1883:  {"MQTT", "iot"},
	5683:  {"CoAP", "iot"},
	6881:  {"BitTorrent", "p2p"},
	27017: {"MongoDB", "database"},
}

var categoryNames = map[string]string{
	"web":       "Web",
	"dns":       "DNS",
	"mail":      "Mail",
	"admin":     "Admin/Remote",
	"file":      "File Transfer",
	"streaming": "Streaming",
	"voip":      "VoIP",
	"chat":      "Chat",
	"iot":       "IoT",
	"p2p":       "P2P",
	"database":  "Database",
	"other":     "Other",
}

func ProbeDPI() *DPIProbe {
	f, err := os.Open("/proc/net/nf_conntrack")
	if err != nil {
		return &DPIProbe{Applicable: false}
	}
	defer f.Close()

	protoMap := map[string]*DPIProtocol{}
	var totalBytes int64
	totalFlows := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		proto, dport, bytes := parseConntrackLine(line)
		if proto == "" || dport == 0 {
			continue
		}

		name, category := classifyPort(dport, proto)
		key := name

		if _, ok := protoMap[key]; !ok {
			protoMap[key] = &DPIProtocol{Name: name, Category: category}
		}
		protoMap[key].Bytes += bytes
		protoMap[key].Flows++
		totalBytes += bytes
		totalFlows++
	}

	protocols := make([]DPIProtocol, 0)
	for _, p := range protoMap {
		protocols = append(protocols, *p)
	}
	sort.Slice(protocols, func(i, j int) bool { return protocols[i].Bytes > protocols[j].Bytes })

	return &DPIProbe{
		Applicable: true,
		TotalBytes: totalBytes,
		TotalFlows: totalFlows,
		Protocols:  protocols,
	}
}

func parseConntrackLine(line string) (proto string, dport int, bytes int64) {
	fields := strings.Fields(line)
	for _, f := range fields {
		switch {
		case strings.HasPrefix(f, "ipv4") || strings.HasPrefix(f, "ipv6"):
			continue
		case f == "tcp" || f == "udp":
			proto = f
		case strings.HasPrefix(f, "dport="):
			if v, err := strconv.Atoi(strings.TrimPrefix(f, "dport=")); err == nil {
				dport = v
			}
		case strings.HasPrefix(f, "bytes="):
			if v, err := strconv.ParseInt(strings.TrimPrefix(f, "bytes="), 10, 64); err == nil {
				// First bytes= is original direction; sum both directions
				bytes += v
			}
		}
	}
	return
}

func classifyPort(port int, proto string) (string, string) {
	if info, ok := portMap[port]; ok {
		return info.name, info.category
	}
	switch {
	case port >= 49152:
		return "Ephemeral/" + proto, "other"
	case port >= 1024:
		return "High-" + proto, "other"
	default:
		return "Unknown-" + proto, "other"
	}
}

func CategoryName(cat string) string {
	if name, ok := categoryNames[cat]; ok {
		return name
	}
	return cat
}
