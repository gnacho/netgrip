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

// clientMetaMu serialises writes; reads re-parse the JSON on every call
// so manual edits to the file take effect without a service restart
// (#165). The JSON is tiny (one entry per renamed client) so the cost
// is negligible.
var clientMetaMu sync.Mutex

// clientMeta reads the metadata map (MAC -> metadata) fresh from disk.
func clientMeta() map[string]ClientMeta {
	clientMetaMu.Lock()
	defer clientMetaMu.Unlock()
	return readClientMetaLocked()
}

// readClientMetaLocked parses the JSON on disk into a new map. Caller
// MUST hold clientMetaMu (the write path mutates the same file).
func readClientMetaLocked() map[string]ClientMeta {
	out := map[string]ClientMeta{}
	if data, err := os.ReadFile(clientMetaPath); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

// writeClientMeta persists the metadata map to disk. An empty map
// removes the file so a full reset is observable on disk. Caller MUST
// hold clientMetaMu.
func writeClientMeta(data map[string]ClientMeta) error {
	_ = os.MkdirAll("/etc/netgrip", 0o750)
	if len(data) == 0 {
		_ = os.Remove(clientMetaPath)
		return nil
	}
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(clientMetaPath, buf, 0o600)
}

// clientMetaPayload is the response shape for GET /api/clients/meta.
type clientMetaPayload struct {
	Meta map[string]ClientMeta `json:"meta"`
}

// GetClientMeta returns all user-assigned client metadata.
func GetClientMeta() clientMetaPayload {
	return clientMetaPayload{Meta: clientMeta()}
}

// SetClientMeta assigns a custom name and/or device type to a client
// MAC and persists it. If both fields are empty the entry is removed
// entirely (#165). Returns the updated metadata map.
func SetClientMeta(mac, name, deviceType string) (clientMetaPayload, error) {
	mac = normalizeMac(mac)
	if mac == "" {
		return clientMetaPayload{}, os.ErrInvalid
	}
	clientMetaMu.Lock()
	defer clientMetaMu.Unlock()
	data := readClientMetaLocked()
	if name == "" && deviceType == "" {
		delete(data, mac)
	} else {
		data[mac] = ClientMeta{Name: name, DeviceType: deviceType}
	}
	if err := writeClientMeta(data); err != nil {
		return clientMetaPayload{}, err
	}
	return clientMetaPayload{Meta: data}, nil
}

// normalizeMac lowercases and trims a MAC, returning "" if not a valid shape.
func normalizeMac(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if !reMac.MatchString(mac) {
		return ""
	}
	return mac
}
