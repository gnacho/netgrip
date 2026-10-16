import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeftRight, Globe, Plus, TriangleAlert, Trash2 } from "lucide-react";
import { api } from "../api";
import type { UpnpProbe, UpnpRuleConfig, UpnpStatus } from "../types";
import { Banner, Button, Card, ConfirmDialog, EmptyState, Field, Input, Pill, SegmentedControl, SkeletonRows, Toggle, useToast } from "../components/ui";

const fmtInt = new Intl.NumberFormat();

/** Formatea los segundos restantes de un lease: "2h 5m", "1m 30s" o "—" sin
 *  expiración (lease_time 0). */
function fmtLease(sec: number): string {
  if (sec <= 0) return "—";
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

/**
 * UPnP / NAT-PMP (#448): miniupnpd como paquete opcional. Mapeos activos en
 * solo lectura (los dueños son los clientes que los solicitan), ACL por
 * cliente vía perm_rule, y activación con confirmación de riesgo porque UPnP
 * es un vector de abuso. secure_mode queda fijado en el backend al activar.
 */
export function UpnpPage() {
  const { t } = useTranslation();
  const { push } = useToast();
  const [probe, setProbe] = useState<UpnpProbe>();
  const [st, setSt] = useState<UpnpStatus>();
  const [loadError, setLoadError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [confirmEnable, setConfirmEnable] = useState(false);
  const [confirmInstall, setConfirmInstall] = useState(false);
  const [confirmUninstall, setConfirmUninstall] = useState(false);

  const load = useCallback(async () => {
    setLoadError(false);
    api.upnpStatus().then(setSt).catch(() => {});
    try {
      setProbe(await api.upnp());
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const run = useCallback(async (fn: () => Promise<{ state?: UpnpProbe; error?: string }>) => {
    setBusy(true);
    try {
      const res = await fn();
      if (res.state) setProbe(res.state);
      if (res.error) {
        push({ tone: "danger", text: t("upnp.actionFailed"), detail: res.error });
      } else {
        push({ tone: "ok", text: t("upnp.actionOk") });
      }
    } catch (e) {
      push({ tone: "danger", text: t("upnp.actionFailed"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
      load();
    }
  }, [load, push, t]);

  const onToggle = (next: boolean) => {
    if (busy) return;
    if (next) setConfirmEnable(true);
    else run(() => api.upnpAction("disable"));
  };

  const notInstalledView = (
    <>
      <EmptyState
        illustration={<ArrowLeftRight size={120} />}
        title={t("upnp.notInstalledTitle")}
        body={t("upnp.notInstalledBody")}
        action={<Button variant="primary" onClick={() => setConfirmInstall(true)}>{t("upnp.install")}</Button>}
      />
      <ConfirmDialog
        open={confirmInstall}
        onClose={() => setConfirmInstall(false)}
        onConfirm={() => {
          setConfirmInstall(false);
          setBusy(true);
          api.upnpInstall()
            .then((p) => { setProbe(p); setSt({ installed: true, enabled: p.enabled, running: p.running, applicable: p.applicable }); push({ tone: "ok", text: t("upnp.installOk") }); })
            .catch((e) => push({ tone: "danger", text: t("upnp.actionFailed"), detail: e instanceof Error ? e.message : String(e) }))
            .finally(() => setBusy(false));
        }}
        title={t("upnp.installConfirmTitle")}
        consequence={t("upnp.installConfirmBody")}
        confirmLabel={t("upnp.install")}
        busy={busy}
      />
    </>
  );

  if (loadError && !probe && !st) {
    return <EmptyState title={t("common.loadError")} action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>} />;
  }

  if (!probe) {
    if (st && !st.applicable) {
      return <EmptyState illustration={<Globe size={120} />} title={t("upnp.notGatewayTitle")} body={t("upnp.notGatewayBody")} />;
    }
    if (st && !st.installed) return notInstalledView;
    const stActive = !!st && st.enabled && st.running;
    const statusPending = !st;
    return (
      <div className="flex flex-col gap-[var(--card-gap)]">
        <div className="flex flex-wrap items-center gap-2">
          <Pill tone={statusPending ? "muted" : stActive ? "ok" : st?.enabled ? "warn" : "muted"} live={stActive}>
            {statusPending ? t("upnp.stateChecking") : stActive ? t("upnp.stateActive") : st?.enabled ? t("upnp.stateStopped") : t("upnp.stateDisabled")}
          </Pill>
        </div>
        <p className="text-small text-muted -mt-1">{t("upnp.introDesc")}</p>
        <Card><SkeletonRows rows={5} /></Card>
        {notInstalledView}
      </div>
    );
  }

  if (!probe.applicable) {
    return <EmptyState illustration={<Globe size={120} />} title={t("upnp.notGatewayTitle")} body={t("upnp.notGatewayBody")} />;
  }

  if (!probe.installed) return notInstalledView;

  const active = probe.enabled && probe.running;

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* Estado + toggle + uninstall */}
      <div className="flex flex-wrap items-center gap-2">
        <Pill tone={active ? "ok" : probe.enabled ? "warn" : "muted"} live={active}>
          {active ? t("upnp.stateActive") : probe.enabled ? t("upnp.stateStopped") : t("upnp.stateDisabled")}
        </Pill>
        {probe.natpmp && <Pill tone="muted">NAT-PMP</Pill>}
        {probe.upnp && <Pill tone="muted">UPnP</Pill>}
        <span className="flex-1" />
        <Toggle
          checked={active}
          disabled={busy}
          busy={busy}
          onChange={onToggle}
          label={t("upnp.cardTitle")}
        />
        <Button variant="ghost" size="sm" className="text-danger hover:text-danger" disabled={busy} onClick={() => setConfirmUninstall(true)}>
          <Trash2 size={14} aria-hidden="true" /> {t("upnp.uninstall")}
        </Button>
      </div>

      {/* Confirmación de riesgo (activar) + desinstalar */}
      <ConfirmDialog
        open={confirmEnable}
        onClose={() => setConfirmEnable(false)}
        onConfirm={() => { setConfirmEnable(false); run(() => api.upnpAction("enable")); }}
        title={t("upnp.enableConfirmTitle")}
        consequence={t("upnp.enableConfirmBody")}
        confirmLabel={t("upnp.enable")}
        busy={busy}
      />
      <ConfirmDialog
        open={confirmUninstall}
        onClose={() => setConfirmUninstall(false)}
        onConfirm={() => {
          setConfirmUninstall(false);
          setBusy(true);
          api.upnpUninstall()
            .then((p) => { setProbe(p); push({ tone: "ok", text: t("upnp.uninstallOk") }); })
            .catch((e) => push({ tone: "danger", text: t("upnp.actionFailed"), detail: e instanceof Error ? e.message : String(e) }))
            .finally(() => setBusy(false));
        }}
        title={t("upnp.uninstallConfirmTitle")}
        consequence={t("upnp.uninstallConfirmBody")}
        confirmLabel={t("upnp.uninstall")}
        busy={busy}
      />

      {/* Riesgo + secure_mode */}
      <Banner tone="warn" icon={TriangleAlert}>
        {t("upnp.riskNote")}
      </Banner>
      <p className="text-small text-muted -mt-1">{t("upnp.introDesc")}</p>
      {probe.secure_mode && (
        <p className="text-caption text-faint -mt-1">{t("upnp.secureModeOn")}</p>
      )}

      {/* Mapeos activos */}
      <Card icon={ArrowLeftRight} iconTone="accent" title={t("upnp.mappingsTitle")}>
        {probe.leases.length === 0 ? (
          <EmptyState small title={t("upnp.mappingsEmpty")} />
        ) : (
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-border/60 text-left">
                <th className="pb-2 pr-4">{t("upnp.colProto")}</th>
                <th className="pb-2 pr-4 text-right">{t("upnp.colExtPort")}</th>
                <th className="pb-2 pr-4">{t("upnp.colIntAddr")}</th>
                <th className="pb-2 pr-4 text-right">{t("upnp.colIntPort")}</th>
                <th className="pb-2 pr-4 text-right">{t("upnp.colLease")}</th>
                <th className="pb-2 text-right">{t("upnp.colDesc")}</th>
              </tr>
            </thead>
            <tbody>
              {probe.leases.map((l, i) => (
                <tr key={`${l.proto}-${l.ext_port}-${i}`} className="border-b border-border/40 last:border-0">
                  <td className="py-2 pr-4 font-mono text-small">{l.proto}</td>
                  <td className="py-2 pr-4 text-right font-mono text-small tabular-nums">{fmtInt.format(l.ext_port)}</td>
                  <td className="py-2 pr-4 font-mono text-small">{l.int_addr}</td>
                  <td className="py-2 pr-4 text-right font-mono text-small tabular-nums">{fmtInt.format(l.int_port)}</td>
                  <td className="py-2 pr-4 text-right text-small tabular-nums text-muted">{fmtLease(l.lease_time)}</td>
                  <td className="py-2 text-right text-small text-muted max-w-[220px]"><span className="line-clamp-1" title={l.desc}>{l.desc || "—"}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <p className="text-caption text-faint mt-3">{t("upnp.mappingsNote")}</p>
      </Card>

      {/* ACL */}
      <RulesCard probe={probe} busy={busy} onChanged={(p) => { setProbe(p); load(); }} onError={(m) => push({ tone: "danger", text: t("upnp.actionFailed"), detail: m })} />
    </div>
  );
}

// ── ACL por cliente ─────────────────────────────────────────────────────────

function RulesCard({ probe, busy, onChanged, onError }: { probe: UpnpProbe; busy: boolean; onChanged: (p: UpnpProbe) => void; onError: (msg: string) => void }) {
  const { t } = useTranslation();
  const [saving, setSaving] = useState(false);
  const [draft, setDraft] = useState<UpnpRuleConfig>({ action: "allow", int_addr: "", int_ports: "", ext_ports: "", comment: "" });

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!draft.int_addr.trim()) return;
    setSaving(true);
    try {
      const res = await api.upnpRuleAdd({ ...draft, int_ports: draft.int_ports?.trim(), ext_ports: draft.ext_ports?.trim(), comment: draft.comment?.trim() });
      if (res.error) onError(res.error);
      else { setDraft({ action: "allow", int_addr: "", int_ports: "", ext_ports: "", comment: "" }); onChanged(res.state); }
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const remove = async (section: string) => {
    setSaving(true);
    try {
      const res = await api.upnpRuleDelete(section);
      if (res.error) onError(res.error);
      else onChanged(res.state);
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card icon={Plus} iconTone="accent" title={t("upnp.rulesTitle")}>
      <form className="flex flex-wrap items-end gap-2" onSubmit={submit}>
        <SegmentedControl<"allow" | "deny">
          size="sm"
          ariaLabel={t("upnp.ruleAction")}
          value={draft.action}
          onChange={(a) => setDraft((d) => ({ ...d, action: a }))}
          options={[
            { value: "allow", label: t("upnp.ruleAllow") },
            { value: "deny", label: t("upnp.ruleDeny") },
          ]}
        />
        <div className="min-w-[180px] flex-1">
          <Field label={t("upnp.ruleIntAddr")}>
            <Input value={draft.int_addr} onChange={(e) => setDraft((d) => ({ ...d, int_addr: e.target.value }))} placeholder="192.168.1.66" />
          </Field>
        </div>
        <div className="w-24">
          <Field label={t("upnp.ruleIntPorts")}>
            <Input value={draft.int_ports ?? ""} onChange={(e) => setDraft((d) => ({ ...d, int_ports: e.target.value }))} placeholder="1024-65535" />
          </Field>
        </div>
        <div className="w-24">
          <Field label={t("upnp.ruleExtPorts")}>
            <Input value={draft.ext_ports ?? ""} onChange={(e) => setDraft((d) => ({ ...d, ext_ports: e.target.value }))} placeholder="1024-65535" />
          </Field>
        </div>
        <div className="min-w-[160px] flex-1">
          <Field label={t("upnp.ruleComment")}>
            <Input value={draft.comment ?? ""} onChange={(e) => setDraft((d) => ({ ...d, comment: e.target.value }))} />
          </Field>
        </div>
        <Button variant="primary" type="submit" disabled={saving || busy || !draft.int_addr.trim()}>
          <Plus size={14} aria-hidden="true" /> {t("upnp.ruleAdd")}
        </Button>
      </form>

      <ul className="mt-3 divide-y divide-border/60">
        {probe.rules.length === 0 && <li className="py-2 text-small text-muted">{t("upnp.rulesEmpty")}</li>}
        {probe.rules.map((r) => (
          <li key={r.section} className="flex items-center gap-2 py-2">
            <Pill tone={r.action === "allow" ? "ok" : "danger"}>{r.action === "allow" ? t("upnp.ruleAllow") : t("upnp.ruleDeny")}</Pill>
            <span className="flex-1 min-w-0 font-mono text-small truncate">
              {r.int_addr}
              {r.int_ports && <span className="text-faint"> :{r.int_ports}</span>}
              {r.ext_ports && <span className="text-faint"> (ext {r.ext_ports})</span>}
              {r.comment && <span className="text-muted"> · {r.comment}</span>}
            </span>
            {r.managed ? (
              <Button variant="ghost" size="sm" disabled={saving || busy} onClick={() => remove(r.section)}>
                {t("upnp.ruleRemove")}
              </Button>
            ) : (
              <span className="text-caption text-faint" title={t("upnp.ruleReadOnly")}>{t("upnp.ruleReadOnly")}</span>
            )}
          </li>
        ))}
      </ul>
      <p className="text-caption text-faint mt-3">{t("upnp.rulesNote")}</p>
    </Card>
  );
}
