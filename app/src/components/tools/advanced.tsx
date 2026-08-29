import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Lock, ShieldAlert, ShieldCheck } from "lucide-react";
import { api } from "../../api";
import type { IGMPProbe, MACACLProbe, StormProbe } from "../../types";
import { Banner, Button, Card, SegmentedControl, SettingRow } from "../ui";
import { CardLoadError } from "./diagnostics";

/**
 * Cajón avanzado (tools.md §4): IGMP snooping, control de tormentas y
 * listas MAC por boca. Todo con HelpTip, lejos del primer vistazo.
 */

/** IGMP snooping: SettingRow con toggle (`/api/igmp`). */
export function IgmpCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<IGMPProbe>();
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try { setProbe(await api.igmp()); }
    catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("tools.igmp")} icon={ShieldCheck}>
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe || !probe.applicable) return null;

  const toggle = async (v: boolean) => {
    setBusy(true);
    setFailMsg(undefined);
    try { setProbe(await api.setIgmp(v)); }
    catch (e) { setFailMsg(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  return (
    <Card variant="subtle" animate={false}>
      <SettingRow
        icon={ShieldCheck}
        title={t("tools.igmp")}
        description={t("tools.igmpDesc")}
        helpTitle={t("help.igmp.title")}
        help={t("help.igmp.body")}
        checked={probe.enabled}
        busy={busy}
        onChange={toggle}
      />
      {failMsg && <Banner tone="danger" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}

/** Control de tormentas: límite por boca (`/api/storm`, payload en %). */
export function StormControlCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<StormProbe>();
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState<string>();
  const [percent, setPercent] = useState<Record<string, number>>({});
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try { setProbe(await api.stormControl()); }
    catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("tools.stormControl")} icon={ShieldAlert}>
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe?.applicable) return null;

  const wired = probe.ports.filter((p) => p.port.startsWith("lan"));

  const apply = async (port: string) => {
    setBusy(port);
    setFailMsg(undefined);
    try {
      await api.setStormControl(port, percent[port] ?? 0);
      setProbe(await api.stormControl());
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally { setBusy(undefined); }
  };

  return (
    <Card variant="subtle" animate={false} title={t("tools.stormControl")} icon={ShieldAlert}>
      <p className="text-small text-muted mb-3">{t("tools.stormNote")}</p>
      <div className="flex flex-col divide-y divide-border/60">
        {wired.map((p) => {
          const pct = percent[p.port] ?? (p.active ? 10 : 0);
          return (
            <div key={p.port} className="py-2.5">
              <div className="flex items-center gap-2">
                <span className="w-12 shrink-0 font-mono text-small">{p.port}</span>
                <span className="w-14 shrink-0 text-caption text-muted">
                  {p.link_speed_mbps > 0 ? `${p.link_speed_mbps}M` : "?"}
                </span>
                <input
                  type="range"
                  min={0}
                  max={50}
                  value={pct}
                  aria-label={t("tools.stormAria", { port: p.port })}
                  onChange={(e) => setPercent((prev) => ({ ...prev, [p.port]: +e.target.value }))}
                  className="flex-1 accent-accent"
                />
                <span className="w-10 shrink-0 text-right text-caption tabular-nums">{pct}%</span>
                <Button variant="secondary" size="sm" loading={busy === p.port} onClick={() => apply(p.port)}>
                  {t("tools.stormApply")}
                </Button>
              </div>
              <p className="mt-0.5 text-caption text-faint">
                {t("tools.stormNow", { broadcast: p.broadcast_kbps, multicast: p.multicast_kbps })}
              </p>
            </div>
          );
        })}
      </div>
      {failMsg && <Banner tone="danger" className="mt-2" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}

type MacAclMode = "off" | "allow" | "deny";

/** Listas MAC por boca (`/api/mac-acl`). */
export function MacAclCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<MACACLProbe>();
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState<string>();
  const [modes, setModes] = useState<Record<string, MacAclMode>>({});
  const [macText, setMacText] = useState<Record<string, string>>({});
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try {
      const p = await api.macAcl();
      setProbe(p);
      if (p.ports) {
        const m: Record<string, MacAclMode> = {};
        const tx: Record<string, string> = {};
        for (const port of p.ports) {
          m[port.port] = (port.mode === "allow" || port.mode === "deny" ? port.mode : "off");
          tx[port.port] = port.macs.join("\n");
        }
        setModes(m);
        setMacText(tx);
      }
    } catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("tools.macAclTitle")} icon={Lock} help="macacl">
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe?.applicable) return null;

  const wired = probe.ports.filter((p) => p.port.startsWith("lan"));

  const apply = async (port: string) => {
    setBusy(port);
    setFailMsg(undefined);
    try {
      const macs = (macText[port] || "").split("\n").map((m) => m.trim()).filter(Boolean);
      await api.setMacAcl(port, modes[port] || "off", macs);
      const p = await api.macAcl();
      setProbe(p);
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally { setBusy(undefined); }
  };

  return (
    <Card variant="subtle" animate={false} title={t("tools.macAclTitle")} icon={Lock} help="macacl">
      <p className="text-small text-muted mb-3">{t("tools.macAclIntro")}</p>
      <div className="flex flex-col gap-3">
        {wired.map((p) => {
          const mode = modes[p.port] || "off";
          return (
            <div key={p.port} className="rounded-md border border-border p-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="w-12 shrink-0 font-mono text-small">{p.port}</span>
                <SegmentedControl<MacAclMode>
                  size="sm"
                  ariaLabel={t("tools.macAclTitle")}
                  value={mode}
                  onChange={(v) => setModes((prev) => ({ ...prev, [p.port]: v }))}
                  options={[
                    { value: "off", label: t("tools.macAclOff") },
                    { value: "allow", label: t("tools.macAclAllow") },
                    { value: "deny", label: t("tools.macAclDeny") },
                  ]}
                />
                <Button variant="secondary" size="sm" className="ml-auto"
                  loading={busy === p.port} onClick={() => apply(p.port)}>
                  {t("tools.macAclApply")}
                </Button>
              </div>
              {mode !== "off" && (
                <textarea
                  value={macText[p.port] || ""}
                  rows={3}
                  onChange={(e) => setMacText((prev) => ({ ...prev, [p.port]: e.target.value }))}
                  placeholder={t("tools.macAclPlaceholder")}
                  className="mt-2 w-full rounded-[10px] border border-transparent bg-fill px-3 py-2 font-mono text-small
                    placeholder:text-faint outline-none transition-[background-color,border-color,box-shadow] duration-[var(--dur-fast)]
                    hover:border-border-strong focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-accent)]"
                />
              )}
            </div>
          );
        })}
      </div>
      {failMsg && <Banner tone="danger" className="mt-2" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}
