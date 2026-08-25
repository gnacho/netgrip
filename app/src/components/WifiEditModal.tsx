import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Eye, EyeOff, X } from "lucide-react";
import QRCode from "qrcode";
import { api } from "../api";
import type { WifiUI } from "../types";

export function WifiEditModal({ iface, onClose, onSaved }: {
  iface: WifiUI;
  onClose: () => void;
  onSaved: (networkIf: WifiUI) => void;
}) {
  const { t } = useTranslation();
  const [ssid, setSsid] = useState(iface.ssid);
  const [key, setKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [encryption, setEncryption] = useState(iface.encryption || "psk2");
  const [hidden, setHidden] = useState(iface.hidden);
  const [mac, setMac] = useState(iface.mac);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string } | undefined>();
  const [qr, setQr] = useState<string>();

  // The PSK is write-only (never returned by the backend). We can only build
  // a join QR from a key the user types now; we never show a QR for a stored
  // key we cannot read back.
  useEffect(() => {
    (async () => {
      if (!ssid.trim() || key.length < 8) { setQr(undefined); return; }
      try {
        const uri = `WIFI:T:WPA;S:${ssid};P:${key};;`;
        setQr(await QRCode.toDataURL(uri, { width: 180, margin: 1, errorCorrectionLevel: "M" }));
      } catch {
        setQr(undefined);
      }
    })();
  }, [ssid, key, encryption]);

  useEffect(() => {
    setSsid(iface.ssid); setKey(""); setEncryption(iface.encryption || "psk2");
    setHidden(iface.hidden); setMac(iface.mac);
  }, [iface]);

  const save = async () => {
    setBusy(true); setMsg(undefined);
    try {
      const edit: { section: string; ssid: string; encryption: string; hidden: boolean; key?: string; mac?: string } = {
        section: iface.section, ssid, encryption, hidden,
      };
      if (key) edit.key = key;
      if (mac) edit.mac = mac;
      const res = await api.setWifi(edit);
      onSaved(res.state);
      setMsg({ tone: "ok", text: t("access.saved") });
    } catch (err) {
      setMsg({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(false);
    }
  };

  const bandLabel = iface.band === "5g" ? t("wifi.band5") : t("wifi.band24");

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" onClick={onClose}>
      <div className="bg-card border border-border rounded-xl w-full max-w-md p-4 gap-3 flex flex-col" onClick={(e) => e.stopPropagation()}>
        <header className="flex items-center gap-2">
          <div className="flex items-center gap-2 flex-1">
            <span className="w-2.5 h-2.5 rounded-full bg-accent" />
            <h2 className="text-sm font-medium">{bandLabel}</h2>
          </div>
          <button onClick={onClose} className="text-muted hover:text-text p-1"><X size={16} /></button>
        </header>

        <Row label={t("wifi.editBand")} value={
          <span className="text-sm text-muted">{bandLabel}</span>
        } />

        <Row label={t("wifi.ssid")} value={
          <input value={ssid} onChange={(e) => setSsid(e.target.value)}
            className="bg-bg border border-border rounded-lg px-2 py-1 text-sm text-right outline-none focus:border-accent w-48" />
        } />

        <Row label={t("wifi.encryption")} value={
          <select value={encryption} onChange={(e) => setEncryption(e.target.value)}
            className="bg-bg border border-border rounded-lg px-2 py-1 text-sm outline-none focus:border-accent">
            <option value="psk2">WPA2-PSK</option>
            <option value="psk-mixed">WPA/WPA2 Mixed</option>
            <option value="sae">SAE (WPA3)</option>
            <option value="sae-mixed">WPA2/WPA3</option>
            <option value="none">Open</option>
          </select>
        } />

        <Row label={t("wifi.key")} value={
          <div className="relative">
            <input type={showKey ? "text" : "password"} value={key}
              onChange={(e) => setKey(e.target.value)} placeholder={iface.has_key ? "••••••••" : ""}
              autoComplete="new-password"
              className="bg-bg border border-border rounded-lg px-2 py-1 text-sm text-right outline-none focus:border-accent w-48 pr-7" />
            <button type="button" onClick={() => setShowKey(!showKey)}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted hover:text-text">
              {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
          </div>
        } />

        <Row label={t("wifi.visibility")} value={
          <select value={hidden ? "hidden" : "shown"} onChange={(e) => setHidden(e.target.value === "hidden")}
            className="bg-bg border border-border rounded-lg px-2 py-1 text-sm outline-none focus:border-accent">
            <option value="shown">{t("wifi.visible")}</option>
            <option value="hidden">{t("wifi.hiddenLabel")}</option>
          </select>
        } />

        <Row label={t("wifi.bssid")} value={
          <span className="font-mono text-xs text-muted">{iface.bssid}</span>
        } />

        {qr && (
          <div className="flex flex-col items-center gap-1 py-2">
            <img src={qr} alt="QR" className="w-40 h-40 rounded-lg border border-border" />
            <p className="text-[10px] text-muted">{t("wifi.qrNote")}</p>
          </div>
        )}

        {msg && <p className={`text-xs ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}

        <div className="flex justify-end gap-2 mt-1">
          <button onClick={onClose} className="text-sm text-muted hover:text-text px-3 py-1.5 rounded-lg border border-border">
            {t("wifi.cancel")}
          </button>
          <button onClick={save} disabled={busy || !ssid.trim()}
            className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-4 py-1.5 font-medium">
            {busy ? "…" : t("access.save")}
          </button>
        </div>
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1 border-b border-border/40 last:border-0">
      <span className="text-xs text-muted shrink-0">{label}</span>
      <div className="shrink-0">{value}</div>
    </div>
  );
}
