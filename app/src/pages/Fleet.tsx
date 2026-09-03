import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff, Download, ExternalLink, Lock, Network, Plus, RefreshCw, Router, Server, Trash2, Wifi } from "lucide-react";
import { api } from "../api";
import type { FleetNodeStatus, DiscoveredFleetPeer } from "../types";
import {
  AdvancedDisclosure, Button, Card, ConfirmDialog, EmptyState, Field,
  IconTile, KeyValue, Modal, Pill, Skeleton, Toggle, useToast,
} from "../components/ui";
import { IlluFleet } from "../components/ui/illustrations";

/** Enlace sutil a NetPulse (fleet.md §5). Repo de la app hermana. */
const NETPULSE_URL = "https://github.com/gnacho/netpulse";

/** Rol/tipo deducido del nombre (fleet.md §2: "Punto de acceso", "Switch"). */
function roleKey(name: string): "roleAp" | "roleSwitch" | "roleDevice" {
  const n = name.toLowerCase();
  if (/(^|[-_])(ap|wap)([-_.]|$)|wifi|punto/.test(n)) return "roleAp";
  if (/switch|(^|[-_])sw([-_]|$)/.test(n)) return "roleSwitch";
  return "roleDevice";
}

/** Icono lucide por clase de equipo (design-rev2 §5): AP → Wifi, switch →
 *  Network, resto → Router. Siempre en IconTile tone teal (red). */
function roleIcon(name: string) {
  const role = roleKey(name);
  return role === "roleAp" ? Wifi : role === "roleSwitch" ? Network : Router;
}

/** address puede venir con o sin esquema; el panel vive en http(s)://address. */
function panelUrl(address: string): string {
  return /^https?:\/\//i.test(address) ? address : `http://${address}`;
}

