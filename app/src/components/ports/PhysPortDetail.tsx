import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../../api";
import type { PhysPort, SFPInfo } from "../../types";
import { Button, Input, Toggle } from "../ui";

/**
 * "Configuración física" de la banda de detalle de boca (#485): negociación,
 * velocidad/duplex (solo modos que el puerto soporta), MTU y EEE. Solo se
 * monta con modo avanzado; si el chip no soporta MTU > 1500 se muestra un
 * estado limpio en vez del error crudo del driver.
 */
export function PhysConfigSection({ phys, busy, onChanged }: {
  phys: PhysPort;
  busy: boolean;
  onChanged: () => void;
}) {
  const { t } = useTranslation();
  const [draftMtu, setDraftMtu] = useState<string>();
  const [failMsg, setFailMsg] = useState<string>();

  const apply = async (edit: Omit<Parameters<typeof api.setPhysPort>[0], "name">) => {
    setFailMsg(undefined);
    try {
      await api.setPhysPort({ name: phys.name, ...edit });
      onChanged();
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    }
  };

  const modes = phys.supported;
  const currentMode = `${phys.speed_mbps}/${phys.duplex}`;
  const mtuDirty = draftMtu !== undefined && parseInt(draftMtu, 10) !== phys.mtu;

  return (
    <div className="mt-3 border-t border-border/60 pt-3">
      <p className="text-caption font-medium text-muted uppercase tracking-wide mb-2">
        {t("phys.sectionTitle")}
      </p>
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <span className="flex items-center gap-1.5">
          <span className="text-caption text-muted">{t("phys.autoneg")}</span>
          <Toggle
            checked={phys.autoneg}
            busy={busy}
            onChange={(v) => void apply({ autoneg: v })}
            label={`${t("phys.autoneg")} ${phys.name}`}
          />
        </span>
        {!phys.autoneg && modes.length > 0 && (
          <label className="flex items-center gap-1.5">
            <span className="text-caption text-muted">{t("phys.mode")}</span>
            <select
              value={currentMode}
              onChange={(e) => {
                const [speed, duplex] = e.target.value.split("/");
                void apply({ autoneg: false, speed_mbps: parseInt(speed, 10), duplex });
              }}
              aria-label={`${t("phys.mode")} ${phys.name}`}
              className="h-7 rounded-sm border border-border bg-surface px-1.5 text-caption"
            >
              {modes.map((m) => (
                <option key={`${m.speed_mbps}/${m.duplex}`} value={`${m.speed_mbps}/${m.duplex}`}>
                  {m.speed_mbps >= 1000 ? `${m.speed_mbps / 1000} Gb/s` : `${m.speed_mbps} Mb/s`} {m.duplex}
                </option>
              ))}
            </select>
          </label>
        )}
        <span className="flex items-center gap-1.5">
          <span className="text-caption text-muted">MTU</span>
          {phys.mtu_supported ? (
            <>
              <Input
                type="number"
                value={draftMtu ?? String(phys.mtu)}
                onChange={(e) => setDraftMtu(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && mtuDirty && void apply({ mtu: parseInt(draftMtu ?? "", 10) })}
                aria-label={`MTU ${phys.name}`}
                className="!h-7 w-20 text-caption"
              />
              <Button
                size="sm"
                disabled={!mtuDirty}
                loading={busy}
                onClick={() => void apply({ mtu: parseInt(draftMtu ?? "", 10) })}
              >
                {t("stp.apply")}
              </Button>
              <span className="text-caption text-faint">{t("phys.mtuMax", { max: phys.mtu_max })}</span>
            </>
          ) : (
            <span className="text-caption text-faint">{t("phys.mtuUnsupported")}</span>
          )}
        </span>
        {phys.eee_supported && (
          <span className="flex items-center gap-1.5">
            <span className="text-caption text-muted">{t("phys.eee")}</span>
            <Toggle
              checked={phys.eee_enabled}
              busy={busy}
              onChange={(v) => void apply({ eee: v })}
              label={`${t("phys.eee")} ${phys.name}`}
            />
          </span>
        )}
      </div>
      {failMsg && <p className="text-caption text-danger mt-2">{failMsg}</p>}
    </div>
  );
}

/** "Módulo óptico": bloque SFP dentro de la banda de detalle de una jaula. */
export function SfpModuleSection({ sfp }: { sfp: SFPInfo }) {
  const { t } = useTranslation();
  if (sfp.state === "empty") return <p className="text-caption text-faint mt-2">{t("phys.sfpEmpty")}</p>;
  if (sfp.state === "error") return <p className="text-caption text-danger mt-2">{t("phys.sfpError")}</p>;
  if (sfp.state !== "module") return null;
  return (
    <div className="mt-2 border-t border-border/60 pt-2">
      <p className="text-caption font-medium text-muted uppercase tracking-wide mb-1.5">
        {t("phys.sfpTitle")}
      </p>
      <div className="flex flex-wrap gap-x-5 gap-y-1 text-small">
        {sfp.rx_power_dbm !== undefined && (
          <span>RX <span className="font-mono">{sfp.rx_power_dbm.toFixed(2)} dBm</span></span>
        )}
        {sfp.tx_power_dbm !== undefined && (
          <span>TX <span className="font-mono">{sfp.tx_power_dbm.toFixed(2)} dBm</span></span>
        )}
        {sfp.temp_c !== undefined && (
          <span><span className="font-mono">{sfp.temp_c.toFixed(1)} ºC</span></span>
        )}
      </div>
      {(sfp.vendor || sfp.sn) && (
        <p className="text-caption text-faint mt-1 font-mono break-all">
          {[sfp.vendor, sfp.pn, sfp.sn, sfp.date].filter(Boolean).join(" · ")}
        </p>
      )}
    </div>
  );
}
