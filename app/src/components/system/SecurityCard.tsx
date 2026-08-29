import { useState } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound, Lock } from "lucide-react";
import { api } from "../../api";
import { Button, Card, Field, Input, useToast } from "../ui";

/** Fuerza 0–3: vacía / débil / correcta / fuerte. */
function strength(pw: string): 0 | 1 | 2 | 3 {
  if (!pw) return 0;
  let score: 1 | 2 | 3 = 1;
  if (pw.length >= 12 && /\d/.test(pw) && /[a-zA-Z]/.test(pw)) score = 2;
  if (pw.length >= 12 && /[^a-zA-Z0-9]/.test(pw) && /[a-z]/.test(pw) && /[A-Z]/.test(pw)) score = 3;
  return score;
}

const METER_CLS = ["bg-border", "bg-danger", "bg-warn", "bg-ok"] as const;

export function SecurityCard({ onLogout, index = 0 }: { onLogout: () => void; index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const score = strength(next);
  const scoreLabel = score === 1 ? t("security.strengthWeak") : score === 2 ? t("security.strengthOk") : t("security.strengthStrong");

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(undefined);
    if (next.length < 8) { setError(t("security.tooShort", { count: 8 })); return; }
    if (next !== confirm) { setError(t("security.mismatch")); return; }
    setBusy(true);
    try {
      await api.setPassword(current, next);
      push({ tone: "ok", text: t("security.doneToast") });
      setTimeout(onLogout, 2500);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("security.failed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card index={index} title={t("security.cardTitle")} icon={KeyRound}>
      <p className="text-small text-muted mb-3">{t("security.desc")}</p>
      <form onSubmit={submit} className="flex flex-col gap-3">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={t("security.current")} icon={Lock} inputProps={{
            type: "password", value: current, onChange: (e) => setCurrent(e.target.value),
            autoComplete: "current-password",
          }} />
          <Field label={t("security.next")}>
            <div>
              <Input
                type="password" icon={Lock} value={next} onChange={(e) => setNext(e.target.value)}
                autoComplete="new-password"
              />
              {next && (
                <div className="mt-1.5 flex items-center gap-2" role="img" aria-label={scoreLabel}>
                  <div className="flex gap-1 flex-1">
                    {[1, 2, 3].map((i) => (
                      <span key={i} className={`h-1 flex-1 rounded-full transition-colors duration-200 ${i <= score ? METER_CLS[score] : "bg-border"}`} />
                    ))}
                  </div>
                  <span className="text-caption text-muted">{scoreLabel}</span>
                </div>
              )}
            </div>
          </Field>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 sm:items-end">
          <Field label={t("security.confirm")} icon={Lock} inputProps={{
            type: "password", value: confirm, onChange: (e) => setConfirm(e.target.value),
            autoComplete: "new-password",
          }} />
          <Button type="submit" loading={busy} disabled={!current || !next}>{t("security.submit")}</Button>
        </div>
        {error && <p className="text-caption text-danger">{error}</p>}
      </form>
    </Card>
  );
}
