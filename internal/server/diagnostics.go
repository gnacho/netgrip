package server

import (
	"encoding/json"
	"net/http"

	"github.com/gnacho/netgrip/internal/modules"
)

// Diagnostics (issue #305): self-test summary plus one-shot guided tests.

func (s *Server) handleDiagnosticsSelfTest(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.RunSelfTest())
}

type diagnosticsRunRequest struct {
	Test     string `json:"test"`
	Host     string `json:"host"`
	Count    int    `json:"count"`
	Query    string `json:"query"`
	Resolver string `json:"resolver"`
	Port     int    `json:"port"`
}

func (s *Server) handleDiagnosticsRun(w http.ResponseWriter, r *http.Request) {
	var req diagnosticsRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	result, err := modules.RunDiagnostics(req.Test, modules.DiagnosticsParams{
		Host:     req.Host,
		Count:    req.Count,
		Query:    req.Query,
		Resolver: req.Resolver,
		Port:     req.Port,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, result)
}
