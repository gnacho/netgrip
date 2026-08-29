/**
 * Ilustraciones §13 de design.md: SVG inline, trazo fino 1.5px, rellenos
 * *-soft, un único acento, esquinas redondeadas, sin texto. currentColor
 * para el trazo base (el contenedor suele llevar text-faint).
 */
const S = { stroke: "currentColor", strokeWidth: 1.5, fill: "none", strokeLinecap: "round", strokeLinejoin: "round" } as const;
const SOFT = "var(--color-surface-2)";
const ACCENT = "var(--color-accent)";
const OK = "var(--color-ok)";

/** illu-router: router de frente con 3 LEDs (uno verde) y antena. 160×120 */
export function IlluRouter({ size = 160 }: { size?: number }) {
  return (
    <svg viewBox="0 0 160 120" width={size} height={size * 0.75} {...S} aria-hidden="true">
      <line x1="112" y1="52" x2="112" y2="26" />
      <circle cx="112" cy="22" r="4" fill={SOFT} />
      <rect x="28" y="52" width="104" height="40" rx="8" fill={SOFT} />
      <circle cx="46" cy="72" r="3.5" fill={OK} stroke="none" />
      <circle cx="62" cy="72" r="3.5" />
      <circle cx="78" cy="72" r="3.5" />
      <line x1="100" y1="66" x2="120" y2="66" />
      <line x1="100" y1="78" x2="120" y2="78" />
      <line x1="44" y1="92" x2="44" y2="98" />
      <line x1="116" y1="92" x2="116" y2="98" />
    </svg>
  );
}

/** illu-wifi-waves: AP con 3 ondas concéntricas, una en accent. 120×96 */
export function IlluWifiWaves({ size = 120 }: { size?: number }) {
  return (
    <svg viewBox="0 0 120 96" width={size} height={size * 0.8} {...S} aria-hidden="true">
      <path d="M38 52 a31 31 0 0 1 44 0" />
      <path d="M46 62 a20 20 0 0 1 28 0" stroke={ACCENT} />
      <path d="M54 72 a9 9 0 0 1 12 0" />
      <circle cx="60" cy="80" r="4" fill={SOFT} />
    </svg>
  );
}

/** illu-devices: móvil, portátil y TV unidos por líneas de puntos. 160×96 */
export function IlluDevices({ size = 160 }: { size?: number }) {
  return (
    <svg viewBox="0 0 160 96" width={size} height={size * 0.6} {...S} aria-hidden="true">
      <circle cx="80" cy="48" r="6" fill={SOFT} stroke={ACCENT} />
      <rect x="14" y="18" width="20" height="34" rx="4" fill={SOFT} />
      <rect x="62" y="10" width="36" height="24" rx="3" fill={SOFT} />
      <line x1="70" y1="40" x2="90" y2="40" />
      <rect x="120" y="16" width="30" height="20" rx="3" fill={SOFT} />
      <line x1="128" y1="42" x2="142" y2="42" />
      <line x1="34" y1="40" x2="74" y2="47" strokeDasharray="2 4" />
      <line x1="80" y1="34" x2="80" y2="42" strokeDasharray="2 4" />
      <line x1="86" y1="47" x2="124" y2="38" strokeDasharray="2 4" />
    </svg>
  );
}

/** illu-shield: escudo con check verde. 96×96 */
export function IlluShield({ size = 96 }: { size?: number }) {
  return (
    <svg viewBox="0 0 96 96" width={size} height={size} {...S} aria-hidden="true">
      <path d="M48 12 L76 22 V48 c0 18 -12 30 -28 36 C32 78 20 66 20 48 V22 Z" fill={SOFT} />
      <path d="M36 48 l8 8 l16 -18" stroke={OK} strokeWidth={2.5} />
    </svg>
  );
}

