import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  ArrowDown, ArrowRight, ArrowUp, ArrowUpDown, Clapperboard, DoorOpen, Gamepad2, HardDrive,
  Pencil, Plus, Trash2, Video,
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

/** Name NetGrip writes on the redirects it creates (`netgrip-fwd-443`). It is
 * an internal label, never what the card should print: for those rules the
 * friendly "Port 443" is used instead, and the edit dialog starts empty so
 * the automatic name follows the port. */
const AUTO_NAME = /^netgrip-fwd-\d+$/;

function ruleName(t: TFunction, r: FwdRule): string {
  if (!r.name || AUTO_NAME.test(r.name)) return t("fwd.unnamed", { port: r.src_dport });
  return r.name;
}

/** Columns the list can be ordered by. State is left out on purpose: every
 * rule here is active, so ordering by it would do nothing. */
type SortKey = "name" | "port" | "dest" | "proto";

type Sort = { key: SortKey; dir: 1 | -1 };

/** What the dialog hands back. The name only matters when editing: a new rule
 * always gets the automatic one. */
export type FwdDraft = {
  name: string;
  srcDport: string;
  destIP: string;
  destPort: string;
  proto: string;
};

/** Desktop column template shared by the header and the rows, so a header
 * always sits over its column. */
const LIST_GRID =
  "md:grid md:grid-cols-[minmax(0,1.5fr)_4.5rem_minmax(0,1fr)_4.5rem_5.5rem_2.25rem_2.25rem] md:items-center md:gap-2";

function sortValue(t: TFunction, r: FwdRule, key: SortKey): string | number {
  switch (key) {
    case "name":
      return ruleName(t, r).toLowerCase();
    case "port":
      return Number(r.src_dport) || 0;
    case "dest":
      // An input rule ends on the router itself: it sorts before the hosts.
      return r.kind === "input" ? "" : r.dest_ip;
    case "proto":
      return r.proto;
  }
}

