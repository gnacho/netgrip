import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Camera, Cable, GitCompareArrows, RefreshCw, ShieldCheck, Ban, Undo2, Download } from "lucide-react";
import { api } from "../api";
import { Card, Pill } from "../components/Card";
import type { ConfigSnapshot, ConfigDiff, EthPort, IGMPProbe, LoopResult } from "../types";

export function ToolsPage({ ethports }: { ethports: EthPort[] }) {
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <div className="sm:col-span-2">
        <SnapshotsCard />
      </div>
      <IGMPCard />
      <LoopsCard />
      <div className="sm:col-span-2">
        <BounceCard ethports={ethports} />
      </div>
    </div>
  );
}

function SnapshotsCard() {
  const { t } = useTranslation();
  const [snaps, setSnaps] = useState<ConfigSnapshot[]>([]);
  const [busy, setBusy] = useState<string>();
  const [msg, setMsg] = useState<{ text: string; tone: "ok" | "danger" }>();
  const [selA, setSelA] = useState("");
  const [selB, setSelB] = useState("");
  const [diffs, setDiffs] = useState<ConfigDiff[] | null>(null);
  const [confirmId, setConfirmId] = useState<string>();

  const reload = () => api.snapshots().then((r) => setSnaps(r.snapshots ?? [])).catch(() => {});
  useEffect(() => { reload(); }, []);

  const create = async () => {
    setBusy("create");
    try { await api.createSnapshot(); await reload(); }
    catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBusy(undefined); }
  };

  const rollback = async (id: string) => {
    setBusy("rollback-" + id);
    try { await api.rollbackSnapshot(id); setMsg({ text: t("tools.rollbackDone"), tone: "ok" }); }
    catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBusy(undefined); setConfirmId(undefined); }
  };

  const del = async (id: string) => {
    setBusy("del-" + id);
    try { await api.deleteSnapshot(id); await reload(); }
    catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBusy(undefined); setConfirmId(undefined); }
  };

  const compare = async () => {
    if (!selA || !selB || selA === selB) return;
    setBusy("diff");
    try {
      const r = await api.snapshotDiff(selA, selB);
      setDiffs(r.diffs ?? []);
    } catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBusy(undefined); }
  };

  const fmtDate = (ts: number) => new Date(ts * 1000).toLocaleString();

  return (
    <Card title={t("tools.snapshots")} icon={Camera}>
      <p className="text-xs text-muted mb-3">{t("tools.snapshotsIntro")}</p>
      <button onClick={create} disabled={busy === "create"}
        className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium mb-3">
        {busy === "create" ? t("tools.creating") : t("tools.create")}
      </button>

      {snaps.length === 0 && <p className="text-sm text-muted">{t("tools.noSnapshots")}</p>}

      {snaps.length > 0 && (
        <div className="flex flex-col gap-2">
          {snaps.map((s) => (
            <div key={s.id} className="flex items-center gap-2 py-2 border-b border-border/50 last:border-0 text-sm">
              <input type="radio" name="snapA" checked={selA === s.id} onChange={() => setSelA(s.id)} className="accent-accent" />
              <input type="radio" name="snapB" checked={selB === s.id} onChange={() => setSelB(s.id)} className="accent-accent" />
              <span className="flex-1">{fmtDate(s.timestamp)}</span>
              <Pill tone="muted">{t("tools.configs", { n: s.configs })}</Pill>
              <a href={`/api/config/snapshot/export?id=${s.id}`} download
                className="text-muted hover:text-text p-1" title={t("tools.export")}>
                <Download size={12} />
              </a>
              {confirmId === s.id ? (
                <div className="flex gap-1">
                  <button onClick={() => rollback(s.id)} disabled={busy === "rollback-" + s.id}
                    className="text-xs bg-warn/20 hover:bg-warn/30 rounded px-2 py-1">{t("tools.rollback")}</button>
                  <button onClick={() => del(s.id)} disabled={busy === "del-" + s.id}
                    className="text-xs bg-danger/20 hover:bg-danger/30 rounded px-2 py-1">{t("tools.delete")}</button>
                  <button onClick={() => setConfirmId(undefined)} className="text-xs text-muted px-1">x</button>
                </div>
              ) : (
                <button onClick={() => setConfirmId(s.id)} className="text-xs text-muted hover:text-text px-2 py-1">...</button>
              )}
            </div>
          ))}
        </div>
      )}

      {selA && selB && selA !== selB && (
        <button onClick={compare} disabled={busy === "diff"}
          className="text-xs bg-card hover:bg-border/50 rounded-lg px-3 py-1.5 mt-3 flex items-center gap-1">
          <GitCompareArrows size={14} />
          {busy === "diff" ? "..." : t("tools.diff")}
        </button>
      )}

      {diffs !== null && (
        <div className="mt-3 border-t border-border/50 pt-3">
          <p className="text-xs font-medium mb-2">{t("tools.diffTitle")}</p>
          {diffs.length === 0 && <p className="text-sm text-muted">{t("tools.noDiffs")}</p>}
          {diffs.map((d) => (
            <details key={d.config} className="mb-2">
              <summary className="text-sm cursor-pointer">{d.config}</summary>
              <div className="grid grid-cols-2 gap-2 mt-1 text-xs">
                <pre className="bg-bg/50 p-2 rounded overflow-auto max-h-48 whitespace-pre-wrap">{d.before || "(empty)"}</pre>
                <pre className="bg-bg/50 p-2 rounded overflow-auto max-h-48 whitespace-pre-wrap">{d.after || "(empty)"}</pre>
              </div>
            </details>
          ))}
        </div>
      )}

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}

function IGMPCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<IGMPProbe>();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ text: string; tone: "ok" | "danger" }>();

  useEffect(() => { api.igmp().then(setProbe).catch(() => {}); }, []);

  if (!probe || !probe.applicable) return null;

  const toggle = async () => {
    setBusy(true);
    try { setProbe(await api.setIgmp(!probe.enabled)); }
    catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBusy(false); }
  };

  return (
    <Card title={t("tools.igmp")} icon={ShieldCheck}>
      <p className="text-xs text-muted mb-3">{t("tools.igmpIntro")}</p>
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm">{probe.enabled ? t("tools.igmpOn") : t("tools.igmpOff")}</span>
        <input type="checkbox" checked={probe.enabled} disabled={busy}
          onChange={toggle} className="accent-accent" />
      </div>
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}

function LoopsCard() {
  const { t } = useTranslation();
  const [result, setResult] = useState<LoopResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [blockBusy, setBlockBusy] = useState<string>();
  const [confirmBlock, setConfirmBlock] = useState<string>();
  const [msg, setMsg] = useState<{ text: string; tone: "ok" | "danger" }>();

  const check = async () => {
    setBusy(true);
    try { setResult(await api.loops()); } catch { setResult({ loops: [], has_hub: false }); }
    finally { setBusy(false); }
  };

  const blockPort = async (iface: string) => {
    setBlockBusy(iface);
    try {
      await api.blockPort(iface, true);
      setMsg({ text: t("tools.portBlocked"), tone: "ok" });
    } catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBlockBusy(undefined); setConfirmBlock(undefined); }
  };

  const unblockPort = async (iface: string) => {
    setBlockBusy(iface);
    try {
      await api.blockPort(iface, false);
      setMsg({ text: t("tools.portUnblocked"), tone: "ok" });
    } catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBlockBusy(undefined); }
  };

  return (
    <Card title={t("tools.loops")} icon={Cable}>
      <p className="text-xs text-muted mb-3">{t("tools.loopsIntro")}</p>
      <button onClick={check} disabled={busy}
        className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium mb-3">
        {busy ? "..." : t("tools.loops")}
      </button>

      {result && (
        <div className="flex flex-col gap-1 text-sm">
          {result.loops.length === 0 && !result.has_hub && (
            <p className="text-ok">{t("tools.noLoops")}</p>
          )}
          {result.loops.map((l) => (
            <div key={l.mac} className="flex items-center gap-2 py-1 border-b border-border/50 last:border-0">
              <span className="font-mono text-xs">{l.mac}</span>
              <Pill tone="warn">{l.ports.join(", ")}</Pill>
              <div className="flex gap-1 ml-auto">
                {l.ports.map((port) => (
                  confirmBlock === port ? (
                    <div key={port} className="flex gap-0.5">
                      <button onClick={() => blockPort(port)} disabled={blockBusy === port}
                        className="text-xs bg-danger/20 hover:bg-danger/30 rounded px-1.5 py-0.5 flex items-center gap-1">
                        <Ban size={10} /> {blockBusy === port ? "..." : t("tools.blockConfirm")}
                      </button>
                      <button onClick={() => setConfirmBlock(undefined)} className="text-xs text-muted">x</button>
                    </div>
                  ) : (
                    <div key={port} className="flex gap-0.5">
                      <button onClick={() => setConfirmBlock(port)}
                        className="text-xs bg-danger/10 hover:bg-danger/20 text-danger rounded px-1.5 py-0.5 flex items-center gap-1">
                        <Ban size={10} /> {port}
                      </button>
                      <button onClick={() => unblockPort(port)} disabled={blockBusy === port}
                        className="text-xs bg-ok/10 hover:bg-ok/20 text-ok rounded px-1 py-0.5 flex items-center gap-0.5">
                        <Undo2 size={10} />
                      </button>
                    </div>
                  )
                ))}
              </div>
            </div>
          ))}
          {result.has_hub && <p className="text-warn text-xs mt-1">{t("tools.hubDetected")}</p>}
        </div>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}

function BounceCard({ ethports }: { ethports: EthPort[] }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState<string>();
  const [msg, setMsg] = useState<{ text: string; tone: "ok" | "danger" }>();
  const [confirmIface, setConfirmIface] = useState<string>();

  const wired = ethports.filter((p) => !p.wan && p.name.startsWith("lan"));

  const bounce = async (iface: string) => {
    setBusy(iface);
    try {
      await api.bouncePort(iface);
      setMsg({ text: t("tools.bounceDone"), tone: "ok" });
    } catch (e: any) { setMsg({ text: e.message, tone: "danger" }); }
    finally { setBusy(undefined); setConfirmIface(undefined); }
  };

  if (wired.length === 0) return null;

  return (
    <Card title={t("tools.bounce")} icon={RefreshCw}>
      <div className="flex flex-wrap gap-2">
        {wired.map((p) => (
          <div key={p.name} className="flex items-center gap-1">
            {confirmIface === p.name ? (
              <>
                <button onClick={() => bounce(p.name)} disabled={busy === p.name}
                  className="text-xs bg-warn/20 hover:bg-warn/30 rounded px-2 py-1">
                  {busy === p.name ? t("tools.bouncing") : p.name}
                </button>
                <button onClick={() => setConfirmIface(undefined)} className="text-xs text-muted">x</button>
              </>
            ) : (
              <button onClick={() => setConfirmIface(p.name)}
                className="text-xs bg-card hover:bg-border/50 border border-border rounded px-2 py-1">
                {p.name}
              </button>
            )}
          </div>
        ))}
      </div>
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
