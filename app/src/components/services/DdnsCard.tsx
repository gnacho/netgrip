import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { AtSign, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { DDNSProbe, DDNSEntry } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Button, Card, Field, Modal, Pill,
  SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { TechName } from "./shared";

/** "actualizado hace 12 min"; si la fecha no parsea → undefined (mostrar "—"). */
function useRelativeUpdate(iso: string | undefined): { text: string; valid: boolean } {
  const { t } = useTranslation();
  if (!iso) return { text: "—", valid: false };
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return { text: "—", valid: false };
  const mins = Math.max(0, Math.floor((Date.now() - ms) / 60000));
  if (mins < 60) return { text: t("ddns.agoMinutes", { count: mins }), valid: true };
  const hours = Math.floor(mins / 60);
  if (hours < 48) return { text: t("ddns.agoHours", { count: hours }), valid: true };
  return { text: t("ddns.agoDays", { count: Math.floor(hours / 24) }), valid: true };
}

function EntryRow({ entry, wanIp, busySection, onToggle, onDelete, onForce, onEdit }: {
  entry: DDNSEntry;
  wanIp: string;
  busySection: string | null;
  onToggle: (domain: string, enabled: boolean) => void;
  onDelete: (section: string) => void;
  onForce: (section: string) => void;
  onEdit: (entry: DDNSEntry) => void;
}) {
  const { t } = useTranslation();
  const lastUpdate = useRelativeUpdate(entry.last_update);
  const busy = busySection === entry.section;
  const diverged = !!(entry.registered_ip && wanIp && entry.registered_ip !== wanIp);

  return (
    <div className="flex items-start gap-3 py-3 border-b border-border/60 last:border-0">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-medium text-foreground truncate">{entry.domain}</span>
          <Pill tone={entry.running ? "ok" : entry.enabled ? "warn" : "muted"}>
            {entry.running ? t("ddns.running") : entry.enabled ? t("ddns.configured") : t("ddns.off")}
          </Pill>
        </div>
        <div className="text-small text-muted mt-0.5 flex flex-wrap gap-x-3">
          <span>{entry.service_name}</span>
          {entry.registered_ip && <span>{t("ddns.registeredIp")}: <code>{entry.registered_ip}</code></span>}
          {wanIp && <span>{t("ddns.currentIp")}: <code>{wanIp}</code></span>}
          <span>{t("ddns.lastUpdate")}: {lastUpdate.valid ? lastUpdate.text : <span title={entry.last_update}>—</span>}</span>
        </div>
        {diverged && (
          <div className="mt-1.5 flex flex-col items-start gap-1">
            <Pill tone="warn">{t("ddns.diverged")}</Pill>
            <p className="text-small text-warn">{t("ddns.divergedHint")}</p>
          </div>
        )}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <Toggle
          checked={entry.enabled}
          busy={busy}
          onChange={(checked) => onToggle(entry.domain, checked)}
          label={t(entry.enabled ? "ddns.doneOn" : "ddns.doneOff")}
        />
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onEdit(entry)}
          disabled={busy}
          aria-label={`${t("ddns.editEntry")} ${entry.domain}`}
        >
          <Pencil size={16} />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onForce(entry.section)}
          disabled={busy}
          aria-label={t("ddns.force")}
        >
          <RefreshCw size={16} />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="text-danger hover:text-danger hover:bg-danger/10"
          onClick={() => onDelete(entry.section)}
          disabled={busy}
          aria-label={t("common.delete")}
        >
          <Trash2 size={16} />
        </Button>
      </div>
    </div>
  );
}

/**
 * DDNS — "Un nombre fijo para tu casa" (services.md §5). Ahora soporta
 * varias entradas: cada una se crea, activa/desactiva o borra por separado.
 */
