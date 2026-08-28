import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound } from "lucide-react";
import { api } from "../api";
import type { AccessProbe } from "../types";
import { Card } from "./Card";

function NumInput({ value, onChange, placeholder, min = 1, max = 65535 }: {
  value: number;
  onChange: (v: number) => void;
  placeholder?: string;
  min?: number;
  max?: number;
}) {
  return (
    <input
      type="number" value={value || ""} placeholder={placeholder}
      min={min} max={max} onChange={(e) => onChange(Number(e.target.value))}
      className="bg-bg border border-border rounded-lg px-2 py-1 text-sm outline-none focus:border-accent w-20"
    />
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <div className="text-xs text-muted mb-1">{label}</div>
      {children}
    </div>
  );
}

// durationToMin parses a Go time.Duration string ("12h0m0s") into minutes.
function durationToMin(d: string): number {
  if (!d) return 720;
  let total = 0;
  const re = /(\d+)([hms])/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(d)) !== null) {
    const n = Number(m[1]);
    if (m[2] === "h") total += n * 60;
    else if (m[2] === "m") total += n;
    else if (m[2] === "s") total += n / 60;
  }
  return Math.round(total) || 720;
}

export function AccessCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<AccessProbe>();
  const [luciHttp, setLuciHttp] = useState(80);
  const [luciHttps, setLuciHttps] = useState(443);
  const [luciForce, setLuciForce] = useState(false);
  const [sshEnabled, setSshEnabled] = useState(true);
  const [sshPort, setSshPort] = useState("22");
  const [ttlMin, setTtlMin] = useState(720);
  const [busy, setBusy] = useState<string>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => {
    api.access().then((p) => {
      setProbe(p);
      setLuciHttp(p.luci.http_port);
      setLuciHttps(p.luci.https_port);
      setLuciForce(p.luci.force_https);
      setSshEnabled(p.ssh.enabled);
      setSshPort(p.ssh.port || "22");
      setTtlMin(durationToMin(p.panel.session_ttl));
    }).catch(() => {});
  }, []);

  const run = async (id: string, fn: () => Promise<void>, ok: string) => {
    setBusy(id); setMsg(undefined);
    try { await fn(); setMsg({ tone: "ok", text: ok }); }
    catch (err) { setMsg({ tone: "danger", text: err instanceof Error ? err.message : String(err) }); }
    finally { setBusy(undefined); }
  };

  const saveAll = () => run("all", async () => {
    await api.setPanelSessionTtl(ttlMin);
    await api.setLuciAccess({ http_port: luciHttp, https_port: luciHttps, force_https: luciForce, enabled: true });
    await api.setSshAccess({ enabled: sshEnabled, port: sshPort });
    setProbe(await api.access());
  }, t("access.saved"));

  return (
    <Card title={t("access.title")} icon={KeyRound}>
      {!probe ? (
        <p className="text-sm text-muted">…</p>
      ) : (
        <div className="flex flex-col gap-3 text-sm">
          <p className="text-xs text-muted">{t("access.intro")}</p>

          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-x-4 gap-y-3">
            {/* Panel session timeout */}
            <Field label={t("access.sessionTtl")}>
              <div className="flex items-center gap-1">
                <NumInput value={ttlMin} onChange={setTtlMin} min={1} max={100000} />
                <span className="text-xs text-muted">{t("access.minutes")}</span>
              </div>
            </Field>

            {/* LuCI (uhttpd) */}
            <Field label={t("access.luciHttp")}>
              <NumInput value={luciHttp} onChange={setLuciHttp} />
            </Field>
            <Field label={t("access.luciHttps")}>
              <NumInput value={luciHttps} onChange={setLuciHttps} />
            </Field>
            <Field label={t("access.luciForce")}>
              <input type="checkbox" checked={luciForce} onChange={(e) => setLuciForce(e.target.checked)} className="accent-accent" />
            </Field>

            {/* SSH (dropbear) */}
            <Field label={t("access.enableSsh")}>
              <input type="checkbox" checked={sshEnabled} onChange={(e) => setSshEnabled(e.target.checked)} className="accent-accent" />
            </Field>
            <Field label={t("access.sshPort")}>
              <NumInput value={Number(sshPort) || 0} onChange={(v) => setSshPort(String(v))} />
            </Field>
          </div>

          <div className="flex justify-end">
            <button onClick={saveAll} disabled={busy === "all" || ttlMin <= 0}
              className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium">
              {busy === "all" ? "…" : t("access.save")}
            </button>
          </div>

          <div className="text-xs text-muted">
            {t("access.luciHint")} {t("access.sshHint")}
          </div>
          {msg && <p className={`text-xs ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
        </div>
      )}
    </Card>
  );
}
