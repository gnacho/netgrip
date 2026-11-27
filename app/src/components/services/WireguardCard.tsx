import { useState } from "react";
import { useTranslation } from "react-i18next";
import { copyText } from "../../lib/clipboard";
import { Pencil, Plus, QrCode, ShieldCheck, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { WGPeer, WGProbe } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Banner, Button, Card, ConfirmDialog,
  Field, HelpTip, Input, Modal, Pill, SkeletonRows, Toggle, useToast,
} from "../ui";
import { ServiceRow } from "./ServiceRow";
import { useActionCycle } from "../wifi/action";
import { CopyButton, downloadText, qrMatrixSvg, Reveal, shortKey } from "./shared";

/**
 * WireGuard — "Entra en tu casa desde fuera" (services.md §2). Card héroe a
 * ancho completo: SettingRow + detalle (puerto/dirección/clave) + peers con
 * borrado confirmado + alta de dispositivo con QR.
 * Nota: la API solo devuelve la config del peer al crearlo (la clave privada
 * no se guarda), así que el QR grande solo existe justo tras el alta.
 */
/**
 * Ficha de un peer/usuario (#714): tarjetas en rejilla de tres en escritorio.
 * `onDelete` ausente = lista de solo lectura (túneles gestionados por GL.iNet).
 */
function PeerCard({ peer, onEdit, onQr, busy }: {
  peer: WGPeer;
  onEdit?: () => void;
  onQr?: () => void;
  busy?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex min-w-0 flex-col gap-2 rounded-lg border border-border bg-surface-2 p-3">
      <div className="flex min-w-0 items-center gap-2.5">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent-soft text-accent text-small font-semibold" aria-hidden="true">
          {(peer.name || peer.public_key).charAt(0).toUpperCase()}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-body font-medium" title={peer.name || peer.public_key}>
            {peer.name || shortKey(peer.public_key)}
          </span>
          <span className="block truncate font-mono text-caption text-muted">
            {peer.allowed_ips.join(", ") || "-"}
          </span>
        </span>
      </div>
      <div className="mt-auto flex items-center gap-1">
        {peer.admin && <Pill tone="muted">{t("wg.adminPill")}</Pill>}
        <span className="ml-auto inline-flex items-center gap-1">
          {onEdit && (
            <button type="button" onClick={onEdit} disabled={busy}
              title={t("wg.editPeer")} aria-label={`${t("wg.editPeer")} ${peer.name || peer.public_key}`}
              className="inline-flex h-8 w-8 items-center justify-center rounded-sm text-muted hover:text-accent hover:bg-surface transition-colors ring-focus disabled:opacity-40">
              <Pencil size={16} aria-hidden="true" />
            </button>
          )}
          {onQr && (
            <button type="button" onClick={onQr} disabled={busy}
              title={t("wg.showQr")} aria-label={`${t("wg.showQr")} ${peer.name || peer.public_key}`}
              className="inline-flex h-8 w-8 items-center justify-center rounded-sm text-muted hover:text-accent hover:bg-surface transition-colors ring-focus disabled:opacity-40">
              <QrCode size={16} aria-hidden="true" />
            </button>
          )}
        </span>
      </div>
    </div>
  );
}

