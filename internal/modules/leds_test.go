package modules

import (
	"encoding/json"
	"strings"
	"testing"
)

const systemUCI = `system.@system[0]=system
system.@system[0].hostname='casa'
system.led_wan_port_link=led
system.led_wan_port_link.name='WAN-PORT-LINK'
system.led_wan_port_link.sysfs='90000.mdio-1:18:green:wan'
system.led_wan_port_link.trigger='netdev'
system.led_wan_port_link.dev='wan'
system.led_lan1_port_link=led
system.led_lan1_port_link.name='LAN1-PORT-LINK'
system.led_lan1_port_link.sysfs='90000.mdio-1:03:green:lan'
system.led_lan1_port_link.trigger='netdev'
system.led_broken=led
system.led_broken.name='NO-SYSFS'`

func TestParseSystemLEDs(t *testing.T) {
	leds := parseSystemLEDs(systemUCI)
	if len(leds) != 2 {
		t.Fatalf("leds = %d, want 2 (a section without sysfs is skipped)", len(leds))
	}
	if leds[0].Name != "WAN-PORT-LINK" || leds[0].Trigger != "netdev" || leds[0].Dev != "wan" {
		t.Fatalf("first led = %+v", leds[0])
	}
	if leds[0].Section != "led_wan_port_link" {
		t.Fatalf("section = %q", leds[0].Section)
	}
	// The test host has no /sys/class/leds entry: brightness stays -1 and the
	// state is unknown rather than a made-up false.
	if leds[0].Brightness != -1 || leds[0].On != nil {
		t.Fatalf("unreadable brightness = %d on=%v", leds[0].Brightness, leds[0].On)
	}
}

func TestParseSystemLEDsNeverNil(t *testing.T) {
	if parseSystemLEDs("") == nil {
		t.Fatal("empty config returned nil")
	}
	p := &LEDProbe{Applicable: false, LEDs: parseSystemLEDs("")}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") {
		t.Fatalf("payload carries null: %s", raw)
	}
}
