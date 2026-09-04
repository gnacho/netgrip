import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Globe, Network, Pencil } from "lucide-react";
import { api } from "../../api";
import type { LANConfig } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Button, Card, ConfirmDialog,
  Field, Input, SegmentedControl, SettingRow,
} from "../ui";
import { useActionCycle } from "../wifi/action";

const IP_RE = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/;

export function isValidIp(s: string): boolean {
  const m = IP_RE.exec(s.trim());
  return !!m && m.slice(1).every((o) => Number(o) <= 255);
}

const MASKS: Record<string, string> = { "24": "255.255.255.0", "16": "255.255.0.0" };

/** Segundos de concesión DHCP (lease_time viene en segundos, estilo dnsmasq). */
const LEASES = [
  { value: "3600", labelKey: "lan.lease1h" },
  { value: "43200", labelKey: "lan.lease12h" },
  { value: "86400", labelKey: "lan.lease24h" },
  { value: "604800", labelKey: "lan.lease7d" },
];

function fmtLease(t: TFunction, secs: number): string {
  const hit = LEASES.find((l) => l.value === String(secs));
  if (hit) return t(hit.labelKey);
  const h = Math.round(secs / 3600);
  return `${h} h`;
}

/** Card "Las direcciones de tu casa" (lan.md §2): héroe IP + LAN + DHCP en un solo guardado. */
export function LanConfigCard({ cfg, onChange, index = 0 }: {
  cfg: LANConfig;
  onChange: (c: LANConfig) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();

  const [ipaddr, setIpaddr] = useState(cfg.ipaddr);
  const [maskMode, setMaskMode] = useState<"24" | "16" | "custom">("24");
  const [customMask, setCustomMask] = useState(cfg.netmask);
  const [dhcpOn, setDhcpOn] = useState(cfg.dhcp.enabled);
  const [start, setStart] = useState(String(cfg.dhcp.start));
  const [end, setEnd] = useState(String(cfg.dhcp.start + cfg.dhcp.limit - 1));
  const [lease, setLease] = useState(String(cfg.dhcp.lease_time));
  const [gateway, setGateway] = useState(cfg.dhcp.gateway ?? "");
  const [dns1, setDns1] = useState(cfg.dhcp.dns1 ?? "");
  const [dns2, setDns2] = useState(cfg.dhcp.dns2 ?? "");
  const [confirmIp, setConfirmIp] = useState(false);
  const [editingIp, setEditingIp] = useState(false);

  // Re-sincroniza el formulario cuando el padre recarga la config.
  useEffect(() => {
    setIpaddr(cfg.ipaddr);
    const known = Object.entries(MASKS).find(([, v]) => v === cfg.netmask);
    setMaskMode((known?.[0] as "24" | "16" | undefined) ?? "custom");
    setCustomMask(cfg.netmask);
    setDhcpOn(cfg.dhcp.enabled);
    setStart(String(cfg.dhcp.start));
    setEnd(String(cfg.dhcp.start + cfg.dhcp.limit - 1));
    setLease(String(cfg.dhcp.lease_time));
    setGateway(cfg.dhcp.gateway ?? "");
    setDns1(cfg.dhcp.dns1 ?? "");
    setDns2(cfg.dhcp.dns2 ?? "");
  }, [cfg]);

  const netmask = maskMode === "custom" ? customMask.trim() : MASKS[maskMode];
  const startN = Number(start);
  const endN = Number(end);

  const errors = useMemo(() => {
    const e: { ip?: string; mask?: string; range?: string } = {};
    if (!isValidIp(ipaddr)) e.ip = t("lan.invalidIp");
    if (maskMode === "custom" && !isValidIp(customMask)) e.mask = t("lan.invalidMask");
    if (dhcpOn && (!start || !end || !(endN > startN) || startN < 1 || endN > 254)) e.range = t("lan.invalidRange");
    return e;
  }, [ipaddr, maskMode, customMask, dhcpOn, start, end, startN, endN, t]);

  const lanDirty = ipaddr.trim() !== cfg.ipaddr || netmask !== cfg.netmask;
  const dhcpDirty = dhcpOn !== cfg.dhcp.enabled
    || startN !== cfg.dhcp.start
    || (endN - startN + 1) !== cfg.dhcp.limit
    || Number(lease) !== cfg.dhcp.lease_time
    || gateway.trim() !== (cfg.dhcp.gateway ?? "")
    || dns1.trim() !== (cfg.dhcp.dns1 ?? "")
    || dns2.trim() !== (cfg.dhcp.dns2 ?? "");
  const dirty = lanDirty || dhcpDirty;
  const hasErrors = Object.keys(errors).length > 0;
  const ipChanged = ipaddr.trim() !== cfg.ipaddr;

  const save = () => {
    run(async () => {
      let last: Awaited<ReturnType<typeof api.setLan>> | undefined;
      if (lanDirty) {
        last = await api.setLan({ ipaddr: ipaddr.trim(), netmask });
        if (last.status !== "applied") return last;
      }
      if (dhcpDirty) {
        last = await api.setDhcp({
          enabled: dhcpOn,
          start: startN,
          limit: endN - startN + 1,
          lease_time: Number(lease),
          gateway: gateway.trim(),
          dns1: dns1.trim(),
          dns2: dns2.trim(),
        });
      }
      if (!last) throw new Error("nothing to save");
      return last;
    }).then((res) => {
      if (res?.status === "applied") onChange(res.state);
    });
  };

  const leaseKnown = LEASES.some((l) => l.value === lease);

  return (
    <Card index={index} icon={Network} iconTone="teal" title={t("lan.homeTitle")} help="lan">
      {/* Héroe: IP del router */}
      <div className="flex items-center gap-3">
        <div className="flex-1 min-w-0">
          <p className="stat-lg font-mono">{cfg.ipaddr}</p>
          <p className="text-caption text-muted mt-0.5">{t("lan.heroCaption")}</p>
        </div>
        {!editingIp && (
          <Button variant="secondary" size="sm" icon={Pencil} onClick={() => setEditingIp(true)}>
            {t("lan.changeIp")}
          </Button>
        )}
      </div>

      {editingIp && (
        <div className="mt-3">
          <Field label={t("lan.ip")} mono icon={Network} hint={t("lan.ipChangeHint")} error={errors.ip}
            inputProps={{ value: ipaddr, onChange: (e) => setIpaddr(e.target.value), inputMode: "decimal" }} />
          <div className="flex gap-2 mt-2">
            <Button variant="ghost" size="sm" onClick={() => { setEditingIp(false); setIpaddr(cfg.ipaddr); }}>
              {t("common.cancel")}
            </Button>
          </div>
        </div>
      )}

      <div className="mt-1 divide-y divide-border/60">
        <div>
          <div className="flex items-center gap-3 flex-wrap py-2.5">
            <SettingRow
              title={t("lan.dhcpRow")}
              description={t("lan.dhcpRowDesc")}
              help={t("help.dhcp.body")}
              helpTitle={t("help.dhcp.title")}
              checked={dhcpOn}
              onChange={setDhcpOn}
            />
            {dhcpOn && (
              <div className="flex items-center gap-2 shrink-0">
                <span className="text-small text-muted">{t("lan.rangeFrom")}</span>
                <span className="text-small text-muted font-mono">.</span>
                <Input mono className="!w-16" inputMode="numeric" value={start}
                  onChange={(e) => setStart(e.target.value)} aria-label={t("lan.dhcpStart")} />
                <span className="text-small text-muted">{t("lan.rangeTo")}</span>
                <span className="text-small text-muted font-mono">.</span>
                <Input mono className="!w-16" inputMode="numeric" value={end}
                  onChange={(e) => setEnd(e.target.value)} aria-label={t("lan.dhcpEnd")} />
                {errors.range && <span className="text-caption text-danger">{errors.range}</span>}
              </div>
            )}
          </div>
          {dhcpOn && (
            <div className="pb-3 flex flex-col gap-2">
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <div className="flex items-center gap-2">
                  <span className="text-small text-muted">{t("lan.lease")}</span>
                  <SegmentedControl
                    size="sm"
                    ariaLabel={t("lan.lease")}
                    value={lease}
                    onChange={setLease}
                    options={leaseKnown
                      ? LEASES.map((l) => ({ value: l.value, label: t(l.labelKey) }))
                      : [...LEASES.map((l) => ({ value: l.value, label: t(l.labelKey) })),
                         { value: lease, label: fmtLease(t, Number(lease)) }]}
                  />
                </div>
                <AdvancedDisclosure label={t("lan.advancedDhcp")}>
                  <div className="grid gap-3 sm:grid-cols-3">
                    <Field label={t("lan.gateway")} mono hint={t("lan.gatewayHint")}
                      inputProps={{ value: gateway, onChange: (e) => setGateway(e.target.value), placeholder: t("lan.optional") }} />
                    <Field label={t("lan.dns1")} mono icon={Globe}
                      inputProps={{ value: dns1, onChange: (e) => setDns1(e.target.value), placeholder: t("lan.optional") }} />
                    <Field label={t("lan.dns2")} mono icon={Globe}
                      inputProps={{ value: dns2, onChange: (e) => setDns2(e.target.value), placeholder: t("lan.optional") }} />
                  </div>
                </AdvancedDisclosure>
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="mt-3 flex flex-col gap-3">
        {phase && (
          <ActionBanner
            phase={phase}
            text={phase === "done" ? t("lan.saved") : phase === "failed" ? t("lan.rolledBack") : undefined}
            detail={detail}
            onDone={clear}
          />
        )}
        <div>
          <Button onClick={() => (ipChanged ? setConfirmIp(true) : save())}
            disabled={!dirty || hasErrors} loading={busy}>
            {t("lan.save")}
          </Button>
        </div>
      </div>

      <ConfirmDialog
        open={confirmIp}
        onClose={() => setConfirmIp(false)}
        onConfirm={() => { setConfirmIp(false); save(); }}
        title={t("lan.ipConfirmTitle")}
        consequence={t("lan.ipConfirmConsequence")}
        confirmLabel={t("lan.ipConfirmLabel")}
      />
    </Card>
  );
}
