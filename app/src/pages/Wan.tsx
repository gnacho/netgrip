import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff, Globe, KeyRound, Pencil, Save } from "lucide-react";
import { api } from "../api";
import type { WANConfig } from "../api";
import type { FwdProbe, WanStatus } from "../types";
import {
  Banner, Button, Card, Field, Pill, SegmentedControl, SkeletonRows, useToast,
} from "../components/ui";
import { PortForwardCard } from "../components/ports/PortForwardCard";

const PROTO = ["dhcp", "static", "pppoe"] as const;
const PROTO_KEY: Record<string, string> = {
  dhcp: "wan.protoDhcp",
  static: "wan.protoStatic",
  pppoe: "wan.protoPppoe",
};

/** Anillo de estado de conexión: verde = Internet OK, rojo = sin conexión. */
function ConnRing({ up }: { up: boolean }) {
  const { t } = useTranslation();
  const R = 42, C = 2 * Math.PI * R;
  return (
    <div role="img" aria-label={up ? t("wan.up") : t("wan.down")}
      className={`relative inline-flex items-center justify-center shrink-0 ${up ? "animate-internet" : ""}`}
      style={{ width: 104, height: 104 }}>
      <svg viewBox="0 0 104 104" className="absolute inset-0">
        <circle cx="52" cy="52" r={R} fill="none" stroke="currentColor" strokeWidth="8" className="text-faint" />
        <circle cx="52" cy="52" r={R} fill="none" stroke="currentColor" strokeWidth="8"
          strokeLinecap="round" strokeDasharray={C} strokeDashoffset={0}
          className={up ? "text-ok" : "text-danger"} />
      </svg>
      <span className={`relative ${up ? "text-ok" : "text-danger"}`}>
        {up ? <Globe size={42} /> : <CloudOff size={42} />}
      </span>
    </div>
  );
}

