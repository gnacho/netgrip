import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
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
        navigator.clipboard?.writeText(text)
          .then(() => toast.push({ tone: "ok", text: t("services.copied") }))
          .catch(() => {});
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

/** Data URL de QR en vivo para un payload arbitrario (configs WireGuard). */
export function useQrData(payload: string | undefined, size: number): string | undefined {
  const [qr, setQr] = useState<string>();
  useEffect(() => {
    let cancelled = false;
    if (!payload) { setQr(undefined); return; }
    QRCode.toDataURL(payload, { width: size * 2, margin: 1, errorCorrectionLevel: "M" })
      .then((d) => { if (!cancelled) setQr(d); })
      .catch(() => { if (!cancelled) setQr(undefined); });
    return () => { cancelled = true; };
  }, [payload, size]);
  return qr;
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
