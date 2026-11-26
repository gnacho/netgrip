package modules

import (
	"encoding/json"
	"strings"
	"testing"
)

const logFixture = `Thu Nov 26 17:33:14 2026 daemon.err netgrip[24843]: 2026/11/26 16:33:14 netgrip v0.82.0 listening on http://0.0.0.0:8090
Thu Nov 26 17:33:15 2026 daemon.info dnsmasq[1]: 149465 127.0.0.1/38951 query[A] example.org from 127.0.0.1
Thu Nov 26 17:33:16 2026 kern.warn kernel: [12345.678] br-lan: received packet on lan1 with own address as source address
this line has no logread prefix at all`

func TestParseDeviceLogKeepsEveryLine(t *testing.T) {
	entries := parseDeviceLog(logFixture)
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(entries))
	}
	first := entries[0]
	if first.Facility != "daemon" || first.Level != "err" || !strings.HasPrefix(first.Tag, "netgrip") {
		t.Fatalf("first entry parsed wrong: %+v", first)
	}
	if !strings.Contains(first.Message, "listening on") {
		t.Fatalf("message lost: %+v", first)
	}
	if entries[2].Facility != "kern" || entries[2].Level != "warn" {
		t.Fatalf("kernel line parsed wrong: %+v", entries[2])
	}
	// A line without the prefix is kept as the message, not dropped.
	if entries[3].Level != "unknown" || entries[3].Message == "" {
		t.Fatalf("unprefixed line lost: %+v", entries[3])
	}
}

func TestFilterDeviceLogIsCaseInsensitive(t *testing.T) {
	entries := parseDeviceLog(logFixture)
	got := filterDeviceLog(entries, "DNSMASQ")
	if len(got) != 1 || got[0].Tag != "dnsmasq[1]" {
		t.Fatalf("filter returned %+v", got)
	}
	if len(filterDeviceLog(entries, "")) != 4 {
		t.Fatal("an empty filter must keep everything")
	}
	// A regex-looking term is just text, never a pattern.
	if len(filterDeviceLog(entries, ".*")) != 0 {
		t.Fatal("the filter must not be treated as a regular expression")
	}
}

func TestTailDeviceLog(t *testing.T) {
	entries := parseDeviceLog(logFixture)
	if got := tailDeviceLog(entries, 2); len(got) != 2 || got[1].Message != entries[3].Message {
		t.Fatalf("tail = %+v", got)
	}
	if got := tailDeviceLog(entries, 0); len(got) != 4 {
		t.Fatalf("tail with 0 must keep everything, got %d", len(got))
	}
	if got := tailDeviceLog(entries, 99); len(got) != 4 {
		t.Fatalf("tail bigger than the list must keep everything, got %d", len(got))
	}
}

// The payload never carries a null list: the app reads .length from it.
func TestDeviceLogsSerialisesEmptyEntries(t *testing.T) {
	raw, err := json.Marshal(&DeviceLogs{Entries: parseDeviceLog("")})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") {
		t.Fatalf("payload carries null: %s", raw)
	}
}
