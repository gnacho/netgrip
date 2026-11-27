import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Clipboard, KeyRound, Wand2 } from "lucide-react";
import { api } from "../../api";
import { copyText } from "../../lib/clipboard";
import type { MCPSettings as MCPSettingsInfo } from "../../types";
import { Button, Field, Input, SettingRow, useToast } from "../ui";

/**
 * Bloque MCP dentro de "Avisos e integraciones" (#807). Sin Card propia: la
 * envuelve CollapsibleSection. El endpoint existe desde #439, pero solo se
 * armaba por UCI; aquí se enciende, se genera el token y se copia la URL con
 * la que un asistente se conecta. Los ajustes son los mismos tres que lee la
 * capa de auth en cada petición, así que no hace falta reiniciar el servicio.
 */
export function MCPSettings({ onStatus }: { onStatus?: (enabled: boolean) => void }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [info, setInfo] = useState<MCPSettingsInfo>();
  const [failed, setFailed] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [token, setToken] = useState("");
  const [rate, setRate] = useState(60);
  const [busy, setBusy] = useState(false);
  const [generating, setGenerating] = useState(false);

  const apply = (s: MCPSettingsInfo) => {
    setInfo(s);
    setEnabled(s.enabled);
    setToken(s.token);
    setRate(s.rate_per_min);
    onStatus?.(s.enabled);
  };

  const refresh = () => {
    setFailed(false);
    api.mcp().then(apply).catch(() => setFailed(true));
  };

  useEffect(() => {
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const save = async (nextEnabled = enabled) => {
    setBusy(true);
    try {
      apply(await api.setMcp({ enabled: nextEnabled, token, rate_per_min: rate }));
      push({ tone: "ok", text: t("mcp.saved") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
      refresh();
    } finally {
      setBusy(false);
    }
  };

  const generate = async () => {
    setGenerating(true);
    try {
      const r = await api.newMcpToken();
      setToken(r.token);
      push({ tone: "ok", text: t("mcp.generated") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setGenerating(false);
    }
  };

  const copy = async (value: string) => {
    const ok = await copyText(value);
    push({ tone: ok ? "ok" : "danger", text: t(ok ? "mcp.copied" : "mcp.copyFailed") });
  };

  if (failed) return <p className="text-small text-danger">{t("error.network")}</p>;
  if (!info) return null;

  return (
    <>
      <SettingRow
        title={t("mcp.enabled")}
        description={t("mcp.description", { tools: info.tools })}
        checked={enabled}
        onChange={(v) => { setEnabled(v); void save(v); }}
      />

      <div className="mt-3">
        <Field label={t("mcp.url")} hint={t("mcp.urlHint")}>
          <div className="flex gap-2">
            <Input mono readOnly value={info.url} onFocus={(e) => e.currentTarget.select()} />
            <Button size="sm" variant="secondary" icon={Clipboard} onClick={() => void copy(info.url)}
              aria-label={t("mcp.copyUrl")}>
              {t("mcp.copy")}
            </Button>
          </div>
        </Field>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto] gap-3 mt-3 items-end">
        <Field label={t("mcp.token")} hint={t("mcp.tokenHint")}>
          <Input mono value={token} autoComplete="off" onChange={(e) => setToken(e.target.value)} />
        </Field>
        <div className="flex gap-2">
          <Button size="sm" variant="secondary" icon={Wand2} loading={generating} onClick={() => void generate()}>
            {t("mcp.generate")}
          </Button>
          <Button size="sm" variant="secondary" icon={Clipboard} disabled={!token} onClick={() => void copy(token)}
            aria-label={t("mcp.copyToken")}>
            {t("mcp.copy")}
          </Button>
        </div>
      </div>

      <div className="mt-3 sm:max-w-[220px]">
        <Field label={t("mcp.rate")} hint={t("mcp.rateHint")}>
          <Input type="number" mono min={10} max={600} value={rate}
            onChange={(e) => setRate(Number(e.target.value))} />
        </Field>
      </div>

      <div className="flex items-center gap-2 mt-3 text-small text-muted">
        <KeyRound size={14} aria-hidden="true" />
        <span>{t("mcp.readOnly", { tools: info.tools })}</span>
      </div>

      <div className="flex gap-2 mt-3">
        <Button size="sm" onClick={() => void save()} loading={busy}>
          {t("mcp.save")}
        </Button>
      </div>
    </>
  );
}
