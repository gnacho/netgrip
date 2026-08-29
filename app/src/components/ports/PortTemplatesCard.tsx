import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy, Play, Plus, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { PortTemplate } from "../../types";
import { Banner, Button, Card, ConfirmDialog, Field, Input, Toggle } from "../ui";

/** Plantillas de puerto (VLAN por boca). Bajo Opciones avanzadas (ports.md §6). */
export function PortTemplatesCard() {
  const { t } = useTranslation();
  const [templates, setTemplates] = useState<PortTemplate[]>([]);
  const [ports, setPorts] = useState<string[]>([]);
  const [busy, setBusy] = useState<string>();
  const [showCreate, setShowCreate] = useState(false);
  const [confirmDel, setConfirmDel] = useState<PortTemplate>();
  const [applyPorts, setApplyPorts] = useState<{ name: string; selected: string[] }>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [form, setForm] = useState({ name: "", description: "", vlans: [{ vid: 1, tagged: false }], admin_up: true, speed_mbps: 0 });

  useEffect(() => {
    api.portTemplates().then((r) => setTemplates(r.templates ?? [])).catch(() => {});
    api.switchPorts().then((r) => { if (r.applicable) setPorts(r.ports.map((p) => p.name)); }).catch(() => {});
  }, []);

  const reload = () => api.portTemplates().then((r) => setTemplates(r.templates ?? [])).catch(() => {});

  if (templates.length === 0 && ports.length === 0 && !showCreate) return null;

  const create = async () => {
    setBusy("create"); setMsg(undefined);
    try {
      await api.savePortTemplate(form);
      setShowCreate(false);
      setForm({ name: "", description: "", vlans: [{ vid: 1, tagged: false }], admin_up: true, speed_mbps: 0 });
      await reload();
      setMsg({ tone: "ok", text: t("portTemplates.saved") });
    } catch (e) { setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) }); }
    finally { setBusy(undefined); }
  };

  const del = async (name: string) => {
    setBusy("del-" + name); setMsg(undefined);
    try {
      await api.deletePortTemplate(name);
      await reload();
      setMsg({ tone: "ok", text: t("portTemplates.deleted") });
    } catch (e) { setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) }); }
    finally { setBusy(undefined); }
  };

  const apply = async (name: string) => {
    if (!applyPorts || applyPorts.selected.length === 0) {
      setMsg({ tone: "danger", text: t("portTemplates.selectPorts") });
      return;
    }
    setBusy("apply-" + name); setMsg(undefined);
    try {
      await api.applyPortTemplate(name, applyPorts.selected);
      setMsg({ tone: "ok", text: t("portTemplates.applied") });
      setApplyPorts(undefined);
    } catch (e) { setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) }); }
    finally { setBusy(undefined); }
  };

  const toggleApplyPort = (port: string) => {
    if (!applyPorts) return;
    const selected = applyPorts.selected.includes(port)
      ? applyPorts.selected.filter((p) => p !== port)
      : [...applyPorts.selected, port];
    setApplyPorts({ ...applyPorts, selected });
  };

  return (
    <Card variant="subtle" animate={false} icon={Copy} title={t("portTemplates.title")}
      action={
        <Button size="sm" variant="secondary" icon={Plus} onClick={() => setShowCreate(!showCreate)}>
          {t("portTemplates.create")}
        </Button>
      }>
      <p className="text-small text-muted mb-3">{t("portTemplates.intro")}</p>

      {showCreate && (
        <div className="bg-surface border border-border rounded-md p-3 mb-3 flex flex-col gap-3 animate-fade-up">
          <Field label={t("portTemplates.templateName")}
            inputProps={{ value: form.name, onChange: (e) => setForm({ ...form, name: e.target.value }) }} />
          <Field label={t("portTemplates.templateDesc")}
            inputProps={{ value: form.description, onChange: (e) => setForm({ ...form, description: e.target.value }) }} />
          <div className="flex items-center gap-4 flex-wrap">
            <label className="flex items-center gap-2 text-small">
              <Toggle checked={form.admin_up} onChange={(v) => setForm({ ...form, admin_up: v })} label={t("portTemplates.adminUp")} />
              {t("portTemplates.adminUp")}
            </label>
            <Input type="number" mono value={form.speed_mbps || ""} placeholder={t("portTemplates.speed")}
              onChange={(e) => setForm({ ...form, speed_mbps: Number(e.target.value) })}
              className="!w-32" aria-label={t("portTemplates.speed")} />
          </div>
          <div className="flex gap-2 justify-end">
            <Button variant="ghost" size="sm" onClick={() => setShowCreate(false)}>{t("portTemplates.cancel")}</Button>
            <Button size="sm" onClick={create} disabled={!form.name} loading={busy === "create"}>
              {t("portTemplates.save")}
            </Button>
          </div>
        </div>
      )}

      {templates.length === 0 ? (
        <p className="text-small text-muted">{t("portTemplates.noTemplates")}</p>
      ) : (
        <div className="flex flex-col gap-2">
          {templates.map((tpl) => (
            <div key={tpl.name} className="bg-surface border border-border rounded-md p-3">
              <div className="flex items-start gap-2">
                <div className="flex-1 min-w-0">
                  <span title={tpl.name} className="block text-body font-medium truncate">{tpl.name}</span>
                  {tpl.description && <p className="text-small text-muted">{tpl.description}</p>}
                  <div className="flex gap-1 mt-1.5 flex-wrap">
                    {tpl.vlans.map((v) => (
                      <span key={v.vid} className={`text-caption font-mono px-1.5 py-0.5 rounded-sm ${v.tagged ? "bg-accent-soft text-accent" : "bg-ok-soft text-ok"}`}>
                        VLAN {v.vid} {v.tagged ? "T" : "U"}
                      </span>
                    ))}
                    {tpl.admin_up && <span className="text-caption px-1.5 py-0.5 rounded-sm bg-ok-soft text-ok">UP</span>}
                    <span className="text-caption font-mono px-1.5 py-0.5 rounded-sm bg-surface-2 text-muted">
                      {tpl.speed_mbps > 0 ? `${tpl.speed_mbps}M` : t("portTemplates.speedAuto")}
                    </span>
                  </div>
                </div>
                <div className="flex gap-1 shrink-0">
                  <Button size="sm" variant="secondary" icon={Play}
                    onClick={() => setApplyPorts(applyPorts?.name === tpl.name ? undefined : { name: tpl.name, selected: [] })}
                    disabled={ports.length === 0}>
                    {t("portTemplates.apply")}
                  </Button>
                  <button type="button" onClick={() => setConfirmDel(tpl)}
                    aria-label={`${t("portTemplates.confirmDel")} ${tpl.name}`}
                    className="inline-flex h-9 w-9 items-center justify-center rounded-md text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus">
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>

              {applyPorts?.name === tpl.name && (
                <div className="mt-3 pt-3 border-t border-border/50 animate-fade-up">
                  <Banner tone="warn" className="mb-2">{t("portTemplates.applyWarn")}</Banner>
                  <div className="flex flex-wrap gap-1.5 items-center">
                    {ports.map((p) => (
                      <button key={p} type="button" onClick={() => toggleApplyPort(p)}
                        className={`text-caption font-mono px-2 py-1 rounded-sm ring-focus transition-colors duration-[var(--dur-fast)]
                          ${applyPorts.selected.includes(p) ? "bg-accent text-on-accent" : "bg-surface-2 border border-border text-muted hover:text-text"}`}>
                        {p}
                      </button>
                    ))}
                    <Button size="sm" onClick={() => apply(tpl.name)} loading={busy === "apply-" + tpl.name}
                      disabled={applyPorts.selected.length === 0}>
                      {t("portTemplates.applyBtn")}
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setApplyPorts(undefined)}>{t("common.cancel")}</Button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}

      <ConfirmDialog
        open={!!confirmDel}
        onClose={() => setConfirmDel(undefined)}
        onConfirm={() => { const tpl = confirmDel; setConfirmDel(undefined); if (tpl) del(tpl.name); }}
        title={t("portTemplates.deleteTitle", { name: confirmDel?.name ?? "" })}
        consequence={t("portTemplates.deleteConsequence")}
        confirmLabel={t("portTemplates.deleteConfirm")}
        busy={!!busy}
      />
    </Card>
  );
}
