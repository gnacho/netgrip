import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ShieldCheck } from "lucide-react";
import { api } from "../api";
import { Card } from "./Card";

export function SecurityCard({ onLogout }: { onLogout: () => void }) {
  const { t } = useTranslation();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setMsg(undefined);
    if (next.length < 8) { setMsg({ tone: "danger", text: t("security.tooShort", { count: 8 }) }); return; }
    if (next !== confirm) { setMsg({ tone: "danger", text: t("security.mismatch") }); return; }
    setBusy(true);
    try {
      await api.setPassword(current, next);
      setMsg({ tone: "ok", text: t("security.done") });
      setTimeout(onLogout, 2500);
    } catch (err) {
      setMsg({ tone: "danger", text: err instanceof Error ? err.message : t("security.failed") });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card title={t("security.title")} icon={ShieldCheck}>
      <form onSubmit={submit} className="flex flex-col gap-2">
        <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)}
          placeholder={t("security.current")} autoComplete="current-password"
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
        <input type="password" value={next} onChange={(e) => setNext(e.target.value)}
          placeholder={t("security.next")} autoComplete="new-password"
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
        <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)}
          placeholder={t("security.confirm")} autoComplete="new-password"
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
        {msg && <p className={`text-xs ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
        <button type="submit" disabled={busy || !current || !next}
          className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium self-start">
          {t("security.submit")}
        </button>
      </form>
    </Card>
  );
}
