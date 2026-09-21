import { useCallback, useState } from "react";

const KEY = "netgrip:labs";

function storedLabs(): boolean {
  try {
    return localStorage.getItem(KEY) === "1";
  } catch {
    return false;
  }
}

/**
 * Funciones labs: desbloquean funcionalidad experimental e inestable
 * (multiWAN). Apagado por defecto; la elección vive en Sistema > Opciones
 * y persiste en localStorage("netgrip:labs") por navegador.
 */
export function useLabs() {
  const [labs, setLabsState] = useState<boolean>(storedLabs);
  const setLabs = useCallback((next: boolean) => {
    try {
      localStorage.setItem(KEY, next ? "1" : "0");
    } catch { /* sin persistencia */ }
    setLabsState(next);
  }, []);
  return { labs, setLabs };
}
