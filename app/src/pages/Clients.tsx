import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp, ArrowUpDown, Ban, MoreVertical, Pencil, Pin, Search, Smartphone, Wifi, Cable, Eye } from "lucide-react";
import { api } from "../api";
import type { Client, DeviceType } from "../types";
import { Banner, Button, Card, ConfirmDialog, EmptyState, Field, Input, Modal, Pill, SkeletonRows } from "../components/ui";
import { DEVICE_TYPES, DEVICE_TYPE_KEYS, deviceTypeIcon } from "../components/clients/catalog";
import { IlluDevices } from "../components/ui/illustrations";
import { fmtBytes, fmtRate } from "../lib/format";

/** Fila de detalle: etiqueta a la izquierda, valor a la derecha. */
function DetailRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2 border-b border-border/60 last:border-0">
      <span className="text-small text-muted shrink-0">{label}</span>
      <span className={`text-small font-medium text-right min-w-0 break-all ${mono ? "font-mono" : ""}`}>{value}</span>
    </div>
  );
}

/** Par de flechas ↓/↑ con tasa y color. */
function RatePair({ down, up, downColor = "var(--color-ok)", upColor = "var(--color-text)" }: { down: number; up: number; downColor?: string; upColor?: string }) {
  return (
    <span className="inline-flex flex-col items-end text-small" style={{ fontVariantNumeric: "tabular-nums" }}>
      <span className="inline-flex items-center gap-1"><ArrowDown size={13} style={{ color: downColor }} aria-hidden="true" />{fmtRate(down)}</span>
      <span className="inline-flex items-center gap-1"><ArrowUp size={13} style={{ color: upColor }} aria-hidden="true" />{fmtRate(up)}</span>
    </span>
  );
}

type SortKey = "name" | "traffic" | "ip" | "type";
type SortDir = "asc" | "desc";

function SignalBars({ signal }: { signal?: number }) {
  const level = signal === undefined ? 0 : signal >= -55 ? 4 : signal >= -65 ? 3 : signal >= -75 ? 2 : 1;
  const tone = level >= 3 ? "var(--color-ok)" : "var(--color-warn)";
  return (
    <span className="inline-flex items-end gap-[2px]" role="img" aria-label={signal !== undefined ? `${signal} dBm` : "—"}>
      {[1, 2, 3, 4].map((i) => (
        <span key={i} className="w-[3px] rounded-full"
          style={{ height: 3 + i * 2.5, background: i <= level ? tone : "var(--color-border-strong)" }} />
      ))}
    </span>
  );
}

const BAND_KEYS: Record<string, string> = { "2g": "clients.band2g", "5g": "clients.band5g", "6g": "clients.band6g" };

/** Pill de bloqueo (#160): total (todas las bandas) vs parcial (p. ej. solo 5 GHz). */
function BlockedPill({ c }: { c: Client }) {
  const { t } = useTranslation();
  if (c.blocked) return <Pill tone="danger">{t("clients.blocked")}</Pill>;
  if (c.blocked_on?.length) {
    const bands = c.blocked_on.map((b) => t(BAND_KEYS[b] ?? b)).join(" + ");
    return <Pill tone="warn">{t("clients.blockedOn", { bands })}</Pill>;
  }
  return null;
}

function blockedDetail(c: Client, t: (k: string, o?: Record<string, string>) => string): string {
  if (c.blocked) return t("clients.on");
  if (c.blocked_on?.length) {
    const bands = c.blocked_on.map((b) => t(BAND_KEYS[b] ?? b)).join(" + ");
    return t("clients.blockedOn", { bands });
  }
  return t("clients.off");
}

function connectionKey(c: Client): string {
  return c.type === "wifi5" ? "5G" : c.type === "wifi24" ? "2.4G" : "Cable";
}