export function DdnsCard({ probe, onChange, index = 0 }: {
  probe: DDNSProbe | undefined;
  onChange: (p: DDNSProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy: actionBusy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [busyDomain, setBusyDomain] = useState<string | null>(null);
  const [editing, setEditing] = useState<DDNSEntry>();
  const [eService, setEService] = useState("");
  const [eDomain, setEDomain] = useState("");
  const [eLookup, setELookup] = useState("");
  const [eUser, setEUser] = useState("");
  const [ePass, setEPass] = useState("");

  const [serviceName, setServiceName] = useState("");
  const [domain, setDomain] = useState("");
  const [lookupHost, setLookupHost] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const entries = useMemo(() => probe?.entries ?? [], [probe]);

  useEffect(() => {
    setServiceName("");
    setDomain("");
    setLookupHost("");
    setUsername("");
    setPassword("");
  }, [entries.length]);

  const handleResult = (res: { state: DDNSProbe; status: string; error?: string } | undefined, successMsg?: string) => {
    if (!res) return;
    onChange(res.state);
    if (res.status === "applied") {
      if (successMsg) setDoneMsg(successMsg);
    } else if (res.status === "rolled_back" || res.status === "failed") {
      setDoneMsg(res.error || t("action.failed"));
    }
  };

  const toggle = async (domain: string, enabled: boolean) => {
    setBusyDomain(domain);
    const res = await run(() => api.setDdns({ enabled, domain }));
    handleResult(res, enabled ? t("ddns.doneOn") : t("ddns.doneOff"));
    setBusyDomain(null);
  };

  const add = async () => {
    if (!serviceName.trim() || !domain.trim()) return;
    const res = await run(() => api.setDdns({
      enabled: true,
      service_name: serviceName.trim(),
      domain: domain.trim(),
      lookup_host: lookupHost.trim(),
      username: username.trim(),
      password: password.trim(),
    }));
    if (res?.status === "applied") {
      setPassword("");
    }
    handleResult(res, t("ddns.doneOn"));
  };

  const openEdit = (entry: DDNSEntry) => {
    setEditing(entry);
    setEService(entry.service_name);
    setEDomain(entry.domain);
    setELookup(entry.lookup_host);
    setEUser(entry.username);
    setEPass("");
  };

  const saveEdit = async () => {
    if (!editing || !eService.trim() || !eDomain.trim()) return;
    setBusyDomain(editing.section);
    const res = await run(() => api.updateDdns(editing.section, {
      // Editar no cambia el estado: se conserva el que tenía la entrada.
      enabled: editing.enabled,
      service_name: eService.trim(),
      domain: eDomain.trim(),
      lookup_host: eLookup.trim(),
      username: eUser.trim(),
      password: ePass.trim(),
    }));
    setBusyDomain(null);
    handleResult(res, t("ddns.editSaved"));
    if (res?.status === "applied") setEditing(undefined);
  };

  const remove = async (section: string) => {
    setBusyDomain(section);
    const res = await run(() => api.deleteDdns(section));
    handleResult(res);
    setBusyDomain(null);
  };

  const force = async (section: string) => {
    setBusyDomain(section);
    const res = await run(() => api.forceDdns(section));
    handleResult(res, t("ddns.forceDone"));
    setTimeout(async () => onChange(await api.ddns()), 6000);
    setBusyDomain(null);
  };

  return (
    <Card index={index} title={t("ddns.title")} icon={AtSign}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <p className="text-small text-muted -mt-1 mb-2">{t("ddns.desc")}</p>
          <TechName>DDNS</TechName>

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          {entries.length === 0 ? (
            <p className="text-small text-muted py-4">{t("ddns.empty")}</p>
          ) : (
            <div className="mt-2">
              {entries.map((entry) => (
                <EntryRow
                  key={entry.section}
                  entry={entry}
                  wanIp={probe.wan_ip}
                  busySection={busyDomain}
                  onToggle={toggle}
                  onDelete={remove}
                  onForce={force}
                  onEdit={openEdit}
                />
              ))}
            </div>
          )}

          <Modal
            open={!!editing}
            onClose={() => setEditing(undefined)}
            title={t("ddns.editTitle")}
            footer={
              <>
                <Button variant="ghost" onClick={() => setEditing(undefined)}>{t("common.cancel")}</Button>
                <Button onClick={saveEdit} loading={busyDomain === editing?.section}
                  disabled={!eService.trim() || !eDomain.trim()}>{t("common.save")}</Button>
              </>
            }
          >
            <div className="flex flex-col gap-3">
              <Field label={t("ddns.providerLabel")} mono
                inputProps={{ value: eService, onChange: (e) => setEService(e.target.value), placeholder: "duckdns.org" }} />
              <Field label={t("ddns.domainLabel")} mono
                inputProps={{ value: eDomain, onChange: (e) => setEDomain(e.target.value), placeholder: "casa.duckdns.org" }} />
              <Field label={t("ddns.lookupHostLabel")} mono
                inputProps={{ value: eLookup, onChange: (e) => setELookup(e.target.value) }} />
              <Field label={t("ddns.username")}
                inputProps={{ value: eUser, onChange: (e) => setEUser(e.target.value), autoComplete: "off" }} />
              <Field label={t("ddns.password")} hint={t("ddns.passwordKeep")}
                inputProps={{ type: "password", value: ePass, onChange: (e) => setEPass(e.target.value), autoComplete: "new-password" }} />
            </div>
          </Modal>

          <AdvancedDisclosure label={t("ddns.addEntry")} className="mt-2">
            <div className="flex flex-col gap-3">
              <Field label={t("ddns.providerLabel")} mono
                inputProps={{ value: serviceName, onChange: (e) => setServiceName(e.target.value), placeholder: "duckdns.org" }} />
              <Field label={t("ddns.domainLabel")} mono
                inputProps={{ value: domain, onChange: (e) => setDomain(e.target.value), placeholder: "casa.duckdns.org" }} />
              <Field label={t("ddns.lookupHostLabel")} mono
                inputProps={{ value: lookupHost, onChange: (e) => setLookupHost(e.target.value) }} />
              <Field label={t("ddns.username")}
                inputProps={{ value: username, onChange: (e) => setUsername(e.target.value), autoComplete: "off" }} />
              <Field label={t("ddns.password")}
                inputProps={{ type: "password", value: password, onChange: (e) => setPassword(e.target.value), autoComplete: "new-password" }} />
              <Button onClick={add} loading={actionBusy} disabled={!serviceName.trim() || !domain.trim()}>
                <Plus size={16} /> {t("ddns.addEntry")}
              </Button>
            </div>
          </AdvancedDisclosure>
        </>
      )}
    </Card>
  );
}
