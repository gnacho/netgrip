package modules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

func checkNodeUpdate(node FleetNode) FleetNodeStatus {
	status := FleetNodeStatus{
		ID:      node.ID,
		Name:    node.Name,
		Address: node.Address,
	}

	client := &http.Client{Timeout: 5 * time.Second}

	loginReq := map[string]string{"password": node.Password}
	loginBody, _ := json.Marshal(loginReq)
	loginResp, err := client.Post(fmt.Sprintf("http://%s/api/login", node.Address), "application/json", bytes.NewReader(loginBody))
	if err != nil {
		status.Error = fmt.Sprintf("login: %v", err)
		return status
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != 200 {
		status.Error = "login failed"
		return status
	}

	var loginResult struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		status.Error = "login decode"
		return status
	}

	checkReq, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/api/selfupdate", node.Address), nil)
	checkReq.Header.Set("Authorization", "Bearer "+loginResult.Token)
	checkResp, err := client.Do(checkReq)
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

	client := &http.Client{Timeout: 10 * time.Second}

	loginReq := map[string]string{"password": node.Password}
	loginBody, _ := json.Marshal(loginReq)
	loginResp, err := client.Post(fmt.Sprintf("http://%s/api/login", node.Address), "application/json", bytes.NewReader(loginBody))
	if err != nil {
		return fmt.Errorf("login: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != 200 {
		return fmt.Errorf("login failed")
	}

	var loginResult struct {
		Token string `json:"token"`
	}
	body, _ := io.ReadAll(loginResp.Body)
	if err := json.Unmarshal(body, &loginResult); err != nil {
		return fmt.Errorf("login decode")
	}

	updateReq, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/api/selfupdate", node.Address), nil)
	updateReq.Header.Set("Authorization", "Bearer "+loginResult.Token)
	updateResp, err := client.Do(updateReq)
	if err != nil {
		return fmt.Errorf("update: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != 200 {
		return fmt.Errorf("update failed: HTTP %d", updateResp.StatusCode)
	}

	return nil
}
