import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, Lock } from "lucide-react";
import { api, enableDemo, UnauthorizedError } from "../api";
import { Banner, Button, Card, Field, HelpTip, LangToggle, ThemeToggle } from "../components/ui";
import { Logo } from "../components/ui/illustrations";

/**
 * Login según login.md: centrado 400px, calma y confianza, puerta al modo
 * demo. La clase `dark` ya la aplicó el script inline de index.html.
 */
export function Login({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [error, setError] = useState<"credentials" | "network">();
  const [busy, setBusy] = useState(false);
  const [backendDown, setBackendDown] = useState(false);
  const [shake, setShake] = useState(false);

  // Sondeo: si /api/me falla por red (no 401), ofrecer el modo demo.
  useEffect(() => {
    api.me().catch((e) => {
      if (!(e instanceof UnauthorizedError)) setBackendDown(true);
    });
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    try {
      await api.login(password);
      onSuccess();
    } catch (err) {
      const kind = err instanceof UnauthorizedError ? "credentials" : "network";
      setError(kind);
      if (kind === "network") setBackendDown(true);
      if (kind === "credentials") {
        setShake(true);
        setTimeout(() => setShake(false), 260);
      }
    } finally {
      setBusy(false);
    }
  };

  const enterDemo = () => {
    enableDemo();
    window.location.reload();
  };

  return (
    <main className="min-h-screen bg-bg flex flex-col items-center justify-center p-6">
      <div className="w-full max-w-[400px] flex flex-col items-center">
        {/* cabecera: logo + claim */}
        <div className="flex flex-col items-center text-center animate-fade-up" style={{ "--i": 0 } as React.CSSProperties}>
          <span className="text-accent mb-3"><Logo size={40} /></span>
          <h1 className="text-display">{t("login.claim")}</h1>
          <p className="text-small text-muted mt-1">{t("login.tagline")}</p>
        </div>

        {/* card de entrada */}
        <div className="w-full mt-8 animate-fade-up" style={{ "--i": 2 } as React.CSSProperties}>
          <Card animate={false}>
            <div className="flex items-center gap-1 mb-4">
              <h2 className="text-h2 flex-1">{t("login.title")}</h2>
              <HelpTip title={t("help.login.title")} body={t("help.login.body")} />
            </div>
            <form onSubmit={submit} className={shake ? "animate-shake" : ""}>
              <Field
                label={t("login.password")}
                icon={Lock}
                hint={error === "credentials" ? undefined : t("login.passwordHint")}
                inputProps={{
                  type: "password",
                  value: password,
                  onChange: (e) => setPassword(e.target.value),
                  autoComplete: "current-password",
                  autoFocus: true,
                  disabled: busy,
                  error: error === "credentials",
                }}
              />
              {error && (
                <Banner tone="danger" className="mt-3">
                  {error === "credentials" ? t("login.error") : t("login.errorNetwork")}
                </Banner>
              )}
              <Button type="submit" loading={busy} disabled={!password} className="w-full mt-4">
                {busy ? t("login.submitting") : t("login.submit")}
              </Button>
            </form>
          </Card>

          {/* idioma + tema */}
          <div className="mt-5 flex items-center justify-center gap-3">
            <LangToggle />
            <ThemeToggle />
          </div>
        </div>

        {/* pie demo */}
        <div className="w-full mt-8 flex flex-col items-center animate-fade-up" style={{ "--i": 4 } as React.CSSProperties}>
          <p className="text-small text-faint mb-3">── {t("login.demoDivider")} ──</p>
          {backendDown ? (
            <Button variant="secondary" onClick={enterDemo} className="w-full">
              {t("login.demoCta")} <ArrowRight size={16} aria-hidden="true" />
            </Button>
          ) : (
            <button
              type="button"
              onClick={enterDemo}
              className="text-small text-muted hover:text-accent ring-focus rounded-sm transition-colors"
            >
              {t("login.viewDemo")} →
            </button>
          )}
        </div>
      </div>
    </main>
  );
}
