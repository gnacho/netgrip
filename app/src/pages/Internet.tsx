import type { FwdProbe, MultiWanProbe } from "../types";
import { WanPage } from "./Wan";
import { ForwardsPage } from "./Forwards";
import { DpiPage } from "./Dpi";
import { BanipPage } from "./Banip";
import { UpnpPage } from "./Upnp";

/**
 * Internet (#487): la pantalla de salida a Internet para la puerta de enlace.
 * Compone lo que antes eran cuatro entradas sueltas del menú (WAN, Reenvío de
 * puertos, Tráfico por app y BanIP) en una sola página apilada, como en el
 * mockup de rediseño (viewInternet usa tarjetas apiladas, sin pestañas).
 * Las páginas se reutilizan tal cual: nada de su funcionalidad se pierde
 * (multiWAN sigue en WanPage, los labs incluidos).
 */
export function InternetPage({ mwan, onMwanChange, fwd, onFwdChange }: {
  mwan?: MultiWanProbe;
  onMwanChange: (p: MultiWanProbe) => void;
  fwd?: FwdProbe;
  onFwdChange: (p: FwdProbe) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* La tarjeta de modo del dispositivo no se repite aquí: ya vive en
          Sistema. */}
      {/* Cada bloque lleva id para que las tarjetas de otras páginas puedan
          enlazar a su sección concreta (#628). */}
      <div id="page-wan"><WanPage mwan={mwan} onMwanChange={onMwanChange} hideMode /></div>
      <div id="page-forwards"><ForwardsPage fwd={fwd} onFwdChange={onFwdChange} /></div>
      <div id="page-upnp"><UpnpPage /></div>
      <div id="page-dpi"><DpiPage /></div>
      <div id="page-banip"><BanipPage /></div>
    </div>
  );
}