function SortHeader({ label, col, sort, onSort }: {
  label: string;
  col: SortKey;
  sort: Sort;
  onSort: (k: SortKey) => void;
}) {
  const active = sort.key === col;
  const Icon = !active ? ArrowUpDown : sort.dir === 1 ? ArrowUp : ArrowDown;
  return (
    <button
      type="button"
      onClick={() => onSort(col)}
      aria-pressed={active}
      className={`inline-flex items-center gap-1 text-eyebrow uppercase tracking-wide ring-focus rounded-sm
        transition-colors duration-[var(--dur-fast)] ${active ? "text-text" : "text-faint hover:text-text"}`}
    >
      {label}
      <Icon size={12} aria-hidden="true" />
    </button>
  );
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
  // External port first: the firewall is read by port, and it keeps the list
  // predictable instead of following whatever order UCI happens to keep.
  const [sort, setSort] = useState<Sort>({ key: "port", dir: 1 });

  const applicable = !!probe?.has_wan && !!probe?.firewall;
  const rules = probe?.rules ?? [];

  const sorted = useMemo(() => {
    const dir = sort.dir;
    return [...rules].sort((a, b) => {
      const av = sortValue(t, a, sort.key);
      const bv = sortValue(t, b, sort.key);
      const cmp = typeof av === "number" && typeof bv === "number"
        ? av - bv
        : String(av).localeCompare(String(bv), undefined, { numeric: true });
      if (cmp !== 0) return cmp * dir;
      // Stable tie-break so equal names do not shuffle between polls.
      return Number(a.src_dport) - Number(b.src_dport);
    });
  }, [rules, sort, t]);

  const toggleSort = (key: SortKey) => {
    setSort((s) => (s.key === key ? { key, dir: s.dir === 1 ? -1 : 1 } : { key, dir: 1 }));
  };

  const addRule = (srcDport: string, destIP: string, destPort: string, proto: string) => {
    run(() => api.addFwdRule(srcDport, destIP, destPort, proto)).then((res) => {
      if (res?.status === "applied") {
        onChange(res.state);
        setModal(undefined);
      }
    });
  };

  const updateRule = (section: string, draft: FwdDraft) => {
    run(() => api.updateFwdRule(section, draft.name, draft.srcDport, draft.destIP, draft.destPort, draft.proto)).then((res) => {
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
            {rules.length === 0 ? (
              <>
                <EmptyState
                  illustration={<IlluPlug size={120} />}
                  title={t("fwd.emptyHero")}
                  action={<Button icon={Plus} onClick={() => setModal(TEMPLATES[0])}>{t("fwd.firstRule")}</Button>}
                />
                <p className="text-caption text-muted text-center max-w-md mx-auto -mt-2">{t("fwd.vpnHint")}</p>
              </>
            ) : (
              <div>
                {/* Phones have no room for five headers: the same ordering
                    lives in a compact control above the list. */}
                {rules.length > 1 && (
                  <div className="mb-2 flex items-center gap-2 md:hidden">
                    <SegmentedControl
                      size="sm"
                      ariaLabel={t("fwd.sortBy")}
                      value={sort.key}
                      onChange={(k) => setSort((s) => ({ key: k as SortKey, dir: s.key === k ? s.dir : 1 }))}
                      options={[
                        { value: "name", label: t("fwd.colName") },
                        { value: "port", label: t("fwd.colPort") },
                        { value: "dest", label: t("fwd.colDest") },
                      ]}
                    />
                    <button
                      type="button"
                      onClick={() => setSort((s) => ({ ...s, dir: s.dir === 1 ? -1 : 1 }))}
                      aria-label={sort.dir === 1 ? t("fwd.sortDesc") : t("fwd.sortAsc")}
                      className="text-faint hover:text-text transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1"
                    >
                      {sort.dir === 1 ? <ArrowUp size={16} /> : <ArrowDown size={16} />}
                    </button>
                  </div>
                )}

                <div className={`hidden border-b border-border/60 px-1 py-1.5 ${LIST_GRID}`}>
                  <SortHeader label={t("fwd.colName")} col="name" sort={sort} onSort={toggleSort} />
                  <SortHeader label={t("fwd.colPort")} col="port" sort={sort} onSort={toggleSort} />
                  <SortHeader label={t("fwd.colDest")} col="dest" sort={sort} onSort={toggleSort} />
                  <SortHeader label={t("fwd.colProto")} col="proto" sort={sort} onSort={toggleSort} />
                  <span className="text-eyebrow uppercase tracking-wide text-faint">{t("fwd.colState")}</span>
                  <span className="sr-only">{t("fwd.colActions")}</span>
                </div>

                <div className="divide-y divide-border/50">
                  {sorted.map((r) => (
                    <div key={r.section} className={`flex flex-wrap items-center gap-2.5 py-2.5 animate-fade-up ${LIST_GRID}`}>
                      <span title={ruleName(t, r)} className="text-body font-medium min-w-0 truncate md:col-start-1 md:row-start-1">
                        {ruleName(t, r)}
                      </span>
                      <span className="font-mono text-caption bg-surface-2 border border-border rounded-sm px-1.5 py-0.5 md:col-start-2 md:row-start-1">
                        :{r.src_dport}
                      </span>
                      <ArrowRight size={14} className="text-faint shrink-0 md:hidden" aria-hidden="true" />
                      {/* A forward lands on a host; an input rule ends on the
                          router itself and has no destination to show. */}
                      <span className="font-mono text-small min-w-0 truncate md:col-start-3 md:row-start-1">
                        {r.kind === "input" ? t("fwd.thisRouter") : `${r.dest_ip}:${r.dest_port}`}
                      </span>
                      <span className="font-mono text-caption text-muted uppercase bg-surface-2 border border-border rounded-sm px-1.5 py-0.5 md:col-start-4 md:row-start-1">
                        {r.proto}
                      </span>
                      <span className="flex-1 md:hidden" />
                      {/* Editar va antes del estado (#617): es la accion, y asi
                          no queda escondida detras de las pildoras. Cualquier
                          reenvio se puede editar, tambien los de fuera; borrar
                          sigue siendo solo de los nuestros (#613). */}
                      {r.kind === "forward" && (
                        <button
                          type="button"
                          onClick={() => setEditing(r)}
                          disabled={busy}
                          aria-label={`${t("fwd.edit")} ${ruleName(t, r)}`}
                          className="text-faint hover:text-accent transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1 md:col-start-6 md:row-start-1"
                        >
                          <Pencil size={16} />
                        </button>
                      )}
                      <Pill tone="ok" className="md:col-start-5 md:row-start-1">{t("fwd.active")}</Pill>
                      {/* Only what NetGrip created is removable from here;
                          anything set up elsewhere is shown, not touched. */}
                      <span className="md:col-start-7 md:row-start-1" title={r.managed === false ? t("fwd.externalHint") : undefined}>
                        {r.managed === false ? (
                          <Pill tone="muted">{t("fwd.external")}</Pill>
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
                      </span>
                    </div>
                  ))}
                </div>
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
          ? (d) => updateRule(editing.section, d)
          : (d) => addRule(d.srcDport, d.destIP, d.destPort, d.proto)}
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
  onConfirm: (draft: FwdDraft) => void;
}) {
  const { t } = useTranslation();
  const [sel, setSel] = useState<FwdTemplate>(TEMPLATES[0]);
  const [name, setName] = useState("");
  const [destIP, setDestIP] = useState("");
  const [srcDport, setSrcDport] = useState("");
  const [destPort, setDestPort] = useState("");
  const [proto, setProto] = useState("tcp");
  const [clients, setClients] = useState<Client[]>([]);

  // Pre-relleno: los datos de la regla que se edita, o la plantilla elegida.
  useEffect(() => {
    if (rule) {
      // The generated name is not a name the person chose: start empty so
      // saving without typing keeps it following the port.
      setName(AUTO_NAME.test(rule.name) ? "" : rule.name);
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
          <Button onClick={() => onConfirm({ name, srcDport, destIP, destPort, proto })} disabled={!valid} loading={busy}>
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

      {/* Nombre: al editar se puede cambiar; al crear lo pone el panel. */}
      {rule && (
        <>
          <p className="text-small font-medium mt-4 mb-2">{t("fwd.stepName")}</p>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={64}
            placeholder={t("fwd.unnamed", { port: srcDport || "443" })}
            aria-label={t("fwd.nameLabel")}
          />
          <p className="text-caption text-muted mt-1">{t("fwd.nameHint")}</p>
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
