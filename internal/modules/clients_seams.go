package modules

import (
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// Device-touch seams for the clients path (plans/backend-seam-clients
// phase 3). Each variable defaults to the real device read and exists so
// tests - including handler tests in other packages - can make
// ListClients run without a router. Zero-value fields in ClientsSeams
// keep the current default.
var (
	leasesFilePath   = "/tmp/dhcp.leases"
	arpFilePath      = "/proc/net/arp"
	bridgeFdbRead    = bridgeFdb
	serviceEnabledFn = executor.ServiceEnabled
	probeModeFn      = ProbeMode
	cableBlockedFn   = cableBlockedSetImpl
)

// ClientsSeams overrides the device reads on the clients path. Every
// field is optional: a zero value keeps the live default.
type ClientsSeams struct {
	// LeaseFile backs leasesForClients (default /tmp/dhcp.leases).
	LeaseFile string
	// ArpFile backs localArp (default /proc/net/arp).
	ArpFile string
	// BridgeFdb backs the wired-clients read (default sysfs + brctl).
	BridgeFdb func() map[string][]string
	// ServiceEnabled backs the firewall check for blockability.
	ServiceEnabled func(name string) bool
	// ProbeMode backs the AP/router classification in ListClients.
	ProbeMode func() *ModeProbe
	// RouterClock backs routerLocalNow (default exec `date`).
	RouterClock func() (time.Time, bool)
	// ParentalPath / ParentalAppliedPath back the parental rules and
	// the scheduler ownership ledger.
	ParentalPath        string
	ParentalAppliedPath string
	// CableBlocked backs the wired-block read (default exec uci|grep).
	CableBlocked func() map[string]bool
	// ClientMetaPath / QuotaConfigFile / QuotaStateFile back the local
	// JSON state the listing annotates clients with.
	ClientMetaPath  string
	QuotaConfigFile string
	QuotaStateFile  string
}

// SetClientsSeamsForTest applies the non-zero seams for the duration of
// the test and restores the previous values on cleanup. Regular file (not
// _test.go) so other packages' handler tests can use it, like
// SetUciExecForTest and SetGatewaySSHForTest.
func SetClientsSeamsForTest(t testT, seams ClientsSeams) {
	t.Helper()
	oldLeases, oldArp := leasesFilePath, arpFilePath
	oldFdb, oldSvc, oldProbe := bridgeFdbRead, serviceEnabledFn, probeModeFn
	oldClock := routerLocalNow
	oldParental, oldApplied := parentalPath, parentalAppliedPath
	oldCable, oldMeta := cableBlockedFn, clientMetaPath
	oldQuotaCfg, oldQuotaState := quotaConfigFile, quotaStateFile
	t.Cleanup(func() {
		leasesFilePath, arpFilePath = oldLeases, oldArp
		bridgeFdbRead, serviceEnabledFn, probeModeFn, cableBlockedFn = oldFdb, oldSvc, oldProbe, oldCable
		routerLocalNow = oldClock
		parentalPath, parentalAppliedPath = oldParental, oldApplied
		clientMetaPath = oldMeta
		quotaConfigFile, quotaStateFile = oldQuotaCfg, oldQuotaState
	})
	if seams.LeaseFile != "" {
		leasesFilePath = seams.LeaseFile
	}
	if seams.ArpFile != "" {
		arpFilePath = seams.ArpFile
	}
	if seams.BridgeFdb != nil {
		bridgeFdbRead = seams.BridgeFdb
	}
	if seams.ServiceEnabled != nil {
		serviceEnabledFn = seams.ServiceEnabled
	}
	if seams.ProbeMode != nil {
		probeModeFn = seams.ProbeMode
	}
	if seams.RouterClock != nil {
		routerLocalNow = seams.RouterClock
	}
	if seams.ParentalPath != "" {
		parentalPath = seams.ParentalPath
	}
	if seams.ParentalAppliedPath != "" {
		parentalAppliedPath = seams.ParentalAppliedPath
	}
	if seams.CableBlocked != nil {
		cableBlockedFn = seams.CableBlocked
	}
	if seams.ClientMetaPath != "" {
		clientMetaPath = seams.ClientMetaPath
	}
	if seams.QuotaConfigFile != "" {
		quotaConfigFile = seams.QuotaConfigFile
	}
	if seams.QuotaStateFile != "" {
		quotaStateFile = seams.QuotaStateFile
	}
}

// cableBlockedSet is the seam-aware entry point used by ListClients.
func cableBlockedSet() map[string]bool { return cableBlockedFn() }
