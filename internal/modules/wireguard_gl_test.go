package modules

import (
	"encoding/json"
	"strings"
	"testing"
)

// Real `uci show network` shape captured on the Flint2 (GL 4.9.1-op25):
// a GL wgserver interface plus a standard wg0 that must be ignored.
const wgGLNetworkFixture = `network.lan=interface
network.lan.device='br-lan'
network.lan.proto='static'
network.wgserver=interface
network.wgserver.proto='wgserver'
network.wgserver.config='main_server'
network.wgserver.disabled='0'
network.wg0=interface
network.wg0.proto='wireguard'
network.wg0.private_key='8HsDWqXe+LMWaP3XyPfLEtBgGnULCd02/8A6QT5xkV4='
network.wg0.listen_port='51820'
network.wg0.addresses='10.66.0.1/24'
`

// Real `uci show wireguard_server` shape captured on the same Flint2:
// one servers section plus peers (renamed count, one deprecated peer added
// to cover the GL soft-delete filter). Private keys sit next to the public
// ones and must never reach the API.
const wgGLServerFixture = `wireguard_server.main_server=servers
wireguard_server.main_server.address_v4='10.1.0.1/24'
wireguard_server.main_server.port='59999'
wireguard_server.main_server.access='ACCEPT'
wireguard_server.main_server.client_to_client='0'
wireguard_server.main_server.masq='1'
wireguard_server.main_server.address_v6='fd00:1e84:8f70:af74::1/64'
wireguard_server.main_server.private_key='AOjat21QKy+OBbB8jRdNfbFerpS0lXJFr8RqW0ICglw='
wireguard_server.main_server.public_key='fVOXQebJWhXsIHb/E9Rt2VqoHhIDiNx1xxGGxqA6R0o='
wireguard_server.peer_3087=peers
wireguard_server.peer_3087.name='nacho-movil'
wireguard_server.peer_3087.peer_id='3087'
wireguard_server.peer_3087.presharedkey_enable='0'
wireguard_server.peer_3087.dns='10.1.0.1,fd00:1e84:8f70:af74::1,64.6.64.6'
wireguard_server.peer_3087.allowed_ips='0.0.0.0/0, ::/0'
wireguard_server.peer_3087.mtu='1420'
wireguard_server.peer_3087.persistent_keepalive='25'
wireguard_server.peer_3087.public_key='2NNHZIbUfQKzq7I9AqXAr5mEgJy0fyCx7Td54G6g1Vc='
wireguard_server.peer_3087.private_key='MOX0kFhNYQwqegTPfjC+RKGbwMcuiZzwkFUkZn4t0E4='
wireguard_server.peer_3087.client_ip='10.1.0.2/24,fd00:1e84:8f70:af74::2/64'
wireguard_server.peer_3087.deprecated='0'
wireguard_server.peer_7665=peers
wireguard_server.peer_7665.name='ana-movil'
wireguard_server.peer_7665.peer_id='7665'
wireguard_server.peer_7665.public_key='RdCMMffqmUA9knY0Hbk2PfAXfe1G3gk0YH9O9axp2w8='
wireguard_server.peer_7665.private_key='iN5URtoOwY9a9COSubGEOK6c09WPfSsW5Rh0zBpqcU8='
wireguard_server.peer_7665.client_ip='10.1.0.3/24,fd00:1e84:8f70:af74::3/64'
wireguard_server.peer_7665.allowed_ips='0.0.0.0/0,::/0'
wireguard_server.peer_7665.deprecated='0'
wireguard_server.peer_9999=peers
wireguard_server.peer_9999.name='old-phone'
wireguard_server.peer_9999.public_key='zzzzIbUfQKzq7I9AqXAr5mEgJy0fyCx7Td54G6g1Vc='
wireguard_server.peer_9999.private_key='yyyykFhNYQwqegTPfjC+RKGbwMcuiZzwkFUkZn4t0E4='
wireguard_server.peer_9999.client_ip='10.1.0.9/24,fd00:1e84:8f70:af74::9/64'
wireguard_server.peer_9999.deprecated='1'
`

func TestParseWGGLTunnelsFlint2Shape(t *testing.T) {
	tunnels := parseWGGLTunnels(wgGLNetworkFixture, wgGLServerFixture)
	if len(tunnels) != 1 {
		t.Fatalf("want 1 GL tunnel (wg0 must be ignored), got %d: %+v", len(tunnels), tunnels)
	}
	tun := tunnels[0]
	if tun.Iface != "wgserver" {
		t.Errorf("iface: want wgserver, got %q", tun.Iface)
	}
	if tun.Address != "10.1.0.1/24" || tun.Port != "59999" {
		t.Errorf("server details wrong: %+v", tun)
	}
	if tun.PublicKey != "fVOXQebJWhXsIHb/E9Rt2VqoHhIDiNx1xxGGxqA6R0o=" {
		t.Errorf("server public key wrong: %q", tun.PublicKey)
	}
	// 3 peers in the file, one deprecated: 2 must surface.
	if len(tun.Peers) != 2 {
		t.Fatalf("want 2 live peers (deprecated filtered), got %d: %+v", len(tun.Peers), tun.Peers)
	}
	first := tun.Peers[0]
	if first.Name != "nacho-movil" || first.Section != "peer_3087" {
		t.Errorf("peer identity wrong: %+v", first)
	}
	if len(first.AllowedIPs) != 2 || first.AllowedIPs[0] != "10.1.0.2/24" || first.AllowedIPs[1] != "fd00:1e84:8f70:af74::2/64" {
		t.Errorf("client_ip not split into allowed ips: %+v", first.AllowedIPs)
	}
	for _, p := range tun.Peers {
		if p.Name == "old-phone" {
			t.Errorf("deprecated peer surfaced: %+v", p)
		}
	}
}

func TestParseWGGLTunnelsNeverLeakPrivateKeys(t *testing.T) {
	data, err := json.Marshal(parseWGGLTunnels(wgGLNetworkFixture, wgGLServerFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"private_key", "MOX0kFhNYQwqegTPfjC", "AOjat21QKy+OBbB8jRdN"} {
		if strings.Contains(string(data), leaked) {
			t.Errorf("private material leaked through the API: %q in %s", leaked, data)
		}
	}
}

func TestParseWGGLTunnelsStockOpenWrt(t *testing.T) {
	// rt3 shape: standard wg0, no wgserver proto, no wireguard_server config.
	tunnels := parseWGGLTunnels(wgGLNetworkFixture[:strings.Index(wgGLNetworkFixture, "network.wgserver")], "")
	if len(tunnels) != 0 || tunnels == nil {
		t.Errorf("stock OpenWrt must yield an empty (non-nil) list, got %+v", tunnels)
	}
	data, err := json.Marshal(&WGProbe{Peers: []WGPeer{}, GLTunnels: tunnels})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Errorf("gl_tunnels must never serialize as null: %s", data)
	}
}

func TestSplitGLList(t *testing.T) {
	got := splitGLList("10.1.0.2/24, fd00:1e84:8f70:af74::2/64 ,,")
	if len(got) != 2 || got[0] != "10.1.0.2/24" || got[1] != "fd00:1e84:8f70:af74::2/64" {
		t.Errorf("split wrong: %+v", got)
	}
	if got := splitGLList(""); len(got) != 0 {
		t.Errorf("empty value must yield empty slice, got %+v", got)
	}
}
