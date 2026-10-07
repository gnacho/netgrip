package modules

import "os/exec"

// uciExec runs a uci command and returns its stdout. It exists as a
// variable (same seam idea as ubus.DefaultBackend or auth.secretPath):
// on a device it forks the real uci, and tests swap it for a fake so
// reads like dhcpReservations or blockedBands run without a router.
// Add new uci reads through this instead of exec.Command("uci", ...).
var uciExec = func(args ...string) ([]byte, error) {
	return exec.Command("uci", args...).Output()
}

// SetUciExecForTest swaps uciExec for fn for the duration of the test and
// restores the previous one on cleanup. It lives in a regular file (not
// _test.go) because other packages' tests need it too, like
// auth.SecretPathForTest and ubus.SetBackendForTest.
func SetUciExecForTest(t testT, fn func(args ...string) ([]byte, error)) {
	t.Helper()
	old := uciExec
	uciExec = fn
	t.Cleanup(func() { uciExec = old })
}

// testT is the *testing.T surface this helper needs, declared locally so
// the seam file does not import "testing" for one signature.
type testT interface {
	Helper()
	Cleanup(func())
}
