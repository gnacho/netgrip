package modules

import (
	"slices"
	"testing"
)

func TestMinimalSetupIncludesPasswordRPC(t *testing.T) {
	if !slices.Contains(SetupPackagesForMode(SetupModeMinimal, nil), "rpcd-mod-rpcsys") {
		t.Fatal("minimal setup must install the RPC method used to change the root password")
	}
}
