import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Globe, KeyRound, Save } from "lucide-react";
import { api } from "../api";
import type { WANConfig } from "../api";
import type { FwdProbe, WanStatus } from "../types";
import {
  Banner, Button, Card, Field, SegmentedControl, SkeletonRows, useToast,
} from "../components/ui";
import { PortForwardCard } from "../components/ports/PortForwardCard";

const PROTO = ["dhcp", "static", "pppoe"] as const;

/** Página WAN (#243): estado de salida a Internet + configuración + forwards. */
export function WanPage({ fwd, onFwdChange }: {
  fwd?: FwdProbe;
  onFwdChange?: (p: FwdProbe) => void;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [status, setStatus] = useState<WanStatus>();
  const [cfg, setCfg] = useState<WANConfig>();
  const [form, setForm] = useState<WANConfig>({ proto: "dhcp" });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(false);

  const load = () => {
    api.wan().then(setStatus).catch(() => setError(true));
    api.wanConfig().then((c) => { setCfg(c); setForm(c); }).catch(() => setError(true));
  };
  useEffect(load, []);

  const set = (patch: Partial<WANConfig>) => setForm((f) => ({ ...f, ...patch }));

  const save = async () => {
    setSaving(true);
    try {
      const res = await api.setWanConfig(form);
      setCfg(res);
      setForm(res);
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
    uptime: status?.uptime ? t("wan.uptime", { s: status.uptime }) : "—",
  }), [status, t]);

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card index={0} icon={Globe} title={t("wan.title")}>
        {error ? (
          <Banner tone="danger">{t("common.loadError")}</Banner>
        ) : !status ? (
          <SkeletonRows rows={3} />
        ) : (
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <span className={`h-2.5 w-2.5 rounded-full ${status.up ? "bg-ok" : "bg-danger"}`} />
              <span className="text-body font-medium">{status.up ? t("wan.up") : t("wan.down")}</span>
            </div>
            <dl className="grid sm:grid-cols-2 gap-x-6 gap-y-2 text-small">
              <div className="flex justify-between gap-3"><dt className="text-muted">{t("wan.ip")}</dt><dd className="font-mono text-right break-all">{stats.ip}</dd></div>
              <div className="flex justify-between gap-3"><dt className="text-muted">{t("lan.gateway")}</dt><dd className="font-mono text-right break-all">{stats.gateway}</dd></div>
              <div className="flex justify-between gap-3"><dt className="text-muted">{t("wan.dns")}</dt><dd className="font-mono text-right break-all">{stats.dns}</dd></div>
              <div className="flex justify-between gap-3"><dt className="text-muted">{t("wan.uptimeLabel")}</dt><dd className="font-mono text-right">{stats.uptime}</dd></div>
            </dl>
          </div>
        )}
      </Card>

      <Card index={1} icon={Save} title={t("wan.configTitle")}>
        {cfg === undefined ? (
          <SkeletonRows rows={4} />
        ) : (
          <>
            <div className="mb-3">
              <span className="text-caption text-muted block mb-1.5">{t("wan.proto")}</span>
              <SegmentedControl
                ariaLabel={t("wan.proto")}
                value={form.proto}
                onChange={(v) => set({ proto: v as WANConfig["proto"] })}
                options={PROTO.map((p) => ({ value: p, label: t(`wan.proto${p}`) }))}
              />
            </div>

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

            <div className="mt-3 grid sm:grid-cols-3 gap-3">
              <Field label="VLAN ID" mono inputProps={{ value: form.vlanid ?? "", onChange: (e) => set({ vlanid: e.target.value }), inputMode: "numeric" }} />
              <Field label="MTU" mono inputProps={{ value: form.mtu ?? "", onChange: (e) => set({ mtu: e.target.value }), inputMode: "numeric" }} />
              <Field label={t("wan.device")} mono inputProps={{ value: form.device ?? "", onChange: (e) => set({ device: e.target.value }) }} />
            </div>

            <div className="mt-4">
              <Button onClick={save} loading={saving}>{t("lan.save")}</Button>
            </div>
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
