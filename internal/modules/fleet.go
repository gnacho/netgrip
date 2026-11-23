package modules

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	fleetConfigPath        = "/etc/netgrip/fleet.json"
	legacyFleetConfigPath  = "/etc/owpanel/fleet.json"
)

type FleetNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Password string `json:"password,omitempty"`
}

type FleetConfig struct {
	Nodes []FleetNode `json:"nodes"`
}

type FleetNodeStatus struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Address         string `json:"address"`
	Reachable       bool   `json:"reachable"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	Error           string `json:"error,omitempty"`
}

type FleetStatus struct {
	Nodes []FleetNodeStatus `json:"nodes"`
}

var (
	fleetMu       sync.RWMutex
	fleetStatuses = make(map[string]FleetNodeStatus)
)

func LoadFleetConfig() (FleetConfig, error) {
	data, err := os.ReadFile(fleetConfigPath)
	if os.IsNotExist(err) && fileExists(legacyFleetConfigPath) {
		// Migrate a pre-rename fleet file if present (one shot, best effort).
		if legacy, lerr := os.ReadFile(legacyFleetConfigPath); lerr == nil {
			if merr := SaveFleetConfigFromBytes(legacy); merr == nil {
				_ = os.Remove(legacyFleetConfigPath)
			}
			data = legacy
			err = nil
		}
	}
	if err != nil {
		if os.IsNotExist(err) {
			return FleetConfig{Nodes: []FleetNode{}}, nil
		}
		return FleetConfig{}, err
	}
	var cfg FleetConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return FleetConfig{}, err
	}
	return cfg, nil
}

func SaveFleetConfig(cfg FleetConfig) error {
	if err := os.MkdirAll("/etc/netgrip", 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return SaveFleetConfigFromBytes(data)
}

// SaveFleetConfigFromBytes writes raw fleet JSON to the current path.
func SaveFleetConfigFromBytes(data []byte) error {
	return os.WriteFile(fleetConfigPath, data, 0600)
}

func AddFleetNode(node FleetNode) error {
	cfg, err := LoadFleetConfig()
	if err != nil {
		return err
	}
	for _, n := range cfg.Nodes {
		if n.ID == node.ID {
			return fmt.Errorf("node already exists")
		}
	}
	cfg.Nodes = append(cfg.Nodes, node)
	return SaveFleetConfig(cfg)
}

func RemoveFleetNode(id string) error {
	cfg, err := LoadFleetConfig()
	if err != nil {
		return err
	}
	var filtered []FleetNode
	for _, n := range cfg.Nodes {
		if n.ID != id {
			filtered = append(filtered, n)
		}
	}
	cfg.Nodes = filtered
	return SaveFleetConfig(cfg)
}

func ListFleet() ([]FleetNodeStatus, error) {
	cfg, err := LoadFleetConfig()
	if err != nil {
		return nil, err
	}

	fleetMu.RLock()
	nodes := make([]FleetNodeStatus, 0, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		status, ok := fleetStatuses[n.ID]
		if !ok {
			status = FleetNodeStatus{
				ID:      n.ID,
				Name:    n.Name,
				Address: n.Address,
			}
		}
		nodes = append(nodes, status)
	}
	fleetMu.RUnlock()

	return nodes, nil
}

func CheckFleetNode(id string) (FleetNodeStatus, error) {
	cfg, err := LoadFleetConfig()
	if err != nil {
		return FleetNodeStatus{}, err
	}

	var node *FleetNode
	for i := range cfg.Nodes {
		if cfg.Nodes[i].ID == id {
			node = &cfg.Nodes[i]
			break
		}
	}
	if node == nil {
		return FleetNodeStatus{}, fmt.Errorf("node not found")
	}

	status := checkNodeUpdate(*node)

	fleetMu.Lock()
	fleetStatuses[id] = status
	fleetMu.Unlock()

	return status, nil
}

func CheckAllFleet() ([]FleetNodeStatus, error) {
	cfg, err := LoadFleetConfig()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	results := make(chan FleetNodeStatus, len(cfg.Nodes))

	for _, node := range cfg.Nodes {
		wg.Add(1)
		go func(n FleetNode) {
			defer wg.Done()
			status := checkNodeUpdate(n)
			results <- status
		}(node)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	fleetMu.Lock()
	nodes := []FleetNodeStatus{}
	for status := range results {
		fleetStatuses[status.ID] = status
		nodes = append(nodes, status)
	}
	fleetMu.Unlock()

	return nodes, nil
}

// fleetHost strips any scheme the user typed: the address is host[:port].
func fleetHost(address string) string {
	address = strings.TrimSpace(address)
	address = strings.TrimPrefix(address, "https://")
	address = strings.TrimPrefix(address, "http://")
	return strings.TrimSuffix(address, "/")
}

// fleetLogin opens a session against a peer panel and returns a client that
// carries it.
//
// El panel NO responde con un token: valida la contraseña contra el router y
// contesta 204 con una cookie de sesión. Esperar un 200 con {"token"} hacía
// que cualquier adopción fallara con "login failed" aunque la contraseña
// fuera correcta (#636). Se prueba HTTPS primero porque muchos equipos solo
// sirven ahí y el certificado es autofirmado por diseño; si no hay TLS, HTTP.
func fleetLogin(address, password string, timeout time.Duration) (*http.Client, string, error) {
	host := fleetHost(address)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, "", err
	}
	insecureTLS := &http.Client{
		Timeout: timeout,
		Jar:     jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // autofirmado por diseño
		},
	}
	plain := &http.Client{Timeout: timeout, Jar: jar}

	attempts := []struct {
		base   string
		client *http.Client
	}{
		{"https://" + host, insecureTLS},
		{"http://" + host, plain},
	}
	var lastErr error
	for _, a := range attempts {
		body, _ := json.Marshal(map[string]string{"password": password})
		resp, err := a.client.Post(a.base+"/api/login", "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK, http.StatusNoContent:
			return a.client, a.base, nil
		case http.StatusUnauthorized:
			return nil, "", fmt.Errorf("login failed: wrong root password")
		case http.StatusBadGateway:
			return nil, "", fmt.Errorf("login failed: the peer cannot validate passwords (rpcd unreachable)")
		}
		lastErr = fmt.Errorf("%s answered HTTP %d on login", a.base, resp.StatusCode)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unreachable")
	}
	return nil, "", lastErr
}

func checkNodeUpdate(node FleetNode) FleetNodeStatus {
	status := FleetNodeStatus{
		ID:      node.ID,
		Name:    node.Name,
		Address: node.Address,
	}

	client, base, err := fleetLogin(node.Address, node.Password, 5*time.Second)
	if err != nil {
		status.Error = err.Error()
		return status
	}

	checkResp, err := client.Get(base + "/api/selfupdate")
	if err != nil {
		status.Error = fmt.Sprintf("check: %v", err)
		return status
	}
	defer checkResp.Body.Close()

	if checkResp.StatusCode != 200 {
		status.Error = "check failed"
		return status
	}

	var update SelfUpdateCheck
	if err := json.NewDecoder(checkResp.Body).Decode(&update); err != nil {
		status.Error = "check decode"
		return status
	}

	status.Reachable = true
	status.CurrentVersion = update.Current
	status.LatestVersion = update.Latest
	status.UpdateAvailable = update.Available

	return status
}

func UpdateFleetNode(id string) error {
	cfg, err := LoadFleetConfig()
	if err != nil {
		return err
	}

	var node *FleetNode
	for i := range cfg.Nodes {
		if cfg.Nodes[i].ID == id {
			node = &cfg.Nodes[i]
			break
		}
	}
	if node == nil {
		return fmt.Errorf("node not found")
	}

	client, base, err := fleetLogin(node.Address, node.Password, 10*time.Second)
	if err != nil {
		return err
	}

	updateResp, err := client.Post(base+"/api/selfupdate", "application/json", nil)
	if err != nil {
		return fmt.Errorf("update: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != 200 {
		return fmt.Errorf("update failed: HTTP %d", updateResp.StatusCode)
	}

	return nil
}
