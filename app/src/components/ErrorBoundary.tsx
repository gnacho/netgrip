import { Component, type ErrorInfo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { RefreshCw } from "lucide-react";
import { Button } from "./ui/Button";
import { Logo } from "./ui/illustrations";

/**
 * Barrera de errores (#515): un error de render en cualquier tarjeta o
 * página dejaba la pantalla en blanco, sin pista ni vía de escape. Aquí
 * se convierte en una pantalla recuperable con recarga. Importa solo
 * módulos hoja (Button/Logo), nunca el barrel de ui, para que el propio
 * fallback pueda pintarse aunque el fallo venga de un componente grande.
 */
function CrashFallback({ onReload }: { onReload: () => void }) {
  const { t } = useTranslation();
  return (
    <main className="min-h-screen bg-bg flex flex-col items-center justify-center p-6">
      <div className="w-full max-w-[400px] flex flex-col items-center text-center">
        <span className="text-accent mb-3"><Logo size={40} /></span>
        <h1 className="text-display">{t("error.crashTitle")}</h1>
        <p className="text-small text-muted mt-2">{t("error.crashBody")}</p>
        <Button className="mt-6" icon={RefreshCw} onClick={onReload}>
          {t("error.reload")}
        </Button>
      </div>
    </main>
  );
}

export class ErrorBoundary extends Component<{ children: ReactNode }, { crashed: boolean }> {
  state = { crashed: false };

  static getDerivedStateFromError() {
    return { crashed: true };
  }

  componentDidCatch(error: unknown, info: ErrorInfo) {
    // Sin telemetría: dejar traza en consola para diagnóstico en el
    // propio navegador del usuario.
    console.error("netgrip ui crashed", error, info);
  }

  render() {
    if (this.state.crashed) {
      return <CrashFallback onReload={() => window.location.reload()} />;
    }
    return this.props.children;
  }
}
