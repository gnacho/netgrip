import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Info, Plus, Settings } from "lucide-react";
import { api } from "../api";
import type { GuestProbe, IoTProbe, WifiUI } from "../types";
import { Toggle } from "../components/Toggle";
import { WifiEditModal } from "../components/WifiEditModal";
import { IotWifiCard } from "../components/IotWifiCard";
import { GuestWifiCard } from "../components/GuestWifiCard";

export function WifiPage({ iot, onIotChange, guest, onGuestChange }: {
  iot: IoTProbe | undefined;
  onIotChange: (p: IoTProbe) => void;
  guest: GuestProbe | undefined;
  onGuestChange: (p: GuestProbe) => void;
}) {
  const { t } = useTranslation();
  const [ifaces, setIfaces] = useState<WifiUI[]>([]);
  const [editing, setEditing] = useState<WifiUI>();

  useEffect(() => {
    api.wifi().then((r) => setIfaces(r.interfaces)).catch(() => {});
  }, []);

  const main = ifaces.filter((i) => i.section.startsWith("default_radio"));

  return (
    <div className="flex flex-col gap-4">
      <SectionTitle title={t("wifi.sectionMain")} />

      <div className="grid gap-4 sm:grid-cols-2">
        {main.map((iface) => (
          <RadioCard key={iface.section} iface={iface} onEdit={() => setEditing(iface)} />
        ))}
        {main.length === 0 && <p className="text-sm text-muted">…</p>}
      </div>

      {guest?.gateway && (
        <>
          <SectionTitle title={t("wifi.sectionGuest")} />
          <AddBanner text={t("guest.banner")} onAdd={() => {}} />
          <GuestWifiCard probe={guest} onChange={onGuestChange} />
        </>
      )}

      <SectionTitle title={t("wifi.sectionIot")} />
      <AddBanner text={t("iot.banner")} onAdd={() => {}} />
      <IotWifiCard probe={iot} onChange={onIotChange} />

      {editing && (
        <WifiEditModal iface={editing} onClose={() => setEditing(undefined)} onSaved={(u) => {
          setIfaces((prev) => prev.map((p) => (p.section === u.section ? u : p)));
          setEditing(undefined);
        }} />
      )}
    </div>
  );
}

function RadioCard({ iface, onEdit }: { iface: WifiUI; onEdit: () => void }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [enabled, setEnabled] = useState(!iface.disabled);

  const toggleEnabled = async (v: boolean) => {
    setBusy(true);
    try {
      await api.setWifi({ section: iface.section, disabled: !v });
      setEnabled(v);
    } catch {
      setEnabled(!iface.disabled);
    } finally {
      setBusy(false);
    }
  };

  const bandLabel = iface.band === "5g" ? t("wifi.band5") : t("wifi.band24");

  return (
    <div className="bg-card border border-border rounded-xl p-4">
      <header className="flex items-center gap-2 mb-2">
        <span className={`w-2.5 h-2.5 rounded-full ${enabled ? "bg-accent" : "bg-border"}`} />
        <span className="text-sm font-medium flex-1">{bandLabel}</span>
        <button onClick={onEdit} className="text-muted hover:text-text p-1" title={t("wifi.editBand")}>
          <Settings size={16} />
        </button>
        <Toggle checked={enabled} busy={busy} onChange={toggleEnabled} />
      </header>

      <div className="flex items-center justify-between text-sm mt-2">
        <span className="text-muted">{iface.ssid}</span>
        <span className="font-mono text-xs text-border">{iface.has_key ? "••••••••••" : ""}</span>
      </div>

      <div className="flex flex-wrap gap-1.5 mt-2">
        <Chip>{t("wifi.chipMode")}</Chip>
        <Chip className="border-accent/40 text-accent">{iface.encryption}</Chip>
        <Chip>{iface.hidden ? t("wifi.hiddenLabel") : t("wifi.visible")}</Chip>
        <Chip>{t("wifi.clients", { count: iface.clients.length })}</Chip>
      </div>

      <div className="font-mono text-[10px] text-muted mt-2">BSSID {iface.bssid}</div>
    </div>
  );
}

function SectionTitle({ title }: { title: string }) {
  return <h2 className="text-sm font-semibold text-muted uppercase tracking-wider mt-2">{title}</h2>;
}

function AddBanner({ text, onAdd }: { text: string; onAdd: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="bg-muted/10 border border-border rounded-xl p-3 flex items-center gap-2">
      <Info size={16} className="text-accent shrink-0" />
      <p className="text-xs text-muted flex-1">{text}</p>
      <button onClick={onAdd} className="bg-accent hover:bg-accent/85 text-white text-xs rounded-full px-3 py-1.5 flex items-center gap-1">
        <Plus size={14} /> {t("guest.add")}
      </button>
    </div>
  );
}

function Chip({ children, className }: { children: React.ReactNode; className?: string }) {
  return <span className={`text-[10px] px-1.5 py-0.5 rounded bg-border/40 text-muted ${className ?? ""}`}>{children}</span>;
}
