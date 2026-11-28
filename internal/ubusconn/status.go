package ubusconn

import "fmt"

// Status codes carried by a ubus STATUS message (enum ubus_msg_status).
const (
	StatusOK               = 0
	StatusInvalidCommand   = 1
	StatusInvalidArgument  = 2
	StatusMethodNotFound   = 3
	StatusNotFound         = 4
	StatusNoData           = 5
	StatusPermissionDenied = 6
	StatusTimeout          = 7
	StatusNotSupported     = 8
	StatusUnknownError     = 9
)

// StatusError is the server's own answer: ubusd reached the object and the
// method ran, and the method returned a non-zero status. It is not a broken
// connection, which matters to callers that have to tell "the router refused
// this" from "the router did not answer" (a login, for instance).
type StatusError struct {
	// Op is "lookup" or "call".
	Op     string
	Object string
	Method string
	Code   int
}

func (e *StatusError) Error() string {
	if e.Op == "lookup" {
		return fmt.Sprintf("ubusconn: lookup %q: status %d", e.Object, e.Code)
	}
	return fmt.Sprintf("ubusconn: call %s.%s: status %d", e.Object, e.Method, e.Code)
}
