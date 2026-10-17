import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, Clock, Globe, Home, Plus, Router, ShieldAlert, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { FirewallProbe, FWRule, FirewallRuleAdd, FWZone } from "../../types";
import {
  ActionBanner, Banner, Button, Card, ConfirmDialog, EmptyState, Field, Modal,
  Pill, SegmentedControl, SkeletonRows, type PillTone,
} from "../ui";
import { useActionCycle } from "../wifi/action";

const TARGET_LABELS: Record<string, string> = {
  ACCEPT: "trafficRules.allow",
  DROP: "trafficRules.deny",
  REJECT: "trafficRules.reject",
};

function toneFor(v: string): PillTone {
  if (v === "ACCEPT") return "ok";
  if (v === "REJECT" || v === "DROP") return "danger";
  return "muted";
}

/* ── Anti lock-out (espejo del backend lockoutRisk) ─────────────────── */

function ipv4ToInt(ip: string): number {
  const parts = ip.split(".").map(Number);
  if (parts.length !== 4 || parts.some((n) => Number.isNaN(n) || n < 0 || n > 255)) return NaN;
  return ((parts[0] << 24) >>> 0) + (parts[1] << 16) + (parts[2] << 8) + parts[3];
}

function cidrToRange(cidr: string): { start: number; end: number } | null {
  const [ip, bitsStr] = cidr.split("/");
  const bits = Number(bitsStr);
  if (!Number.isInteger(bits) || bits < 0 || bits > 32) return null;
  const start = ipv4ToInt(ip);
  if (Number.isNaN(start)) return null;
  const mask = bits === 0 ? 0 : (0xffffffff << (32 - bits)) >>> 0;
  const netStart = (start & mask) >>> 0;
  const end = (netStart | (~mask >>> 0)) >>> 0;
  return { start: netStart, end };
}

function cidrContains(outer: string, inner: string): boolean {
  const o = cidrToRange(outer);
  const i = cidrToRange(inner);
  if (!o || !i) return false;
  return o.start <= i.start && i.end <= o.end;
}

function lockoutRisk(rule: FirewallRuleAdd, mgmtCidr?: string): boolean {
  if (rule.target !== "DROP" && rule.target !== "REJECT") return false;
  if (rule.dest_ip || rule.dest_port || rule.src_port) return false;
  const src = (rule.src_ip ?? "").trim();
  if (!src || src === "0.0.0.0/0" || src === "0.0.0.0") return true;
  if (mgmtCidr && cidrContains(src, mgmtCidr)) return true;
  return false;
}

/* ── Diagrama de zonas (solo lectura) ───────────────────────────────── */

