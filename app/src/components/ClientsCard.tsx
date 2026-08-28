import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable, Monitor, Users, Wifi, Zap } from "lucide-react";
import { api } from "../api";
import type { Client } from "../types";
import { Card, Pill } from "./Card";
import { Toggle } from "./Toggle";

function fmtBytes(bytes: number): string {
  if (bytes > 1073741824) return `${(bytes / 1073741824).toFixed(2)} GB`;
  if (bytes > 1048576) return `${(bytes / 1048576).toFixed(2)} MB`;
  if (bytes > 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${Math.round(bytes)} B`;
}

function fmtRate(bytesPerSec: number): string {
  if (bytesPerSec > 1048576) return `${(bytesPerSec / 1048576).toFixed(1)} MB/s`;
  if (bytesPerSec > 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`;
  return `${Math.round(bytesPerSec)} B/s`;
}

type Rates = Record<string, { rx: number; tx: number }>;

export function ClientsCard() {
  const { t } = useTranslation();
  const [clients, setClients] = useState<Client[]>([]);
  const [rates, setRates] = useState<Rates>({});
  const [busyMac, setBusyMac] = useState<string>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const prev = useRef<{ clients: Client[]; ts: number } | undefined>(undefined);

  const load = async () => {
    try {
      const next = await api.clients();
      const before = prev.current;
      if (before) {
        const dt = (next.ts - before.ts) / 1000;
        const nextRates: Rates = {};
        for (const c of next.clients) {
          const old = before.clients.find((o) => o.mac === c.mac);
          if (old && dt > 0 && (c.rx_bytes || c.tx_bytes)) {
            nextRates[c.mac] = {
              rx: Math.max(0, (c.rx_bytes - old.rx_bytes) / dt),
              tx: Math.max(0, (c.tx_bytes - old.tx_bytes) / dt),
            };
          }
        }
        setRates(nextRates);
      }
      prev.current = next;
      setClients(next.clients);
    } catch { /* keep previous */ }
  };

  useEffect(() => {
    load();
    const id = setInterval(load, 3000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggleReserve = async (c: Client, reserved: boolean) => {
    setBusyMac(c.mac);
    setMsg(undefined);
    try {
      await api.reserveClient(c.mac, c.ip ?? "", reserved);
      await load();
      setMsg({ tone: "ok", text: t("clients.applied") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("clients.failed") });
    } finally {
      setBusyMac(undefined);
    }
  };

  const toggleBlock = async (c: Client, blocked: boolean) => {
    setBusyMac(c.mac);
    setMsg(undefined);
    try {
      await api.blockClient(c.mac, c.type, blocked);
      await load();
      setMsg({ tone: "ok", text: blocked ? t("clients.blockedMsg", { name: c.name }) : t("clients.applied") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("clients.failed") });
    } finally {
      setBusyMac(undefined);
    }
  };

  return (
    <Card title={t("clients.title")} icon={Users} action={
      <Pill tone="muted">{t("clients.online", { count: clients.length })}</Pill>
    }>
      {clients.length === 0 ? (
        <p className="text-sm text-muted">{t("clients.empty")}</p>
      ) : (
        <div className="overflow-x-auto -mx-1">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-muted border-b border-border">
                <th className="text-left font-medium py-1.5 px-1">{t("clients.name")}</th>
                <th className="text-left font-medium py-1.5 px-1">{t("clients.ipMac")}</th>
                <th className="text-left font-medium py-1.5 px-1">{t("clients.speed")}</th>
                <th className="text-left font-medium py-1.5 px-1">{t("clients.traffic")}</th>
                <th className="text-center font-medium py-1.5 px-1">{t("clients.reserved")}</th>
                <th className="text-center font-medium py-1.5 px-1">{t("clients.block")}</th>
                <th className="text-center font-medium py-1.5 px-1">{t("clients.wol")}</th>
              </tr>
            </thead>
            <tbody>
              {clients.map((c) => (
                <tr key={c.mac} className="border-b border-border/40 last:border-0">
                  <td className="py-1.5 px-1">
                    <div className="flex items-center gap-1.5">
                      {c.type === "cable"
                        ? <Monitor size={14} className="text-muted shrink-0" />
                        : <Wifi size={14} className="text-muted shrink-0" />}
                      <span className="truncate max-w-32">{c.name}</span>
                      {c.self && <Pill tone="muted">{t("clients.self")}</Pill>}
                      {c.blocked && <Pill tone="danger">{t("clients.blocked")}</Pill>}
                    </div>
                  </td>
                  <td className="py-1.5 px-1">
                    <div className="flex items-center gap-1.5">
                      {c.type !== "cable" && (
                        <span className="text-[9px] px-1 rounded bg-accent/20 text-accent font-mono">
                          {c.type === "wifi5" ? "5G" : "2.4G"}
                        </span>
                      )}
                      {c.type === "cable" && <Cable size={12} className="text-muted" />}
                      <div>
                        <div className="font-mono text-xs">{c.ip || "—"}</div>
                        <div className="font-mono text-[10px] text-muted">{c.mac}</div>
                      </div>
                    </div>
                  </td>
                  <td className="py-1.5 px-1 text-xs whitespace-nowrap">
                    {rates[c.mac] ? (
                      <>
                        <div><span className="text-ok">↓</span> {fmtRate(rates[c.mac].tx)}</div>
                        <div><span className="text-accent">↑</span> {fmtRate(rates[c.mac].rx)}</div>
                      </>
                    ) : "—"}
                  </td>
                  <td className="py-1.5 px-1 text-xs whitespace-nowrap">
                    {c.tx_bytes || c.rx_bytes ? (
                      <>
                        <div><span className="text-ok">↓</span> {fmtBytes(c.tx_bytes)}</div>
                        <div><span className="text-accent">↑</span> {fmtBytes(c.rx_bytes)}</div>
                      </>
                    ) : "—"}
                  </td>
                  <td className="py-1.5 px-1 text-center">
                    <div className="flex justify-center" title={c.reservable ? "" : t("clients.gatewayOnly")}>
                      <Toggle
                        checked={c.reserved}
                        busy={busyMac === c.mac}
                        disabled={!c.reservable}
                        onChange={(v) => toggleReserve(c, v)}
                      />
                    </div>
                  </td>
                  <td className="py-1.5 px-1 text-center">
                    <div className="flex justify-center" title={c.blockable ? "" : t("clients.gatewayOnly")}>
                      <Toggle
                        checked={c.blocked}
                        busy={busyMac === c.mac}
                        disabled={!c.blockable}
                        onChange={(v) => toggleBlock(c, v)}
                      />
                    </div>
                  </td>
                  <td className="py-1.5 px-1 text-center">
                    {c.type === "cable" && (
                      <button
                        onClick={() => {
                          setBusyMac(c.mac);
                          api.wakeOnLan(c.mac)
                            .then(() => setMsg({ tone: "ok", text: t("clients.wolSent", { name: c.name }) }))
                            .catch((e: any) => setMsg({ tone: "danger", text: e.message }))
                            .finally(() => setBusyMac(undefined));
                        }}
                        disabled={busyMac === c.mac}
                        className="text-muted hover:text-accent p-1 disabled:opacity-40"
                        title={t("clients.wolHint")}
                      >
                        <Zap size={12} />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
