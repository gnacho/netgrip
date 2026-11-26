import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  ArrowRight, Clapperboard, DoorOpen, Gamepad2, HardDrive, Pencil, Plus, Trash2, Video,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { api } from "../../api";
import type { Client, FwdProbe, FwdRule } from "../../types";
import {
  ActionBanner, Banner, Button, Card, ConfirmDialog, EmptyState,
  Field, Input, Modal, Pill, SegmentedControl, SkeletonRows,
} from "../ui";
import { IlluPlug } from "../ui/illustrations";
import { useActionCycle } from "../wifi/action";
import { isValidIp } from "../lan/LanConfigCard";

interface FwdTemplate {
  id: string;
  icon: LucideIcon;
  port: string;
  proto: string;
  nameKey: string;
  descKey: string;
}

/** Set curado de plantillas comunes (ports.md §3). Sin API: rellenan el modal. */
const TEMPLATES: FwdTemplate[] = [
  { id: "nas", icon: HardDrive, port: "443", proto: "tcp", nameKey: "fwd.tplNas", descKey: "fwd.tplNasDesc" },
  { id: "minecraft", icon: Gamepad2, port: "25565", proto: "tcp", nameKey: "fwd.tplMinecraft", descKey: "fwd.tplMinecraftDesc" },
  { id: "jellyfin", icon: Clapperboard, port: "8096", proto: "tcp", nameKey: "fwd.tplJellyfin", descKey: "fwd.tplJellyfinDesc" },
  { id: "camera", icon: Video, port: "554", proto: "tcpudp", nameKey: "fwd.tplCamera", descKey: "fwd.tplCameraDesc" },
  { id: "custom", icon: Pencil, port: "", proto: "tcp", nameKey: "fwd.tplCustom", descKey: "fwd.tplCustomDesc" },
];

function ruleName(t: TFunction, r: FwdRule): string {
  return r.name || t("fwd.unnamed", { port: r.src_dport });
}

