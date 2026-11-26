package modules

import (
	"encoding/json"
	"strings"
	"testing"
)

// A kernel without CONFIG_NF_CONNTRACK_PROCFS has no conntrack file, and the
// probe is not applicable there. The payload still has to carry an empty
// protocols array: a nil slice becomes null and the interface reads .length
// from it, which took the whole Internet page down (#661).
func TestEmptyDPIProbeSerialisesEmptyProtocols(t *testing.T) {
	raw, err := json.Marshal(emptyDPIProbe())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "null") {
		t.Fatalf("probe serialises a null slice: %s", raw)
	}
	if !strings.Contains(string(raw), `"protocols":[]`) {
		t.Fatalf("protocols is not an empty array: %s", raw)
	}
}

func TestProbeDPIFromEmptyReaderKeepsProtocolsEmpty(t *testing.T) {
	probe := probeDPIFrom(strings.NewReader(""))
	if probe.Protocols == nil {
		t.Fatal("protocols is nil for an empty conntrack table")
	}
	if len(probe.Protocols) != 0 {
		t.Fatalf("expected no protocols, got %d", len(probe.Protocols))
	}
	if !probe.Applicable {
		t.Fatal("an empty conntrack table is still applicable")
	}
}

func TestProbeDPIFromCountsEveryFlowOnce(t *testing.T) {
	table := strings.Join([]string{
		"ipv4     2 tcp      6 431999 ESTABLISHED src=192.168.1.10 dst=142.250.1.1 sport=50100 dport=443 src=142.250.1.1 dst=192.168.1.10 sport=443 dport=50100 [ASSURED] mark=0 use=1 bytes=1000",
		"ipv4     2 udp     17 29 src=192.168.1.10 dst=8.8.8.8 sport=40000 dport=53 src=8.8.8.8 dst=192.168.1.10 sport=53 dport=40000 mark=0 use=1 bytes=200",
	}, "\n")

	probe := probeDPIFrom(strings.NewReader(table))
	if probe.TotalFlows != 2 {
		t.Fatalf("expected 2 flows, got %d", probe.TotalFlows)
	}
	if probe.TotalBytes != 1200 {
		t.Fatalf("expected 1200 bytes, got %d", probe.TotalBytes)
	}
	if len(probe.Protocols) != 2 {
		t.Fatalf("expected 2 protocols, got %d", len(probe.Protocols))
	}
}
