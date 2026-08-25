package modules

import (
	"fmt"

	"github.com/gnacho/owpanel/internal/executor"
)

// OffloadProbe is the read-only network acceleration state.
type OffloadProbe struct {
	Applicable     bool `json:"applicable"` // needs WAN + firewall
	Software       bool `json:"software"`   // flow_offloading (nftables)
	Hardware       bool `json:"hardware"`   // flow_offloading_hw (NSS)
	HardwareKnown  bool `json:"hardware_known"`
	HardwareActive bool `json:"hardware_active"`
}

// ProbeOffload reads the flow offloading state.
func ProbeOffload() *OffloadProbe {
	p := &OffloadProbe{
		Applicable: fwdApplicableCheck(),
		Software:   defaultBool("firewall.@defaults[0].flow_offloading"),
		Hardware:   defaultBool("firewall.@defaults[0].flow_offloading_hw"),
	}
	// Hardware offload is reported as supported only when the kernel/soc can
	// carry it; on OpenWrt mainline IPQ8074 the fallback is software, so we
	// surface it honestly rather than pretending a toggle that does nothing.
	p.HardwareKnown = false
	p.HardwareActive = false
	return p
}

func defaultBool(option string) bool {
	return uciGet(option) == "1"
}

// SetOffload applies the software flow offload toggle with snapshot, reload
// and rollback. Hardware offload is not offered on mainline (see ProbeOffload).
func SetOffload(enabled bool) (*OffloadProbe, bool, error) {
	probe := ProbeOffload()
	if !probe.Applicable {
		return probe, false, notApplicableOffload()
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return probe, false, err
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	val := "0"
	if enabled {
		val = "1"
	}
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"firewall.@defaults[0].flow_offloading", val}},
		{Kind: "uci_commit", Args: []string{"firewall"}},
		{Kind: "initd", Args: []string{"firewall", "reload"}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeOffload(), true, err
	}
	after := ProbeOffload()
	if after.Software != enabled {
		rollback()
		return ProbeOffload(), true, errOffloadHealth()
	}
	return after, false, nil
}

func notApplicableOffload() error {
	return fmt.Errorf("network acceleration only applies on the gateway (needs WAN and firewall)")
}

func errOffloadHealth() error {
	return fmt.Errorf("flow offload healthcheck failed, rolled back")
}
