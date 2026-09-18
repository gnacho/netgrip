package modules

import (
	"fmt"
	"hash/fnv"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// Optional local DNS aliases for LAN services (#322). A service with an
// alias gets a stable, human-friendly name (e.g. jellyfin.lan) that follows
// the service host's DHCP address automatically. dnsmasq expresses this as a
// CNAME pointing at the service hostname, so it survives IP changes without
// any periodic refresh.

// lanAliasSuffix is the fixed DNS suffix appended to a service alias. It is
// not configurable: ".lan" is the de-facto reserved pseudo-TLD on OpenWrt
// LANs and keeps validation to a single label.
const lanAliasSuffix = ".lan"

// lanAliasSectionPrefix names the NetGrip-owned dnsmasq cname sections, so
// removals only ever touch entries NetGrip created.
const lanAliasSectionPrefix = "netgrip_alias_"

// normalizeLanAlias validates and normalizes a user-supplied alias. It
// accepts a bare label ("jellyfin") or a fully-qualified name ending in the
// fixed suffix ("jellyfin.lan"); the stored value is always the bare label.
// Any other dot is rejected.
func normalizeLanAlias(in string) (string, error) {
	alias := strings.ToLower(strings.TrimSpace(in))
	if alias == "" {
		return "", nil
	}
	if strings.HasSuffix(alias, lanAliasSuffix) {
		alias = strings.TrimSuffix(alias, lanAliasSuffix)
	}
	if strings.Contains(alias, ".") {
		return "", fmt.Errorf("alias must be a single label (no dots)")
	}
	if !reHostname.MatchString(alias) {
		return "", fmt.Errorf("invalid alias %q", alias)
	}
	return alias, nil
}

// lanAliasFQDN returns the fully-qualified alias name.
func lanAliasFQDN(alias string) string {
	return alias + lanAliasSuffix
}

// lanAliasSection returns the dnsmasq UCI section name for an alias. UCI
// section names only allow [a-zA-Z0-9_], while aliases are DNS labels that
// may contain hyphens (#334): disallowed chars map to "_" and a deterministic
// hash suffix disambiguates aliases that would collide after sanitization
// ("a-b" vs "a_b"). Aliases that need no sanitization keep their plain
// section name, as before.
func lanAliasSection(alias string) string {
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, alias)
	if name != alias {
		h := fnv.New32a()
		h.Write([]byte(alias))
		name = fmt.Sprintf("%s_%04x", name, h.Sum32()&0xffff)
	}
	return lanAliasSectionPrefix + name
}

// suggestLanAlias proposes an alias label from the service name (sluggified)
// or, when the name is empty, from a catalog kind.
func suggestLanAlias(s LanService) string {
	if slug := slugLanServiceID(s.Name); slug != "" && slug != "service" {
		return slug
	}
	if isCatalogKind(s.Kind) {
		return s.Kind
	}
	return ""
}

// lanAliasConflicts reports whether aliasFQDN collides with another service's
// alias, an existing dnsmasq cname, or an existing hosts-file hostname. All
// comparisons are case-insensitive.
func lanAliasConflicts(aliasFQDN string, services []LanService, selfID string, cnames []string, hostNames []string) bool {
	alias := strings.TrimSuffix(aliasFQDN, lanAliasSuffix)
	for _, s := range services {
		if s.ID == selfID {
			continue
		}
		if s.Alias == alias {
			return true
		}
	}
	for _, c := range cnames {
		if strings.EqualFold(strings.TrimSpace(c), aliasFQDN) {
			return true
		}
	}
	for _, h := range hostNames {
		h = strings.TrimSpace(h)
		if strings.EqualFold(h, aliasFQDN) || strings.EqualFold(h, alias) {
			return true
		}
	}
	return false
}

// readCnameList returns every dnsmasq cname currently configured in the dhcp
// package, as fully-qualified alias names. dnsmasq.init iterates sections of
// TYPE cname (config_foreach filter_dnsmasq cname dhcp_cname_add) and reads
// their cname/target options, so the UCI shape is a named section, not a list
// on @dnsmasq[0].
func readCnameList() []string {
	out, err := exec.Command("uci", "show", "dhcp").Output()
	if err != nil {
		return nil
	}
	var cnames []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ".cname='") {
			continue
		}
		if vals := uciListValues(line); len(vals) > 0 {
			cnames = append(cnames, vals[0])
		}
	}
	return cnames
}

// dnsHostNames returns the hostnames currently in the dnsmasq hosts file.
func dnsHostNames() []string {
	var names []string
	for _, e := range parseHostsFile(hostsPath()) {
		if e.Hostname != "" {
			names = append(names, e.Hostname)
		}
	}
	return names
}

// reconcileLanServiceAlias syncs the dnsmasq cname section for a service.
// add is the new service state; remove is the previous state whose cname must
// be dropped (rename or delete). On non-gateway devices it is a no-op: the
// alias is stored but has no DNS effect, matching the gateway-only nature of
// the dnsmasq feature.
func reconcileLanServiceAlias(add, remove LanService) error {
	if !dnsApplicable() {
		return nil
	}
	delSection := ""
	if remove.Alias != "" {
		delSection = lanAliasSection(remove.Alias)
	}
	addSection := ""
	if add.Alias != "" {
		addSection = lanAliasSection(add.Alias)
	}
	if delSection == "" && addSection == "" {
		return nil
	}

	snap, err := executor.Snapshot("dhcp")
	if err != nil {
		return err
	}
	rollback := func() {
		_ = executor.Restore("dhcp", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	}

	var ops []executor.Op
	// A rename or a clear drops the old NetGrip-owned section. Only sections
	// with the NetGrip prefix are ever deleted here.
	if delSection != "" && delSection != addSection {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"dhcp." + delSection}})
	}
	if addSection != "" {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"dhcp." + addSection, "cname"}},
			executor.Op{Kind: "uci_set", Args: []string{"dhcp." + addSection + ".cname", lanAliasFQDN(add.Alias)}},
			executor.Op{Kind: "uci_set", Args: []string{"dhcp." + addSection + ".target", add.Host}},
		)
	}
	if len(ops) == 0 {
		return nil
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
		executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return err
	}
	return nil
}
