import type { MultiWanProbe } from "../types";
import { WanPage } from "./Wan";
import { DpiPage } from "./Dpi";
import { UpnpPage } from "./Upnp";

/**
 * Internet (#487): la pantalla de salida a Internet para la puerta de enlace.
 * Compone lo que antes eran cuatro entradas sueltas del menú (WAN, Reenvío de
 * puertos, Tráfico por app y BanIP) en una sola página apilada, como en el
 * mockup de rediseño (viewInternet usa tarjetas apiladas, sin pestañas).
 * Las páginas se reutilizan tal cual: nada de su funcionalidad se pierde
 * (multiWAN sigue en WanPage, los labs incluidos).
 */
export function InternetPage({ mwan, onMwanChange }: {
  mwan?: MultiWanProbe;
  onMwanChange: (p: MultiWanProbe) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* La tarjeta de modo del dispositivo no se repite aquí: ya vive en
          Sistema. */}
      {/* Cada bloque lleva id para que las tarjetas de otras páginas puedan
          enlazar a su sección concreta (#628). */}
      <div id="page-wan"><WanPage mwan={mwan} onMwanChange={onMwanChange} hideMode /></div>
      <div id="page-dpi"><DpiPage /></div>
      {/* UPnP goes last (#790): it is the least used block of the page and,
          when miniupnpd is missing, all it shows is an install prompt. */}
      <div id="page-upnp"><UpnpPage /></div>
    </div>
  );
}
