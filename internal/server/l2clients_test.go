package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gnacho/netgrip/internal/modules"
)

// GET /api/l2clients (behind requireAuth, inside the probe cache) returns the
// L2 inventory at the root, without touching real netlink. It is the switch
// counterpart to /api/clients (#520).
func TestL2ClientsHandlerWithSeam(t *testing.T) {
	modules.SetL2ClientsSeamsForTest(t, func() *modules.L2Probe {
		return &modules.L2Probe{
			Applicable: true,
			Bridge:     "br-lan",
			Ports:      []string{"lan1", "lan2"},
			Clients: []modules.L2Client{
				{MAC: "aa:bb:cc:00:00:01", Port: "lan1", Vlans: []int{10, 20}},
			},
		}
	})
	modules.InvalidateKey("l2clients")

	s := New("http://127.0.0.1/ubus", "test", false, "")
	rec := authedRequest(t, s, "GET", "/api/l2clients", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/l2clients: %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var probe struct {
		Applicable bool               `json:"applicable"`
		Bridge     string             `json:"bridge"`
		Clients    []modules.L2Client `json:"clients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &probe); err != nil {
		t.Fatalf("response is not the l2clients payload: %v", err)
	}
	if !probe.Applicable || probe.Bridge != "br-lan" {
		t.Fatalf("probe = %+v, want applicable br-lan", probe)
	}
	if len(probe.Clients) != 1 || probe.Clients[0].Port != "lan1" {
		t.Fatalf("clients = %v, want one client on lan1", probe.Clients)
	}
}
