package modules

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

const clientMetaPath = "/etc/netgrip/clients.json"

// ClientMeta is user-assigned metadata for one client (its MAC).
type ClientMeta struct {
	Name       string `json:"name,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
}

var (
	clientMetaMu   sync.Mutex
	clientMetaData map[string]ClientMeta
)

// clientMeta reads the persisted metadata map (MAC -> metadata), once.
func clientMeta() map[string]ClientMeta {
	clientMetaMu.Lock()
	defer clientMetaMu.Unlock()
	if clientMetaData != nil {
		return clientMetaData
	}
	clientMetaData = map[string]ClientMeta{}
	if data, err := os.ReadFile(clientMetaPath); err == nil {
		_ = json.Unmarshal(data, &clientMetaData)
	}
	if clientMetaData == nil {
		clientMetaData = map[string]ClientMeta{}
	}
	return clientMetaData
}

// saveClientMeta persists the metadata map to disk.
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
// persists it. Returns the updated metadata map.
func SetClientMeta(mac, name, deviceType string) (clientMetaPayload, error) {
	mac = normalizeMac(mac)
	if mac == "" {
		return clientMetaPayload{}, os.ErrInvalid
	}
	mu := clientMeta()
	mu[mac] = ClientMeta{Name: name, DeviceType: deviceType}
	clientMetaMu.Lock()
	defer clientMetaMu.Unlock()
	if err := saveClientMeta(); err != nil {
		return clientMetaPayload{}, err
	}
	return GetClientMeta(), nil
}

// normalizeMac lowercases and trims a MAC, returning "" if not a valid shape.
func normalizeMac(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if !reMac.MatchString(mac) {
		return ""
	}
	return mac
}
