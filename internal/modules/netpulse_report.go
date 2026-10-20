package modules

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// npReportUpgradeProgress y npReportUpgradeResult (#549) informan a NetPulse
// de los pasos y del resultado terminal del self-update disparado remotamente
// (evento SSE "upgrade"). Usan el mismo contrato que el agente standalone
// (POST /api/agents/{slug}/upgrade-progress|upgrade-result con el token del
// agente en Bearer). Fail-silent con log, al estilo de registerExecutorToken:
// un fallo de red al reportar nunca debe impedir el self-update en sí.
func npReportUpgradeProgress(cfg NetPulseConfig, step string, pct int) {
	npPostUpgradeReport(cfg, "/api/agents/"+cfg.Slug+"/upgrade-progress",
		map[string]any{"slug": cfg.Slug, "step": step, "pct": pct})
}

func npReportUpgradeResult(cfg NetPulseConfig, ok bool, errMsg string) {
	npPostUpgradeReport(cfg, "/api/agents/"+cfg.Slug+"/upgrade-result",
		map[string]any{"slug": cfg.Slug, "ok": ok, "error": errMsg})
}

func npPostUpgradeReport(cfg NetPulseConfig, path string, body map[string]any) {
	if cfg.Server == "" || cfg.Slug == "" || cfg.Token == "" {
		return
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", strings.TrimRight(cfg.Server, "/")+path, bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("netpulse: reporte de upgrade no entregado: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("netpulse: reporte de upgrade respondió %d", resp.StatusCode)
	}
}
