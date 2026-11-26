package modules

import (
	"os"
	"strconv"
	"strings"
)

// DeviceLED is one LED configured in /etc/config/system. Read-only on purpose:
// every LED on a stock OpenWrt router carries a trigger (netdev for the port
// LEDs, heartbeat for the status one), and writing `default` to switch it off
// drops that trigger and the indication with it. The panel reports what is
// there so the app can show it; it does not dim the chassis.
type DeviceLED struct {
	Section string `json:"section"`
	Name    string `json:"name"`
	Sysfs   string `json:"sysfs"`
	Trigger string `json:"trigger,omitempty"`
	Dev     string `json:"dev,omitempty"`
	// Brightness is the current value of the sysfs entry, -1 when unreadable.
	// `on` is derived from it and left out when unknown.
	Brightness int   `json:"brightness"`
	On         *bool `json:"on,omitempty"`
}

// LEDProbe is the LED inventory.
type LEDProbe struct {
	Applicable bool        `json:"applicable"`
	LEDs       []DeviceLED `json:"leds"`
}

// parseSystemLEDs reads the `uci show system` output and builds the LED list.
func parseSystemLEDs(out string) []DeviceLED {
	leds := []DeviceLED{}
	bySection := map[string]*DeviceLED{}
	order := []string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "system.") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := line[:eq]
		val := strings.Trim(line[eq+1:], "'")
		parts := strings.Split(key, ".")
		if len(parts) < 2 {
			continue
		}
		section := parts[1]
		if len(parts) == 2 {
			if val != "led" {
				continue
			}
			if _, ok := bySection[section]; !ok {
				bySection[section] = &DeviceLED{Section: section, Brightness: -1}
				order = append(order, section)
			}
			continue
		}
		l, ok := bySection[section]
		if !ok {
			continue
		}
		switch parts[2] {
		case "name":
			l.Name = val
		case "sysfs":
			l.Sysfs = val
		case "trigger":
			l.Trigger = val
		case "dev":
			l.Dev = val
		}
	}
	for _, section := range order {
		l := bySection[section]
		if l.Sysfs == "" {
			continue
		}
		if data, err := os.ReadFile("/sys/class/leds/" + l.Sysfs + "/brightness"); err == nil {
			if v, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil {
				l.Brightness = v
				on := v > 0
				l.On = &on
			}
		}
		leds = append(leds, *l)
	}
	return leds
}

// ProbeLEDs lists the LEDs the router knows about. Read-only.
func ProbeLEDs() *LEDProbe {
	out, ok := uciShowCached("system")
	if !ok {
		return &LEDProbe{Applicable: false, LEDs: []DeviceLED{}}
	}
	leds := parseSystemLEDs(out)
	return &LEDProbe{Applicable: len(leds) > 0, LEDs: leds}
}