function ZoneDiagram({ zones }: { zones: FWZone[] }) {
  const { t } = useTranslation();
  const wan = zones.find((z) => z.name === "wan");
  const lan = zones.find((z) => z.name === "lan");
  const inP = wan?.input ?? "-";
  const outP = wan?.output ?? "-";
  const fwdP = wan?.forward ?? "-";
  const homeIn = lan?.input ?? "-";
  const homeFwd = lan?.forward ?? "-";
  const policyColor = (p: string) => p === "ACCEPT" ? "var(--color-ok)" : p === "REJECT" || p === "DROP" ? "var(--color-danger)" : "var(--color-muted)";
  const policyLabel = (p: string) => t(TARGET_LABELS[p] ?? "", { defaultValue: p });
  const dot = (label: string, p: string) => (
    <span key={label} className="inline-flex items-center gap-1 text-[10px] text-faint">
      <span className="h-2 w-2 rounded-full" style={{ background: policyColor(p) }} aria-hidden="true" />
      {label}
    </span>
  );
  const Connector = ({ p, label }: { p: string; label: string }) => (
    <div className="flex flex-1 flex-col items-center justify-center gap-1 px-1 min-w-0">
      <div className="rounded-full border px-2 py-0.5 text-[10px] font-semibold"
        style={{ color: policyColor(p), borderColor: policyColor(p), background: "var(--color-surface-2)" }}>
        {policyLabel(p)}
      </div>
      <ArrowRight size={16} className="text-faint shrink-0" aria-hidden="true" />
      <span className="text-[10px] text-faint">{label}</span>
    </div>
  );
  const ZoneCard = ({ icon: Icon, title, network, dots, tone }: {
    icon: typeof Globe; title: string; network: string; dots: React.ReactNode; tone?: "accent";
  }) => (
    <div className={`flex-1 min-w-0 rounded-lg border px-3 py-2.5 ${tone ? "border-accent bg-accent-soft" : "border-border-strong bg-surface-2"}`}>
      <div className="flex items-center gap-2">
        <span className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${tone ? "bg-accent text-accent" : "bg-surface text-muted"}`} aria-hidden="true">
          <Icon size={15} />
        </span>
        <span className={`truncate text-small font-semibold ${tone ? "text-accent" : "text-text"}`}>{title}</span>
      </div>
      <p className="mt-1 font-mono text-caption text-muted truncate">{network}</p>
      <div className="mt-1 flex flex-wrap gap-x-3">{dots}</div>
    </div>
  );

  return (
    <div className="flex items-stretch gap-1" role="img"
      aria-label={`${t("trafficRules.diagramInternet")} → ${t("trafficRules.diagramRouter")} → ${t("trafficRules.diagramHome")}`}>
      <ZoneCard icon={Globe} title={t("trafficRules.diagramInternet")} network={wan?.network.join(", ") || "wan"}
        dots={<>{dot("in", inP)}{dot("out", outP)}{dot("fwd", fwdP)}</>} />
      <Connector p={inP} label={t("trafficRules.policyIn", { policy: policyLabel(inP) })} />
      <ZoneCard icon={Router} title={t("trafficRules.diagramRouter")} tone="accent"
        network={wan?.masq ? `NAT · ${policyLabel(inP)} ← wan` : policyLabel(inP)}
        dots={<>{dot("in", inP)}{dot("fwd", fwdP)}</>} />
      <Connector p={homeIn} label={t("trafficRules.policyIn", { policy: policyLabel(homeIn) })} />
      <ZoneCard icon={Home} title={t("trafficRules.diagramHome")} network={lan?.network.join(", ") || "lan"}
        dots={<>{dot("in", homeIn)}{dot("fwd", homeFwd)}</>} />
    </div>
  );
}

/* ── Editor de reglas de tráfico ────────────────────────────────────── */

export function TrafficRulesCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<FirewallProbe>();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [modal, setModal] = useState(false);
  const [toDelete, setToDelete] = useState<FWRule>();

  useEffect(() => { api.firewall().then(setProbe).catch(() => {}); }, []);

  const managed = probe?.rules.filter((r) => r.managed) ?? [];
  const systemRules = probe?.rules.filter((r) => !r.managed) ?? [];

  const addRule = (rule: FirewallRuleAdd) => {
    run(() => api.addFirewallRule(rule)).then((res) => {
      if (res?.status === "applied") {
        setProbe(res.state);
        setModal(false);
      }
    });
  };

  const delRule = (section: string) => {
    run(() => api.deleteFirewallRule(section)).then((res) => {
      if (res) setProbe(res.state);
    });
  };

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card
        index={0}
        icon={ShieldAlert}
        iconTone="ok"
        title={t("trafficRules.title")}
        help="firewall"
        action={
          probe?.applicable && (
            <Button size="sm" icon={Plus} onClick={() => setModal(true)}>{t("trafficRules.newRule")}</Button>
          )
        }
      >
        {!probe ? (
          <SkeletonRows rows={3} />
        ) : !probe.applicable ? (
          <Banner tone="warn">{t("trafficRules.notApplicable")}</Banner>
        ) : (
          <>
            <ZoneDiagram zones={probe.zones ?? []} />

            <div className="flex flex-wrap gap-2 mt-3">
              {(probe.zones ?? []).map((z) => (
                <span key={z.name} className="inline-flex items-center gap-1.5 rounded-full bg-surface-2 border border-border/60 px-2.5 py-1 text-caption">
                  <span className="font-semibold">{z.name}</span>
                  <span className="text-muted font-mono">{(z.network ?? []).join(", ") || "-"}</span>
                  <Pill tone={toneFor(z.input)}>{t(TARGET_LABELS[z.input] ?? "trafficRules.allow", { defaultValue: z.input })}</Pill>
                  {z.masq && <Pill tone="warn">NAT</Pill>}
                </span>
              ))}
            </div>

            {phase && (
              <div className="mt-3">
                <ActionBanner phase={phase} detail={detail} onDone={clear} />
              </div>
            )}

            <div className="mt-4">
              <p className="text-eyebrow text-faint mb-2">{t("trafficRules.rulesTitle", { count: managed.length })}</p>
              {managed.length === 0 && systemRules.length === 0 ? (
                <EmptyState
                  small
                  title={t("trafficRules.empty")}
                  action={<Button size="sm" icon={Plus} onClick={() => setModal(true)}>{t("trafficRules.firstRule")}</Button>}
                />
              ) : (
                <ul className="divide-y divide-border/50">
                  {managed.map((r) => <RuleRow key={r.section} r={r} onDelete={() => setToDelete(r)} busy={busy} />)}
                  {systemRules.length > 0 && (
                    <li className="pt-3 mt-1">
                      <p className="text-caption text-muted">{t("trafficRules.systemRules")}</p>
                      <ul className="divide-y divide-border/40">
                        {systemRules.map((r) => <RuleRow key={r.section} r={r} readOnly busy={busy} />)}
                      </ul>
                    </li>
                  )}
                </ul>
              )}
            </div>
          </>
        )}
      </Card>

      <NewTrafficRuleModal
        open={modal}
        mgmtCidr={probe?.mgmt_cidr}
        busy={busy}
        onClose={() => setModal(false)}
        onConfirm={addRule}
      />

      <ConfirmDialog
        open={!!toDelete}
        onClose={() => setToDelete(undefined)}
        onConfirm={() => { const r = toDelete; setToDelete(undefined); if (r) delRule(r.section); }}
        title={t("trafficRules.deleteTitle", { name: toDelete?.name || "" })}
        consequence={t("trafficRules.deleteConsequence")}
        confirmLabel={t("trafficRules.deleteConfirm")}
        busy={busy}
      />
    </div>
  );
}

function RuleRow({ r, readOnly = false, onDelete, busy }: {
  r: FWRule;
  readOnly?: boolean;
  onDelete?: () => void;
  busy: boolean;
}) {
  const { t } = useTranslation();
  const src = r.src_ip ? r.src_ip : t("trafficRules.any");
  const dest = r.dest_ip ? `${r.dest_ip}${r.dest_port ? `:${r.dest_port}` : ""}` : r.dest_port ? `*:${r.dest_port}` : t("trafficRules.any");
  const schedule = useMemo(() => {
    const parts: string[] = [];
    if (r.start_time || r.stop_time) parts.push(`${r.start_time || "00:00"}-${r.stop_time || "23:59"}`);
    if (r.weekdays) parts.push(r.weekdays);
    return parts.join(" · ");
  }, [r.start_time, r.stop_time, r.weekdays]);

  return (
    <li className="flex items-center gap-2.5 py-2.5 flex-wrap animate-fade-up">
      <span className="text-body font-medium min-w-0 truncate flex-1">{r.name || r.section}</span>
      <Pill tone="muted">{t("trafficRules.from")} {src}</Pill>
      <Pill tone="muted">{t("trafficRules.to")} {dest}</Pill>
      {r.proto && <span className="font-mono text-caption text-muted uppercase">{r.proto}</span>}
      <Pill tone={toneFor(r.target)}>{t(TARGET_LABELS[r.target] ?? "trafficRules.allow", { defaultValue: r.target })}</Pill>
      {schedule && (
        <span className="inline-flex items-center gap-1 text-caption text-muted">
          <Clock size={12} aria-hidden="true" />{schedule}
        </span>
      )}
      {!r.enabled && <Pill tone="muted">{t("trafficRules.disabled")}</Pill>}
      {readOnly ? (
        <span title={t("trafficRules.externalHint")}><Pill tone="muted">{t("trafficRules.system")}</Pill></span>
      ) : (
        <button
          type="button"
          onClick={onDelete}
          disabled={busy}
          aria-label={`${t("trafficRules.deleteConfirm")} ${r.name}`}
          className="text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1"
        >
          <Trash2 size={16} />
        </button>
      )}
    </li>
  );
}

/* ── Modal "Nueva regla" ────────────────────────────────────────────── */

const DAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

function NewTrafficRuleModal({ open, mgmtCidr, busy, onClose, onConfirm }: {
  open: boolean;
  mgmtCidr?: string;
  busy: boolean;
  onClose: () => void;
  onConfirm: (rule: FirewallRuleAdd) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [target, setTarget] = useState("ACCEPT");
  const [srcMode, setSrcMode] = useState<"any" | "ip">("any");
  const [srcIp, setSrcIp] = useState("");
  const [destIp, setDestIp] = useState("");
  const [destPort, setDestPort] = useState("");
  const [proto, setProto] = useState("");
  const [startTime, setStartTime] = useState("");
  const [stopTime, setStopTime] = useState("");
  const [days, setDays] = useState<Set<string>>(new Set());
  const [utc, setUtc] = useState(false);

  const rule: FirewallRuleAdd = {
    name,
    proto,
    src_ip: srcMode === "any" ? "" : srcIp.trim(),
    dest_ip: destIp.trim(),
    dest_port: destPort.trim(),
    target,
    start_time: startTime.trim(),
    stop_time: stopTime.trim(),
    weekdays: [...days].join(" "),
    utc_time: utc,
  };

  const risk = lockoutRisk(rule, mgmtCidr);

  const validName = name.trim() !== "";
  const validSrc = srcMode === "any" || srcIp.trim() !== "";
  const portOk = (p: string) => !p || /^(\d{1,5})(-\d{1,5})?$/.test(p);
  const valid = validName && validSrc && portOk(destPort);

  const toggleDay = (d: string) => {
    setDays((prev) => {
      const next = new Set(prev);
      if (next.has(d)) next.delete(d); else next.add(d);
      return next;
    });
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("trafficRules.modalTitle")}
      wide
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={() => onConfirm(rule)} disabled={!valid} loading={busy}>
            {t("trafficRules.save")}
          </Button>
        </>
      }
    >
      {risk && (
        <Banner tone="warn" className="mb-4">{t("trafficRules.lockoutWarning")}</Banner>
      )}

      <Field label={t("trafficRules.description")}
        inputProps={{ value: name, onChange: (e) => setName(e.target.value), maxLength: 60, placeholder: t("trafficRules.descriptionPlaceholder") }} />

      <div className="mt-4">
        <p className="text-caption font-medium text-muted mb-1.5">{t("trafficRules.action")}</p>
        <SegmentedControl
          size="sm"
          ariaLabel={t("trafficRules.action")}
          value={target}
          onChange={setTarget}
          options={[
            { value: "ACCEPT", label: t("trafficRules.allow") },
            { value: "DROP", label: t("trafficRules.deny") },
            { value: "REJECT", label: t("trafficRules.reject") },
          ]}
        />
      </div>

      <div className="mt-4">
        <p className="text-caption font-medium text-muted mb-1.5">{t("trafficRules.from")}</p>
        <SegmentedControl
          size="sm"
          ariaLabel={t("trafficRules.from")}
          value={srcMode}
          onChange={setSrcMode}
          options={[
            { value: "any", label: t("trafficRules.any") },
            { value: "ip", label: "IP/CIDR" },
          ]}
        />
        {srcMode === "ip" && (
          <div className="mt-2">
            <Field mono label={t("trafficRules.srcIp")}
              inputProps={{ value: srcIp, onChange: (e) => setSrcIp(e.target.value), placeholder: "192.168.1.0/24" }} />
          </div>
        )}
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3">
        <Field mono label={t("trafficRules.destIp")} hint={t("trafficRules.destIpHint")}
          inputProps={{ value: destIp, onChange: (e) => setDestIp(e.target.value), placeholder: "10.0.0.9" }} />
        <Field mono label={t("trafficRules.destPort")} hint={t("trafficRules.destPortHint")}
          inputProps={{ value: destPort, onChange: (e) => setDestPort(e.target.value), placeholder: "80-443", inputMode: "numeric" }} />
      </div>

      <div className="mt-4">
        <p className="text-caption font-medium text-muted mb-1.5">{t("trafficRules.proto")}</p>
        <SegmentedControl
          size="sm"
          ariaLabel={t("trafficRules.proto")}
          value={proto}
          onChange={setProto}
          options={[
            { value: "", label: t("trafficRules.any") },
            { value: "tcp", label: "TCP" },
            { value: "udp", label: "UDP" },
          ]}
        />
      </div>

      <div className="mt-4">
        <p className="text-caption font-medium text-muted mb-1.5">{t("trafficRules.schedule")}</p>
        <div className="grid grid-cols-2 gap-3">
          <Field mono label={t("trafficRules.startTime")}
            inputProps={{ value: startTime, onChange: (e) => setStartTime(e.target.value), placeholder: "22:00" }} />
          <Field mono label={t("trafficRules.stopTime")}
            inputProps={{ value: stopTime, onChange: (e) => setStopTime(e.target.value), placeholder: "07:00" }} />
        </div>
        <div className="flex flex-wrap gap-1.5 mt-3">
          {DAYS.map((d) => (
            <button
              key={d}
              type="button"
              onClick={() => toggleDay(d)}
              aria-pressed={days.has(d)}
              className={`h-8 px-2 rounded-md text-caption font-medium ring-focus transition-colors duration-[var(--dur-fast)]
                ${days.has(d) ? "bg-accent-soft text-accent border border-accent" : "bg-surface-2 text-muted border border-border hover:text-text"}`}
            >
              {d}
            </button>
          ))}
        </div>
        <label className="flex items-center gap-2 mt-3 text-caption text-muted cursor-pointer">
          <input type="checkbox" checked={utc} onChange={(e) => setUtc(e.target.checked)} className="accent-[var(--color-accent)]" />
          {t("trafficRules.utc")}
        </label>
      </div>
    </Modal>
  );
}
