package modules

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// clientMetaPath is a var so tests can point it at a temp dir.
var clientMetaPath = "/etc/netgrip/clients.json"

// ClientMeta is user-assigned metadata for one client (its MAC).
type ClientMeta struct {
	Name       string `json:"name,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
}

var (
	clientMetaMu   sync.Mutex
	clientMetaData map[string]ClientMeta
)

// clientMeta reads a COPY of the persisted metadata map (MAC -> metadata).
// Callers get a snapshot: never hand out the live map (readers would race
// with writers).
func clientMeta() map[string]ClientMeta {
	clientMetaMu.Lock()
	defer clientMetaMu.Unlock()
	loadClientMetaLocked()
	out := make(map[string]ClientMeta, len(clientMetaData))
	for k, v := range clientMetaData {
		out[k] = v
	}
	return out
}

// loadClientMetaLocked loads the map from disk on first use. Caller MUST hold
// clientMetaMu.
func loadClientMetaLocked() {
	if clientMetaData != nil {
		return
	}
	clientMetaData = map[string]ClientMeta{}
	if data, err := os.ReadFile(clientMetaPath); err == nil {
		_ = json.Unmarshal(data, &clientMetaData)
	}
	if clientMetaData == nil {
		clientMetaData = map[string]ClientMeta{}
	}
}

// saveClientMeta persists the metadata map to disk. Caller MUST hold
// clientMetaMu (reads clientMetaData without locking).
func saveClientMeta() error {
	data, err := json.MarshalIndent(clientMetaData, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll("/etc/netgrip", 0o750)
	return os.WriteFile(clientMetaPath, data, 0o600)
}

// clientMetaPayload is the response shape for GET /api/clients/meta.
type clientMetaPayload struct {
	Meta map[string]ClientMeta `json:"meta"`
}

// GetClientMeta returns all user-assigned client metadata.
func GetClientMeta() clientMetaPayload {
	return clientMetaPayload{Meta: clientMeta()}
}

// SetClientMeta assigns a custom name and/or device type to a client MAC and
// persists it. Returns the updated metadata map. Self-deadlock fixed
// (29-Ago-2026): the old version called GetClientMeta() while holding
// clientMetaMu and hung every POST /api/clients/meta.
func SetClientMeta(mac, name, deviceType string) (clientMetaPayload, error) {
	mac = normalizeMac(mac)
	if mac == "" {
		return clientMetaPayload{}, os.ErrInvalid
	}
	clientMetaMu.Lock()
	defer clientMetaMu.Unlock()
	loadClientMetaLocked()
	clientMetaData[mac] = ClientMeta{Name: name, DeviceType: deviceType}
	if err := saveClientMeta(); err != nil {
		return clientMetaPayload{}, err
	}
	out := make(map[string]ClientMeta, len(clientMetaData))
	for k, v := range clientMetaData {
		out[k] = v
	}
	return clientMetaPayload{Meta: out}, nil
}

// normalizeMac lowercases and trims a MAC, returning "" if not a valid shape.
func normalizeMac(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if !reMac.MatchString(mac) {
		return ""
	}
	return mac
}