/** illu-plug: cable ethernet con conector RJ45 y tick. 120×96 */
export function IlluPlug({ size = 120 }: { size?: number }) {
  return (
    <svg viewBox="0 0 120 96" width={size} height={size * 0.8} {...S} aria-hidden="true">
      <path d="M14 70 C 34 70 36 52 52 52" />
      <rect x="52" y="40" width="26" height="24" rx="4" fill={SOFT} />
      <rect x="78" y="46" width="16" height="12" rx="2" fill={SOFT} />
      <line x1="84" y1="46" x2="84" y2="42" />
      <line x1="89" y1="46" x2="89" y2="42" />
      <circle cx="100" cy="28" r="11" fill={SOFT} stroke={OK} />
      <path d="M95 28 l4 4 l7 -8" stroke={OK} strokeWidth={2} />
    </svg>
  );
}

/** illu-disk: disco USB con barra de capacidad al 40 %. 120×96 */
export function IlluDisk({ size = 120 }: { size?: number }) {
  return (
    <svg viewBox="0 0 120 96" width={size} height={size * 0.8} {...S} aria-hidden="true">
      <rect x="30" y="30" width="60" height="40" rx="7" fill={SOFT} />
      <circle cx="60" cy="50" r="11" />
      <circle cx="60" cy="50" r="3" fill={SOFT} />
      <rect x="34" y="76" width="52" height="4" rx="2" fill={SOFT} stroke="none" />
      <rect x="34" y="76" width="21" height="4" rx="2" fill={ACCENT} stroke="none" />
    </svg>
  );
}

/** illu-fleet: dos routers pequeños unidos por una línea con nodo. 140×96 */
export function IlluFleet({ size = 140 }: { size?: number }) {
  return (
    <svg viewBox="0 0 140 96" width={size} height={size * 0.69} {...S} aria-hidden="true">
      <rect x="14" y="34" width="42" height="26" rx="6" fill={SOFT} />
      <circle cx="26" cy="47" r="3" fill={OK} stroke="none" />
      <rect x="84" y="34" width="42" height="26" rx="6" fill={SOFT} />
      <circle cx="96" cy="47" r="3" fill={OK} stroke="none" />
      <line x1="56" y1="47" x2="84" y2="47" strokeDasharray="2 4" />
      <circle cx="70" cy="47" r="4" fill={SOFT} stroke={ACCENT} />
    </svg>
  );
}

/** illu-party: cohete minimalista con check. 120×96 */
export function IlluParty({ size = 120 }: { size?: number }) {
  return (
    <svg viewBox="0 0 120 96" width={size} height={size * 0.8} {...S} aria-hidden="true">
      <path d="M60 14 c10 8 14 20 12 34 l-12 8 -12 -8 c-2 -14 2 -26 12 -34 Z" fill={SOFT} />
      <circle cx="60" cy="36" r="5" stroke={ACCENT} />
      <path d="M48 56 l-8 12 M72 56 l8 12 M60 58 v14" />
      <path d="M52 74 l5 5 l11 -11" stroke={OK} strokeWidth={2.5} />
    </svg>
  );
}

/** logo NetGrip: pinza/puño estilizado sobre nodo de red. 32×32 */
export function Logo({ size = 32 }: { size?: number }) {
  return (
    <svg viewBox="0 0 32 32" width={size} height={size} fill="none" stroke="currentColor"
      strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="16" cy="7" r="3" />
      <path d="M16 10 v5" />
      <path d="M16 15 L9 22 M16 15 l7 7" />
      <circle cx="9" cy="25" r="2.6" fill="currentColor" stroke="none" />
      <circle cx="23" cy="25" r="2.6" />
    </svg>
  );
}

/** qr-frame: marco decorativo tipo escáner para códigos QR. 220×220 */
export function QrFrame({ size = 220 }: { size?: number }) {
  return (
    <svg viewBox="0 0 220 220" width={size} height={size} {...S} stroke={ACCENT} strokeWidth={3} aria-hidden="true">
      <path d="M8 40 V16 a8 8 0 0 1 8 -8 h24" />
      <path d="M180 8 h24 a8 8 0 0 1 8 8 v24" />
      <path d="M212 180 v24 a8 8 0 0 1 -8 8 h-24" />
      <path d="M40 212 h-24 a8 8 0 0 1 -8 -8 v-24" />
    </svg>
  );
}
