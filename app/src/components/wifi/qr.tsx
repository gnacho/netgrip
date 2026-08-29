import { useEffect, useState } from "react";
import QRCode from "qrcode";
import { QrFrame } from "../ui/illustrations";

/** Escapa caracteres especiales del formato WIFI: del QR de unión. */
function escapeWifi(s: string): string {
  return s.replace(/([\\;,:"'])/g, "\\$1");
}

/**
 * QR de unión WiFi en vivo. La PSK es write-only en la API: solo podemos
 * generar QR cuando la clave se ha escrito en esta sesión (o la red es
 * abierta). Devuelve undefined mientras no haya datos suficientes.
 */
export function useWifiQr(ssid: string, key: string, encryption: string, size: number): string | undefined {
  const [qr, setQr] = useState<string>();
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const open = encryption === "none";
      if (!ssid.trim() || (!open && key.length < 8)) {
        if (!cancelled) setQr(undefined);
        return;
      }
      try {
        const uri = open
          ? `WIFI:T:nopass;S:${escapeWifi(ssid)};;`
          : `WIFI:T:WPA;S:${escapeWifi(ssid)};P:${escapeWifi(key)};;`;
        const data = await QRCode.toDataURL(uri, { width: size * 2, margin: 1, errorCorrectionLevel: "M" });
        if (!cancelled) setQr(data);
      } catch {
        if (!cancelled) setQr(undefined);
      }
    })();
    return () => { cancelled = true; };
  }, [ssid, key, encryption, size]);
  return qr;
}

/**
 * QR con marco decorativo qr-frame (§13). Al regenerarse entra con
 * banner-in (200ms, opacidad) según §6 de wifi.md.
 */
export function QrBox({ data, size = 120, alt = "QR" }: { data: string; size?: number; alt?: string }) {
  const pad = Math.round(size * 0.1);
  return (
    <span className="relative inline-block shrink-0" style={{ width: size + pad * 2, height: size + pad * 2 }}>
      <span className="absolute inset-0" aria-hidden="true">
        <QrFrame size={size + pad * 2} />
      </span>
      <img
        key={data}
        src={data}
        alt={alt}
        width={size}
        height={size}
        className="absolute rounded-sm bg-white animate-banner-in"
        style={{ top: pad, left: pad }}
      />
    </span>
  );
}
