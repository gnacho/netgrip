import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff } from "lucide-react";
import { api } from "../api";
import type { LANConfig } from "../types";
import { Button, Card, EmptyState, SkeletonRows } from "../components/ui";
import { LanConfigCard } from "../components/lan/LanConfigCard";
import { ReservationsCard } from "../components/lan/ReservationsCard";
import { DnsCard } from "../components/lan/DnsCard";

/**
 * Red local (lan.md): "las direcciones de tu casa", DNS y "direcciones que
 * no cambian". Lo avanzado (VLANs, perfiles, modos de switch) vive en la
 * página Puertos, donde están esas features en el código real.
 */
export function LanPage() {
  const { t } = useTranslation();
  const [cfg, setCfg] = useState<LANConfig>();
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setError(false);
    try {
      setCfg(await api.lan());
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  if (cfg && !cfg.applicable) return null;

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {error ? (
        <Card index={0}>
          <EmptyState
            small
            illustration={<CloudOff size={24} />}
            title={t("common.loadError")}
            action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}
          />
        </Card>
      ) : !cfg ? (
        <>
          <Card index={0}><SkeletonRows rows={4} /></Card>
          <Card index={1}><SkeletonRows rows={4} /></Card>
          <Card index={2}><SkeletonRows rows={3} /></Card>
        </>
      ) : (
        <>
          <LanConfigCard cfg={cfg} onChange={setCfg} index={0} />
          <DnsCard index={1} />
          <ReservationsCard cfg={cfg} onChange={setCfg} index={2} />
        </>
      )}
    </div>
  );
}
