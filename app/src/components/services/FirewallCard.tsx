import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, ShieldAlert, Trash2, Globe, Router, Home, ArrowRight } from "lucide-react";
import { api } from "../../api";
import type { FirewallProbe, FWRule, FWZone } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Button, Card, ConfirmDialog, Input,
  Pill, SkeletonRows, type PillTone,
} from "../ui";
import { useActionCycle } from "../wifi/action";

/** Selects con el mismo estilo "filled" del Input de foundations (§4):
 *  fill + borde transparente en reposo, anillo accent al focus. */
const SELECT_CLS = `h-[var(--input-h)] rounded-[10px] border border-transparent bg-fill px-2 text-small
  outline-none transition-[background-color,border-color,box-shadow] duration-[var(--dur-fast)]
  hover:border-border-strong focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-accent)]`;

/**
 * Cortafuegos (services.md §9): informativo, sin toggle global (no existe en
 * la API). Mini-diagrama SVG Internet → Router → Tu casa con la política de
 * entrada real de la zona wan; zonas como chips; reglas bajo disclosure con
 * alta validada y borrado con ConfirmDialog.
 */
export function FirewallCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<FirewallProbe>();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [showAdd, setShowAdd] = useState(false);
  const [delTarget, setDelTarget] = useState<FWRule>();
  const [form, setForm] = useState({ name: "", src: "wan", dest: "", proto: "tcp", dest_port: "", target: "ACCEPT" });

  useEffect(() => { api.firewall().then(setProbe).catch(() => {}); }, []);

  if (probe && !probe.applicable) return null;

  const addRule = async () => {
    setDoneMsg(undefined);
    const res = await run(() => api.addFirewallRule(form));
    if (res) {
      setProbe(res.state);
      if (res.status === "applied") {
        setShowAdd(false);
        setForm({ name: "", src: "wan", dest: "", proto: "tcp", dest_port: "", target: "ACCEPT" });
        setDoneMsg(t("firewall.ruleAdded"));
      }
    }
  };

  const delRule = async (section: string) => {
    setDoneMsg(undefined);
    const res = await run(() => api.deleteFirewallRule(section));
    if (res) {
      setProbe(res.state);
      if (res.status === "applied") setDoneMsg(t("firewall.ruleDeleted"));
    }
  };

  return (
    <Card index={index}
      title={t("firewall.title")} icon={ShieldAlert} iconTone="ok" help="firewall"
      action={probe && <Pill tone="ok">{t("firewall.protecting")}</Pill>}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <ZoneDiagram zones={probe.zones ?? []} />

          {/* Zonas como chips */}
          <div className="flex flex-wrap gap-2 mt-3">
            {(probe.zones ?? []).map((z) => (
              <span key={z.name} className="inline-flex items-center gap-1.5 rounded-full bg-surface-2 border border-border/60 px-2.5 py-1 text-caption">
                <span className="font-semibold">{z.name}</span>
                <span className="text-muted font-mono">{(z.network ?? []).join(", ") || "—"}</span>
                <Pill tone={toneFor(z.input)}>{t(TARGET_LABELS[z.input] ?? "firewall.accept", { defaultValue: z.input })}</Pill>
                {z.masq && <Pill tone="warn">NAT</Pill>}
              </span>
            ))}
          </div>

          {phase && (
            <div className="mt-3">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          <AdvancedDisclosure label={t("firewall.rules")} className="mt-2">
            <div className="flex flex-col gap-2">
              {(probe.rules ?? []).length === 0 ? (
                <p className="text-small text-muted py-1">{t("firewall.noRules")}</p>
              ) : (
                <>
                  <div className="hidden sm:grid grid-cols-[1fr_auto_auto_auto_auto_auto] gap-x-3 text-caption text-faint px-1">
                    <span>{t("firewall.colName")}</span>
                    <span>{t("firewall.colFrom")}</span>
                    <span>{t("firewall.colTo")}</span>
                    <span>{t("firewall.colPort")}</span>
                    <span>{t("firewall.colAction")}</span>
                    <span />
                  </div>
                  <ul>
                    {(probe.rules ?? []).map((r) => (
                      <li key={r.section} className="grid grid-cols-[1fr_auto] sm:grid-cols-[1fr_auto_auto_auto_auto_auto] items-center gap-x-3 gap-y-1 py-2 px-1 border-b border-border/60 last:border-0 text-small">
                        <span className="font-medium truncate">{r.name || r.section}</span>
                        <span className="flex items-center gap-1 justify-self-end sm:justify-self-start">
                          <Pill tone="muted">{r.src || "*"}</Pill>
                        </span>
                        <span className="hidden sm:flex items-center gap-1">
                          <Pill tone="muted">{r.dest || "*"}</Pill>
                        </span>
                        <span className="hidden sm:block font-mono text-caption text-muted">{r.proto} {r.dest_port}</span>
                        <span className="hidden sm:block"><Pill tone={toneFor(r.target)}>{t(TARGET_LABELS[r.target] ?? "firewall.accept", { defaultValue: r.target })}</Pill></span>
                        <button
                          type="button"
                          onClick={() => setDelTarget(r)}
                          title={t("firewall.confirmDel")}
                          aria-label={t("firewall.confirmDel")}
                          className="inline-flex h-8 w-8 items-center justify-center justify-self-end rounded-sm text-muted hover:text-danger hover:bg-surface-2 ring-focus transition-colors"
                        >
                          <Trash2 size={16} aria-hidden="true" />
                        </button>
                      </li>
                    ))}
                  </ul>
                </>
              )}

              {showAdd ? (
                <div className="rounded-md border border-border bg-surface-2 p-3 grid grid-cols-2 gap-3">
                  <div className="col-span-2">
                    <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
                      placeholder={t("firewall.ruleName")} maxLength={40} />
                  </div>
                  <select value={form.src} onChange={(e) => setForm({ ...form, src: e.target.value })}
                    aria-label={t("firewall.colFrom")}
                    className={SELECT_CLS}>
                    <option value="wan">wan</option>
                    <option value="lan">lan</option>
                    {(probe.zones ?? []).filter((z) => z.name !== "wan" && z.name !== "lan").map((z) => (
                      <option key={z.name} value={z.name}>{z.name}</option>
                    ))}
                  </select>
                  <select value={form.proto} onChange={(e) => setForm({ ...form, proto: e.target.value })}
                    aria-label="proto"
                    className={SELECT_CLS}>
                    <option value="tcp">TCP</option>
                    <option value="udp">UDP</option>
                    <option value="tcp udp">TCP+UDP</option>
                  </select>
                  <Input mono value={form.dest_port} onChange={(e) => setForm({ ...form, dest_port: e.target.value })}
                    placeholder={t("firewall.destPort")} inputMode="numeric" />
                  <select value={form.target} onChange={(e) => setForm({ ...form, target: e.target.value })}
                    aria-label={t("firewall.colAction")}
                    className={SELECT_CLS}>
                    <option value="ACCEPT">ACCEPT</option>
                    <option value="REJECT">REJECT</option>
                    <option value="DROP">DROP</option>
                  </select>
                  <div className="col-span-2 flex justify-end gap-2">
                    <Button variant="ghost" size="sm" onClick={() => setShowAdd(false)}>{t("common.cancel")}</Button>
                    <Button size="sm" loading={busy} disabled={!form.name.trim() || !form.dest_port.trim()} onClick={addRule}>
                      {t("firewall.add")}
                    </Button>
                  </div>
                </div>
              ) : (
                <div>
                  <Button variant="secondary" size="sm" icon={Plus} onClick={() => setShowAdd(true)}>
                    {t("firewall.addRule")}
                  </Button>
                </div>
              )}
            </div>
          </AdvancedDisclosure>

          <ConfirmDialog
            open={!!delTarget}
            onClose={() => setDelTarget(undefined)}
            onConfirm={() => {
              const r = delTarget;
              setDelTarget(undefined);
              if (r) delRule(r.section);
            }}
            title={t("firewall.deleteTitle", { name: delTarget?.name || delTarget?.section || "" })}
            consequence={t("firewall.deleteConsequence")}
            confirmLabel={t("firewall.deleteConfirm")}
            busy={busy}
          />
        </>
      )}
    </Card>
  );
}

