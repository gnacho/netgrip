import { useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertTriangle, Copy, Download, Globe, Lock, Plus, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { OVPNProbe } from "../../types";
import {
  ActionBanner, Banner, Button, Card, ConfirmDialog, Input,
  KeyValue, Modal, Pill, SettingRow, SkeletonRows, Toggle, useToast,
} from "../ui";
import { ServiceRow } from "./ServiceRow";
import { useActionCycle } from "../wifi/action";
import { downloadText, Reveal, TechName } from "./shared";

/**
 * OpenVPN - "VPN compatible con todo" (services.md #3). Mantiene el alta de
 * cliente con descarga automática del .ovpn y la revocación. Si falta la PKI
 * solo podemos avisar: la API no expone acción para crear los certificados.
 */
export function OpenvpnCard({ probe, onChange, index = 0 }: {
  probe: OVPNProbe | undefined;
  onChange: (p: OVPNProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const toast = useToast();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [clientName, setClientName] = useState("");
  const [delTarget, setDelTarget] = useState<string>();
  const [addedConfig, setAddedConfig] = useState<{ name: string; config: string } | undefined>();
  const [hostDraft, setHostDraft] = useState<string | undefined>(undefined);
  const [savingHost, setSavingHost] = useState(false);

  const toggle = async (v: boolean) => {
    setDoneMsg(undefined);
    const res = await run(() => api.setOpenvpn(v ? "enable" : "disable"));
    if (res) {
      onChange(res.state);
      if (res.status === "applied") setDoneMsg(v ? t("ovpn.doneOn") : t("ovpn.doneOff"));
    } else {
      onChange(await api.openvpn());
    }
  };

  const addClient = async (e: React.FormEvent) => {
    e.preventDefault();
    setDoneMsg(undefined);
    const res = await run(async () => {
      const r = await api.addOvpnClient(clientName);
      return { status: "applied" as const, state: r.state, config: r.config, name: clientName };
    });
    if (res) {
      onChange(res.state);
      setClientName("");
      setAddedConfig({ name: res.name, config: res.config });
      setDoneMsg(t("ovpn.clientAdded", { name: res.name }));
    } else {
      onChange(await api.openvpn());
    }
  };

  const deleteClient = async (name: string) => {
    setDoneMsg(undefined);
    const res = await run(async () => {
      const r = await api.deleteOvpnClient(name);
      return { status: "applied" as const, state: r.state };
    });
    if (res) {
      onChange(res.state);
      setDoneMsg(t("ovpn.clientDeleted", { name }));
    } else {
      onChange(await api.openvpn());
    }
  };

  const active = probe?.active ?? false;
  const [installing, setInstalling] = useState(false);
  const install = async () => {
    setInstalling(true);
    try {
      await api.wizardPackages(["openvpn"]);
      onChange(await api.openvpn());
    } catch { /* el probe refresca el estado */ }
    setInstalling(false);
  };

  const hostValue = hostDraft !== undefined ? hostDraft : probe?.public_host ?? "";
  const saveHost = async (host: string) => {
    setSavingHost(true);
    try {
      onChange(await api.setOvpnPublicHost(host.trim()));
      setHostDraft(undefined);
      toast.push({ tone: "ok", text: host.trim() ? t("ovpn.hostSaved") : t("ovpn.hostCleared") });
    } catch {
      toast.push({ tone: "danger", text: t("ovpn.hostSaveFailed") });
    }
    setSavingHost(false);
  };

  return (
    <Card index={index}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : !probe.installed ? (
        <ServiceRow
          icon={<Lock size={18} />}
          title={t("ovpn.notInstalled")}
          description={t("services.installHint")}
          action={<Button size="sm" onClick={install} loading={installing}>{t("services.installNow")}</Button>}
        />
      ) : (
        <>
          <SettingRow
            icon={Lock}
            iconTone="violet"
            title={t("ovpn.cardTitle")}
            description={t("ovpn.desc")}
            help={t("help.openvpn.body")}
            helpTitle={t("help.openvpn.title")}
            checked={active}
            busy={busy}
            onChange={toggle}
            control={
              <span className="flex items-center gap-2">
                <Pill className="max-w-24 sm:max-w-32" tone={probe.running ? "ok" : active ? "warn" : "muted"}>
                  {probe.running ? t("ovpn.running") : active ? t("ovpn.configured") : t("ovpn.off")}
                </Pill>
                <Toggle checked={active} busy={busy} onChange={toggle} label={t("ovpn.cardTitle")} />
              </span>
            }
          />
          <TechName>OpenVPN</TechName>

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          {!probe.has_pki && (
            <div className="mt-2">
              <Banner tone="warn">{t("ovpn.noPki")}</Banner>
              <p className="text-caption text-muted mt-1.5">{t("ovpn.noPkiSteps")}</p>
            </div>
          )}

          <Reveal open={active}>
            <div className="pt-2 flex flex-col gap-3">
              {!probe.public_host && (
                <div role="alert" className="flex gap-3 rounded-lg bg-warn-soft px-3.5 py-3">
                  <AlertTriangle size={18} className="mt-0.5 shrink-0 text-warn" aria-hidden="true" />
                  <div className="min-w-0 text-small leading-snug text-warn">
                    <p className="font-semibold">{t("ovpn.ddnsWarnTitle")}</p>
                    <p className="mt-0.5">{t("ovpn.ddnsWarnBody", { ip: probe.wan_ip || "—" })}</p>
                    {probe.ddns_domains.length > 0 && (
                      <>
                        <p className="mt-2 font-medium">{t("ovpn.ddnsSuggest")}</p>
                        <div className="mt-1.5 flex flex-wrap gap-1.5">
                          {probe.ddns_domains.map((d) => (
                            <button
                              key={d}
                              type="button"
                              onClick={() => setHostDraft(d)}
                              className="rounded-full border border-warn/40 bg-surface px-2.5 py-1 font-mono text-caption text-warn ring-focus transition-colors hover:bg-warn/10"
                            >
                              {d}
                            </button>
                          ))}
                        </div>
                      </>
                    )}
                  </div>
                </div>
              )}

              <div className="flex flex-col gap-2 rounded-lg border border-border bg-surface-2 px-3.5 py-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="flex items-center gap-2 text-small font-medium">
                    <Globe size={14} className="text-accent" aria-hidden="true" />
                    {t("ovpn.publicHostLabel")}
                  </span>
                  <span title={probe.public_host ? probe.public_host : t("ovpn.wanChipTitle")}>
                    <Pill tone={probe.public_host ? "ok" : "warn"} className="max-w-[16rem] truncate font-mono">
                      {probe.public_host || probe.wan_ip || "—"}
                    </Pill>
                  </span>
                </div>
                <div className="flex flex-col gap-2 sm:flex-row">
                  {probe.ddns_domains.length > 0 && (
                    <select
                      className="h-10 rounded-md border border-border/60 bg-surface px-3 text-small text-text ring-focus"
                      value={probe.ddns_domains.includes(hostValue) ? hostValue : ""}
                      onChange={(e) => setHostDraft(e.target.value)}
                      aria-label={t("ovpn.ddnsPick")}
                    >
                      <option value="">{t("ovpn.ddnsPick")}</option>
                      {probe.ddns_domains.map((d) => (
                        <option key={d} value={d}>{d}</option>
                      ))}
                    </select>
                  )}
                  <div className="flex flex-1 gap-2">
                    <div className="flex-1">
                      <Input
                        value={hostValue}
                        onChange={(e) => setHostDraft(e.target.value)}
                        placeholder={t("ovpn.publicHostPlaceholder")}
                        maxLength={253}
                        aria-label={t("ovpn.publicHostLabel")}
                      />
                    </div>
                    <Button
                      variant="secondary"
                      size="sm"
                      className="h-10"
                      loading={savingHost}
                      disabled={hostValue.trim() === (probe.public_host ?? "")}
                      onClick={() => saveHost(hostValue)}
                    >
                      {t("ovpn.hostSave")}
                    </Button>
                  </div>
                </div>
                <p className="text-caption text-muted">{t("ovpn.publicHostHint")}</p>
              </div>

              <KeyValue items={[
                { label: t("ovpn.port"), value: probe.port, mono: true },
                { label: t("ovpn.subnet"), value: probe.subnet, mono: true },
              ]} />

              <div>
                <p className="text-small font-medium mb-1">{t("ovpn.clients")}</p>
                {probe.clients.length === 0 && (
                  <p className="text-small text-muted py-1">{t("ovpn.noClients")}</p>
                )}
                <ul>
                  {probe.clients.map((c) => (
                    <li key={c.name} className="flex items-center gap-3 py-2 border-b border-border/60 last:border-0">
                      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent-soft text-accent text-small font-semibold" aria-hidden="true">
                        {c.name.charAt(0).toUpperCase()}
                      </span>
                      <span className="min-w-0 flex-1 truncate text-body font-medium">{c.name}</span>
                      <button
                        type="button"
                        onClick={() => setDelTarget(c.name)}
                        title={t("ovpn.deleteClient")}
                        aria-label={t("ovpn.deleteClient")}
                        className="inline-flex h-8 w-8 items-center justify-center rounded-sm text-muted hover:text-danger hover:bg-surface-2 ring-focus transition-colors"
                      >
                        <Trash2 size={16} aria-hidden="true" />
                      </button>
                    </li>
                  ))}
                </ul>
                <form onSubmit={addClient} className="mt-2 flex gap-2">
                  <div className="flex-1">
                    <Input value={clientName} onChange={(e) => setClientName(e.target.value)}
                      placeholder={t("ovpn.clientName")} required maxLength={40} />
                  </div>
                  <Button type="submit" variant="secondary" size="sm" icon={Plus} className="h-10"
                    loading={busy} disabled={!clientName.trim()}>
                    {t("ovpn.addClient")}
                  </Button>
                </form>
                <p className="text-caption text-muted mt-1.5 flex items-center gap-1">
                  <Download size={12} aria-hidden="true" /> {t("ovpn.configHint")}
                </p>
              </div>
            </div>
          </Reveal>

          <Modal
            open={!!addedConfig}
            onClose={() => setAddedConfig(undefined)}
            title={addedConfig ? t("ovpn.configModalTitle", { name: addedConfig.name }) : ""}
            footer={
              <Button variant="ghost" onClick={() => setAddedConfig(undefined)}>
                {t("common.close")}
              </Button>
            }
          >
            {addedConfig && (
              <OvpnConfigView
                name={addedConfig.name}
                config={addedConfig.config}
              />
            )}
          </Modal>

          <ConfirmDialog
            open={!!delTarget}
            onClose={() => setDelTarget(undefined)}
            onConfirm={() => {
              const name = delTarget;
              setDelTarget(undefined);
              if (name) deleteClient(name);
            }}
            title={t("ovpn.revokeTitle", { name: delTarget ?? "" })}
            consequence={t("ovpn.revokeConsequence")}
            confirmLabel={t("ovpn.revokeConfirm")}
            busy={busy}
          />
        </>
      )}
    </Card>
  );
}

function OvpnConfigView({ name, config }: { name: string; config: string }) {
  const { t } = useTranslation();
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(config);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
      toast.push({ tone: "ok", text: t("ovpn.copyConfig") });
    } catch {
      // clipboard bloqueado
    }
  };
  const download = () => downloadText(`${name}.ovpn`, config, "application/x-openvpn-profile");
  return (
    <div className="flex flex-col gap-3">
      <p className="text-small text-muted text-center">{t("ovpn.configHint")}</p>
      <textarea
        readOnly
        value={config}
        rows={10}
        className="w-full rounded-md border border-border/60 bg-surface-2 px-3 py-2 font-mono text-caption focus:outline-none focus:ring-2 focus:ring-accent resize-none"
        aria-label={t("ovpn.viewConfig")}
      />
      <div className="flex flex-wrap items-center justify-center gap-2">
        <Button variant="secondary" size="sm" icon={Copy} onClick={copy}>
          {copied ? t("services.copied") : t("ovpn.copyConfig")}
        </Button>
        <Button variant="secondary" size="sm" icon={Download} onClick={download}>
          {t("ovpn.downloadConf")}
        </Button>
      </div>
    </div>
  );
}
