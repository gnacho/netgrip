package modules

import "testing"

func TestParseCableBlocked(t *testing.T) {
	out := `firewall.cfg0dfa8f=redirect
firewall.netgrip_block_001122334455.name='netgrip-block-00:11:22:33:44:55'
firewall.netgrip_block_001122334455.src='lan'
firewall.netgrip_block_001122334455.src_mac='00:11:22:33:44:55'
firewall.netgrip_block_aabbccddeeff.name='netgrip-block-aa:bb:cc:dd:ee:ff'
firewall.netgrip_block_aabbccddeeff.src_mac='AA:BB:CC:DD:EE:FF'
firewall.other.src_mac='99:88:77:66:55:44'`
	got := parseCableBlocked(out)
	if len(got) != 2 {
		t.Fatalf("expected 2 blocked, got %d: %#v", len(got), got)
	}
	for _, mac := range []string{"00:11:22:33:44:55", "aa:bb:cc:dd:ee:ff"} {
		if !got[mac] {
			t.Fatalf("expected %s blocked", mac)
		}
	}
	// The non-netgrip-block src_mac must be ignored.
	if got["99:88:77:66:55:44"] {
		t.Fatal("non-netgrip src_mac should not count")
	}
}