/** Etiqueta + icono de conexión (2.4G / 5G / cable). */
function ConnectionChip({ c }: { c: Client }) {
  const { t } = useTranslation();
  const key = connectionKey(c);
  const label = c.type === "wifi5" ? t("overview.band5") : c.type === "wifi24" ? t("overview.band24") : t("overview.cable");
  const Icon = c.type === "cable" ? Cable : Wifi;
  const tone = c.type === "cable" ? "muted" : c.type === "wifi5" ? "accent" : "ok";
  return (
    <span className="inline-flex items-center gap-1.5">
      <Icon size={14} className="text-muted shrink-0" aria-hidden="true" />
      <Pill tone={tone as "muted" | "accent" | "ok"}>{key}</Pill>
      <span className="sr-only">{label}</span>
    </span>
  );
}

/** Clientes (clients.md): inventario de dispositivos conectados, con asignación
 *  de nombre y tipo de dispositivo, reserva de IP y bloqueo. */
export function ClientsPage() {
  const { t } = useTranslation();
  const [clients, setClients] = useState<Client[]>();
  const [bands, setBands] = useState<string[]>([]);
  const [rates, setRates] = useState<Record<string, { rx: number; tx: number }>>({});
  const [error, setError] = useState(false);
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<{ key: SortKey; dir: SortDir }>({ key: "name", dir: "asc" });
  const [busyMac, setBusyMac] = useState<string>();
  const [menuMac, setMenuMac] = useState<string>();
  const [editTarget, setEditTarget] = useState<Client>();
  const [detailTarget, setDetailTarget] = useState<Client>();
  const [blockTarget, setBlockTarget] = useState<Client>();
  const [actionError, setActionError] = useState<string>();

  const lastSnap = useRef<Record<string, { rx: number; tx: number; ts: number }>>({});

  const load = useCallback(async () => {
    setError(false);
    try {
      const c = await api.clients();
      setBands(c.bands ?? []);
      const now = Date.now();
      const newRates: Record<string, { rx: number; tx: number }> = {};
      for (const cl of c.clients) {
        const prev = lastSnap.current[cl.mac];
        if (prev && now > prev.ts) {
          const dt = (now - prev.ts) / 1000;
          newRates[cl.mac] = {
            rx: Math.max(0, (cl.rx_bytes - prev.rx) / dt),
            tx: Math.max(0, (cl.tx_bytes - prev.tx) / dt),
          };
        }
        lastSnap.current[cl.mac] = { rx: cl.rx_bytes, tx: cl.tx_bytes, ts: now };
      }
      setRates((r) => ({ ...r, ...newRates }));
      setClients(c.clients);
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id); }, [load]);

  const filtered = useMemo(() => {
    if (!clients) return undefined;
    const q = query.trim().toLowerCase();
    if (!q) return clients;
    return clients.filter((c) =>
      c.name.toLowerCase().includes(q) ||
      (c.ip ?? "").includes(q) ||
      c.mac.toLowerCase().includes(q) ||
      (c.device_type ?? "").includes(q)
    );
  }, [clients, query]);

  const sorted = useMemo(() => {
    if (!filtered) return undefined;
    const { key, dir } = sort;
    const mult = dir === "asc" ? 1 : -1;
    return [...filtered].sort((a, b) => {
      switch (key) {
        case "name": return a.name.localeCompare(b.name) * mult;
        case "traffic": return ((a.rx_bytes + a.tx_bytes) - (b.rx_bytes + b.tx_bytes)) * mult;
        case "ip": return ((a.ip ?? "") <= (b.ip ?? "") ? -1 : 1) * mult;
        case "type": return connectionKey(a).localeCompare(connectionKey(b)) * mult;
      }
    });
  }, [filtered, sort]);

  const toggleSort = (key: SortKey) => setSort((s) => s.key === key ? { key, dir: s.dir === "asc" ? "desc" : "asc" } : { key, dir: "asc" });

  const typeSummary = useMemo(() => {
    const byType = new Map<string, number>();
    for (const c of clients ?? []) {
      const key = c.device_type || "other";
      byType.set(key, (byType.get(key) ?? 0) + 1);
    }
    return [...byType.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6);
  }, [clients]);

  const pin = async (c: Client) => {
    setBusyMac(c.mac); setActionError(undefined);
    try {
      await api.reserveClient(c.mac, c.ip ?? "", !c.reserved);
      await load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : t("clients.failed"));
    } finally {
      setBusyMac(undefined);
    }
  };

  const block = async (c: Client, blocked: boolean, band?: string) => {
    setBusyMac(c.mac); setActionError(undefined);
    try {
      await api.blockClient(c.mac, c.type, blocked, band);
      await load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : t("clients.failed"));
    } finally {
      setBusyMac(undefined);
      setBlockTarget(undefined);
    }
  };

  const saveEdit = async (mac: string, name: string, deviceType: string) => {
    setBusyMac(mac); setActionError(undefined);
    try {
      await api.setClientMeta(mac, name, deviceType);
      setEditTarget(undefined);
      await load();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : t("clients.failed"));
    } finally {
      setBusyMac(undefined);
    }
  };

  const row = (c: Client) => {
    const DevIcon = deviceTypeIcon(c.device_type, c.name);
    return (
      <>
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-teal-soft text-teal" aria-hidden="true">
          <DevIcon size={16} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-1.5">
            <span className="truncate text-body font-medium">{c.name || <span className="font-mono text-small">{c.mac}</span>}</span>
            {c.self && <Pill tone="accent">{t("overview.thisDevice")}</Pill>}
            <BlockedPill c={c} />
            {c.reserved && <Pill tone="muted">{t("overview.fixed")}</Pill>}
          </span>
          <span className="block text-caption text-muted">
            {c.device_type ? t(DEVICE_TYPES[c.device_type as DeviceType]?.labelKey ?? "clients.type.other") : c.mac}
          </span>
        </span>
      </>
    );
  };

  const actions = (c: Client) => (
    <span className="flex items-center gap-1 shrink-0">
      <span className="relative">
        <button type="button" disabled={busyMac === c.mac} onClick={() => setMenuMac(menuMac === c.mac ? undefined : c.mac)}
          title={t("clients.more")} aria-label={t("clients.more")}
          className="inline-flex h-8 w-8 items-center justify-center rounded-sm ring-focus transition-colors text-muted hover:text-text hover:bg-surface-2">
          <MoreVertical size={16} />
        </button>
        {menuMac === c.mac && (
          <>
            <div className="fixed inset-0 z-10" onClick={() => setMenuMac(undefined)} aria-hidden="true" />
            <div className="absolute z-20 right-0 mt-1 w-44 overflow-hidden rounded-md border border-border bg-surface p-1 shadow-elevated">
              <button type="button" onClick={() => { setMenuMac(undefined); setDetailTarget(c); }}
                className="flex w-full items-center gap-2 rounded-sm px-2.5 py-1.5 text-small ring-focus hover:bg-surface-2 transition-colors">
                <Eye size={14} className="text-muted" aria-hidden="true" />{t("clients.details")}
              </button>
              <button type="button" onClick={() => { setMenuMac(undefined); setEditTarget(c); }}
                className="flex w-full items-center gap-2 rounded-sm px-2.5 py-1.5 text-small ring-focus hover:bg-surface-2 transition-colors">
                <Pencil size={14} className="text-muted" aria-hidden="true" />{t("clients.edit")}
              </button>
            </div>
          </>
        )}
      </span>
      {c.reservable && (
        <button type="button" disabled={busyMac === c.mac} onClick={() => pin(c)}
          title={c.reserved ? t("overview.unpinIp") : t("overview.pinIp")}
          aria-label={c.reserved ? t("overview.unpinIp") : t("overview.pinIp")}
          className={`inline-flex h-8 w-8 items-center justify-center rounded-sm ring-focus transition-colors disabled:opacity-40
            ${c.reserved ? "text-accent" : "text-muted hover:text-text hover:bg-surface-2"}`}>
          <Pin size={16} />
        </button>
      )}
      {c.blockable && !c.self && (
        <button type="button" disabled={busyMac === c.mac}
          onClick={() => c.type === "cable"
            ? (c.blocked ? block(c, false, "") : setBlockTarget(c))
            : setBlockTarget(c)}
          title={c.blocked ? t("overview.unblock") : t("overview.block")}
          aria-label={c.blocked ? t("overview.unblock") : t("overview.block")}
          className={`inline-flex h-8 w-8 items-center justify-center rounded-sm ring-focus transition-colors disabled:opacity-40
            ${c.blocked ? "text-danger" : "text-muted hover:text-danger hover:bg-surface-2"}`}>
          <Ban size={16} />
        </button>
      )}
    </span>
  );

  const SortTh = ({ label, k, className = "" }: { label: string; k: SortKey; className?: string }) => (
    <th className={`px-3 py-2 text-left font-medium ${className}`}>
      <button type="button" onClick={() => toggleSort(k)}
        className="inline-flex items-center gap-1 text-muted hover:text-text ring-focus rounded-sm transition-colors">
        {label}
        {sort.key === k ? (sort.dir === "asc" ? <ArrowUp size={12} /> : <ArrowDown size={12} />) : <ArrowUpDown size={12} className="opacity-40" />}
      </button>
    </th>
  );

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card index={0} title={t("clients.summaryTitle")} icon={Smartphone} iconTone="teal">
        {typeSummary.length === 0 ? (
          <EmptyState small title={t("overview.devicesEmpty")} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {typeSummary.map(([type, count]) => {
              const Icon = deviceTypeIcon(type);
              return (
                <span key={type} className="inline-flex items-center gap-2 rounded-full bg-surface-2 border border-border/60 px-3 py-1.5 text-small">
                  <Icon size={14} className="text-teal" aria-hidden="true" />
                  <span className="font-medium">{t(DEVICE_TYPES[type as DeviceType]?.labelKey ?? "clients.type.other")}</span>
                  <span className="text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>{count}</span>
                </span>
              );
            })}
          </div>
        )}
      </Card>

      <Card index={1} title={t("clients.title")} icon={Cable} iconTone="teal"
        action={<div className="relative hidden sm:block">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted" aria-hidden="true" />
          <Input value={query} onChange={(e) => setQuery(e.target.value)}
            placeholder={t("clients.search")} className="w-56 pl-8" aria-label={t("clients.search")} />
        </div>}>
        {error ? (
          <EmptyState small title={t("common.loadError")} action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>} />
        ) : !sorted ? (
          <SkeletonRows rows={4} />
        ) : sorted.length === 0 ? (
          <EmptyState illustration={<IlluDevices size={120} />} title={t("overview.devicesEmpty")} body={t("overview.devicesEmptyBody")} />
        ) : (
          <>
            <table className="hidden md:table w-full text-small">
              <thead>
                <tr className="text-caption border-b border-border/60">
                  <SortTh label={t("clients.name")} k="name" className="w-auto" />
                  <SortTh label={t("clients.connection")} k="type" className="w-28" />
                  <th className="px-3 py-2 w-16"><span className="sr-only">{t("clients.signal")}</span></th>
                  <SortTh label={t("clients.traffic")} k="traffic" className="w-28" />
                  <SortTh label={t("clients.ip")} k="ip" className="w-36" />
                  <th className="pl-3 w-32 text-right"><span className="sr-only">S</span></th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((c) => (
                  <tr key={c.mac} style={{ height: "var(--row-min-h)" }}
                    className="border-b border-border/60 last:border-0 hover:bg-surface-2 transition-colors">
                    <td className="py-1.5 pr-3"><span className="flex items-center gap-3">{row(c)}</span></td>
                    <td className="py-1.5 px-3 w-28"><ConnectionChip c={c} /></td>
                    <td className="py-1.5 px-3 w-16">{c.type !== "cable" && <SignalBars signal={c.signal} />}</td>
                    <td className="py-1.5 px-3 w-28 font-mono text-caption whitespace-nowrap" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {fmtBytes(c.rx_bytes + c.tx_bytes)}
                    </td>
                    <td className="py-1.5 px-3 w-36"><span className="font-mono text-caption">{c.ip ?? "—"}</span></td>
                    <td className="py-1.5 pl-3 w-32 text-right">{actions(c)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="md:hidden space-y-2 flex flex-col">
              {sorted.map((c) => (
                <div key={c.mac} className="rounded-md border border-border/60 p-2.5">
                  <div className="flex items-center gap-3">
                    {row(c)}
                    <ConnectionChip c={c} />
                  </div>
                  <div className="mt-1.5 flex items-center justify-between pl-12">
                    <span className="font-mono text-caption text-muted">
                      {c.ip ?? c.mac} · {fmtBytes(c.rx_bytes + c.tx_bytes)}
                    </span>
                    {actions(c)}
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
        {actionError && <Banner tone="danger" className="mt-3">{actionError}</Banner>}
      </Card>

      <DetailClientModal
        client={detailTarget}
        rate={detailTarget ? rates[detailTarget.mac] : undefined}
        onClose={() => setDetailTarget(undefined)}
      />

      <EditClientModal
        client={editTarget}
        busy={busyMac === editTarget?.mac}
        onSave={saveEdit}
        onClose={() => setEditTarget(undefined)}
      />

      {blockTarget && blockTarget.type === "cable" ? (
        <ConfirmDialog
          open={!!blockTarget}
          onClose={() => setBlockTarget(undefined)}
          onConfirm={() => blockTarget && block(blockTarget, true, "")}
          title={t("overview.blockTitle", { name: blockTarget?.name ?? blockTarget?.mac ?? "" })}
          consequence={t("overview.blockBody")}
          confirmLabel={t("overview.blockConfirm")}
          busy={busyMac === blockTarget?.mac}
        />
      ) : (
        <BlockBandModal
          client={blockTarget}
          bands={bands}
          busy={busyMac === blockTarget?.mac}
          onApply={(blocked, band) => blockTarget && block(blockTarget, blocked, band || undefined)}
          onClose={() => setBlockTarget(undefined)}
        />
      )}
    </div>
  );
}

/** Select desplegable de tipo de dispositivo, con icono en cada opción.
 *  Se renderiza en un portal a <body> (fixed) para no quedar recortado por
 *  el overflow del modal ni de la card. */
function DeviceTypeSelect({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number; width: number } | undefined>();
  const btnRef = useRef<HTMLButtonElement>(null);
  const Icon = deviceTypeIcon(value);

  const toggle = () => {
    if (!open && btnRef.current) {
      const r = btnRef.current.getBoundingClientRect();
      setPos({ top: r.bottom + 4, left: r.left, width: r.width });
    }
    setOpen((o) => !o);
  };

  return (
    <>
      <button ref={btnRef} type="button" onClick={toggle}
        className="w-full flex items-center gap-2 h-[var(--input-h)] rounded-[10px] border border-transparent bg-fill px-3 text-small ring-focus transition-colors hover:border-border-strong">
        <Icon size={16} className="text-teal shrink-0" aria-hidden="true" />
        <span className="flex-1 truncate text-left">{t(DEVICE_TYPES[value as DeviceType]?.labelKey ?? "clients.type.other")}</span>
        <span className="text-muted">{open ? "▲" : "▼"}</span>
      </button>
      {open && pos && createPortal(
        <>
          <div className="fixed inset-0 z-[60]" onClick={() => setOpen(false)} aria-hidden="true" />
          <ul
            style={{ top: pos.top, left: pos.left, width: pos.width, maxHeight: 260 }}
            className="fixed z-[70] overflow-auto rounded-[10px] border border-border bg-surface p-1 shadow-elevated">
            {DEVICE_TYPE_KEYS.map((k) => (
              <li key={k}>
                <button type="button" onClick={() => { onChange(k); setOpen(false); }}
                  className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-small ring-focus transition-colors
                    ${k === value ? "bg-surface-2 font-medium" : "hover:bg-surface-2"}`}>
                  {(() => { const I = DEVICE_TYPES[k].icon; return <I size={15} className="text-teal shrink-0" aria-hidden="true" />; })()}
                  <span className="truncate">{t(DEVICE_TYPES[k].labelKey)}</span>
                </button>
              </li>
            ))}
          </ul>
        </>,
        document.body
      )}
    </>
  );
}

/** Modal de detalle: editar nombre y tipo, y ver la info del cliente. */
/** Modal de detalle (solo lectura): info del cliente estilo GL. */
function DetailClientModal({ client, rate, onClose }: {
  client?: Client;
  rate?: { rx: number; tx: number };
  onClose: () => void;
}) {
  const { t } = useTranslation();
  if (!client) return null;
  const connLabel = client.type === "wifi5" ? t("overview.band5") : client.type === "wifi24" ? t("overview.band24") : t("overview.cable");
  const connTone = client.type === "cable" ? "muted" : client.type === "wifi5" ? "accent" : "ok";
  const ConnIcon = client.type === "cable" ? Cable : Wifi;
  const DevIcon = deviceTypeIcon(client.device_type, client.name);
  return (
    <Modal open onClose={onClose} title={t("clients.detailTitle")}
      footer={<Button onClick={onClose}>{t("common.close")}</Button>}>
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-3">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-teal-soft text-teal" aria-hidden="true">
            <DevIcon size={18} />
          </span>
          <div className="min-w-0 flex-1">
            <span className="block truncate text-h2">{client.name || client.mac}</span>
            <span className="inline-flex items-center gap-1.5 text-caption text-muted">
              <span className="h-1.5 w-1.5 rounded-full bg-ok" aria-hidden="true" />
              {connLabel}
            </span>
          </div>
        </div>
        <div className="rounded-md border border-border/60 px-3 divide-y divide-border/60">
          <DetailRow label={t("clients.host")} value={client.name || client.mac} />
          <DetailRow label={t("clients.typeLabel")}
            value={<span className="inline-flex items-center gap-1.5"><span className="text-teal"><DevIcon size={14} aria-hidden="true" /></span>{t(DEVICE_TYPES[client.device_type as DeviceType]?.labelKey ?? "clients.type.other")}</span>} />
          <DetailRow label={t("clients.connection")}
            value={<span className="inline-flex items-center gap-1.5"><Pill tone={connTone as "muted" | "accent" | "ok"}>{connLabel}</Pill><ConnIcon size={13} className="text-muted" aria-hidden="true" /></span>} />
          <DetailRow label={t("clients.mac")} value={client.mac} mono />
          <DetailRow label={t("clients.speed")}
            value={rate ? <RatePair down={rate.rx} up={rate.tx} /> : <span className="text-muted">—</span>} />
          <DetailRow label={t("clients.traffic")}
            value={<RatePair down={client.rx_bytes} up={client.tx_bytes} downColor="var(--color-ok)" upColor="var(--color-text)" />} />
          <DetailRow label={t("clients.reserved")}
            value={client.reserved ? t("clients.on") : t("clients.off")} />
          <DetailRow label={t("clients.block")}
            value={blockedDetail(client, t)} />
          <DetailRow label={t("clients.ip")} value={client.ip ?? "—"} mono />
        </div>
      </div>
    </Modal>
  );
}

/** Modal de gestión de bloqueo por banda (#163): bloquear en una banda,
 *  desbloquear por banda o todo. */
function BlockBandModal({ client, bands, busy, onApply, onClose }: {
  client?: Client;
  bands: string[];
  busy: boolean;
  onApply: (blocked: boolean, band: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [scope, setScope] = useState<string>("");
  useEffect(() => { setScope(""); }, [client]);
  if (!client) return null;
  const blockedBands = client.blocked_on ?? [];
  const hasBlocks = client.blocked || blockedBands.length > 0;
  const scopeBlocked = scope === "" ? client.blocked : blockedBands.includes(scope);
  return (
    <Modal open onClose={onClose} title={t("clients.blockManageTitle", { name: client.name ?? client.mac })}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={() => onApply(true, scope)} loading={busy} disabled={!scope || scopeBlocked}>
            {t("clients.blockApply")}
          </Button>
        </>
      }>
      <div className="flex flex-col gap-5">
        {hasBlocks && (
          <section>
            <p className="mb-2 text-caption text-muted">{t("clients.unblockTitle")}</p>
            <div className="flex flex-wrap gap-2">
              {blockedBands.map((b) => (
                <button key={b} type="button" disabled={busy}
                  onClick={() => onApply(false, b)}
                  className="inline-flex items-center gap-1.5 rounded-md border border-border bg-surface px-3 py-1.5 text-small ring-focus transition-colors hover:bg-surface-2 disabled:opacity-40">
                  <Ban size={13} className="text-danger" aria-hidden="true" />
                  {t(BAND_KEYS[b] ?? b)}
                </button>
              ))}
              {blockedBands.length > 1 && (
                <button type="button" disabled={busy} onClick={() => onApply(false, "")}
                  className="inline-flex items-center gap-1.5 rounded-md border border-danger/40 bg-danger/5 px-3 py-1.5 text-small text-danger ring-focus transition-colors hover:bg-danger/10 disabled:opacity-40">
                  {t("clients.unblockAll")}
                </button>
              )}
            </div>
          </section>
        )}
        <section>
          <p className="mb-2 text-caption text-muted">{t("clients.blockScope")}</p>
          <div className="grid grid-cols-3 gap-2">
            <button type="button" onClick={() => setScope("")}
              className={`rounded-md border p-3 text-small text-left ring-focus transition-colors ${scope === "" ? "border-accent bg-accent/5 font-medium" : "border-border hover:bg-surface-2"}`}>
              <span className="block">{t("clients.blockAllBands")}</span>
              <span className="block text-caption text-muted mt-1">{t("clients.blockAllHint")}</span>
            </button>
            {bands.map((b) => (
              <button key={b} type="button" onClick={() => setScope(b)}
                className={`rounded-md border p-3 text-small text-left ring-focus transition-colors ${scope === b ? "border-accent bg-accent/5 font-medium" : "border-border hover:bg-surface-2"}`}>
                <span className="block">{t(BAND_KEYS[b] ?? b)}</span>
                <span className="block text-caption text-muted mt-1">{t("clients.blockBandHint")}</span>
              </button>
            ))}
          </div>
        </section>
      </div>
    </Modal>
  );
}

/** Modal de edición: solo nombre y tipo de dispositivo. */
function EditClientModal({ client, busy, onSave, onClose }: {
  client?: Client;
  busy: boolean;
  onSave: (mac: string, name: string, deviceType: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [deviceType, setDeviceType] = useState<string>("other");

  useEffect(() => {
    if (!client) return;
    setName(client.name && client.name !== client.mac ? client.name : "");
    setDeviceType(client.device_type || "other");
  }, [client]);

  if (!client) return null;

  return (
    <Modal open onClose={onClose} title={t("clients.editTitle")}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={() => onSave(client.mac, name.trim(), deviceType)} loading={busy}>
            {t("clients.save")}
          </Button>
        </>
      }>
      <div className="flex flex-col gap-4">
        <Field label={t("clients.nameLabel")} inputProps={{
          value: name, onChange: (e) => setName(e.target.value), placeholder: t("clients.namePlaceholder"), maxLength: 40,
        }} />
        <div className="relative">
          <Field label={t("clients.typeLabel")}>
            <DeviceTypeSelect value={deviceType} onChange={setDeviceType} />
          </Field>
        </div>
      </div>
    </Modal>
  );
}
