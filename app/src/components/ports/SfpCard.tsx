import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Waves } from "lucide-react";
import { api } from "../../api";
import type { PhysPortsProbe } from "../../types";
import { Card, Pill, SkeletonRows } from "../ui";

/**
 * "Ópticos": resumen de jaulas SFP del switch (#485). Vive en Switch
 * avanzado; cada fila es una jaula con módulo, RX power y temperatura.
 */
export function SfpCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<PhysPortsProbe>();
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setError(false);
    try { setProbe(await api.physPorts()); }
    catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error || (probe && !probe.applicable)) return null;
  const cages = probe?.ports.filter((p) => p.sfp) ?? [];
  if (probe && cages.length === 0) return null;

  return (
    <Card index={index} title={t("phys.sfpCardTitle")} icon={Waves}>
      {!probe ? <SkeletonRows rows={2} /> : (
        <div className="overflow-x-auto">
          <table className="w-full text-small">
            <thead>
              <tr className="text-left text-caption text-muted border-b border-border">
                <th className="py-1.5 pr-2 font-medium">{t("stp.colPort")}</th>
                <th className="py-1.5 pr-2 font-medium">{t("phys.sfpPresent")}</th>
                <th className="py-1.5 pr-2 font-medium">RX</th>
                <th className="py-1.5 font-medium">{t("phys.sfpTemp")}</th>
              </tr>
            </thead>
            <tbody>
              {cages.map((p) => (
                <tr key={p.name} className="border-b border-border/40 last:border-0">
                  <td className="py-1.5 pr-2 font-mono">{p.name}</td>
                  <td className="py-1.5 pr-2">
                    {p.sfp?.state === "module" ? (
                      <span className="text-caption">{[p.sfp.vendor, p.sfp.pn].filter(Boolean).join(" ")}</span>
                    ) : (
                      <Pill tone="muted">{t(p.sfp?.state === "empty" ? "phys.sfpEmpty" : "phys.sfpError")}</Pill>
                    )}
                  </td>
                  <td className="py-1.5 pr-2 font-mono">
                    {p.sfp?.rx_power_dbm !== undefined ? `${p.sfp.rx_power_dbm.toFixed(2)} dBm` : "-"}
                  </td>
                  <td className="py-1.5 font-mono">
                    {p.sfp?.temp_c !== undefined ? `${p.sfp.temp_c.toFixed(1)} ºC` : "-"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}