function fmtDur(s: number): string {
  if (!s || s <= 0) return "—";
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

/** Página WAN (#243): estado de salida a Internet + configuración (lectura con
 *  Editar; el form no abre por defecto) + port-forwarding. */
export function WanPage({ fwd, onFwdChange }: {
  fwd?: FwdProbe;
  onFwdChange?: (p: FwdProbe) => void;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [status, setStatus] = useState<WanStatus>();
  const [cfg, setCfg] = useState<WANConfig>();
  const [form, setForm] = useState<WANConfig>({ proto: "dhcp" });
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(false);

  const load = () => {
    api.wan().then(setStatus).catch(() => setError(true));
    api.wanConfig().then((c) => { setCfg(c); setForm(c); }).catch(() => setError(true));
  };
  useEffect(load, []);

  const set = (patch: Partial<WANConfig>) => setForm((f) => ({ ...f, ...patch }));
  const protoLabel = (p: string) => t(PROTO_KEY[p] ?? p);

  const save = async () => {
    setSaving(true);
    try {
      const res = await api.setWanConfig(form);
      setCfg(res);
      setForm(res);
      setEditing(false);
      push({ tone: "ok", text: t("wan.saved") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setSaving(false);
    }
  };

  const stats = useMemo(() => ({
    ip: status?.ipv4?.join(", ") ?? "—",
    gateway: status?.gateway ?? "—",
    dns: status?.dns?.join(", ") || t("wan.dhcpDns"),
    uptime: status?.uptime ? fmtDur(status.uptime) : "—",
  }), [status, t]);

  // Datos de configuración mostrados en la vista de lectura.
  const cfgRows: [string, string][] = [];
  if (cfg) {
    cfgRows.push([t("wan.proto"), protoLabel(cfg.proto)]);
    if (cfg.proto === "static") {
      if (cfg.ipaddr) cfgRows.push([t("wan.ip"), cfg.ipaddr]);
      if (cfg.netmask) cfgRows.push([t("wan.netmask"), cfg.netmask]);
      if (cfg.gateway) cfgRows.push([t("lan.gateway"), cfg.gateway]);
      if (cfg.dns) cfgRows.push([t("wan.dns"), cfg.dns]);
    }
    if (cfg.proto === "pppoe" && cfg.username) cfgRows.push([t("wan.username"), cfg.username]);
    if (cfg.device) cfgRows.push([t("wan.device"), cfg.device]);
    if (cfg.mtu) cfgRows.push(["MTU", cfg.mtu]);
    if (cfg.vlanid) cfgRows.push(["VLAN ID", cfg.vlanid]);
  }

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card index={0} icon={Globe} title={t("wan.title")}>
        {error ? (
          <Banner tone="danger">{t("common.loadError")}</Banner>
        ) : !status ? (
          <SkeletonRows rows={3} />
        ) : (
          <div className="flex items-start gap-5">
            <ConnRing up={status.up} />
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <Pill tone={status.up ? "ok" : "danger"}>
                  {status.up ? t("wan.internetOk") : t("wan.down")}
                </Pill>
                {status.up && <span className="text-small text-ok font-medium">{t("wan.allGood")}</span>}
              </div>
              <p className="font-mono text-h2 mt-1.5 break-all">{stats.ip}</p>
              <dl className="mt-1 grid sm:grid-cols-2 gap-x-6 gap-y-0.5 text-small">
                <div className="flex gap-3"><dt className="text-muted">{t("lan.gateway")}</dt><dd className="font-mono text-right break-all">{stats.gateway}</dd></div>
                <div className="flex gap-3"><dt className="text-muted">{t("wan.dns")}</dt><dd className="font-mono text-right break-all">{stats.dns}</dd></div>
                <div className="flex gap-3"><dt className="text-muted">{t("wan.uptimeLabel")}</dt><dd className="font-mono">{stats.uptime}</dd></div>
              </dl>
            </div>
          </div>
        )}
      </Card>

      <Card index={1} icon={Save} title={t("wan.configTitle")}
        action={
          !editing && cfg !== undefined ? (
            <Button variant="secondary" size="sm" icon={Pencil} onClick={() => setEditing(true)}>{t("wan.edit")}</Button>
          ) : undefined
        }>
        {cfg === undefined ? (
          <SkeletonRows rows={4} />
        ) : editing ? (
          <>
            <div className="space-y-3">
              <span className="text-caption text-muted block">{t("wan.proto")}</span>
              <SegmentedControl
                ariaLabel={t("wan.proto")}
                value={form.proto}
                onChange={(v) => set({ proto: v as WANConfig["proto"] })}
                options={PROTO.map((p) => ({ value: p, label: protoLabel(p) }))}
              />

              {form.proto === "pppoe" && (
                <div className="space-y-3">
                  <Field label={t("wan.username")} mono inputProps={{ value: form.username ?? "", onChange: (e) => set({ username: e.target.value }) }} />
                  <Field label={t("wan.password")} icon={KeyRound} inputProps={{ type: "password", value: form.password ?? "", onChange: (e) => set({ password: e.target.value }), placeholder: t("wan.passwordKeep") }} />
                </div>
              )}

              {form.proto === "static" && (
                <div className="space-y-3">
                  <Field label={t("wan.ip")} mono inputProps={{ value: form.ipaddr ?? "", onChange: (e) => set({ ipaddr: e.target.value }) }} />
                  <Field label={t("wan.netmask")} mono inputProps={{ value: form.netmask ?? "", onChange: (e) => set({ netmask: e.target.value }) }} />
                  <Field label={t("lan.gateway")} mono inputProps={{ value: form.gateway ?? "", onChange: (e) => set({ gateway: e.target.value }) }} />
                  <Field label={t("wan.dns")} mono inputProps={{ value: form.dns ?? "", onChange: (e) => set({ dns: e.target.value }) }} />
                </div>
              )}

              <div className="grid sm:grid-cols-3 gap-3">
                <Field label="VLAN ID" mono inputProps={{ value: form.vlanid ?? "", onChange: (e) => set({ vlanid: e.target.value }), inputMode: "numeric" }} />
                <Field label="MTU" mono inputProps={{ value: form.mtu ?? "", onChange: (e) => set({ mtu: e.target.value }), inputMode: "numeric" }} />
                <Field label={t("wan.device")} mono inputProps={{ value: form.device ?? "", onChange: (e) => set({ device: e.target.value }) }} />
              </div>
            </div>

            <div className="mt-4 flex gap-2">
              <Button onClick={save} loading={saving}>{t("lan.save")}</Button>
              <Button variant="ghost" onClick={() => { setForm(cfg); setEditing(false); }}>{t("common.cancel")}</Button>
            </div>
          </>
        ) : (
          <>
            <dl className="grid sm:grid-cols-2 gap-x-6 gap-y-2 text-small">
              {cfgRows.map(([k, v]) => (
                <div key={k} className="flex justify-between gap-3">
                  <dt className="text-muted">{k}</dt>
                  <dd className="font-mono text-right break-all">{v}</dd>
                </div>
              ))}
            </dl>
            {cfg.proto === "pppoe" && <p className="text-caption text-muted mt-2">{t("wan.passwordHidden")}</p>}
          </>
        )}
      </Card>

      {fwd && onFwdChange && (
        <div className="md:col-span-2">
          <PortForwardCard probe={fwd} onChange={onFwdChange} />
        </div>
      )}
    </div>
  );
}
