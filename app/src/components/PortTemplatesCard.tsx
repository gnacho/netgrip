import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy, Plus, Trash2, Play } from "lucide-react";
import { api } from "../api";
import type { PortTemplate } from "../types";
import { Card } from "./Card";

export function PortTemplatesCard() {
  const { t } = useTranslation();
  const [templates, setTemplates] = useState<PortTemplate[]>([]);
  const [ports, setPorts] = useState<string[]>([]);
  const [busy, setBusy] = useState<string>();
  const [showCreate, setShowCreate] = useState(false);
  const [confirmDel, setConfirmDel] = useState<string>();
  const [applyPorts, setApplyPorts] = useState<{ name: string; selected: string[] }>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [form, setForm] = useState({ name: "", description: "", vlans: [{ vid: 1, tagged: false }], admin_up: true, speed_mbps: 0 });

  useEffect(() => {
    api.portTemplates().then((r) => setTemplates(r.templates ?? [])).catch(() => {});
    api.switchPorts().then((r) => { if (r.applicable) setPorts(r.ports.map((p) => p.name)); }).catch(() => {});
  }, []);

  const reload = () => api.portTemplates().then((r) => setTemplates(r.templates ?? [])).catch(() => {});

  const create = async () => {
    setBusy("create"); setMsg(undefined);
    try {
      await api.savePortTemplate(form);
      setShowCreate(false);
      setForm({ name: "", description: "", vlans: [{ vid: 1, tagged: false }], admin_up: true, speed_mbps: 0 });
      await reload();
      setMsg({ tone: "ok", text: t("portTemplates.saved") });
    } catch (e: any) { setMsg({ tone: "danger", text: e.message }); }
    finally { setBusy(undefined); }
  };

  const del = async (name: string) => {
    setBusy("del-" + name); setMsg(undefined);
    try {
      await api.deletePortTemplate(name);
      await reload();
      setMsg({ tone: "ok", text: t("portTemplates.deleted") });
    } catch (e: any) { setMsg({ tone: "danger", text: e.message }); }
    finally { setBusy(undefined); setConfirmDel(undefined); }
  };

  const apply = async (name: string) => {
    if (!applyPorts || applyPorts.name !== name) {
      setApplyPorts({ name, selected: [] });
      return;
    }
    if (applyPorts.selected.length === 0) {
      setMsg({ tone: "danger", text: t("portTemplates.selectPorts") });
      return;
    }
    setBusy("apply-" + name); setMsg(undefined);
    try {
      await api.applyPortTemplate(name, applyPorts.selected);
      setMsg({ tone: "ok", text: t("portTemplates.applied") });
      setApplyPorts(undefined);
    } catch (e: any) { setMsg({ tone: "danger", text: e.message }); }
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
    <Card title={t("portTemplates.title")} icon={Copy}>
      <p className="text-xs text-muted mb-3">{t("portTemplates.intro")}</p>

      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-medium">{t("portTemplates.savedTemplates")}</span>
        <button onClick={() => setShowCreate(!showCreate)}
          className="text-xs bg-accent/15 text-accent px-2 py-0.5 rounded-lg hover:bg-accent/25 flex items-center gap-1">
          <Plus size={10} /> {t("portTemplates.create")}
        </button>
      </div>

      {showCreate && (
        <div className="bg-bg/50 border border-border/50 rounded-lg p-2 mb-2 space-y-2 text-xs">
          <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder={t("portTemplates.templateName")} className="bg-bg border border-border rounded px-2 py-1 w-full" />
          <input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
            placeholder={t("portTemplates.templateDesc")} className="bg-bg border border-border rounded px-2 py-1 w-full" />
          <div className="flex gap-2">
            <label className="flex items-center gap-1">
              <input type="checkbox" checked={form.admin_up} onChange={(e) => setForm({ ...form, admin_up: e.target.checked })} className="accent-accent" />
              {t("portTemplates.adminUp")}
            </label>
            <input type="number" value={form.speed_mbps || ""} onChange={(e) => setForm({ ...form, speed_mbps: Number(e.target.value) })}
              placeholder={t("portTemplates.speed")} className="bg-bg border border-border rounded px-2 py-1 w-24" />
          </div>
          <div className="flex gap-2 justify-end">
            <button onClick={() => setShowCreate(false)} className="text-muted px-2 py-1">{t("portTemplates.cancel")}</button>
            <button onClick={create} disabled={busy === "create" || !form.name}
              className="bg-accent text-white px-3 py-1 rounded-lg disabled:opacity-50">
              {busy === "create" ? "…" : t("portTemplates.save")}
            </button>
          </div>
        </div>
      )}

      {templates.length === 0 ? (
        <p className="text-sm text-muted">{t("portTemplates.noTemplates")}</p>
      ) : (
        <div className="flex flex-col gap-2">
          {templates.map((tpl) => (
            <div key={tpl.name} className="bg-bg/50 border border-border/50 rounded-lg p-2">
              <div className="flex items-center gap-2">
                <div className="flex-1">
                  <span className="text-sm font-medium">{tpl.name}</span>
                  {tpl.description && <p className="text-xs text-muted">{tpl.description}</p>}
                  <div className="flex gap-1 mt-0.5">
                    {tpl.vlans.map((v) => (
                      <span key={v.vid} className={`text-[10px] px-1 rounded ${v.tagged ? "bg-accent/20 text-accent" : "bg-ok/20 text-ok"}`}>
                        VLAN {v.vid} {v.tagged ? "T" : "U"}
                      </span>
                    ))}
                    {tpl.admin_up && <span className="text-[10px] px-1 rounded bg-ok/20 text-ok">UP</span>}
                    {tpl.speed_mbps > 0 && <span className="text-[10px] px-1 rounded bg-muted/20 text-muted">{tpl.speed_mbps}M</span>}
                  </div>
                </div>
                <div className="flex gap-1">
                  {applyPorts?.name === tpl.name ? (
                    <div className="flex flex-wrap gap-1 items-center">
                      {ports.map((p) => (
                        <button key={p} onClick={() => toggleApplyPort(p)}
                          className={`text-[10px] px-1.5 py-0.5 rounded ${applyPorts.selected.includes(p) ? "bg-accent text-white" : "bg-bg border border-border"}`}>
                          {p}
                        </button>
                      ))}
                      <button onClick={() => apply(tpl.name)} disabled={busy === "apply-" + tpl.name}
                        className="text-[10px] bg-ok/20 text-ok px-2 py-0.5 rounded flex items-center gap-0.5">
                        <Play size={8} /> {busy === "apply-" + tpl.name ? "…" : t("portTemplates.applyBtn")}
                      </button>
                      <button onClick={() => setApplyPorts(undefined)} className="text-[10px] text-muted">x</button>
                    </div>
                  ) : (
                    <>
                      <button onClick={() => apply(tpl.name)} disabled={busy === "apply-" + tpl.name}
                        className="text-xs bg-accent/15 text-accent px-2 py-1 rounded-lg hover:bg-accent/25 flex items-center gap-1">
                        <Play size={10} /> {t("portTemplates.apply")}
                      </button>
                      {confirmDel === tpl.name ? (
                        <div className="flex gap-0.5">
                          <button onClick={() => del(tpl.name)} disabled={busy === "del-" + tpl.name}
                            className="text-xs bg-danger/20 hover:bg-danger/30 rounded px-1.5 py-1">
                            {busy === "del-" + tpl.name ? "…" : t("portTemplates.confirmDel")}
                          </button>
                          <button onClick={() => setConfirmDel(undefined)} className="text-xs text-muted">x</button>
                        </div>
                      ) : (
                        <button onClick={() => setConfirmDel(tpl.name)} disabled={busy === "del-" + tpl.name}
                          className="text-muted hover:text-danger p-1">
                          <Trash2 size={10} />
                        </button>
                      )}
                    </>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
