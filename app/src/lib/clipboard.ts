/**
 * Copiar al portapapeles (#666). `navigator.clipboard` solo existe en
 * contextos seguros (HTTPS o localhost); el panel normal se sirve por HTTP en
 * una IP de LAN, así que ahí es undefined y el botón fallaba siempre. Orden:
 * Clipboard API, luego un textarea oculto con `execCommand("copy")`, y si el
 * navegador rechaza las dos, `false` para que el llamador muestre el texto.
 */
export async function copyText(text: string): Promise<boolean> {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Permiso denegado o contexto no seguro a medias: probar el fallback.
    }
  }
  try {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    // Fuera de pantalla pero seleccionable: display:none no se puede copiar.
    area.style.position = "fixed";
    area.style.top = "-1000px";
    area.style.left = "0";
    area.style.opacity = "0";
    document.body.appendChild(area);
    const selection = document.getSelection();
    const previous = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null;
    area.select();
    area.setSelectionRange(0, area.value.length);
    const ok = document.execCommand("copy");
    document.body.removeChild(area);
    if (previous && selection) {
      selection.removeAllRanges();
      selection.addRange(previous);
    }
    return ok;
  } catch {
    return false;
  }
}