function toneFor(v: string): PillTone {
  if (v === "ACCEPT") return "ok";
  if (v === "REJECT" || v === "DROP") return "danger";
  return "muted";
}

const TARGET_LABELS: Record<string, string> = {
  ACCEPT: "firewall.accept",
  REJECT: "firewall.reject",
  DROP: "firewall.drop",
};

function ZoneDiagram({ zones }: { zones: FWZone[] }) {
  const { t } = useTranslation();
  const wan = zones.find((z) => z.name === "wan");
  const lan = zones.find((z) => z.name === "lan");
  const inP = wan?.input ?? "—";
  const outP = wan?.output ?? "—";
  const fwdP = wan?.forward ?? "—";
  const homeIn = lan?.input ?? "—";
  const homeFwd = lan?.forward ?? "—";
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
      aria-label={`${t("firewall.diagramInternet")} → ${t("firewall.diagramRouter")} → ${t("firewall.diagramHome")}`}>
      <ZoneCard icon={Globe} title={t("firewall.diagramInternet")} network={wan?.network.join(", ") || "wan"}
        dots={<>{dot("in", inP)}{dot("out", outP)}{dot("fwd", fwdP)}</>} />

      <Connector p={inP} label={t("firewall.policyIn", { policy: policyLabel(inP) })} />

      <ZoneCard icon={Router} title={t("firewall.diagramRouter")} tone="accent"
        network={wan?.masq ? `NAT · ${policyLabel(inP)} ← wan` : policyLabel(inP)}
        dots={<>{dot("in", inP)}{dot("fwd", fwdP)}</>} />

      <Connector p={homeIn} label={t("firewall.policyIn", { policy: policyLabel(homeIn) })} />

      <ZoneCard icon={Home} title={t("firewall.diagramHome")} network={lan?.network.join(", ") || "lan"}
        dots={<>{dot("in", homeIn)}{dot("fwd", homeFwd)}</>} />
    </div>
  );
}
