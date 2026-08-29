import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Download, Lock, Plus, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { OVPNProbe } from "../../types";
import {
  ActionBanner, Banner, Button, Card, ConfirmDialog, EmptyState, Input,
  KeyValue, Pill, SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { downloadText, Reveal, TechName } from "./shared";

/**
 * OpenVPN — "VPN compatible con todo" (services.md §3). Mantiene el alta de
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
  const [doneMsg, setDoneMsg] = useState<string>();
  const [clientName, setClientName] = useState("");
  const [delTarget, setDelTarget] = useState<string>();

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
      // El .ovpn con todo embebido se descarga al añadir (comportamiento actual)
      downloadText(`${res.name}.ovpn`, res.config, "application/x-openvpn-profile");
      setClientName("");
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

  return (
    <Card index={index}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : !probe.installed ? (
        <EmptyState
          small
          illustration={<Lock size={24} />}
          title={t("ovpn.notInstalled")}
          body={t("services.installFromTools")}
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
            </div>
          )}

          <Reveal open={active}>
            <div className="pt-2 flex flex-col gap-3">
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
                  <Download size={12} aria-hidden="true" /> {t("ovpn.downloadHint")}
                </p>
              </div>
            </div>
          </Reveal>

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
