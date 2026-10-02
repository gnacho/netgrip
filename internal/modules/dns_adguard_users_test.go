package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// pinAdGuardPaths points the AdGuard config and credential files at a temp
// dir for the duration of the test.
func pinAdGuardPaths(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldCfg, oldCred := adGuardConfigPath, adGuardCredPath
	adGuardConfigPath = filepath.Join(dir, "adguardhome.yaml")
	adGuardCredPath = filepath.Join(dir, "adguard-cred.json")
	t.Cleanup(func() { adGuardConfigPath, adGuardCredPath = oldCfg, oldCred })
}

const adGuardHandYAML = `http:
  address: 0.0.0.0:3000
dns:
  bind_hosts:
    - 192.168.1.1
  port: 5353
  upstream_dns:
    - https://dns10.quad9.net/dns-query
filters:
  - enabled: true
    url: https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt
users:
  - name: pedro
    password: $2a$10$somehashvalue
schema_version: 29
`

func TestParseAdGuardUsers(t *testing.T) {
	if got := parseAdGuardUsers([]byte(adGuardMinimalYAML(5353))); len(got) != 0 {
		t.Errorf("minimal yaml: users = %v, want none", got)
	}
	got := parseAdGuardUsers([]byte(adGuardHandYAML))
	if len(got) != 1 || got[0] != "pedro" {
		t.Errorf("hand-written yaml: users = %v, want [pedro]", got)
	}
	multi := "users:\n  - name: ana\n    password: x\n  - name: luis\n    password: y\n"
	if got := parseAdGuardUsers([]byte(multi)); len(got) != 2 {
		t.Errorf("two users: got %v", got)
	}
	// The 25.12 package ships an inline empty list; it means no users.
	if got := parseAdGuardUsers([]byte("users: []\nauth_attempts: 5\n")); len(got) != 0 {
		t.Errorf("inline empty list: got %v, want none", got)
	}
	// Inline content we cannot parse must not read as "no users".
	if got := parseAdGuardUsers([]byte("users: [{name: ana}]\n")); len(got) == 0 {
		t.Error("unparseable inline users must be reported, not read as none")
	}
}

func TestReplaceAdGuardUsersInlineEmpty(t *testing.T) {
	in := "http:\n  address: 0.0.0.0:3000\nusers: []\nauth_attempts: 5\n"
	out := string(replaceAdGuardUsers([]byte(in), adGuardAdminUser, "$2a$10$newhash"))
	if got := parseAdGuardUsers([]byte(out)); len(got) != 1 || got[0] != adGuardAdminUser {
		t.Errorf("users = %v, want [%s]", got, adGuardAdminUser)
	}
	if strings.Contains(out, "users: []") || !strings.Contains(out, "auth_attempts: 5") {
		t.Errorf("inline list not replaced or neighbours lost:\n%s", out)
	}
	if strings.Count(out, "users:") != 1 {
		t.Errorf("duplicate users keys:\n%s", out)
	}
}

func TestAdGuardConfigPathFallsBackToThePinnedVar(t *testing.T) {
	// Without uci (development machine) and no 25.12 file, the resolver
	// returns the 24.10 variable tests pin.
	pinAdGuardPaths(t)
	if got := adGuardConfigPathNow(); got != adGuardConfigPath {
		t.Errorf("resolver = %q, want the pinned %q", got, adGuardConfigPath)
	}
}

func TestReplaceAdGuardUsersPreservesTheRest(t *testing.T) {
	out := string(replaceAdGuardUsers([]byte(adGuardHandYAML), adGuardAdminUser, "$2a$10$newhash"))
	if !strings.Contains(out, "filters:") || !strings.Contains(out, "schema_version: 29") || !strings.Contains(out, "upstream_dns:") {
		t.Errorf("unrelated keys lost:\n%s", out)
	}
	if strings.Contains(out, "pedro") {
		t.Errorf("old user still present:\n%s", out)
	}
	if got := parseAdGuardUsers([]byte(out)); len(got) != 1 || got[0] != adGuardAdminUser {
		t.Errorf("users = %v, want [%s]", got, adGuardAdminUser)
	}
}

func TestReplaceAdGuardUsersAppendsWhenMissing(t *testing.T) {
	out := string(replaceAdGuardUsers([]byte(adGuardMinimalYAML(5353)), adGuardAdminUser, "$2a$10$newhash"))
	if got := parseAdGuardUsers([]byte(out)); len(got) != 1 || got[0] != adGuardAdminUser {
		t.Errorf("users = %v, want [%s]", got, adGuardAdminUser)
	}
	if p := parseAdGuardDNSPort([]byte(out)); p != 5353 {
		t.Errorf("dns port = %d, want 5353", p)
	}
}

