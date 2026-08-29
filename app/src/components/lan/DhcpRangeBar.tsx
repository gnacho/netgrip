import { useTranslation } from "react-i18next";

const IP_RE = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/;

/** Último octeto si `ip` pertenece al /24 `base` ("192.168.8"); si no, undefined. */
function lastOctet(ip: string, base: string): number | undefined {
  const m = IP_RE.exec(ip.trim());
  if (!m) return undefined;
  if (`${m[1]}.${m[2]}.${m[3]}` !== base) return undefined;
  const o = Number(m[4]);
  return o <= 255 ? o : undefined;
}

/**
 * Visual del rango DHCP (design-rev2 §5): barra horizontal que representa la
 * subred /24 del router (último octeto 0–255, x ≡ octeto), con el tramo del
 * pool start→end resaltado en accent-soft/accent y ticks violeta para las
 * reservas fijas (tooltip "Nombre · IP" vía <title>). Reactivo a los campos.
 */
export function DhcpRangeBar({ routerIp, start, end, reservations }: {
  routerIp: string;
  start: number;
  end: number;
  reservations: { ip: string; name?: string }[];
}) {
  const { t } = useTranslation();
  const m = IP_RE.exec(routerIp.trim());
  if (!m || m.slice(1).some((o) => Number(o) > 255)) return null;
  const base = `${m[1]}.${m[2]}.${m[3]}`;

  const s = Math.max(0, Math.min(255, Math.round(start)));
  const e = Math.max(0, Math.min(255, Math.round(end)));
  const poolOk = e > s;
  const markers = reservations
    .map((r) => ({ octet: lastOctet(r.ip, base), label: r.name ? `${r.name} · ${r.ip}` : r.ip }))
    .filter((r): r is { octet: number; label: string } => r.octet !== undefined);

  return (
    <div>
      <svg
        viewBox="0 0 256 16"
        width="100%"
        height="16"
        preserveAspectRatio="none"
        role="img"
        aria-label={t("lan.rangeCaption")}
        className="block"
      >
        {/* pista: la subred completa (.0–.255) */}
        <rect x={0} y={4} width={256} height={8} rx={4}
          fill="var(--color-fill)" stroke="var(--color-border)" strokeWidth={1} vectorEffect="non-scaling-stroke" />
        {/* tramo del pool */}
        {poolOk && (
          <rect x={s} y={3} width={Math.max(1.5, e - s + 1)} height={10} rx={3}
            fill="var(--color-accent-soft)" stroke="var(--color-accent)" strokeWidth={1} vectorEffect="non-scaling-stroke" />
        )}
        {/* reservas fijas */}
        {markers.map((r) => (
          <rect key={r.label} x={Math.max(0, Math.min(253.5, r.octet - 1.25))} y={2} width={2.5} height={12} rx={1.25}
            fill="var(--color-violet)" stroke="var(--color-surface)" strokeWidth={1} vectorEffect="non-scaling-stroke">
            <title>{r.label}</title>
          </rect>
        ))}
      </svg>
      <p className="text-caption text-muted mt-1.5">{t("lan.rangeCaption")}</p>
    </div>
  );
}
