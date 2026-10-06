package auth

import "testing"

// SecretPathForTest points the session secret and epoch files at dir for the
// duration of the test, so handlers behind requireAuth can be exercised
// without running as root. It lives in a regular file (not _test.go) because
// other packages' tests need it too.
func SecretPathForTest(t *testing.T, dir string) {
	t.Helper()
	oldSecret, oldEpoch := secretPath, epochPath
	secretPath = dir + "/netgrip.secret"
	epochPath = dir + "/netgrip.epoch"
	t.Cleanup(func() {
		secretPath, epochPath = oldSecret, oldEpoch
	})
}
