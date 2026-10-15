import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./i18n";
import "./index.css";
import { ErrorBoundary } from "./components/ErrorBoundary";
import App from "./App";

// Un chunk perezoso que ya no existe (panel actualizado con la pestaña
// abierta) rechaza la importación dinámica; recargar una vez trae el
// index.html nuevo (se sirve sin caché) y con él los chunks vigentes.
// La marca de sesión evita un bucle si el fallo persiste.
window.addEventListener("vite:preloadError", () => {
  if (sessionStorage.getItem("netgrip:preload-reload")) return;
  sessionStorage.setItem("netgrip:preload-reload", "1");
  window.location.reload();
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
);
