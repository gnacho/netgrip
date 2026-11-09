package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// confirmTimeout is how long a connectivity-risky apply stays pending before
// the router auto-reverts. It must stay strictly longer than the UI countdown
// so a last-second "Keep" reaches the server before the timer fires.
const confirmTimeout = 120 * time.Second

// confirmEntry is a single pending apply-rollback window. The revert closure
// restores the pre-change UCI state; it is captured so tests can inject a
// no-op and assert on timer/keep/revert behaviour without a real uci binary.
type confirmEntry struct {
	id      string
	revert  func() error
	expires time.Time
}

func newConfirmID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// armConfirm registers a pending confirm and schedules its auto-revert.
// Single-flight: if another confirm is already in flight, it is finalised as
// kept (the user just triggered a further change, so they are still connected)
// before the new one takes over.
func (s *Server) armConfirm(revert func() error, timeout time.Duration) (id string, expires time.Time) {
	s.confirmMu.Lock()
	s.confirm = nil // single-flight: drop any in-flight confirm without reverting
	id = newConfirmID()
	expires = time.Now().Add(timeout)
	s.confirm = &confirmEntry{id: id, revert: revert, expires: expires}
	s.confirmMu.Unlock()

	time.AfterFunc(timeout, func() { s.expireConfirm(id) })
	return id, expires
}

// expireConfirm reverts the pending confirm when its timer fires, unless it
// was already kept/reverted (id no longer matches) in the meantime.
func (s *Server) expireConfirm(id string) {
	s.confirmMu.Lock()
	e := s.confirm
	if e == nil || e.id != id {
		s.confirmMu.Unlock()
		return
	}
	s.confirm = nil
	rev := e.revert
	s.confirmMu.Unlock()
	if rev != nil {
		_ = rev()
	}
}

// keepConfirm cancels the pending confirm, keeping the applied changes.
func (s *Server) keepConfirm(id string) bool {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	if s.confirm == nil || s.confirm.id != id {
		return false
	}
	s.confirm = nil
	return true
}

// revertConfirm immediately reverts the pending confirm.
func (s *Server) revertConfirm(id string) bool {
	s.confirmMu.Lock()
	e := s.confirm
	if e == nil || e.id != id {
		s.confirmMu.Unlock()
		return false
	}
	s.confirm = nil
	rev := e.revert
	s.confirmMu.Unlock()
	if rev != nil {
		_ = rev()
	}
	return true
}

// confirmStatus reports the currently pending confirm, if any.
func (s *Server) confirmStatus() (id string, expires time.Time, ok bool) {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	if s.confirm == nil {
		return "", time.Time{}, false
	}
	return s.confirm.id, s.confirm.expires, true
}

// captureRiskySnapshots exports the UCI configs a connectivity-risky change
// can touch, so they can be restored on revert. Best effort: a missing config
// (e.g. no wireless on a switch) is simply skipped.
func captureRiskySnapshots() map[string]string {
	out := map[string]string{}
	if v, err := executor.Snapshot("network"); err == nil {
		out["network"] = v
	}
	if v, err := executor.Snapshot("wireless"); err == nil {
		out["wireless"] = v
	}
	return out
}

// revertSnapshots restores the captured configs and reloads the given init.d
// services so the restored state takes effect on the running stack.
func revertSnapshots(snapshots map[string]string, reloads []string) error {
	for config, content := range snapshots {
		if err := executor.Restore(config, content); err != nil {
			return err
		}
	}
	for _, svc := range reloads {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{svc, "reload"}})
	}
	return nil
}

// ---- HTTP handlers ----

func (s *Server) handleConfirmStatus(w http.ResponseWriter, _ *http.Request) {
	id, expires, ok := s.confirmStatus()
	if !ok {
		writeJSON(w, map[string]any{"pending": false})
		return
	}
	writeJSON(w, map[string]any{
		"pending": true,
		"id":      id,
		"expires": expires.Unix(),
	})
}

func (s *Server) handleConfirmKeep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "confirm id required")
		return
	}
	if !s.keepConfirm(req.ID) {
		writeError(w, http.StatusNotFound, "no pending confirm")
		return
	}
	writeJSON(w, map[string]string{"status": "kept"})
}

func (s *Server) handleConfirmRevert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "confirm id required")
		return
	}
	if !s.revertConfirm(req.ID) {
		writeError(w, http.StatusNotFound, "no pending confirm")
		return
	}
	writeJSON(w, map[string]string{"status": "reverted"})
}
