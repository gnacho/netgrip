package ubus

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/ubusconn"
)

// Call executes a ubus call and returns the raw JSON payload. It prefers the
// native client over the ubus socket (#461: ~3x faster than forking the CLI
// on mipsle) and falls back to `ubus call` whenever the socket is missing,
// the object is unknown, or the server answers with a non-zero status, so a
// regression in the native path can never break a read it used to serve.
// Set NETGRIP_UBUS_NATIVE=0 to force the CLI path.
func Call(object, method string) (json.RawMessage, error) {
	if os.Getenv("NETGRIP_UBUS_NATIVE") != "0" {
		if out, err := callNative(object, method); err == nil {
			return out, nil
		}
	}
	out, err := exec.Command("ubus", "call", object, method).Output()
	if err != nil {
		return nil, fmt.Errorf("ubus call %s %s: %w", object, method, err)
	}
	return json.RawMessage(out), nil
}

// callNative dials per call: the client is not concurrent-safe, callers are,
// and a per-call dial still measured ~3 ms on mipsle against ~12 ms for the
// CLI fork (see docs/ubus-client-spike.md).
func callNative(object, method string) (json.RawMessage, error) {
	c, err := ubusconn.Dial("")
	if err != nil {
		return nil, err
	}
	defer c.Close()
	res, err := c.Call(object, method, nil)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

// ListObjects returns all ubus objects with the given prefix (e.g. "hostapd.").
func ListObjects(prefix string) ([]string, error) {
	out, err := exec.Command("ubus", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("ubus list: %w", err)
	}
	var objects []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			objects = append(objects, line)
		}
	}
	return objects, nil
}

var macRe = regexp.MustCompile(`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)

// WirelessClient is a single associated station on a hostapd interface.
// Rx/Tx bytes are cumulative counters from the AP perspective: tx is
// client download, rx is client upload.
type WirelessClient struct {
	MAC     string `json:"mac"`
	Signal  int    `json:"signal,omitempty"`
	RxBytes int64  `json:"rx_bytes,omitempty"`
	TxBytes int64  `json:"tx_bytes,omitempty"`
}

// WirelessClients returns the associated stations per hostapd ubus object.
func WirelessClients() (map[string][]WirelessClient, error) {
	objects, err := ListObjects("hostapd.")
	if err != nil {
		return nil, err
	}
	result := make(map[string][]WirelessClient, len(objects))
	for _, obj := range objects {
		raw, err := Call(obj, "get_clients")
		if err != nil {
			continue
		}
		var payload struct {
			Clients map[string]struct {
				Signal int `json:"signal"`
				Bytes  struct {
					Rx int64 `json:"rx"`
					Tx int64 `json:"tx"`
				} `json:"bytes"`
			} `json:"clients"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}
		clients := make([]WirelessClient, 0, len(payload.Clients))
		for key, c := range payload.Clients {
			if !macRe.MatchString(key) {
				continue
			}
			clients = append(clients, WirelessClient{
				MAC:     key,
				Signal:  c.Signal,
				RxBytes: c.Bytes.Rx,
				TxBytes: c.Bytes.Tx,
			})
		}
		result[obj] = clients
	}
	return result, nil
}

// Lease is one entry of the dnsmasq leases file.
type Lease struct {
	Expires  time.Time `json:"expires"`
	MAC      string    `json:"mac"`
	IP       string    `json:"ip"`
	Hostname string    `json:"hostname"`
	ClientID string    `json:"client_id"`
}

// ReadLeases parses a dnsmasq leases file. Missing file is not an error:
// pure access points do not run dnsmasq.
func ReadLeases(path string) ([]Lease, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Lease{}, nil
		}
		return nil, fmt.Errorf("read leases: %w", err)
	}
	leases := ParseLeases(string(data))
	if leases == nil {
		return []Lease{}, nil
	}
	return leases, nil
}

// ParseLeases parses the content of a dnsmasq leases file:
// <expiry_epoch> <mac> <ip> <hostname> <client_id>
func ParseLeases(content string) []Lease {
	var leases []Lease
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		var epoch int64
		if _, err := fmt.Sscanf(fields[0], "%d", &epoch); err != nil {
			continue
		}
		lease := Lease{
			Expires:  time.Unix(epoch, 0),
			MAC:      fields[1],
			IP:       fields[2],
			Hostname: fields[3],
		}
		if len(fields) > 4 {
			lease.ClientID = fields[4]
		}
		leases = append(leases, lease)
	}
	return leases
}
