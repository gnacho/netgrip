package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gnacho/netgrip/internal/ubusconn"
)

// socketCandidates are the ubus sockets probed when the caller does not name
// one. The first is the current OpenWrt default, the second the path used
// before rpcd moved it under /var/run/ubus.
var socketCandidates = []string{"/var/run/ubus/ubus.sock", "/var/run/ubus.sock"}

// Backend validates the router's admin password.
//
// The ubus socket is the primary path. The HTTP endpoint is the fallback, for
// firmware where the socket lives somewhere unexpected, and it is useful only
// because it is already probed at startup.
type Backend struct {
	// Socket is an explicit ubus socket path. Empty probes the candidates.
	Socket string
	// URL is the rpcd JSON-RPC endpoint over HTTP. Empty disables the
	// fallback.
	URL string
}

// ValidatePassword reports whether the credentials are the router's, trying
// the ubus socket first.
func (b Backend) ValidatePassword(username, password string) (bool, error) {
	var failures []string
	for _, socket := range b.sockets() {
		token, err := SocketLogin(socket, username, password)
		if err == nil {
			return token != "", nil
		}
		failures = append(failures, err.Error())
	}
	if b.URL == "" {
		return false, fmt.Errorf("no ubus socket answered: %s", strings.Join(failures, "; "))
	}
	ok, err := validatePasswordHTTP(b.URL, username, password)
	if err != nil && len(failures) > 0 {
		return false, fmt.Errorf("%s; rpcd %s: %w", strings.Join(failures, "; "), b.URL, err)
	}
	return ok, err
}

func (b Backend) sockets() []string {
	if b.Socket != "" {
		return []string{b.Socket}
	}
	return socketCandidates
}

// SocketLogin authenticates against rpcd over the ubus unix socket and returns
// the session token, "" when the credentials are wrong.
//
// It is the same `session.login` the router's own tools call, so the password
// check is rpcd's and nothing new is stored. Routing it over the socket
// instead of HTTP is what lets a router built without LuCI log in: /ubus is
// served by uhttpd-mod-ubus, a package that only arrives with LuCI, so an
// image without it has no endpoint at all (#813). rpcd itself is a hard
// dependency of the package, runs as root, and ubusd exempts uid 0 from its
// ACLs, which is the user the panel runs as.
func SocketLogin(socket, username, password string) (string, error) {
	c, err := ubusconn.Dial(socket)
	if err != nil {
		return "", err
	}
	defer c.Close()
	res, err := c.Call("session", "login", map[string]any{
		"username": username,
		"password": password,
	})
	if err != nil {
		// A refused password is rpcd answering "no", not a broken route:
		// report it as wrong credentials so the panel says what happened
		// instead of blaming the network.
		var status *ubusconn.StatusError
		if errors.As(err, &status) && status.Code == ubusconn.StatusPermissionDenied {
			return "", nil
		}
		return "", err
	}
	token, _ := res["ubus_rpc_session"].(string)
	return token, nil
}