func TestAdGuardMinimalYAMLWithUsers(t *testing.T) {
	data := adGuardMinimalYAMLWithUsers(5353, adGuardAdminUser, "$2a$10$hash")
	if got := parseAdGuardUsers([]byte(data)); len(got) != 1 || got[0] != adGuardAdminUser {
		t.Errorf("users = %v, want [%s]", got, adGuardAdminUser)
	}
	if p := parseAdGuardDNSPort([]byte(data)); p != 5353 {
		t.Errorf("dns port = %d, want 5353", p)
	}
}

func TestAdGuardCredState(t *testing.T) {
	pinAdGuardPaths(t)

	if got := adGuardCredStateNow(); got != adGuardCredNone {
		t.Errorf("no config file: state = %q, want %q", got, adGuardCredNone)
	}

	writeYAML := func(content string) {
		t.Helper()
		if err := os.WriteFile(adGuardConfigPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeYAML(adGuardMinimalYAML(5353))
	if got := adGuardCredStateNow(); got != adGuardCredNone {
		t.Errorf("config without users: state = %q, want %q", got, adGuardCredNone)
	}

	writeYAML(adGuardHandYAML)
	if got := adGuardCredStateNow(); got != adGuardCredExternal {
		t.Errorf("somebody else's user: state = %q, want %q", got, adGuardCredExternal)
	}

	// Our user in the YAML but no stored plaintext: not ours to rotate.
	writeYAML(adGuardMinimalYAMLWithUsers(5353, adGuardAdminUser, "$2a$10$hash"))
	if got := adGuardCredStateNow(); got != adGuardCredExternal {
		t.Errorf("admin without cred file: state = %q, want %q", got, adGuardCredExternal)
	}

	if err := saveAdGuardCred(&adGuardCredFile{Username: adGuardAdminUser, Password: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := adGuardCredStateNow(); got != adGuardCredManaged {
		t.Errorf("admin plus cred file: state = %q, want %q", got, adGuardCredManaged)
	}

	// A second user appearing means somebody else is driving.
	writeYAML(adGuardMinimalYAMLWithUsers(5353, adGuardAdminUser, "$2a$10$hash") + "  - name: otro\n    password: y\n")
	if got := adGuardCredStateNow(); got != adGuardCredExternal {
		t.Errorf("admin plus a second user: state = %q, want %q", got, adGuardCredExternal)
	}
}

func TestAdGuardProvisionUsers(t *testing.T) {
	pinAdGuardPaths(t)
	if err := os.WriteFile(adGuardConfigPath, []byte(adGuardMinimalYAML(5353)), 0o600); err != nil {
		t.Fatal(err)
	}

	cred, err := adGuardProvisionUsers()
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if cred.Username != adGuardAdminUser || len(cred.Password) != 20 {
		t.Errorf("cred = %+v, want admin with a 20-char password", cred)
	}

	// The hash in the YAML must match the stored plaintext.
	data, err := os.ReadFile(adGuardConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var hash string
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "password:"); ok {
			hash = strings.TrimSpace(rest)
		}
	}
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(cred.Password)) != nil {
		t.Error("the YAML hash does not verify against the stored password")
	}

	// The config keeps a backup and the state now reads managed.
	if _, err := os.Stat(adGuardConfigPath + ".bak-users"); err != nil {
		t.Error("no .bak-users backup written")
	}
	if got := adGuardCredStateNow(); got != adGuardCredManaged {
		t.Errorf("state = %q, want %q", got, adGuardCredManaged)
	}

	// Provisioning over existing users refuses and touches nothing.
	before, _ := os.ReadFile(adGuardConfigPath)
	if _, err := adGuardProvisionUsers(); err == nil {
		t.Error("provisioning over existing users should fail")
	}
	after, _ := os.ReadFile(adGuardConfigPath)
	if string(before) != string(after) {
		t.Error("refused provisioning still modified the config")
	}
}

func TestAdGuardRotatePassword(t *testing.T) {
	pinAdGuardPaths(t)
	if err := os.WriteFile(adGuardConfigPath, []byte(adGuardMinimalYAML(5353)), 0o600); err != nil {
		t.Fatal(err)
	}
	cred, err := adGuardProvisionUsers()
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	if err := adGuardRotatePassword(); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	now := loadAdGuardCred()
	if now == nil || now.Password == cred.Password {
		t.Fatal("the password did not change")
	}
	view := AdGuardCredentialsNow()
	if view.State != adGuardCredManaged || view.Password != now.Password || view.Username != adGuardAdminUser {
		t.Errorf("credentials view = %+v", view)
	}
}
