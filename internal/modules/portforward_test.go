package modules

import (
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

// The section name encodes the external port and the destination, so an edit
// that changes either has to move the rule: a stale name would collide with
// a later rule for the old port.
func TestFwdSectionEncodesPortAndDestination(t *testing.T) {
	if got := fwdSection("443", "192.168.1.10"); got != "netgrip_fwd_443_192_168_1_10" {
		t.Fatalf("fwdSection() = %q", got)
	}
}

func TestFwdUpdateOpsMoveWhenTheSectionChanges(t *testing.T) {
	const old = "netgrip_fwd_443_192_168_1_10"

	moved := fwdUpdateOps(old, "netgrip_fwd_8443_192_168_1_20", "netgrip-fwd-8443", "netgrip-fwd-443",
		"8443", "192.168.1.20", "8443", "tcp")
	if len(moved) == 0 || moved[0].Kind != "uci_delete" || moved[0].Args[0] != "firewall."+old {
		t.Fatalf("the first op must delete the old section: %+v", moved)
	}
	if moved[len(moved)-2].Kind != "uci_commit" || moved[len(moved)-1].Kind != "initd" {
		t.Fatalf("the batch must end in commit + reload: %+v", moved)
	}

	same := fwdUpdateOps(old, old, "netgrip-fwd-443", "netgrip-fwd-443",
		"443", "192.168.1.10", "8443", "tcp")
	for _, op := range same {
		if op.Kind == "uci_delete" {
			t.Fatalf("without a section move nothing is deleted: %+v", same)
		}
	}
}

// A renamed rule has to write its new label; leaving it out would show the
// old name until the next unrelated edit.
func TestFwdUpdateOpsWritesTheNewName(t *testing.T) {
	const section = "netgrip_fwd_443_192_168_1_10"
	ops := fwdUpdateOps(section, section, "Mi NAS", "netgrip-fwd-443",
		"443", "192.168.1.10", "443", "tcp")
	if got := opsString(ops); !strings.Contains(got, "uci_set firewall."+section+".name Mi NAS") {
		t.Fatalf("the new name is missing from the batch:\n%s", got)
	}
}

// A rule created elsewhere keeps its label when the name field is left empty:
// the update must not write an empty option over it.
func TestFwdUpdateOpsKeepsTheNameOfAForeignRule(t *testing.T) {
	const section = "mi_regla"
	ops := fwdUpdateOps(section, section, "", "Mi regla", "443", "192.168.1.10", "443", "tcp")
	if got := opsString(ops); strings.Contains(got, ".name") {
		t.Fatalf("an unchanged foreign name must not be rewritten:\n%s", got)
	}
}

func TestResolveFwdName(t *testing.T) {
	cases := []struct {
		name    string
		request string
		current string
		managed bool
		port    string
		want    string
		why     string
	}{
		{"custom name wins", "  Mi NAS  ", "netgrip-fwd-443", true, "443", "Mi NAS", "the label the person typed is kept, trimmed"},
		{"empty on a managed rule returns to automatic", "", "Mi NAS", true, "8443", "netgrip-fwd-8443", "clearing the field means the generated name again"},
		{"empty on a foreign rule keeps the current one", "", "Mi regla", false, "8443", "Mi regla", "never erase a label the panel does not own"},
		{"stale automatic name follows the port", "netgrip-fwd-443", "netgrip-fwd-443", true, "8443", "netgrip-fwd-8443", "an automatic name must not lie about the port"},
		{"automatic name on a foreign rule still follows the port", "netgrip-fwd-443", "netgrip-fwd-443", false, "8443", "netgrip-fwd-8443", "same rule, same generated shape"},
	}
	for _, c := range cases {
		if got := resolveFwdName(c.request, c.current, c.managed, c.port); got != c.want {
			t.Errorf("%s: resolveFwdName(%q, %q, %v, %q) = %q, want %q (%s)",
				c.name, c.request, c.current, c.managed, c.port, got, c.want, c.why)
		}
	}
}

func TestValidFwdName(t *testing.T) {
	if !validFwdName("") {
		t.Error("an empty name is the automatic one")
	}
	if !validFwdName("Mi NAS de casa") {
		t.Error("a plain label with spaces must be accepted")
	}
	if !validFwdName(strings.Repeat("a", maxFwdNameLen)) {
		t.Error("a name at the limit must be accepted")
	}
	if validFwdName(strings.Repeat("a", maxFwdNameLen+1)) {
		t.Error("a name over the limit must be rejected")
	}
	if validFwdName("nas\nrm -rf") {
		t.Error("control characters must be rejected")
	}
	if validFwdName("nas\x1b[2J") {
		t.Error("terminal escapes must be rejected")
	}
}

// Every op the edit builds has to pass the executor allowlist, name included:
// the name reaches `uci set` like any other value.
func TestFwdUpdateOpsPassTheAllowlist(t *testing.T) {
	for _, ops := range [][]executor.Op{
		fwdUpdateOps("netgrip_fwd_443_192_168_1_10", "netgrip_fwd_443_192_168_1_10",
			"Mi NAS", "netgrip-fwd-443", "443", "192.168.1.10", "443", "tcp"),
		fwdUpdateOps("netgrip_fwd_443_192_168_1_10", "netgrip_fwd_8443_192_168_1_20",
			"netgrip-fwd-8443", "netgrip-fwd-443", "8443", "192.168.1.20", "8443", "tcp"),
	} {
		for _, op := range ops {
			if err := executor.Validate(op); err != nil {
				t.Fatalf("op rejected by the allowlist: %v (%+v)", err, op)
			}
		}
	}
}

// The delete gate accepts any section the card lists, not just NetGrip's own:
// the person administering the router can remove a forward or an input rule
// created elsewhere. An arbitrary string is refused so this stays a rule
// delete and not a generic UCI delete.
func TestFwdRuleListed(t *testing.T) {
	rules := []FwdRule{
		{Section: "netgrip_fwd_443_192_168_1_10", Kind: "forward", Managed: true},
		{Section: "@redirect[4]", Kind: "forward"},
		{Section: "Allow-WireGuard", Kind: "input"},
	}
	for _, s := range []string{"netgrip_fwd_443_192_168_1_10", "@redirect[4]", "Allow-WireGuard"} {
		if !fwdRuleListed(rules, s) {
			t.Errorf("%q is listed and should be deletable", s)
		}
	}
	for _, s := range []string{"", "network.lan", "../../etc/shadow", "otra_cosa"} {
		if fwdRuleListed(rules, s) {
			t.Errorf("%q is not listed and must be refused", s)
		}
	}
}
