package ubus

import "testing"

// Backend is the seam between the handlers/modules and the device: the
// reads they need, expressed as methods, so a test can swap the real ubus
// implementation for a fake and run a full HTTP handler without a router.
// Keep it narrow: add a method only when a migrated read needs it, mirroring
// how the seam grows module by module (#462).
type Backend interface {
	// SystemInfo mirrors `ubus call system info` (see GetSystemInfo).
	SystemInfo() (*SystemInfo, error)
	// WanStatus mirrors the network.interface dump + pick done by
	// GetWanStatus (see its doc).
	WanStatus() (*WanStatus, error)
}

// realBackend is the production implementation: the existing package-level
// reads against the live device.
type realBackend struct{}

func (realBackend) SystemInfo() (*SystemInfo, error) { return GetSystemInfo() }

func (realBackend) WanStatus() (*WanStatus, error) { return GetWanStatus() }

// DefaultBackend is the package-level backend every read goes through.
// It defaults to the real device and exists as a variable (not a Server
// field) so modules that are not methods of Server can reach it too,
// the same pattern as auth.secretPath or mcp.uciGet.
var DefaultBackend Backend = realBackend{}

// SetBackendForTest swaps DefaultBackend for b for the duration of the
// test and restores the previous one on cleanup. It lives in a regular
// file (not _test.go) because other packages' tests need it too, like
// auth.SecretPathForTest.
func SetBackendForTest(t *testing.T, b Backend) {
	t.Helper()
	old := DefaultBackend
	DefaultBackend = b
	t.Cleanup(func() { DefaultBackend = old })
}