export function WireguardCard({ probe, onChange, index = 0 }: {
  probe: WGProbe | undefined;
  onChange: (p: WGProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const { push } = useToast();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [addOpen, setAddOpen] = useState(false);
  const [peerName, setPeerName] = useState("");
  const [peerKey, setPeerKey] = useState("");
  const [peerAdmin, setPeerAdmin] = useState(false);
  const [qrConfig, setQrConfig] = useState<string>();
  const [endpointOpen, setEndpointOpen] = useState(false);
  const [endpointDraft, setEndpointDraft] = useState("");
  const [delTarget, setDelTarget] = useState<WGPeer>();
  const [renamePeer, setRenamePeer] = useState<WGPeer>();
  const [renameName, setRenameName] = useState("");
  // Peer whose private key was never stored: se ofrece generar claves nuevas.
  const [keylessPeer, setKeylessPeer] = useState<WGPeer>();
  const [peerBusy, setPeerBusy] = useState<string>();

  const apply = async (fn: () => Promise<{ state: WGProbe; status: string; error?: string }>, okText?: string) => {
    setDoneMsg(undefined);
    const res = await run(fn);
    if (res) {
      onChange(res.state);
      if (res.status === "applied" && okText) setDoneMsg(okText);
    } else {
      onChange(await api.wireguard());
    }
    return res;
  };

  const toggle = (v: boolean) =>
    apply(() => api.setWireguard(v ? "enable" : "disable"), v ? t("wg.doneOn") : t("wg.doneOff"));

  // #756: volver a pintar el QR de un usuario ya creado.
  const showPeerQr = async (p: WGPeer) => {
    setPeerBusy(p.public_key);
    try {
      const r = await api.wgPeerConfig(p.public_key);
      setQrConfig(r.config);
    } catch {
      setKeylessPeer(p);
    } finally {
      setPeerBusy(undefined);
    }
  };

  const rotateKeyless = async () => {
    if (!keylessPeer) return;
    const p = keylessPeer;
    setPeerBusy(p.public_key);
    try {
      const r = await api.wgRotatePeer(p.public_key);
      onChange(r.state);
      setKeylessPeer(undefined);
      setQrConfig(r.config);
      push({ tone: "ok", text: t("wg.rotated") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setPeerBusy(undefined);
    }
  };

  const saveRename = async () => {
    if (!renamePeer) return;
    setPeerBusy(renamePeer.public_key);
    try {
      const r = await api.wgRenamePeer(renamePeer.public_key, renameName.trim());
      if (r.state) onChange(r.state);
      setRenamePeer(undefined);
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setPeerBusy(undefined);
    }
  };

  const addWithQr = async () => {
    setDoneMsg(undefined);
    const res = await run(async () => {
      const r = await api.addWgPeerQr(peerName, peerAdmin);
      return { status: "applied" as const, state: r.state, config: r.config };
    });
    if (res) {
      onChange(res.state);
      setQrConfig(res.config);
      setAddOpen(false);
      setPeerName(""); setPeerKey(""); setPeerAdmin(false);
    } else {
      onChange(await api.wireguard());
    }
  };

  const addWithKey = () =>
    apply(() => api.addWgPeer(peerName, peerKey, peerAdmin), t("wg.peerAdded")).then((res) => {
      if (res?.status === "applied") {
        setAddOpen(false);
        setPeerName(""); setPeerKey(""); setPeerAdmin(false);
      }
    });

  const saveEndpoint = async (value: string) => {
    setDoneMsg(undefined);
    try {
      onChange(await api.setWgEndpoint(value.trim()));
      setEndpointOpen(false);
    } catch (e) {
      push({ tone: "danger", text: t("wg.endpointFailed"), detail: e instanceof Error ? e.message : String(e) });
    }
  };

  const active = probe?.active ?? false;
  const glManaged = probe?.managed_by === "gl_firmware";
  const glTunnels = probe?.gl_tunnels ?? [];
  const [installing, setInstalling] = useState(false);
  const install = async () => {
    setInstalling(true);
    try {
      await api.wizardPackages(["wireguard"]);
      onChange(await api.wireguard());
    } catch { /* el probe refresca el estado */ }
    setInstalling(false);
  };

  return (
    <Card index={index} title="WireGuard" icon={ShieldCheck} iconTone="violet">
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : !probe.installed ? (
        <ServiceRow
          icon={<ShieldCheck size={18} />}
          title={t("wg.notInstalled")}
          description={t("services.installHint")}
          action={<Button size="sm" onClick={install} loading={installing}>{t("services.installNow")}</Button>}
        />
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-3">
            <p className="flex-1 min-w-0 text-small text-muted">{t("wg.desc")}</p>
            <span className="flex shrink-0 items-center gap-2">
              <Pill className="max-w-24 sm:max-w-32" tone={active ? "ok" : "muted"}>
                {active ? t("wg.activeCount", { count: probe.peers.length }) : t("wg.off")}
              </Pill>
              <Toggle checked={active} busy={busy} disabled={glManaged && !active} onChange={toggle} label="WireGuard" />
              <HelpTip title={t("help.wireguard.title")} body={t("help.wireguard.body")} />
            </span>
          </div>

          {glManaged && (
            <div className="mt-3 flex flex-col gap-3" data-testid="wg-gl-managed">
              <div className="flex flex-wrap items-center gap-2">
                <Pill tone="ok">{t("wg.running")}</Pill>
                <Pill tone="muted">{t("wg.glManagedBadge")}</Pill>
              </div>
              {glTunnels.map((tun) => (
                <div key={tun.iface} className="rounded-md border border-border/60 bg-surface-2 p-3 flex flex-col gap-2">
                  <div className="grid gap-x-6 gap-y-3 sm:grid-cols-3">
                    <div>
                      <p className="text-caption text-muted">{t("wg.port")}</p>
                      <p className="font-mono text-body">{tun.port || "-"}</p>
                    </div>
                    <div>
                      <p className="text-caption text-muted">{t("wg.tunnelAddress")}</p>
                      <p className="font-mono text-body">{tun.address || "-"}</p>
                    </div>
                    <div className="min-w-0">
                      <p className="text-caption text-muted">{t("wg.serverKey")}</p>
                      <span className="inline-flex items-center gap-1">
                        <span className="font-mono text-body truncate">{shortKey(tun.public_key)}</span>
                        <CopyButton text={tun.public_key} label={t("wg.copyKey")} />
                      </span>
                    </div>
                  </div>
                  <div>
                    <p className="text-small font-medium mb-1">{t("wg.yourDevices")}</p>
                    {tun.peers.length === 0 && (
                      <p className="text-small text-muted py-1">{t("wg.noPeers")}</p>
                    )}
                    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                      {tun.peers.map((p) => (
                        <PeerCard key={p.public_key} peer={p} />
                      ))}
                    </div>
                  </div>
                </div>
              ))}
              <p className="text-small text-muted">{t("wg.glNote")}</p>
            </div>
          )}

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          <Reveal open={active}>
            <div className="pt-2 flex flex-col gap-3">
              <div className="grid gap-x-6 gap-y-3 sm:grid-cols-3">
                <div>
                  <p className="text-caption text-muted">{t("wg.port")}</p>
                  <p className="font-mono text-body">{probe.port}</p>
                </div>
                <div>
                  <p className="text-caption text-muted">{t("wg.tunnelAddress")}</p>
                  <p className="font-mono text-body">{probe.address}</p>
                </div>
                <div className="min-w-0">
                  <p className="text-caption text-muted">{t("wg.serverKey")}</p>
                  <span className="inline-flex items-center gap-1">
                    <span className="font-mono text-body truncate">{shortKey(probe.public_key)}</span>
                    <CopyButton text={probe.public_key} label={t("wg.copyKey")} />
                  </span>
                </div>
              </div>

              {/* El nombre externo es lo que hace que la VPN sobreviva a un
                  cambio de IP del operador (#629). */}
              <div className="rounded-md border border-border/60 px-3 py-2">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="inline-flex items-center gap-1 text-small font-medium">
                    {t("wg.endpointTitle")}
                    <HelpTip title={t("wg.endpointTitle")} body={t("wg.endpointHelp")} />
                  </span>
                  <span className="ml-auto flex items-center gap-2">
                    <span className="font-mono text-body">{probe.endpoint || t("wg.endpointWanIp")}</span>
                    <Button variant="ghost" size="sm" disabled={glManaged || busy}
                      onClick={() => { setEndpointDraft(probe.endpoint); setEndpointOpen(true); }}>
                      {t("wg.endpointEdit")}
                    </Button>
                  </span>
                </div>
                {!probe.endpoint && (
                  <div className="mt-2">
                    <Banner tone="warn"
                      action={probe.suggested_endpoint ? (
                        <Button size="sm" variant="secondary" onClick={() => saveEndpoint(probe.suggested_endpoint!)}>
                          {t("wg.endpointUseSuggested", { host: probe.suggested_endpoint })}
                        </Button>
                      ) : undefined}>
                      {t("wg.endpointRecommend")}
                    </Banner>
                  </div>
                )}
              </div>

              <div>
                <p className="text-small font-medium mb-1">{t("wg.yourDevices")}</p>
                {probe.peers.length === 0 && (
                  <p className="text-small text-muted py-1">{t("wg.noPeers")}</p>
                )}
                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  {probe.peers.map((p) => (
                    <PeerCard key={p.public_key} peer={p} busy={peerBusy === p.public_key}
                      onEdit={() => { setRenamePeer(p); setRenameName(p.name ?? ""); }}
                      onQr={() => showPeerQr(p)} />
                  ))}
                </div>
                <div className="mt-3">
                  <Button size="sm" icon={Plus} disabled={glManaged} title={glManaged ? t("wg.glDisabledReason") : undefined} onClick={() => setAddOpen(true)}>{t("wg.addDevice")}</Button>
                </div>
              </div>
            </div>
          </Reveal>

          <Modal
            open={endpointOpen}
            onClose={() => setEndpointOpen(false)}
            title={t("wg.endpointTitle")}
            footer={
              <>
                <Button variant="ghost" onClick={() => setEndpointOpen(false)}>{t("common.cancel")}</Button>
                <Button onClick={() => saveEndpoint(endpointDraft)}>{t("common.save")}</Button>
              </>
            }
          >
            <Field label={t("wg.endpointLabel")} mono
              inputProps={{
                value: endpointDraft,
                onChange: (e) => setEndpointDraft(e.target.value),
                placeholder: "casa.midominio.net",
              }} />
            <p className="mt-2 text-caption text-muted">{t("wg.endpointHelp")}</p>
            <p className="mt-1 text-caption text-faint">{t("wg.endpointBlank")}</p>
            {probe.suggested_endpoint && (
              <div className="mt-3">
                <Button variant="secondary" size="sm" onClick={() => setEndpointDraft(probe.suggested_endpoint!)}>
                  {t("wg.endpointUseSuggested", { host: probe.suggested_endpoint })}
                </Button>
              </div>
            )}
          </Modal>

          {/* Modal: añadir dispositivo (QR por defecto, clave pública avanzada) */}
          <Modal open={addOpen} onClose={() => setAddOpen(false)} title={t("wg.addDevice")}>
            <div className="flex flex-col gap-3">
              <Input value={peerName} onChange={(e) => setPeerName(e.target.value)}
                placeholder={t("wg.peerName")} maxLength={40} />
              <p className="text-caption text-muted">
                {t("wg.qrUses", { host: probe.endpoint || t("wg.endpointWanIp") })}
              </p>
              <Button icon={QrCode} loading={busy} onClick={addWithQr}>
                {t("wg.generateQr")}
              </Button>
              <AdvancedDisclosure label={t("wg.advancedKey")}>
                <div className="flex flex-col gap-2">
                  <Input mono value={peerKey} onChange={(e) => setPeerKey(e.target.value)}
                    placeholder={t("wg.peerKey")} />
                  <label className="flex items-center gap-2 text-small text-muted">
                    <input type="checkbox" checked={peerAdmin} onChange={(e) => setPeerAdmin(e.target.checked)} />
                    {t("wg.peerAdmin")}
                  </label>
                  <div>
                    <Button variant="secondary" size="sm" loading={busy} disabled={!peerKey.trim()} onClick={addWithKey}>
                      {t("wg.addWithKey")}
                    </Button>
                  </div>
                </div>
              </AdvancedDisclosure>
            </div>
          </Modal>

          {/* Modal: QR grande + descarga de la config recién creada */}
          <QrConfigModal config={qrConfig} onClose={() => setQrConfig(undefined)} />

          <Modal
            open={!!renamePeer}
            onClose={() => setRenamePeer(undefined)}
            title={t("wg.renameTitle")}
            footer={
              <span className="flex w-full flex-wrap items-center gap-2">
                {renamePeer && !renamePeer.admin && (
                  <button type="button"
                    onClick={() => { const p = renamePeer; setRenamePeer(undefined); setDelTarget(p); }}
                    className="inline-flex h-9 items-center gap-2 rounded-md px-3 text-small font-medium text-danger hover:bg-danger-soft ring-focus transition-colors">
                    <Trash2 size={16} aria-hidden="true" />
                    {t("wg.deletePeer")}
                  </button>
                )}
                <span className="ml-auto flex items-center gap-2">
                  <Button variant="ghost" onClick={() => setRenamePeer(undefined)}>{t("common.cancel")}</Button>
                  <Button loading={renamePeer ? peerBusy === renamePeer.public_key : false} onClick={saveRename}>
                    {t("common.save")}
                  </Button>
                </span>
              </span>
            }
          >
            <Input value={renameName} onChange={(e) => setRenameName(e.target.value)}
              placeholder={t("wg.peerName")} maxLength={40} autoFocus />
            {renamePeer?.admin && (
              <p className="mt-2 text-caption text-muted">{t("wg.adminCannotDelete")}</p>
            )}
          </Modal>

          <Modal
            open={!!keylessPeer}
            onClose={() => setKeylessPeer(undefined)}
            title={t("wg.keylessTitle")}
            footer={
              <>
                <Button variant="ghost" onClick={() => setKeylessPeer(undefined)}>{t("common.cancel")}</Button>
                <Button loading={keylessPeer ? peerBusy === keylessPeer.public_key : false} onClick={rotateKeyless}>
                  {t("wg.rotate")}
                </Button>
              </>
            }
          >
            <p className="text-small text-muted">{t("wg.keylessBody")}</p>
          </Modal>

          <ConfirmDialog
            open={!!delTarget}
            onClose={() => setDelTarget(undefined)}
            onConfirm={() => {
              const p = delTarget;
              setDelTarget(undefined);
              if (p) apply(() => api.deleteWgPeer(p.public_key), t("wg.peerDeleted"));
            }}
            title={t("wg.deleteTitle", { name: delTarget?.name || shortKey(delTarget?.public_key ?? "") })}
            consequence={t("wg.deleteConsequence")}
            confirmLabel={t("wg.deleteConfirm")}
            busy={busy}
          />
        </>
      )}
    </Card>
  );
}

function QrConfigModal({ config, onClose }: { config: string | undefined; onClose: () => void }) {
  const { t } = useTranslation();
  const toast = useToast();
  const svg = qrMatrixSvg(config);
  const [copied, setCopied] = useState(false);
  const copyConfig = async () => {
    if (!config) return;
    if (!(await copyText(config))) return;
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
    toast.push({ tone: "ok", text: t("wg.copyConfig") });
  };
  // El usuario ya sabe que este comando sí dibuja un QR legible; se le da
  // listo para pegar con el fichero que acaba de descargar.
  const copyQrencode = async () => {
    try {
      await navigator.clipboard.writeText("qrencode -t ansiutf8 < wireguard-client.conf");
      toast.push({ tone: "ok", text: t("wg.qrencodeCopied") });
    } catch {
      // clipboard bloqueado
    }
  };
  return (
    <Modal open={!!config} onClose={onClose} title={t("wg.qrModalTitle")}>
      <div className="flex flex-col items-center gap-3 py-2">
        {svg && (
          <span className="block bg-white" style={{ width: 300, height: 300 }}
            role="img" aria-label={t("wg.qrModalTitle")}
            dangerouslySetInnerHTML={{ __html: svg }} />
        )}
        <p className="text-small text-muted text-center">{t("wg.qrHint")}</p>
        {config && (
          <div className="w-full">
            <label className="block text-caption text-muted mb-1.5">{t("wg.viewConfig")}</label>
            <textarea
              readOnly
              value={config}
              rows={8}
              className="w-full rounded-md border border-border/60 bg-surface-2 px-3 py-2 font-mono text-caption focus:outline-none focus:ring-2 focus:ring-accent resize-none"
              aria-label={t("wg.viewConfig")}
            />
          </div>
        )}
        <div className="flex flex-wrap items-center justify-center gap-2 w-full">
          {config && (
            <Button variant="secondary" size="sm" onClick={copyConfig}>
              {copied ? t("services.copied") : t("wg.copyConfig")}
            </Button>
          )}
          <Button variant="secondary" size="sm"
            onClick={() => config && downloadText("wireguard-client.conf", config)}>
            {t("wg.downloadConf")}
          </Button>
          <Button variant="secondary" size="sm" onClick={copyQrencode}>
            {t("wg.copyQrencode")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
