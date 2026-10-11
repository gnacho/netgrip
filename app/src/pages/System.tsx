import type { ReactNode } from "react";
import type { Board, UpdateCheck } from "../types";
import { useMode } from "../hooks/useMode";
import { SecurityCard } from "../components/system/SecurityCard";
import { RemoteAccessCard } from "../components/system/RemoteAccessCard";
import { AccessCard } from "../components/system/AccessCard";
import { ModeCard } from "../components/system/ModeCard";
import { IdentityCard } from "../components/system/IdentityCard";
import { IntegrationsCard } from "../components/system/IntegrationsCard";
import { NetPulseCard } from "../components/system/NetPulseCard";
import { ConfigBackupCard } from "../components/ConfigBackupCard";
import { UpdateCard } from "../components/system/UpdateCard";
import { SelfUpdateCard } from "../components/system/SelfUpdateCard";
import { WizardRelaunchCard } from "../components/system/WizardRelaunchCard";
import { AdvancedCard } from "../components/system/AdvancedCard";
import { OptionsCard } from "../components/system/OptionsCard";
import { NetPulseStandaloneBanner, NetPulseStatusChip } from "../components/system/NetPulseStatus";
import { useTranslation } from "react-i18next";

// Hidden by design (#146): zero-touch NetPulse integration. The embedded
// agent is always on and self-enrolls, so the manual configuration card has
// no place in the UI for now. Code (component, i18n keys, API) is kept
// compiled; flip this flag to bring the card back.
const NETPULSE_CARD_HIDDEN = true;

function GroupLabel({ children }: { children: ReactNode }) {
  return <p className="text-eyebrow text-faint mb-2 animate-fade-up">{children}</p>;
}

export function System({ board, update, onUpdateChange, onLogout }: {
  board: Board | undefined;
  update: UpdateCheck | undefined;
  onUpdateChange: (u: UpdateCheck) => void;
  onLogout: () => void;
}) {
  const { t } = useTranslation();
  // (#484) Solo el gateway gestiona acceso remoto, modo de operación y el
  // asistente inicial. Las tres tarjetas son gateway-only: no se pintan
  // hasta conocer el rol (useMode hidrata desde caché, así que en recargas
  // la decisión es inmediata y en la primera visita esperan al probe en
  // vez de quedarse visibles por defecto).
  const { mode, modeReady } = useMode();
  const role = mode?.role ?? (mode?.mode === "ap" ? "ap" : "router");
  const isRouter = modeReady && role === "router";
  // Un AP puede volver a ser router (y viceversa): la tarjeta de modo se
  // ofrece en ambos. Un switch no tiene modo que conmutar (#487).
  const canSwitchMode = modeReady && role !== "switch";
  return (
    <div className="flex flex-col gap-6">
      {/* Actualizaciones (#499): el propio NetGrip y la del sistema, juntas. */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupLabel>{t("system.groupUpdates")}</GroupLabel>
        <SelfUpdateCard index={0} />
        <UpdateCard
          board={board}
          update={update}
          onChange={onUpdateChange}
        />
      </section>

      {/* Protección y acceso (#499): contraseña y accesos. */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupLabel>{t("system.groupProtection")}</GroupLabel>
        <SecurityCard index={0} onLogout={onLogout} />
        <AccessCard index={1} />
        {isRouter && <RemoteAccessCard index={2} />}
      </section>

      {/* Este equipo: modo, identidad, avisos, copia de config y
          preferencias. #499/#500: Telegram/ntfy/MQTT colapsados en una sola
          tarjeta; SNMP sale de aquí (vive en Servicios). Tarjetas SIEMPRE al
          100% del ancho: a medias dejan huecos imposibles de casar (#499). */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupLabel>{t("system.groupDevice")}</GroupLabel>
        <NetPulseStandaloneBanner />
        <NetPulseStatusChip />
        {canSwitchMode && <ModeCard index={0} />}
        <IdentityCard index={1} board={board} />
        {!NETPULSE_CARD_HIDDEN && <NetPulseCard index={2} />}
        <IntegrationsCard index={2} />
        <ConfigBackupCard index={3} />
        <OptionsCard index={4} />
        <AdvancedCard index={5} />
        {isRouter && <WizardRelaunchCard index={6} />}
      </section>
    </div>
  );
}