/** Card "Abrir algo a Internet" (ports.md §2) + plantillas rápidas (§3). */
export function PortForwardCard({ probe, onChange }: {
  probe: FwdProbe | undefined;
  onChange: (p: FwdProbe) => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [modal, setModal] = useState<FwdTemplate>();
  const [editing, setEditing] = useState<FwdRule>();
  const [toDelete, setToDelete] = useState<FwdRule>();

  const applicable = !!probe?.has_wan && !!probe?.firewall;

  const addRule = (srcDport: string, destIP: string, destPort: string, proto: string) => {
    run(() => api.addFwdRule(srcDport, destIP, destPort, proto)).then((res) => {
      if (res?.status === "applied") {
        onChange(res.state);
        setModal(undefined);
      }
    });
  };

  const updateRule = (section: string, srcDport: string, destIP: string, destPort: string, proto: string) => {
    run(() => api.updateFwdRule(section, srcDport, destIP, destPort, proto)).then((res) => {
      if (res?.status === "applied") {
        onChange(res.state);
        setEditing(undefined);
      }
    });
  };

  const deleteRule = (r: FwdRule) => {
    run(() => api.deleteFwdRule(r.section)).then(async (res) => {
      if (res?.status === "applied") onChange(res.state);
      else onChange(await api.portforward().catch(() => probe!));
    });
  };

  return (
    <>
      <Card
        index={0}
        icon={DoorOpen}
        title={t("fwd.openTitle")}
        help="portforward"
        className={!probe || applicable ? "" : "opacity-60"}
        action={
          <span className="flex items-center gap-2">
            {probe && applicable && <Pill tone="accent">{t("fwd.rules", { count: probe.rules.length })}</Pill>}
            {probe && applicable && (
              <Button size="sm" icon={Plus} onClick={() => setModal(TEMPLATES[0])}>{t("fwd.newRule")}</Button>
            )}
          </span>
        }
      >
        {!probe ? (
          <SkeletonRows rows={3} />
        ) : !applicable ? (
          <Banner tone="warn">{t("fwd.noWan")}</Banner>
        ) : (
          <>
            {probe.rules.length === 0 ? (
              <>
                <EmptyState
                  illustration={<IlluPlug size={120} />}
                  title={t("fwd.emptyHero")}
                  action={<Button icon={Plus} onClick={() => setModal(TEMPLATES[0])}>{t("fwd.firstRule")}</Button>}
                />
                <p className="text-caption text-muted text-center max-w-md mx-auto -mt-2">{t("fwd.vpnHint")}</p>
              </>
            ) : (
              <div className="divide-y divide-border/50">
                {probe.rules.map((r) => (
                  <div key={r.section} className="flex items-center gap-2.5 py-2.5 animate-fade-up flex-wrap">
                    <span title={ruleName(t, r)} className="text-body font-medium min-w-0 truncate">{ruleName(t, r)}</span>
                    <span className="font-mono text-caption bg-surface-2 border border-border rounded-sm px-1.5 py-0.5">
                      :{r.src_dport}
                    </span>
                    <ArrowRight size={14} className="text-faint shrink-0" aria-hidden="true" />
                    {/* A forward lands on a host; an input rule ends on the
                        router itself and has no destination to show. */}
                    <span className="font-mono text-small">
                      {r.kind === "input" ? t("fwd.thisRouter") : `${r.dest_ip}:${r.dest_port}`}
                    </span>
                    <span className="font-mono text-caption text-muted uppercase bg-surface-2 border border-border rounded-sm px-1.5 py-0.5">
                      {r.proto}
                    </span>
                    <span className="flex-1" />
                    {/* Editar va antes del estado (#617): es la accion, y asi
                        no queda escondida detras de las pildoras. Cualquier
                        reenvio se puede editar, tambien los de LuCI; borrar
                        sigue siendo solo de los nuestros (#613). */}
                    {r.kind === "forward" && (
                      <button
                        type="button"
                        onClick={() => setEditing(r)}
                        disabled={busy}
                        aria-label={`${t("fwd.edit")} ${ruleName(t, r)}`}
                        className="text-faint hover:text-accent transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1"
                      >
                        <Pencil size={16} />
                      </button>
                    )}
                    <Pill tone="ok">{t("fwd.active")}</Pill>
                    {/* Only what NetGrip created is removable from here;
                        anything set up elsewhere is shown, not touched. */}
                    {r.managed === false ? (
                      <span title={t("fwd.externalHint")}>
                        <Pill tone="muted">{t("fwd.external")}</Pill>
                      </span>
                    ) : (
                      <button
                        type="button"
                        onClick={() => setToDelete(r)}
                        disabled={busy}
                        aria-label={`${t("fwd.delete")} ${ruleName(t, r)}`}
                        className="text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1"
                      >
                        <Trash2 size={16} />
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}

            {phase && (
              <div className="mt-3">
                <ActionBanner
                  phase={phase}
                  text={phase === "done" ? t("fwd.applied") : phase === "failed" ? t("fwd.rolledBack") : undefined}
                  detail={detail}
                  onDone={clear}
                />
              </div>
            )}
          </>
        )}
      </Card>

      <NewRuleModal
        template={modal}
        rule={editing}
        probe={probe}
        busy={busy}
        onClose={() => { setModal(undefined); setEditing(undefined); }}
        onConfirm={editing
          ? (srcDport, destIP, destPort, proto) => updateRule(editing.section, srcDport, destIP, destPort, proto)
          : addRule}
      />

      <ConfirmDialog
        open={!!toDelete}
        onClose={() => setToDelete(undefined)}
        onConfirm={() => { const r = toDelete; setToDelete(undefined); if (r) deleteRule(r); }}
        title={t("fwd.deleteTitle", { name: toDelete ? ruleName(t, toDelete) : "" })}
        consequence={t("fwd.deleteConsequence", { name: toDelete ? ruleName(t, toDelete) : "" })}
        confirmLabel={t("fwd.deleteConfirm")}
        busy={busy}
      />
    </>
  );
}

/* ══════════ Modal "Nueva regla" (ports.md §2) ══════════ */

export function NewRuleModal({ template, rule, probe, busy, onClose, onConfirm }: {
  template: FwdTemplate | undefined;
  /** Regla existente que se está editando (solo forwards propios). */
  rule?: FwdRule;
  probe: FwdProbe | undefined;
  busy: boolean;
  onClose: () => void;
  onConfirm: (srcDport: string, destIP: string, destPort: string, proto: string) => void;
}) {
  const { t } = useTranslation();
  const [sel, setSel] = useState<FwdTemplate>(TEMPLATES[0]);
  const [destIP, setDestIP] = useState("");
  const [srcDport, setSrcDport] = useState("");
  const [destPort, setDestPort] = useState("");
  const [proto, setProto] = useState("tcp");
  const [clients, setClients] = useState<Client[]>([]);

  // Pre-relleno: los datos de la regla que se edita, o la plantilla elegida.
  useEffect(() => {
    if (rule) {
      setSrcDport(rule.src_dport);
      setDestPort(rule.dest_port);
      setDestIP(rule.dest_ip);
      setProto(rule.proto === "tcp udp" ? "tcpudp" : rule.proto);
      return;
    }
    if (!template) return;
    setSel(template);
    setSrcDport(template.port);
    setDestPort(template.port);
    setProto(template.proto);
    setDestIP("");
  }, [template, rule]);

  // Inventario de clientes para "¿A qué equipo?".
  useEffect(() => {
    if (!template && !rule) return;
    api.clients().then((r) => setClients(r.clients.filter((c) => !!c.ip && !c.self))).catch(() => {});
  }, [template, rule]);

  const portOk = (p: string) => /^\d+$/.test(p) && Number(p) >= 1 && Number(p) <= 65535;
  const duplicate = probe?.rules.some((r) =>
    r.section !== rule?.section && r.src_dport === srcDport && srcDport !== "");
  const valid = portOk(srcDport) && portOk(destPort) && isValidIp(destIP) && !duplicate;

  return (
    <Modal open={!!template || !!rule} onClose={onClose}
      title={rule ? t("fwd.editTitle") : t("fwd.modalTitle")}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={() => onConfirm(srcDport, destIP, destPort, proto)} disabled={!valid} loading={busy}>
            {rule ? t("common.save") : t("fwd.open")}
          </Button>
        </>
      }
    >
      {/* 1. ¿Qué quieres abrir? (al editar ya está decidido) */}
      {!rule && (
        <>
      <p className="text-small font-medium mb-2">{t("fwd.stepWhat")}</p>
      <div className="grid grid-cols-2 gap-2">
        {TEMPLATES.map((tpl) => (
          <button
            key={tpl.id}
            type="button"
            onClick={() => {
              setSel(tpl);
              if (tpl.port) { setSrcDport(tpl.port); setDestPort(tpl.port); }
              setProto(tpl.proto);
            }}
            className={`flex items-start gap-2 rounded-md border p-2.5 text-left ring-focus transition-colors duration-[var(--dur-fast)]
              ${sel.id === tpl.id ? "border-accent bg-accent-soft" : "border-border bg-surface-2 hover:bg-surface"}`}
          >
            <tpl.icon size={16} className={`mt-0.5 shrink-0 ${sel.id === tpl.id ? "text-accent" : "text-muted"}`} aria-hidden="true" />
            <span>
              <span className="block text-small font-medium">{t(tpl.nameKey)}</span>
              <span className="block text-caption text-muted">{t(tpl.descKey)}</span>
            </span>
          </button>
        ))}
      </div>
        </>
      )}

      {/* 2. ¿A qué equipo? */}
      <p className="text-small font-medium mt-4 mb-2">{t("fwd.stepWhere")}</p>
      <Input mono list="netgrip-devices" value={destIP} onChange={(e) => setDestIP(e.target.value)}
        placeholder={t("fwd.manualIp")} aria-label={t("fwd.manualIp")}
        error={!!destIP && !isValidIp(destIP)} />
      <datalist id="netgrip-devices">
        {clients.map((c) => (
          <option key={c.mac} value={c.ip} label={c.name}>{`${c.name} · ${c.ip}`}</option>
        ))}
      </datalist>
        {destIP && !isValidIp(destIP) && <p className="text-caption text-danger mt-1">{t("fwd.invalidIp")}</p>}

      {/* 3. Puertos y protocolo (auto-rellenados por la plantilla) */}
      <div className="mt-4 grid grid-cols-2 gap-3">
        <Field label={t("fwd.extPortLabel")} mono
          error={srcDport && !portOk(srcDport) ? t("fwd.invalidPort") : duplicate ? t("fwd.duplicate") : undefined}
          inputProps={{ value: srcDport, onChange: (e) => setSrcDport(e.target.value), inputMode: "numeric", placeholder: "443" }} />
        <Field label={t("fwd.intPortLabel")} mono
          error={destPort && !portOk(destPort) ? t("fwd.invalidPort") : undefined}
          inputProps={{ value: destPort, onChange: (e) => setDestPort(e.target.value), inputMode: "numeric", placeholder: "443" }} />
      </div>
      <div className="mt-3">
        <SegmentedControl
          size="sm"
          ariaLabel="Protocolo"
          value={proto}
          onChange={setProto}
          options={[
            { value: "tcp", label: "TCP" },
            { value: "udp", label: "UDP" },
            { value: "tcpudp", label: "TCP+UDP" },
          ]}
        />
      </div>
    </Modal>
  );
}
