import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff } from "lucide-react";
import { api } from "../api";
import type { IPv6Probe, LANConfig } from "../types";
import { Button, Card, EmptyState, SkeletonRows } from "../components/ui";
import { LanConfigCard } from "../components/lan/LanConfigCard";
import { ReservationsCard } from "../components/lan/ReservationsCard";
import { DnsCard } from "../components/lan/DnsCard";
import { Ipv6Card } from "../components/services/Ipv6Card";

/**
 * Red local (lan.md): "las direcciones de tu casa", DNS y "direcciones que
 * no cambian". Lo avanzado (VLANs, perfiles, modos de switch) vive en la
 * página Puertos, donde están esas features en el código real.
 */
export function LanPage({ ipv6, onIpv6Change }: {
  ipv6: IPv6Probe | undefined;
  onIpv6Change: (p: IPv6Probe) => void;
}) {
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
          <Ipv6Card probe={ipv6} onChange={onIpv6Change} index={3} />
        </>
      )}
    </div>
  );
}
