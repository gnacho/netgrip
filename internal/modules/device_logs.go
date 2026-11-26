package modules

import (
	"bufio"
	"os/exec"
	"regexp"
	"strings"
)

// DeviceLogEntry is one line of the system log, parsed.
type DeviceLogEntry struct {
	Timestamp string `json:"ts"`
	Facility  string `json:"facility"`
	Level     string `json:"level"`
	Tag       string `json:"tag"`
	Message   string `json:"message"`
}

// DeviceLogs is the read-only view of the ring buffer, newest last, the way
// logread prints it.
type DeviceLogs struct {
	Entries []DeviceLogEntry `json:"entries"`
	Total   int              `json:"total"`
	Error   string           `json:"error,omitempty"`
}

// `Thu Nov 26 17:33:14 2026 daemon.err netgrip[24843]: 2026/11/26 16:33:14 ok`
// The timestamp is free text so a kernel line without the usual prefix still
// yields a message instead of being dropped.
var reLogLine = regexp.MustCompile(`^([A-Z][a-z]{2} [A-Z][a-z]{2} +\d+ \d{2}:\d{2}:\d{2} \d{4})\s+(\w+)\.(\w+)\s+([^:]+):\s?(.*)$`)

// parseDeviceLog parses logread output. Lines that do not match are returned
// with the whole text as the message, so nothing is silently lost.
func parseDeviceLog(out string) []DeviceLogEntry {
	entries := []DeviceLogEntry{}
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := reLogLine.FindStringSubmatch(line)
		if m == nil {
			entries = append(entries, DeviceLogEntry{Level: "unknown", Message: line})
			continue
		}
		entries = append(entries, DeviceLogEntry{
			Timestamp: m[1],
			Facility:  m[2],
			Level:     m[3],
			Tag:       strings.TrimSpace(m[4]),
			Message:   m[5],
		})
	}
	return entries
}

// filterDeviceLog keeps the entries whose tag or message contains the term,
// case-insensitively. It runs in Go on purpose: logread -e takes a regex, and
// whatever the caller types must not be one.
func filterDeviceLog(entries []DeviceLogEntry, term string) []DeviceLogEntry {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return entries
	}
	out := []DeviceLogEntry{}
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Tag), term) ||
			strings.Contains(strings.ToLower(e.Message), term) {
			out = append(out, e)
		}
	}
	return out
}

// tailDeviceLog keeps the last n entries.
func tailDeviceLog(entries []DeviceLogEntry, n int) []DeviceLogEntry {
	if n <= 0 || n >= len(entries) {
		return entries
	}
	return entries[len(entries)-n:]
}

// ProbeDeviceLogs reads the ring buffer and returns the last `lines` entries
// that match `filter`. Read-only: nothing is rotated or cleared from here.
func ProbeDeviceLogs(lines int, filter string) *DeviceLogs {
	if lines <= 0 {
		lines = 200
	}
	if lines > 1000 {
		lines = 1000
	}
	out, err := exec.Command("logread").Output()
	if err != nil {
		return &DeviceLogs{Entries: []DeviceLogEntry{}, Error: "logread failed: " + err.Error()}
	}
	entries := parseDeviceLog(string(out))
	total := len(entries)
	entries = filterDeviceLog(entries, filter)
	entries = tailDeviceLog(entries, lines)
	return &DeviceLogs{Entries: entries, Total: total}
}
