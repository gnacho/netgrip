package modules

import (
	"testing"

	"github.com/gnacho/netgrip/internal/ubus"
)

func TestIwinfoChannelParsesRealOutput(t *testing.T) {
	info := "phy0-ap0   ESSID: \"Casa-2.4\"\n" +
		"          Mode: Master  Channel: 1 (2.412 GHz)  HT Mode: HE20\n"
	ch, ok := iwinfoChannel(info)
	if !ok || ch != 1 {
		t.Errorf("iwinfoChannel = %d, %v; want 1, true", ch, ok)
	}
}

func TestIwinfoChannelDetectsUnknownChannel(t *testing.T) {
	// #452: hostapd dead after a channel change leaves this behind while
	// ubus still reports the radio up.
	info := "phy1-ap0   ESSID: unknown\n" +
		"          Mode: Master  Channel: 0 (unknown GHz)  HT Mode: HE80\n"
	ch, ok := iwinfoChannel(info)
	if ok && ch > 0 {
		t.Errorf("iwinfoChannel = %d, %v; want 0 or !ok for 'Channel: 0 unknown'", ch, ok)
	}
}

func TestIwinfoChannelMissingLine(t *testing.T) {
	if _, ok := iwinfoChannel("Mode: Client\n"); ok {
		t.Error("no channel line must report ok=false")
	}
}

func TestHostapdServingNothingToVerify(t *testing.T) {
	serving, verified := hostapdServing(nil)
	if !serving || verified {
		t.Errorf("nil radio = %v, %v; want true, false (nothing to verify)", serving, verified)
	}
	serving, verified = hostapdServing(&ubus.WirelessRadio{})
	if serving || !verified {
		t.Errorf("radio without interfaces = %v, %v; want false, true (nothing serving)", serving, verified)
	}
}
