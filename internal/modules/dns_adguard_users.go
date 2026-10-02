package modules

// AdGuard web credentials (#424): when NetGrip owns the AdGuard Home config
// (it writes the minimal YAML as root for the DNS protection handoff), the
// admin UI on :3000 should not ask for a password the user never chose.
// NetGrip provisions one: a random password whose bcrypt hash lives in the
// `users:` section of /etc/adguardhome.yaml, with the plaintext kept
// server-side in /etc/netgrip/adguard-cred.json (0600) so the panel can
// show and copy it. A config whose `users:` NetGrip did not write - an
// AdGuard the user set up by hand, or one where extra users appeared later -
// is reported as "external" and never touched.

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/gnacho/netgrip/internal/executor"
)

const (
	adGuardAdminUser = "admin"

	// Credential states reported in DNSConfig.AdGuardCredentials.
	adGuardCredManaged  = "managed"  // NetGrip provisioned the only user
	adGuardCredExternal = "external" // users exist that NetGrip did not write
	adGuardCredNone     = "none"     // no users: the dashboard would ask to set one up
)

// adGuardCredPath keeps the plaintext the panel shows on demand. Same
// plain-file pattern as the dnsmasq backup next to it. A variable so tests
// can pin it.
var adGuardCredPath = "/etc/netgrip/adguard-cred.json"

