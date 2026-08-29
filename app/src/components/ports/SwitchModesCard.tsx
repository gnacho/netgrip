import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Layers } from "lucide-react";
import { api } from "../../api";
import type { SwitchMode } from "../../types";
import { Button, Card, ConfirmDialog } from "../ui";

/** Modos de switch (config predefinida de VLANs/puertos). Bajo Opciones avanzadas. */
export function SwitchModesCard() {
  const { t } = useTranslation();
  const [modes, setModes] = useState<SwitchMode[]>([]);
  const [busy, setBusy] = useState<string>();
  const [confirmMode, setConfirmMode] = useState<SwitchMode>();
  const [uplink, setUplink] = useState("");
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [ports, setPorts] = useState<string[]>([]);

  useEffect(() => {
    api.switchModes().then((r) => setModes(r.modes ?? [])).catch(() => {});
    api.switchPorts().then((r) => {
      if (r.applicable) setPorts(r.ports.map((p) => p.name));
    }).catch(() => {});
  }, []);

  const needsUplink = (id: string) => id === "trunk-uplink" || id === "segmented-home";

  if (modes.length === 0) return null;

  const apply = async (mode: SwitchMode) => {
    if (needsUplink(mode.id) && !uplink) {
      setMsg({ tone: "danger", text: t("switchModes.uplinkRequired") });
      return;
    }
    setBusy(mode.id); setMsg(undefined);
    try {
      await api.applySwitchMode(mode.id, uplink, true);
      setMsg({ tone: "ok", text: t("switchModes.applied") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusy(undefined); }
  };

  return (
    <Card variant="subtle" animate={false} icon={Layers} title={t("switchModes.title")}>
      <p className="text-small text-muted mb-3">{t("switchModes.intro")}</p>

      {ports.length > 0 && modes.some((m) => needsUplink(m.id)) && (
        <div className="mb-3">
          <label className="text-small text-muted block mb-1">{t("switchModes.uplinkPort")}</label>
          <select value={uplink} onChange={(e) => setUplink(e.target.value)}
            className="h-10 rounded-sm border border-border bg-surface-2 px-3 text-body outline-none focus:border-accent ring-focus">
            <option value="">{t("switchModes.selectPort")}</option>
            {ports.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </div>
      )}

      <div className="flex flex-col gap-2">
        {modes.map((mode) => (
          <div key={mode.id} className="flex items-start gap-3 p-3 bg-surface border border-border rounded-md">
            <div className="flex-1 min-w-0">
              <span className="text-body font-medium">{mode.name}</span>
              <p className="text-small text-muted mt-0.5">{mode.description}</p>
            </div>
            <Button size="sm" variant="secondary" onClick={() => setConfirmMode(mode)} loading={busy === mode.id}>
              {t("switchModes.apply")}
            </Button>
          </div>
        ))}
      </div>

      {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}

      <ConfirmDialog
        open={!!confirmMode}
        onClose={() => setConfirmMode(undefined)}
        onConfirm={() => { const m = confirmMode; setConfirmMode(undefined); if (m) apply(m); }}
        title={t("switchModes.applyTitle", { name: confirmMode?.name ?? "" })}
        consequence={t("switchModes.applyConsequence")}
        confirmLabel={t("switchModes.applyConfirm")}
      />
    </Card>
  );
}
