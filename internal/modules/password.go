package modules

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gnacho/owpanel/internal/auth"
)

// MinPasswordLength is the minimum accepted length for a new root password.
const MinPasswordLength = 8

// ChangePassword validates the current password via rpcd and then sets the
// new one with `ubus call rpc-sys password_set` (the method LuCI uses).
// Note: the rpcd HTTP endpoint denies password_set even for root sessions
// (verified on OpenWrt 25.12.5), while the CLI as root is allowed. The new
// password appears briefly in the exec arguments; on OpenWrt only root can
// read the process table, and the window is milliseconds. The alternative
// (writing /etc/shadow from Go) is riskier and was discarded.
func ChangePassword(rpcdURL, current, next string) error {
	if len(next) < MinPasswordLength {
		return fmt.Errorf("new password too short (min %d)", MinPasswordLength)
	}
	ok, err := auth.ValidatePassword(rpcdURL, "root", current)
	if err != nil {
		return fmt.Errorf("rpcd login: %w", err)
	}
	if !ok {
		return errors.New("current password is wrong")
	}
	out, err := exec.Command("ubus", "call", "rpc-sys", "password_set",
		fmt.Sprintf(`{"user":"root","password":%s}`, strconv.Quote(next))).CombinedOutput()
	if err != nil {
		return fmt.Errorf("password_set: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