/** ID interno por defecto a partir del nombre (slug sin espacios). */
function slugify(name: string): string {
  return name.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

export function FleetPage() {
  const { t } = useTranslation();
  const toast = useToast();
  const [nodes, setNodes] = useState<FleetNodeStatus[]>();
  const [discovered, setDiscovered] = useState<DiscoveredFleetPeer[]>();
  const [discoveryEnabled, setDiscoveryEnabled] = useState<boolean>(true);
  const [nodesError, setNodesError] = useState(false);
  const [discoveredError, setDiscoveredError] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [checkingAll, setCheckingAll] = useState(false);
  const [toRemove, setToRemove] = useState<FleetNodeStatus>();
  const [toUpdate, setToUpdate] = useState<FleetNodeStatus>();
  const [toAdopt, setToAdopt] = useState<DiscoveredFleetPeer>();
  const [dialogBusy, setDialogBusy] = useState(false);
  const [discoveryLoading, setDiscoveryLoading] = useState(false);
  const [discoveryConfigLoaded, setDiscoveryConfigLoaded] = useState(false);

  const loadNodes = useCallback(async () => {
    setNodesError(false);
    try {
      const data = await api.fleet();
      setNodes(data.nodes ?? []);
    } catch {
      setNodesError(true);
    }
  }, []);

  const loadDiscovered = useCallback(async () => {
    setDiscoveredError(false);
    try {
      const data = await api.discoveredFleet();
      setDiscovered(data.peers ?? []);
    } catch {
      setDiscoveredError(true);
    }
  }, []);

  const loadDiscoveryConfig = useCallback(async () => {
    try {
      const cfg = await api.fleetDiscoveryConfig();
      setDiscoveryEnabled(cfg.enabled);
    } catch {
      // Si el endpoint no existe (version anterior del backend), mantener
      // el valor por defecto activado y marcar que no se pudo cargar config.
      setDiscoveryEnabled(true);
    } finally {
      setDiscoveryConfigLoaded(true);
    }
  }, []);

  const load = useCallback(async () => {
    await Promise.all([loadNodes(), loadDiscovered(), loadDiscoveryConfig()]);
  }, [loadNodes, loadDiscovered, loadDiscoveryConfig]);

  useEffect(() => { load(); }, [load]);

  const checkAll = async () => {
    setCheckingAll(true);
    try {
      const data = await api.checkAllFleet();
      setNodes(data.nodes ?? []);
    } catch {
      // el próximo "Comprobar todos" lo reintenta
    } finally {
      setCheckingAll(false);
    }
  };

  const checkedOne = (status: FleetNodeStatus) => {
    setNodes((prev) => prev?.map((n) => (n.id === status.id ? status : n)));
  };

  const doRemove = async () => {
    if (!toRemove) return;
    setDialogBusy(true);
    try {
      await api.deleteFleetNode(toRemove.id);
      setNodes((prev) => prev?.filter((n) => n.id !== toRemove.id));
      toast.push({ tone: "ok", text: t("fleet.removedOk", { name: toRemove.name }) });
      setToRemove(undefined);
    } catch (e) {
      toast.push({ tone: "danger", text: t("common.loadError"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setDialogBusy(false);
    }
  };

  const doUpdate = async () => {
    if (!toUpdate) return;
    setDialogBusy(true);
    try {
      await api.updateFleetNode(toUpdate.id);
      setNodes((prev) => prev?.map((n) => (n.id === toUpdate.id
        ? { ...n, current_version: n.latest_version, update_available: false }
        : n)));
      toast.push({ tone: "ok", text: t("fleet.updatedOk", { name: toUpdate.name }) });
      setToUpdate(undefined);
    } catch (e) {
      toast.push({ tone: "danger", text: t("common.loadError"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setDialogBusy(false);
    }
  };

  const toggleDiscovery = async (enabled: boolean) => {
    setDiscoveryLoading(true);
    try {
      await api.setFleetDiscoveryConfig(enabled);
      setDiscoveryEnabled(enabled);
      toast.push({ tone: "ok", text: enabled ? t("fleet.discoveryEnabled") : t("fleet.discoveryDisabled") });
    } catch (e) {
      toast.push({ tone: "danger", text: t("common.loadError"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setDiscoveryLoading(false);
    }
  };

  const loadingNodes = nodes === undefined && !nodesError;
  const nodesEmpty = !loadingNodes && !nodesError && (nodes?.length ?? 0) === 0;

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* ════ Cabecera: Tu red NetGrip (fleet.md §1) ════ */}
      <Card
        index={0}
        icon={Server}
        iconTone="teal"
        title={t("fleet.headerTitle")}
        action={
          <div className="flex gap-2 shrink-0">
            <Button
              variant="secondary" size="sm" icon={RefreshCw}
              loading={checkingAll} disabled={loadingNodes || nodes?.length === 0}
              onClick={checkAll}
            >
              {checkingAll ? t("fleet.checking") : t("fleet.checkAll")}
            </Button>
            <Button size="sm" icon={Plus} onClick={() => setShowAdd(true)}>
              {t("fleet.addNode")}
            </Button>
          </div>
        }
      >
        <p className="text-small text-muted">{t("fleet.headerIntro")}</p>

        {discoveryConfigLoaded && (
          <div className="mt-3 flex items-center gap-3 rounded-lg border border-border bg-surface p-3">
            <Toggle
              checked={discoveryEnabled}
              busy={discoveryLoading}
              onChange={toggleDiscovery}
              label={t("fleet.discoveryEnabled")}
            />
            <div>
              <p className="text-body font-medium">{t("fleet.discoveryEnabled")}</p>
              <p className="text-caption text-muted">{t("fleet.discoveryRestartNote")}</p>
            </div>
          </div>
        )}

        {nodesError ? (
          <EmptyState
            small
            illustration={<CloudOff size={24} />}
            title={t("common.loadError")}
            action={<Button variant="secondary" size="sm" onClick={loadNodes}>{t("common.retry")}</Button>}
          />
        ) : loadingNodes ? (
          <div className="flex flex-col gap-[var(--card-gap)] mt-4">
            <Skeleton className="h-32" />
            <Skeleton className="h-32" />
          </div>
        ) : nodesEmpty ? (
          <EmptyState
            illustration={<IlluFleet size={140} />}
            title={t("fleet.emptyTitle")}
            body={t("fleet.emptyBody")}
            action={<Button icon={Plus} onClick={() => setShowAdd(true)}>{t("fleet.addFirst")}</Button>}
          />
        ) : null}
      </Card>

      {/* ════ Routers descubiertos ════ */}
      {!loadingNodes && !nodesError && discoveredError && (
        <Card index={1} icon={Wifi} iconTone="accent" title={t("fleet.discoveredTitle")}>
          <EmptyState
            small
            illustration={<CloudOff size={24} />}
            title={t("common.loadError")}
            action={<Button variant="secondary" size="sm" onClick={loadDiscovered}>{t("common.retry")}</Button>}
          />
        </Card>
      )}
      {!loadingNodes && !nodesError && !discoveredError && discovered && discovered.length > 0 && (
        <Card index={1} icon={Wifi} iconTone="accent" title={t("fleet.discoveredTitle")}>
          <p className="text-small text-muted mb-3">{t("fleet.discoveredIntro")}</p>
          <div className="flex flex-col gap-[var(--card-gap)]">
            {discovered.map((peer, i) => (
              <article
                key={peer.id}
                style={{ "--i": Math.min(i, 7), animationDelay: `${i * 70}ms` } as CSSProperties}
                className="animate-fade-up rounded-lg border border-border bg-surface p-4 shadow-card flex flex-col sm:flex-row sm:items-center gap-3"
              >
                <span className="relative shrink-0">
                  <IconTile icon={roleIcon(peer.name)} tone="accent" />
                </span>
                <div className="flex-1 min-w-0">
                  <h3 className="text-h3 truncate">{peer.name}</h3>
                  <p className="text-caption text-muted">{peer.address}:{peer.port} · v{peer.version}</p>
                </div>
                <Button size="sm" icon={Plus} onClick={() => setToAdopt(peer)}>
                  {t("fleet.adopt")}
                </Button>
              </article>
            ))}
          </div>
        </Card>
      )}

      {/* ════ Grid de tarjetas de equipo (fleet.md §2) ════ */}
      {!loadingNodes && !nodesError && nodes && nodes.length > 0 && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          {nodes.map((node, i) => (
            <NodeCard
              key={node.id}
              node={node}
              index={i}
              checkingAll={checkingAll}
              onChecked={checkedOne}
              onAskUpdate={() => setToUpdate(node)}
              onAskRemove={() => setToRemove(node)}
            />
          ))}
          {/* Card punteada "Añadir equipo" */}
          <button
            type="button"
            onClick={() => setShowAdd(true)}
            style={{ "--i": Math.min(nodes.length, 7), animationDelay: `${nodes.length * 70}ms` } as CSSProperties}
            className="animate-fade-up rounded-lg border border-dashed border-border-strong p-4
              flex items-center justify-center gap-2 text-muted hover:text-accent hover:border-accent
              transition-colors duration-[var(--dur-fast)] ring-focus"
          >
            <Plus size={20} aria-hidden="true" />
            <span className="text-body font-medium">{t("fleet.addNode")}</span>
          </button>
        </div>
      )}

      {/* ════ Enlace sutil a NetPulse (fleet.md §5) ════ */}
      <p className="text-center text-small text-muted">
        {t("fleet.netpulseHint")}{" "}
        <a href={NETPULSE_URL} target="_blank" rel="noopener noreferrer"
          className="text-accent hover:text-accent-hover ring-focus rounded-sm">
          {t("fleet.netpulseLink")} →
        </a>
      </p>

      <AddNodeModal
        open={showAdd}
        onClose={() => setShowAdd(false)}
        onAdded={(name) => {
          setShowAdd(false);
          toast.push({ tone: "ok", text: t("fleet.addedOk", { name }) });
          loadNodes();
        }}
      />

      <AdoptNodeModal
        peer={toAdopt}
        open={!!toAdopt}
        onClose={() => setToAdopt(undefined)}
        onAdopted={(name) => {
          setToAdopt(undefined);
          toast.push({ tone: "ok", text: t("fleet.adoptedOk", { name }) });
          loadNodes();
          loadDiscovered();
        }}
      />

      <ConfirmDialog
        open={!!toUpdate}
        onClose={() => setToUpdate(undefined)}
        onConfirm={doUpdate}
        title={t("fleet.updateTitle", { name: toUpdate?.name ?? "" })}
        consequence={t("fleet.updateConsequence", { name: toUpdate?.name ?? "" })}
        confirmLabel={t("fleet.updateConfirm")}
        busy={dialogBusy}
      />
      <ConfirmDialog
        open={!!toRemove}
        onClose={() => setToRemove(undefined)}
        onConfirm={doRemove}
        title={t("fleet.removeTitle", { name: toRemove?.name ?? "" })}
        consequence={t("fleet.removeConsequence")}
        confirmLabel={t("fleet.removeConfirm")}
        busy={dialogBusy}
      />
    </div>
  );
}

/* ══════════════ Card de equipo (fleet.md §2) ══════════════ */

function NodeCard({ node, index, checkingAll, onChecked, onAskUpdate, onAskRemove }: {
  node: FleetNodeStatus;
  index: number;
  checkingAll: boolean;
  onChecked: (n: FleetNodeStatus) => void;
  onAskUpdate: () => void;
  onAskRemove: () => void;
}) {
  const { t } = useTranslation();
  const [checking, setChecking] = useState(false);

  const pulsing = checking || checkingAll;
  const open = () => window.open(panelUrl(node.address), "_blank", "noopener,noreferrer");

  const check = async () => {
    setChecking(true);
    try {
      onChecked(await api.checkFleetNode(node.id));
    } catch (e) {
      onChecked({ ...node, reachable: false, error: e instanceof Error ? e.message : String(e) });
    } finally {
      setChecking(false);
    }
  };

  const dotTone = pulsing ? "bg-faint animate-pulse-dot" : node.reachable ? "bg-ok" : "bg-danger";

  return (
    <article
      style={{ "--i": Math.min(index, 7), animationDelay: `${index * 70}ms` } as CSSProperties}
      className="animate-fade-up rounded-lg border border-border bg-surface p-4 shadow-card flex flex-col"
    >
      <div className="flex items-start gap-2.5">
        <span className="relative shrink-0">
          <IconTile icon={roleIcon(node.name)} tone="teal" />
          <span
            aria-hidden="true"
            className={`absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full ring-2 ring-surface transition-colors duration-200 ${dotTone}`}
          />
        </span>
        <div className="flex-1 min-w-0">
          <h2 className="text-h2 truncate">{node.name}</h2>
          <p className="text-caption text-muted">{t(`fleet.${roleKey(node.name)}`)}</p>
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          {pulsing ? (
            <Pill tone="muted">{t("fleet.checking")}</Pill>
          ) : node.reachable ? (
            <Pill tone="ok">{t("fleet.online")}</Pill>
          ) : (
            <span title={node.error}>
              <Pill tone="danger">{t("fleet.offline")}</Pill>
            </span>
          )}
          {node.update_available && !pulsing && (
            <Pill tone="warn">{t("fleet.updatePill", { version: `v${node.latest_version}` })}</Pill>
          )}
        </div>
      </div>

      <KeyValue
        className="mt-2"
        items={[
          {
            label: t("fleet.address"),
            value: (
              <button type="button" onClick={open}
                className="text-accent hover:text-accent-hover ring-focus rounded-sm cursor-pointer">
                {node.address}
              </button>
            ),
            mono: true,
          },
          {
            label: t("fleet.versionCurrent"),
            value: node.reachable && node.current_version ? `v${node.current_version}` : "—",
            mono: true,
          },
        ]}
      />

      {node.error && !pulsing && (
        <AdvancedDisclosure label={t("fleet.technicalDetail")} className="mt-1">
          <pre className="max-h-32 overflow-auto rounded-sm bg-surface-2 border border-border p-2 font-mono text-caption whitespace-pre-wrap">
            {node.error}
          </pre>
        </AdvancedDisclosure>
      )}

      {/* Acciones: fila inferior pegada abajo, 44px touch en móvil (fleet.md §6) */}
      <div className="flex-1 min-h-3" aria-hidden="true" />
      <div className="pt-3 border-t border-border/60 flex flex-wrap items-center gap-1">
        <Button size="sm" icon={ExternalLink} onClick={open} className="max-sm:h-11">
          {t("fleet.openPanel")}
        </Button>
        <Button variant="ghost" size="sm" icon={RefreshCw} loading={checking} onClick={check} className="max-sm:h-11">
          {t("fleet.check")}
        </Button>
        {node.update_available && (
          <Button variant="ghost" size="sm" icon={Download} onClick={onAskUpdate} className="max-sm:h-11">
            {t("fleet.update")}
          </Button>
        )}
        <Button variant="ghost" size="sm" icon={Trash2} onClick={onAskRemove}
          aria-label={t("fleet.removeTitle", { name: node.name })}
          className="ml-auto max-sm:h-11">
          {t("fleet.remove")}
        </Button>
      </div>
    </article>
  );
}

/* ══════════════ Añadir equipo (fleet.md §3) ══════════════ */

function AddNodeModal({ open, onClose, onAdded }: {
  open: boolean;
  onClose: () => void;
  onAdded: (name: string) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [id, setId] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [fail, setFail] = useState<string>();

  const reset = () => {
    setName(""); setAddress(""); setId(""); setPassword(""); setFail(undefined); setBusy(false);
  };

  const close = () => { reset(); onClose(); };

  const submit = async () => {
    setBusy(true);
    setFail(undefined);
    try {
      await api.addFleetNode({ id: id.trim() || slugify(name), name: name.trim(), address: address.trim(), password });
      reset();
      onAdded(name.trim());
    } catch (e) {
      // Validación llana (fleet.md §3) + detalle crudo para el admin técnico.
      setFail(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const valid = name.trim().length > 0 && address.trim().length > 0;

  return (
    <Modal
      open={open}
      onClose={close}
      title={t("fleet.addTitle")}
      footer={
        <>
          <Button variant="ghost" onClick={close}>{t("common.cancel")}</Button>
          <Button onClick={submit} disabled={!valid} loading={busy}>
            {busy ? t("fleet.addChecking") : t("fleet.add")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field
          label={t("fleet.nameLabel")}
          inputProps={{
            value: name,
            onChange: (e) => setName(e.target.value),
            placeholder: t("fleet.namePlaceholder"),
            autoFocus: true,
          }}
        />
        <Field
          label={t("fleet.addressLabel")}
          hint={t("fleet.addressHint")}
          mono
          inputProps={{
            value: address,
            onChange: (e) => setAddress(e.target.value),
            placeholder: "192.168.8.2:8090",
          }}
        />
        <AdvancedDisclosure label={t("common.advanced")}>
          <div className="flex flex-col gap-3 pt-1">
            <Field
              label={t("fleet.nodeIdLabel")}
              hint={t("fleet.nodeIdHint")}
              mono
              inputProps={{
                value: id,
                onChange: (e) => setId(e.target.value),
                placeholder: slugify(name) || t("fleet.namePlaceholder"),
              }}
            />
            <Field
              label={t("fleet.passwordLabel")}
              hint={t("fleet.passwordHint")}
              icon={Lock}
              inputProps={{
                type: "password",
                value: password,
                onChange: (e) => setPassword(e.target.value),
              }}
            />
          </div>
        </AdvancedDisclosure>

        {fail && (
          <div className="rounded-md bg-danger-soft px-3.5 py-3 animate-banner-in" role="alert">
            <p className="text-small text-danger">{t("fleet.addError")}</p>
            <AdvancedDisclosure label={t("fleet.technicalDetail")} className="mt-1">
              <pre className="max-h-32 overflow-auto rounded-sm bg-surface/60 border border-border p-2 font-mono text-caption whitespace-pre-wrap">
                {fail}
              </pre>
            </AdvancedDisclosure>
          </div>
        )}
      </div>
    </Modal>
  );
}

/* ══════════════ Adoptar equipo descubierto (autodiscover #178) ══════════════ */

function AdoptNodeModal({ peer, open, onClose, onAdopted }: {
  peer?: DiscoveredFleetPeer;
  open: boolean;
  onClose: () => void;
  onAdopted: (name: string) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [fail, setFail] = useState<string>();

  useEffect(() => {
    if (peer) {
      setName(peer.name || "");
      setAddress(peer.address ? `${peer.address}:${peer.port || 8090}` : "");
      setPassword("");
      setFail(undefined);
      setBusy(false);
    }
  }, [peer]);

  const reset = () => {
    setName(""); setAddress(""); setPassword(""); setFail(undefined); setBusy(false);
  };

  const close = () => { reset(); onClose(); };

  const submit = async () => {
    if (!peer) return;
    setBusy(true);
    setFail(undefined);
    try {
      await api.adoptFleetPeer({ id: peer.id, name: name.trim(), address: address.trim(), password });
      reset();
      onAdopted(name.trim());
    } catch (e) {
      setFail(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const valid = name.trim().length > 0 && address.trim().length > 0 && password.length > 0;

  return (
    <Modal
      open={open}
      onClose={close}
      title={t("fleet.adoptTitle", { name: peer?.name ?? "" })}
      footer={
        <>
          <Button variant="ghost" onClick={close}>{t("common.cancel")}</Button>
          <Button onClick={submit} disabled={!valid} loading={busy}>
            {busy ? t("fleet.adoptChecking") : t("fleet.adopt")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <p className="text-small text-muted">{t("fleet.adoptIntro", { name: peer?.name ?? "" })}</p>
        <Field
          label={t("fleet.nameLabel")}
          inputProps={{
            value: name,
            onChange: (e) => setName(e.target.value),
            placeholder: t("fleet.namePlaceholder"),
          }}
        />
        <Field
          label={t("fleet.addressLabel")}
          hint={t("fleet.addressHint")}
          mono
          inputProps={{
            value: address,
            onChange: (e) => setAddress(e.target.value),
            placeholder: "192.168.8.2:8090",
          }}
        />
        <Field
          label={t("fleet.passwordLabel")}
          hint={t("fleet.passwordHint")}
          icon={Lock}
          inputProps={{
            type: "password",
            value: password,
            onChange: (e) => setPassword(e.target.value),
          }}
        />
        {fail && (
          <div className="rounded-md bg-danger-soft px-3.5 py-3 animate-banner-in" role="alert">
            <p className="text-small text-danger">{t("fleet.adoptError")}</p>
            <AdvancedDisclosure label={t("fleet.technicalDetail")} className="mt-1">
              <pre className="max-h-32 overflow-auto rounded-sm bg-surface/60 border border-border p-2 font-mono text-caption whitespace-pre-wrap">
                {fail}
              </pre>
            </AdvancedDisclosure>
          </div>
        )}
      </div>
    </Modal>
  );
}
