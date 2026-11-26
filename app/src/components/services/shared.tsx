import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { copyText } from "../../lib/clipboard";
import { Copy } from "lucide-react";
import QRCode from "qrcode";
import { useToast } from "../ui";

/**
 * Detalle que se despliega/colapsa con altura animada 200ms (services.md §10):
 * servicio apagado → solo el SettingRow; al activarse, el detalle se abre.
 */
export function Reveal({ open, children }: { open: boolean; children: ReactNode }) {
  return (
    <div
      className={`grid transition-[grid-template-rows] duration-200 ease-[var(--ease-soft)] ${open ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
      aria-hidden={!open}
    >
      <div className="overflow-hidden">
        {children}
      </div>
    </div>
  );
}

/** Nombre técnico como subtítulo visible (services.md §1). */
export function TechName({ children }: { children: ReactNode }) {
  return <p className="text-small font-mono text-muted -mt-1 mb-1.5">{children}</p>;
}

/** Botón copiar (claves, IPs) con toast "Copiada". */
export function CopyButton({ text, label }: { text: string; label?: string }) {
  const { t } = useTranslation();
  const toast = useToast();
  return (
    <button
      type="button"
      aria-label={label ?? t("services.copy")}
      title={label ?? t("services.copy")}
      onClick={() => {
        void copyText(text).then((ok) => {
          if (ok) toast.push({ tone: "ok", text: t("services.copied") });
        });
      }}
      className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-sm text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors duration-[var(--dur-fast)]"
    >
      <Copy size={14} aria-hidden="true" />
    </button>
  );
}

/** Clave pública truncada para mostrar (mono). */
export function shortKey(key: string): string {
  return key.length > 20 ? `${key.slice(0, 16)}…` : key;
}

/** Data URL de QR en vivo para un payload arbitrario (configs WireGuard).
 *  Margen de 4 módulos y nivel de corrección Q, como qrencode (#659). */
export function useQrData(payload: string | undefined, size: number): string | undefined {
  const [qr, setQr] = useState<string>();
  useEffect(() => {
    let cancelled = false;
    if (!payload) { setQr(undefined); return; }
    QRCode.toDataURL(payload, { width: size * 2, margin: 4, errorCorrectionLevel: "Q" })
      .then((d) => { if (!cancelled) setQr(d); })
      .catch(() => { if (!cancelled) setQr(undefined); });
    return () => { cancelled = true; };
  }, [payload, size]);
  return qr;
}

/**
 * QR dibujado módulo a módulo desde la matriz, en vez de escalar un PNG ya
 * rasterizado. Con un perfil de WireGuard (69x69 en versión 13) el PNG de
 * `toDataURL` llega con los bordes interpolados y la cámara del móvil no
 * encuentra los patrones de posición; dibujar rectángulos exactos más un
 * margen blanco de 4 módulos lo arregla (#659).
 *
 * `light` y `dark` se pasan como color CSS porque el QR va sobre el fondo
 * claro de la tarjeta, también con el panel en oscuro.
 */
export function qrMatrixSvg(payload: string | undefined, light = "#ffffff", dark = "#000000"): string | undefined {
  if (!payload) return undefined;
  let qr: ReturnType<typeof QRCode.create>;
  try {
    qr = QRCode.create(payload, { errorCorrectionLevel: "Q" });
  } catch {
    return undefined;
  }
  const size = qr.modules.size;
  const data = qr.modules.data;
  const quiet = 4;
  let path = "";
  for (let row = 0; row < size; row++) {
    for (let col = 0; col < size; col++) {
      if (data[row * size + col]) path += `M${col + quiet} ${row + quiet}h1v1h-1z`;
    }
  }
  const dim = size + quiet * 2;
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${dim} ${dim}" shape-rendering="crispEdges">` +
    `<rect width="${dim}" height="${dim}" fill="${light}"/>` +
    `<path d="${path}" fill="${dark}"/>` +
    `</svg>`
  );
}

/** Descarga un texto como fichero (configs .conf / .ovpn). */
export function downloadText(filename: string, text: string, mime = "text/plain") {
  const blob = new Blob([text], { type: mime });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = filename;
  a.click();
  URL.revokeObjectURL(a.href);
}
