package modules

import (
	"testing"
)

func TestProbeNetifyd(t *testing.T) {
	oldFns := resetNetifydFns()
	defer oldFns()

	netifydInstalledFn = func() bool { return true }
	netifydEnabledFn = func() bool { return true }
	netifydRunningFn = func() bool { return false }
	lowEndDeviceFn = func() bool { return false }

	p := ProbeNetifyd()
	if !p.Installed || !p.Enabled || p.Running || !p.Applicable || p.LowEnd {
		t.Fatalf("unexpected probe state: %+v", p)
	}
}

func TestProbeNetifydLowEnd(t *testing.T) {
	oldFns := resetNetifydFns()
	defer oldFns()

	netifydInstalledFn = func() bool { return false }
	netifydEnabledFn = func() bool { return false }
	netifydRunningFn = func() bool { return false }
	lowEndDeviceFn = func() bool { return true }

	p := ProbeNetifyd()
	if p.Installed || p.Enabled || p.Running || p.Applicable || !p.LowEnd {
		t.Fatalf("unexpected low-end probe state: %+v", p)
	}
}

func TestSetNetifydRejectsLowEnd(t *testing.T) {
	oldFns := resetNetifydFns()
	defer oldFns()

	netifydInstalledFn = func() bool { return false }
	lowEndDeviceFn = func() bool { return true }

	_, _, err := SetNetifyd(true)
	if err == nil {
		t.Fatal("expected error on low-end device")
	}
}

func TestNetifydAppsReturnsCopy(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.setFlowApp("x", "Netflix")
	tbl.addStats("x", 1, 2, 3, 1)

	// Reset the package-level table and point it to our test table.
	origTable := netifydTable
	netifydTable = tbl
	defer func() { netifydTable = origTable }()

	apps := NetifydApps()
	if len(apps) != 1 || apps[0].Name != "Netflix" {
		t.Fatalf("unexpected apps: %+v", apps)
	}
}

func resetNetifydFns() func() {
	orig := [5]any{netifydInstalledFn, netifydEnabledFn, netifydRunningFn, lowEndDeviceFn, socketExistsFn}
	return func() {
		netifydInstalledFn = orig[0].(func() bool)
		netifydEnabledFn = orig[1].(func() bool)
		netifydRunningFn = orig[2].(func() bool)
		lowEndDeviceFn = orig[3].(func() bool)
		socketExistsFn = orig[4].(func() bool)
	}
}
