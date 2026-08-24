import { useState } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound } from "lucide-react";
import { api } from "../api";

export function Login({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(false);
    try {
      await api.login(password);
      onSuccess();
    } catch {
      setError(true);
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="min-h-screen flex items-center justify-center p-4">
      <form onSubmit={submit} className="w-full max-w-sm bg-card border border-border rounded-xl p-6">
        <div className="flex items-center gap-2 mb-1">
          <KeyRound size={20} className="text-accent" />
          <h1 className="text-lg font-semibold">{t("app.name")}</h1>
        </div>
        <p className="text-sm text-muted mb-4">{t("login.subtitle")}</p>
        {error && <p className="text-sm text-danger mb-3">{t("login.error")}</p>}
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={t("login.password")}
          autoComplete="current-password"
          autoFocus
          className="w-full bg-bg border border-border rounded-lg px-3 py-2 mb-4 outline-none focus:border-accent"
        />
        <button
          type="submit"
          disabled={busy || !password}
          className="w-full bg-accent hover:bg-accent/85 disabled:opacity-50 rounded-lg px-3 py-2 font-medium"
        >
          {t("login.submit")}
        </button>
      </form>
    </main>
  );
}
