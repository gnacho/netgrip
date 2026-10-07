package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gnacho/netgrip/internal/modules"
	"github.com/gnacho/netgrip/internal/ubus"
)

// Phase 4 of plans/backend-seam-clients, closing #462's clients migration:
// GET /api/clients (behind requireAuth, inside the probe cache) runs with
// every device read faked - no ubus, no uci, no ssh, no sysfs - and
// returns the assembled clients payload.
func TestClientsHandlerWithSeams(t *testing.T) {
	dir := t.TempDir()
	leaseFile := filepath.Join(dir, "dhcp.leases")
	if err := os.WriteFile(leaseFile, []byte("0 aa:bb:cc:00:00:01 192.0.2.10 portatil 01\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	arpFile := filepath.Join(dir, "arp")
	if err := os.WriteFile(arpFile, []byte(
		"IP address       HW type     Flags       HW address            Mask     Device\n"+
			"192.0.2.10       0x1         0x2         aa:bb:cc:00:00:01     *        br-lan\n"+
			"192.0.2.20       0x1         0x2         aa:bb:cc:00:00:02     *        br-lan\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	fake := &fakeWirelessBackend{radios: []ubus.WirelessRadio{{
		Name: "radio0", Band: "5g", Up: true,
		Interfaces: []ubus.WirelessInterface{{
			Ifname: "phy0-ap0",
			Clients: []ubus.WirelessClient{
				{MAC: "AA:BB:CC:00:00:01", Signal: -50, RxBytes: 100, TxBytes: 200},
			},
		}},
	}}}
	ubus.SetBackendForTest(t, fake)

	modules.SetUciExecForTest(t, func(args ...string) ([]byte, error) {
		if len(args) == 2 && args[0] == "show" && args[1] == "dhcp" {
			return []byte("dhcp.host1=host\ndhcp.host1.name='reservado'\ndhcp.host1.mac='aa:bb:cc:00:00:02'\ndhcp.host1.ip='192.0.2.20'\n"), nil
		}
		if len(args) == 2 && args[0] == "show" && args[1] == "wireless" {
			return []byte("wireless.default_radio0=wifi-device\n"), nil
		}
		return nil, errors.New("unexpected uci call")
	})
	modules.SetGatewaySSHForTest(t, func(command string) (string, error) {
		return "", errors.New("no gateway in test")
	})
	modules.InvalidateKey("ucishow|dhcp")
	modules.InvalidateKey("ucishow|wireless")
	modules.InvalidateKey("clients|192.0.2.1") // cold cache for httptest's RemoteAddr

	modules.SetClientsSeamsForTest(t, modules.ClientsSeams{
		LeaseFile:           leaseFile,
		ArpFile:             arpFile,
		BridgeFdb:           func() map[string][]string { return map[string][]string{"lan1": {"AA:BB:CC:00:00:02"}} },
		ServiceEnabled:      func(name string) bool { return name == "firewall" },
		ProbeMode:           func() *modules.ModeProbe { return &modules.ModeProbe{Mode: "router"} },
		RouterClock:         func() (time.Time, bool) { return time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), true },
		ParentalPath:        filepath.Join(dir, "parental.json"),
		ParentalAppliedPath: filepath.Join(dir, "parental-applied.json"),
		CableBlocked:        func() map[string]bool { return map[string]bool{} },
		ClientMetaPath:      filepath.Join(dir, "clients.json"),
		QuotaConfigFile:     filepath.Join(dir, "quota.json"),
		QuotaStateFile:      filepath.Join(dir, "quota-state.json"),
	})

	s := New("http://127.0.0.1/ubus", "test", false, "")
	rec := authedRequest(t, s, "GET", "/api/clients", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/clients: %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Clients []clientView `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not the clients payload: %v", err)
	}
	if len(payload.Clients) != 2 {
		t.Fatalf("clients = %d, want 2: %s", len(payload.Clients), rec.Body.String())
	}
	byMAC := map[string]clientView{}
	for _, c := range payload.Clients {
		byMAC[c.MAC] = c
	}
	wifi := byMAC["aa:bb:cc:00:00:01"]
	if wifi.Type != "wifi5" || wifi.Name != "portatil" || wifi.IP != "192.0.2.10" || !wifi.Blockable {
		t.Errorf("wifi client = %+v", wifi)
	}
	// Self compares against the requester IP; httptest's RemoteAddr
	// (192.0.2.1) matches no client here. The self flag itself is
	// covered by the module-level seam test with an explicit requester.
	if wifi.Self {
		t.Errorf("no client should be self for httptest's RemoteAddr")
	}
	cable := byMAC["aa:bb:cc:00:00:02"]
	if cable.Type != "cable" || cable.Iface != "lan1" || cable.Name != "reservado" || !cable.Reserved || cable.Self {
		t.Errorf("wired client = %+v", cable)
	}
}

type clientView struct {
	MAC       string `json:"mac"`
	Type      string `json:"type"`
	Iface     string `json:"iface"`
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Self      bool   `json:"self"`
	Reserved  bool   `json:"reserved"`
	Blockable bool   `json:"blockable"`
}