// AdGuardCredentials is the on-demand view for the credentials endpoint. The
// password only leaves the process here, on explicit request - the DNS probe
// carries the state alone, like the MQTT password is never returned.
type AdGuardCredentials struct {
	State    string `json:"state"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// adGuardCredFile is the stored form of the provisioned credentials.
type adGuardCredFile struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// parseAdGuardUsers returns the usernames in the top-level `users:` block of
// an AdGuard Home YAML config. Line-based like parseAdGuardDNSPort: pulling
// in a YAML library for one block is not worth it on a router, and the file
// is written either by us or by AdGuard Home itself, both with plain
// two-space indentation.
func parseAdGuardUsers(data []byte) []string {
	var users []string
	inUsers := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			inUsers = false
			if rest, ok := strings.CutPrefix(trimmed, "users:"); ok {
				switch v := strings.TrimSpace(rest); v {
				case "":
					inUsers = true // block form: the names follow indented
				case "[]":
					// Inline empty list, how the 25.12 package ships it.
				default:
					// Inline content we do not parse: report it as an
					// unmanageable user so the state reads external and
					// the config is never touched.
					users = append(users, "<inline>")
				}
			}
			continue
		}
		if !inUsers {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "- name:"); ok {
			if name := strings.TrimSpace(rest); name != "" {
				users = append(users, name)
			}
		}
	}
	return users
}

// adGuardUsersBlock renders the `users:` section in the shape AdGuard Home
// writes it.
func adGuardUsersBlock(username, hash string) string {
	return fmt.Sprintf("users:\n  - name: %s\n    password: %s\n", username, hash)
}

// replaceAdGuardUsers swaps the top-level `users:` block for one holding
// exactly username/hash, preserving every other key in the file. With no
// block present it is appended; AdGuard Home rewrites the file with its own
// ordering on the first change anyway.
func replaceAdGuardUsers(data []byte, username, hash string) []byte {
	block := adGuardUsersBlock(username, hash)
	lines := strings.Split(string(data), "\n")
	start, end := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if start < 0 {
			if indent == 0 {
				if rest, ok := strings.CutPrefix(trimmed, "users:"); ok {
					start = i
					if strings.TrimSpace(rest) != "" {
						// Inline form (users: []): the block is this one line.
						end = i + 1
					}
				}
			}
			continue
		}
		if end > 0 {
			break // inline form: already bounded
		}
		if indent == 0 {
			end = i
			break
		}
	}
	if start < 0 {
		out := strings.TrimRight(string(data), "\n") + "\n" + block
		return []byte(out)
	}
	if end < 0 {
		end = len(lines)
	}
	out := make([]string, 0, len(lines)+3)
	out = append(out, lines[:start]...)
	out = append(out, strings.Split(strings.TrimRight(block, "\n"), "\n")...)
	out = append(out, lines[end:]...)
	return []byte(strings.Join(out, "\n"))
}

// randomAdGuardPassword generates 20 base62 characters (~119 bits): strong
// enough to keep forever, still pastable by hand if it ever comes to that.
func randomAdGuardPassword() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i, b := range raw {
		raw[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(raw), nil
}

func saveAdGuardCred(cred *adGuardCredFile) error {
	if err := os.MkdirAll(filepath.Dir(adGuardCredPath), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(adGuardCredPath, data, 0o600)
}

func loadAdGuardCred() *adGuardCredFile {
	data, err := os.ReadFile(adGuardCredPath)
	if err != nil {
		return nil
	}
	var cred adGuardCredFile
	if json.Unmarshal(data, &cred) != nil || cred.Username == "" || cred.Password == "" {
		return nil
	}
	return &cred
}

// adGuardCredStateNow classifies who owns the credentials in the AdGuard
// config. "Managed" requires both sides to agree: the YAML holds exactly our
// admin user and the plaintext is still on disk. A second user appearing, or
// the cred file going missing, means somebody else is driving and the card
// says so instead of offering to regenerate what it cannot know.
func adGuardCredStateNow() string {
	data, err := os.ReadFile(adGuardConfigPathNow())
	if err != nil {
		return adGuardCredNone
	}
	users := parseAdGuardUsers(data)
	if len(users) == 0 {
		return adGuardCredNone
	}
	if len(users) == 1 && users[0] == adGuardAdminUser && loadAdGuardCred() != nil {
		return adGuardCredManaged
	}
	return adGuardCredExternal
}

// AdGuardCredentialsNow is the read model for GET /api/dns/adguard/credentials.
func AdGuardCredentialsNow() *AdGuardCredentials {
	state := adGuardCredStateNow()
	out := &AdGuardCredentials{State: state}
	if state == adGuardCredManaged {
		if cred := loadAdGuardCred(); cred != nil {
			out.Username = cred.Username
			out.Password = cred.Password
		}
	}
	return out
}

// writeAdGuardYAML replaces the config atomically, keeping the previous
// content at path.bak-users so a botched write is one cp away from undone.
func writeAdGuardYAML(path string, data []byte) error {
	if prev, err := os.ReadFile(path); err == nil {
		if err := os.WriteFile(path+".bak-users", prev, 0o600); err != nil {
			return fmt.Errorf("backing up %s: %w", path, err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// adGuardProvisionUsers writes a fresh credential set into a config that has
// no users yet. Caller must have AdGuard Home stopped, or it may overwrite
// the file on its way down.
func adGuardProvisionUsers() (*adGuardCredFile, error) {
	data, err := os.ReadFile(adGuardConfigPathNow())
	if err != nil {
		return nil, err
	}
	if users := parseAdGuardUsers(data); len(users) > 0 {
		return nil, fmt.Errorf("the AdGuard config already has users (%s), not touching them", strings.Join(users, ", "))
	}
	password, err := randomAdGuardPassword()
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := writeAdGuardYAML(adGuardConfigPathNow(), replaceAdGuardUsers(data, adGuardAdminUser, string(hash))); err != nil {
		return nil, err
	}
	cred := &adGuardCredFile{Username: adGuardAdminUser, Password: password}
	if err := saveAdGuardCred(cred); err != nil {
		// Leave no users block without its plaintext: that combination reads
		// as "external" and would lock the panel out of its own credential.
		_ = os.WriteFile(adGuardConfigPathNow(), data, 0o600)
		return nil, err
	}
	return cred, nil
}

// RegenerateAdGuardPassword rotates the managed credential, or provisions
// one when the config has no users yet (upgrades from a NetGrip that wrote
// the YAML before credentials existed). Refuses external configs outright.
// AdGuard Home rereads its users only on startup, so the service is
// restarted when it is running; a stopped service stays stopped and the new
// password is simply there when it comes up.
func RegenerateAdGuardPassword() (*AdGuardCredentials, error) {
	if !pkgInstalled("adguardhome") {
		return nil, fmt.Errorf("adguardhome is not installed")
	}
	state := adGuardCredStateNow()
	if state == adGuardCredExternal {
		return nil, fmt.Errorf("credentials are managed from the AdGuard Home UI, not from here")
	}
	running := executor.ServiceRunning("adguardhome")
	if state == adGuardCredNone && running {
		// Provisioning writes the YAML underneath the service; do it stopped.
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"adguardhome", "stop"}})
	}
	var err error
	if state == adGuardCredNone {
		_, err = adGuardProvisionUsers()
	} else {
		err = adGuardRotatePassword()
	}
	if err != nil {
		if state == adGuardCredNone && running {
			_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"adguardhome", "start"}})
		}
		return nil, err
	}
	if running {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"adguardhome", "restart"}})
	}
	return AdGuardCredentialsNow(), nil
}

// adGuardRotatePassword replaces the hash of the managed user and stores the
// new plaintext. Only call with the state already vetted as managed.
func adGuardRotatePassword() error {
	password, err := randomAdGuardPassword()
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(adGuardConfigPathNow())
	if err != nil {
		return err
	}
	if err := writeAdGuardYAML(adGuardConfigPathNow(), replaceAdGuardUsers(data, adGuardAdminUser, string(hash))); err != nil {
		return err
	}
	return saveAdGuardCred(&adGuardCredFile{Username: adGuardAdminUser, Password: password})
}
