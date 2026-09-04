import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Pin, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { Client, LANConfig } from "../../types";
import {
  ActionBanner, Button, Card, ConfirmDialog, EmptyState, Field, Pill,
} from "../ui";
import { IlluDevices } from "../ui/illustrations";
import { useActionCycle } from "../wifi/action";
import { isValidIp } from "./LanConfigCard";

const MAC_RE = /^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$/;

/** Card "Direcciones que no cambian" (lan.md §4): reservas DHCP. */
export function ReservationsCard({ cfg, onChange, index = 1 }: {
  cfg: LANConfig;
  onChange: (c: LANConfig) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [rName, setRName] = useState("");
  const [rMac, setRMac] = useState("");
  const [rIp, setRIp] = useState("");
  const [confirmClear, setConfirmClear] = useState(false);
  const [clients, setClients] = useState<Client[]>([]);

  useEffect(() => {
    api.clients().then((r) => setClients(r.clients)).catch(() => {});
  }, []);

  const reservations = cfg.reservations;

  // Primera IP libre del rango DHCP (start..start+limit-1) no reservada.
  const defaultIp = () => {
    const start = cfg.dhcp.start || 100;
    const limit = cfg.dhcp.limit || 150;
    const used = new Set(reservations.map((r) => r.ip));
    for (let i = 0; i < limit; i++) {
      const ip = `${cfg.ipaddr.split(".").slice(0, 3).join(".")}.${start + i}`;
      if (!used.has(ip)) return ip;
    }
    return "";
  };

  const fillFromMac = (mac: string) => {
    const c = clients.find((x) => x.mac?.toLowerCase() === mac.trim().toLowerCase());
    if (c) {
      if (c.name) setRName(c.name);
      if (c.ip && isValidIp(c.ip)) setRIp(c.ip);
    }
  };

  const formError = useMemo(() => {
    if (!rMac && !rIp) return undefined;
    if (!MAC_RE.test(rMac.trim())) return t("lan.invalidMac");
    if (!isValidIp(rIp)) return t("lan.invalidIp");
    return undefined;
  }, [rMac, rIp, t]);

  const apply = (fn: () => Promise<Awaited<ReturnType<typeof api.setReservation>>>) => {
    run(fn).then((res) => {
      if (res?.status === "applied") onChange(res.state);
    });
  };

  const add = () => {
    apply(() => api.setReservation(rMac.trim(), rIp.trim(), rName.trim(), true));
    setRName(""); setRMac(""); setRIp("");
  };

  return (
    <Card
      index={index}
      icon={Pin}
      iconTone="muted"
      title={t("lan.fixedTitle")}
      help="reservation"
      action={
        <span className="flex items-center gap-2">
          {reservations.length > 0 && <Pill tone="accent">{t("lan.fixedCount", { count: reservations.length })}</Pill>}
          {reservations.length > 0 && (
            <Button variant="ghost" size="sm" icon={Trash2} onClick={() => setConfirmClear(true)}>
              {t("lan.clearAll")}
            </Button>
          )}
        </span>
      }
    >
      {reservations.length === 0 ? (
        <EmptyState
          small
          illustration={<IlluDevices size={140} />}
          title={t("lan.resEmptyTitle")}
          body={t("lan.resEmptyBody")}
        />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="text-caption text-muted border-b border-border">
                <th className="text-left font-medium py-1.5 pr-2">{t("lan.name")}</th>
                <th className="text-left font-medium py-1.5 pr-2">{t("lan.mac")}</th>
                <th className="text-left font-medium py-1.5 pr-2">IP</th>
                <th className="w-8" aria-label={t("lan.action")} />
              </tr>
            </thead>
            <tbody>
              {reservations.map((r) => (
                <tr key={r.mac} className="border-b border-border/50 last:border-0 animate-fade-up">
                  <td title={r.name || undefined} className="py-2 pr-2 text-body font-medium max-w-40 truncate">{r.name || "—"}</td>
                  <td title={r.mac} className="py-2 pr-2 font-mono text-small text-muted max-w-44 truncate">{r.mac}</td>
                  <td className="py-2 pr-2 font-mono text-small whitespace-nowrap">{r.ip}</td>
                  <td className="py-2 text-right">
                    <button
                      type="button"
                      onClick={() => apply(() => api.setReservation(r.mac, r.ip, r.name ?? "", false))}
                      disabled={busy}
                      aria-label={`${t("fwd.delete")} ${r.name || r.mac}`}
                      className="text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1"
                    >
                      <Trash2 size={16} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Añadir: fila-formulario inline */}
      <div className="mt-4 border-t border-border/60 pt-4">
        <div className="grid gap-2 sm:grid-cols-[1fr_1fr_1fr_auto] items-end">
          <Field label={t("lan.name")}
            inputProps={{ value: rName, onChange: (e) => setRName(e.target.value), placeholder: t("lan.namePlaceholder"), list: "resv-clients-name" }} />
          <Field label={t("lan.mac")} mono error={rMac && !MAC_RE.test(rMac.trim()) ? t("lan.invalidMac") : undefined}
            inputProps={{ value: rMac, onChange: (e) => { setRMac(e.target.value); if (!rIp) setRIp(defaultIp()); }, onBlur: () => fillFromMac(rMac), placeholder: "00:11:22:33:44:55", list: "resv-clients" }} />
          <Field label="IP" mono error={rIp && !isValidIp(rIp) ? t("lan.invalidIp") : undefined}
            inputProps={{ value: rIp, onFocus: () => { if (!rIp) setRIp(defaultIp()); }, onChange: (e) => setRIp(e.target.value), placeholder: "192.168.8.10" }} />
          <datalist id="resv-clients">
            {clients.map((c) => (c.mac ? <option key={c.mac} value={c.mac} label={c.name || c.ip || undefined}>{`${c.name || c.ip || ""} · ${c.mac}`}</option> : null))}
          </datalist>
          <datalist id="resv-clients-name">
            {clients.map((c) => (c.name ? <option key={c.mac} value={c.name} label={c.ip || undefined}>{`${c.name} · ${c.ip || ""}`}</option> : null))}
          </datalist>
          <Button size="sm" onClick={add} disabled={busy || !rMac || !rIp || !!formError} loading={busy}>
            {t("lan.pin")}
          </Button>
        </div>
        <p className="text-caption text-muted mt-2">{t("lan.resTip")}</p>
      </div>

      {phase && (
        <div className="mt-3">
          <ActionBanner
            phase={phase}
            text={phase === "done" ? t("lan.saved") : phase === "failed" ? t("lan.rolledBack") : undefined}
            detail={detail}
            onDone={clear}
          />
        </div>
      )}

      <ConfirmDialog
        open={confirmClear}
        onClose={() => setConfirmClear(false)}
        onConfirm={() => { setConfirmClear(false); apply(() => api.clearReservations()); }}
        title={t("lan.clearTitle")}
        consequence={t("lan.clearConsequence")}
        confirmLabel={t("lan.clearConfirm")}
        busy={busy}
      />
    </Card>
  );
}
