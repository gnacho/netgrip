import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Network, Search } from "lucide-react";
import { api } from "../api";
import type { L2Client, L2Probe } from "../types";
import { Card, EmptyState, Input, Pill, Skeleton } from "../components/ui";
import { IlluDevices } from "../components/ui/illustrations";

/**
 * Inventario de clientes de nivel 2 (#520): en un switch, la vista "Clients"
 * no se alimenta de leases/ARP sino de la FDB del bridge (MAC + puerto +
 * VLAN). Sin semántica DHCP: aquí no hay reservas, bloqueos, cuotas ni
 * parental. Se muestra cuando el rol es switch; el router/AP sigue usando
 * ClientsPage.
 */

/** Edad legible desde segundos (0 = desconocida). */
function fmtAge(age: number | undefined, t: (k: string, o?: Record<string, string | number>) => string): string {
  if (!age) return t("l2clients.ageUnknown");
  if (age < 60) return t("l2clients.ageSeconds", { seconds: age });
  return t("l2clients.ageMinutes", { minutes: Math.floor(age / 60) });
}

/** Una lista de VLANs vacía o con solo [0] significa "sin filtrado". */
function unfiltered(vlans: number[]): boolean {
  return vlans.length === 0 || (vlans.length === 1 && vlans[0] === 0);
}

function vlanText(vlans: number[], t: (k: string, o?: Record<string, string | number>) => string): string {
  if (unfiltered(vlans)) return t("l2clients.unfilteredVlan");
  return vlans.join(", ");
}

export function L2ClientsPage() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<L2Probe>();
  const [query, setQuery] = useState("");

  const load = useCallback(async () => {
    try {
      setProbe(await api.l2clients());
    } catch {
      // Error de red: se conserva el último inventario y el poller reintenta.
    }
  }, []);

  // Mismo refresco que la vista de router: la FDB aprende y caduca sola.
  useEffect(() => { load(); const id = setInterval(load, 5000); return () => clearInterval(id); }, [load]);

  const clients = useMemo(() => {
    const list = probe?.clients ?? [];
    const q = query.trim().toLowerCase();
    if (!q) return list;
    return list.filter((c) =>
      c.mac.toLowerCase().includes(q) ||
      c.port.toLowerCase().includes(q) ||
      (c.vendor ?? "").toLowerCase().includes(q),
    );
  }, [probe, query]);

  // Página entera no aplica (sin bridge resoluble) y se entra por URL.
  if (probe && !probe.applicable) {
    return (
      <Card index={0}>
        <EmptyState illustration={<IlluDevices size={120} />} title={t("l2clients.notApplicable")} />
      </Card>
    );
  }

  const showSearch = (probe?.clients.length ?? 0) > 10;

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card
        index={0}
        title={t("l2clients.title")}
        icon={Network}
        action={probe && (
          <Pill tone="muted">{t("l2clients.subtitle")}</Pill>
        )}
      >
        {!probe ? (
          <div className="grid gap-3">
            <Skeleton className="h-10" />
            <Skeleton className="h-64" />
          </div>
        ) : clients.length === 0 ? (
          <EmptyState illustration={<IlluDevices size={120} />} title={t("l2clients.empty")} />
        ) : (
          <>
            {showSearch && (
              <div className="mb-3 max-w-sm">
                <Input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={t("l2clients.searchPlaceholder")}
                  icon={Search}
                  aria-label={t("l2clients.searchPlaceholder")}
                />
              </div>
            )}
            <div className="overflow-x-auto">
              <table className="w-full text-left text-small">
                <thead>
                  <tr className="text-faint text-caption border-b border-border">
                    <th className="py-2 pr-3 font-medium">{t("l2clients.col.mac")}</th>
                    <th className="py-2 pr-3 font-medium">{t("l2clients.col.vendor")}</th>
                    <th className="py-2 pr-3 font-medium">{t("l2clients.col.port")}</th>
                    <th className="py-2 pr-3 font-medium">{t("l2clients.col.vlans")}</th>
                    <th className="py-2 pr-3 font-medium">{t("l2clients.col.age")}</th>
                  </tr>
                </thead>
                <tbody>
                  {clients.map((c) => (
                    <L2Row key={`${c.mac}\u0000${c.port}`} c={c} t={t} />
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </Card>
    </div>
  );
}

function L2Row({ c, t }: { c: L2Client; t: (k: string, o?: Record<string, string | number>) => string }) {
  return (
    <tr className="border-b border-border/60 last:border-0">
      <td className="py-2 pr-3 font-mono">{c.mac}</td>
      <td className="py-2 pr-3">
        {c.random ? (
          <Pill tone="muted">{t("l2clients.randomMac")}</Pill>
        ) : c.vendor ? (
          c.vendor
        ) : (
          <span className="text-faint">-</span>
        )}
      </td>
      <td className="py-2 pr-3">
        <a href="#/ports" className="text-accent hover:underline">{c.port}</a>
      </td>
      <td className="py-2 pr-3 font-mono tabular-nums">{vlanText(c.vlans, t)}</td>
      <td className="py-2 pr-3">
        <span className="inline-flex items-center gap-1.5">
          {fmtAge(c.age, t)}
          {c.static && <Pill tone="accent">{t("l2clients.staticEntry")}</Pill>}
        </span>
      </td>
    </tr>
  );
}
