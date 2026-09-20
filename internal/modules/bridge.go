package modules

import (
	"os"
	"os/exec"
	"sort"
	"strings"
)

// uciSection is one parsed `uci show` section: its type plus its options.
// Every option is kept as a list so UCI list options (bridge-vlan "ports")
// keep all their values.
type uciSection struct {
	Type    string
	Options map[string][]string
}

// Option returns the first value of an option, or "" when unset.
func (s uciSection) Option(name string) string {
	if v := s.Options[name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// parseUCIShow parses `uci show <pkg>` output into sections keyed by name.
// A section's type is the VALUE of its bare declaration line
// (network.@bridge-vlan[0]=bridge-vlan); attribute lines carry a "." after
// the section name (network.@bridge-vlan[0].vlan='10'). uci show never
// emits a ".type" key - assuming it did is what made the VLAN manager find
// zero VLANs on every router (#328).
func parseUCIShow(show, pkg string) map[string]uciSection {
	prefix := pkg + "."
	sections := map[string]uciSection{}
	for _, line := range strings.Split(show, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		key, val, ok := strings.Cut(strings.TrimPrefix(line, prefix), "=")
		if !ok {
			continue
		}
		name, attr, isAttr := strings.Cut(key, ".")
		if !isAttr {
			s := sections[name]
			s.Type = strings.Trim(val, "'")
			if s.Options == nil {
				s.Options = map[string][]string{}
			}
			sections[name] = s
			continue
		}
		s, ok := sections[name]
		if !ok {
			continue // attribute of a section we never saw declared
		}
		s.Options[attr] = append(s.Options[attr], parseUCIValues(val)...)
		sections[name] = s
	}
	return sections
}

// parseUCIValues splits a `uci show` value into its list elements. uci show
// renders a list option as space-separated quoted tokens
// ('a' 'b' 'c'); a scalar is a single quoted token ('a'). Anything that does
// not match that shape exactly (no quotes, embedded spaces in values) is
// returned as one raw element, matching the previous behavior.
func parseUCIValues(val string) []string {
	val = strings.TrimSpace(val)
	if !strings.HasPrefix(val, "'") || !strings.HasSuffix(val, "'") {
		return []string{val}
	}
	var out []string
	rest := val
	for rest != "" {
		rest = strings.TrimPrefix(rest, "'")
		v, _, ok := strings.Cut(rest, "'")
		if !ok {
			return []string{val} // unbalanced quotes: raw fallback
		}
		out = append(out, v)
		rest = strings.TrimSpace(strings.TrimPrefix(rest, v+"'"))
		if rest != "" && !strings.HasPrefix(rest, "'") {
			return []string{val} // not a clean quoted list: raw fallback
		}
	}
	if len(out) == 0 {
		return []string{val}
	}
	return out
}

func uciShowNetwork() (string, error) {
	out, err := exec.Command("uci", "show", "network").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// LANBridge returns the name of the bridge the LAN lives on. "br-lan" is
// only OpenWrt's default name - real setups rename it ("br0", "br-home") -
// so it is resolved from the running config instead of assumed.
func LANBridge() string {
	if show, err := uciShowNetwork(); err == nil {
		if b := resolveBridge(parseUCIShow(show, "network")); b != "" {
			return b
		}
	}
	if b := sysfsBridge(); b != "" {
		return b
	}
	return "br-lan"
}

// LANDevice returns the L3 device the LAN address sits on: the bridge
// itself on simple setups, or a VLAN sub-interface of it (e.g. "br-lan.10")
// under 802.1Q filtering. Checks that assume the bridge always carries the
// address report a healthy router as dead on VLAN-filtered setups.
func LANDevice() string {
	if show, err := uciShowNetwork(); err == nil {
		if lan, ok := parseUCIShow(show, "network")["lan"]; ok {
			if d := lan.Option("device"); d != "" {
				return d
			}
		}
	}
	return LANBridge()
}

// BridgeDeviceSection returns the UCI section holding the LAN bridge's
// device config (e.g. "@device[0]"), so callers edit the section that
// actually describes the bridge instead of assuming it is the first device
// section declared. Returns "" when the config declares none.
func BridgeDeviceSection() string {
	show, err := uciShowNetwork()
	if err != nil {
		return ""
	}
	bridge := LANBridge()
	for name, s := range parseUCIShow(show, "network") {
		if s.Type == "device" && s.Option("name") == bridge {
			return name
		}
	}
	return ""
}

// resolveBridge picks the LAN bridge out of parsed network sections: the
// bridge the "lan" interface sits on (its device is "<bridge>" or
// "<bridge>.<vid>"), else the bridge the existing bridge-vlan sections are
// attached to, else the first declared bridge device. Returns "" when the
// config declares no bridge at all.
func resolveBridge(sections map[string]uciSection) string {
	declared := map[string]bool{}
	for _, s := range sections {
		if s.Type == "device" && s.Option("type") == "bridge" {
			if n := s.Option("name"); n != "" {
				declared[n] = true
			}
		}
	}

	if lan, ok := sections["lan"]; ok {
		base := lan.Option("device")
		if i := strings.LastIndexByte(base, '.'); i > 0 {
			base = base[:i]
		}
		// A bridge the config declares, or one named like a bridge even if
		// the device section is implicit (legacy configs).
		if base != "" && (declared[base] || strings.HasPrefix(base, "br")) {
			return base
		}
	}

	var vlanBridges []string
	for _, s := range sections {
		if s.Type == "bridge-vlan" {
			if d := s.Option("device"); d != "" {
				vlanBridges = append(vlanBridges, d)
			}
		}
	}
	if len(vlanBridges) > 0 {
		sort.Strings(vlanBridges)
		return vlanBridges[0]
	}

	names := make([]string, 0, len(declared))
	for n := range declared {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

// sysfsBridge returns the first bridge interface present in sysfs, for
// routers whose bridge is not described in the network config at all.
func sysfsBridge() string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if _, err := os.Stat("/sys/class/net/" + e.Name() + "/bridge"); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return ""
}
