package modules

// SetL2ClientsSeamsForTest swaps the L2 client probe for fn for the duration
// of the test and restores the previous one on cleanup. Regular file (not
// _test.go) so the handler test in internal/server can fake the FDB read
// without touching real netlink, like SetClientsSeamsForTest.
func SetL2ClientsSeamsForTest(t testT, fn func() *L2Probe) {
	t.Helper()
	old := probeL2ClientsFn
	probeL2ClientsFn = fn
	t.Cleanup(func() { probeL2ClientsFn = old })
}
