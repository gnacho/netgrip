package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// Un perfil de cliente WireGuard solo se puede volver a pintar si se guarda
// su clave privada: sin ella el QR no se puede reconstruir. Se guarda en un
// fichero 0600 (igual que UCI guarda la clave del servidor) y solo se
// devuelve por la ruta autenticada que pide el QR (#754).

var wgClientsPath = "/etc/netgrip/wg-clients.json"

// ErrNoStoredKey means the peer exists but its private key was never saved
// (it was added with a pasted public key, or created before this feature).
var ErrNoStoredKey = errors.New("no stored private key for this peer")

type storedWGClient struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	CreatedAt  int64  `json:"created_at"`
}

type wgClientsFile struct {
	Clients []storedWGClient `json:"clients"`
}

func readWGClients() wgClientsFile {
	var f wgClientsFile
	data, err := os.ReadFile(wgClientsPath)
	if err != nil {
		return f
	}
	_ = json.Unmarshal(data, &f)
	return f
}

func writeWGClients(f wgClientsFile) error {
	if err := os.MkdirAll(filepath.Dir(wgClientsPath), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp := wgClientsPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, wgClientsPath)
}

// SaveWGClientPrivate stores the private key of a client profile.
func SaveWGClientPrivate(pub, priv string) error {
	if pub == "" || priv == "" {
		return fmt.Errorf("empty key")
	}
	f := readWGClients()
	for i := range f.Clients {
		if f.Clients[i].PublicKey == pub {
			f.Clients[i].PrivateKey = priv
			return writeWGClients(f)
		}
	}
	f.Clients = append(f.Clients, storedWGClient{PublicKey: pub, PrivateKey: priv, CreatedAt: time.Now().Unix()})
	return writeWGClients(f)
}

// ForgetWGClient drops the stored key of a removed peer.
func ForgetWGClient(pub string) {
	f := readWGClients()
	kept := f.Clients[:0]
	for _, c := range f.Clients {
		if c.PublicKey != pub {
			kept = append(kept, c)
		}
	}
	f.Clients = kept
	_ = writeWGClients(f)
}

func storedWGPrivate(pub string) string {
	for _, c := range readWGClients().Clients {
		if c.PublicKey == pub {
			return c.PrivateKey
		}
	}
	return ""
}

// wgClientConfigForPeer renders the profile of an existing peer from its
// stored private key and the live server data.
func wgClientConfigForPeer(pub string) (string, error) {
	priv := storedWGPrivate(pub)
	if priv == "" {
		return "", ErrNoStoredKey
	}
	probe := ProbeWG()
	var peer *WGPeer
	for i := range probe.Peers {
		if probe.Peers[i].PublicKey == pub {
			peer = &probe.Peers[i]
			break
		}
	}
	if peer == nil {
		return "", fmt.Errorf("peer not found")
	}
	address := ""
	if len(peer.AllowedIPs) > 0 {
		address = peer.AllowedIPs[0]
	}
	endpoint := storedWGEndpoint()
	if endpoint == "" {
		endpoint = wanIPv4()
	}
	if endpoint == "" {
		endpoint = lanIPv4()
	}
	return buildWGClientConfig(wgClientConfig{
		PrivateKey:   priv,
		Address:      address,
		DNS:          wgClientDNS(probe),
		ServerKey:    probe.PublicKey,
		PresharedKey: uciGet("network." + peer.Section + ".preshared_key"),
		Endpoint:     endpoint,
		Port:         probe.Port,
	}), nil
}

// WGPeerConfig is the exported entry used by the API to rebuild a QR.
func WGPeerConfig(pub string) (string, error) {
	return wgClientConfigForPeer(pub)
}

// RenameWGPeer changes the label of a peer. An empty name clears it.
func RenameWGPeer(pub, name string) (*WGProbe, bool, error) {
	probe := ProbeWG()
	if probe.ManagedBy == wgManagedGL {
		return probe, false, wgGLManagedError
	}
	var peer *WGPeer
	for i := range probe.Peers {
		if probe.Peers[i].PublicKey == pub {
			peer = &probe.Peers[i]
			break
		}
	}
	if peer == nil {
		return probe, false, fmt.Errorf("peer not found")
	}
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return probe, false, err
	}
	base := "network." + peer.Section
	name = strings.TrimSpace(name)
	var ops []executor.Op
	if name == "" {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{base + ".description"}})
	} else {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".description", name}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)
	rolledBack, err := wgApply(ops, snapNetwork, "", func() bool {
		for _, p := range wgPeers() {
			if p.PublicKey == pub && p.Name == name {
				return true
			}
		}
		return false
	})
	return ProbeWG(), rolledBack, err
}

// RotateWGPeer gives the peer a new keypair: the only way to hand out its QR
// when the original private key was never stored. The old device stops
// connecting until it imports the new profile.
func RotateWGPeer(pub string) (string, *WGProbe, error) {
	probe := ProbeWG()
	if probe.ManagedBy == wgManagedGL {
		return "", probe, wgGLManagedError
	}
	var peer *WGPeer
	for i := range probe.Peers {
		if probe.Peers[i].PublicKey == pub {
			peer = &probe.Peers[i]
			break
		}
	}
	if peer == nil {
		return "", probe, fmt.Errorf("peer not found")
	}
	privOut, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", probe, fmt.Errorf("wg genkey: %w", err)
	}
	priv := strings.TrimSpace(string(privOut))
	newPub := wgPublicKey(priv)
	if newPub == "" {
		return "", probe, fmt.Errorf("wg pubkey failed")
	}
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return "", probe, err
	}
	base := "network." + peer.Section
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{base + ".public_key", newPub}},
		{Kind: "uci_commit", Args: []string{"network"}},
		{Kind: "initd", Args: []string{"network", "reload"}},
	}
	rolledBack, err := wgApply(ops, snapNetwork, "", func() bool {
		for _, p := range wgPeers() {
			if p.PublicKey == newPub {
				return true
			}
		}
		return false
	})
	if err != nil || rolledBack {
		return "", ProbeWG(), err
	}
	if err := SaveWGClientPrivate(newPub, priv); err != nil {
		return "", ProbeWG(), err
	}
	ForgetWGClient(pub)
	cfg, err := wgClientConfigForPeer(newPub)
	return cfg, ProbeWG(), err
}
