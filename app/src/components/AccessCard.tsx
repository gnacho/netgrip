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

  const saveLuci = () => run("luci", async () => {
    await api.setLuciAccess({ http_port: luciHttp, https_port: luciHttps, force_https: luciForce, enabled: true });
    setProbe(await api.access());
  }, t("access.saved"));

  const saveSsh = () => run("ssh", async () => {
    await api.setSshAccess({ enabled: sshEnabled, port: sshPort });
    setProbe(await api.access());
  }, t("access.saved"));

  const saveTtl = () => run("ttl", async () => {
    await api.setPanelSessionTtl(ttlMin);
    setProbe(await api.access());
  }, t("access.saved"));

  return (
    <Card title={t("access.title")} icon={KeyRound}>
      {!probe ? (
        <p className="text-sm text-muted">…</p>
      ) : (
        <div className="flex flex-col gap-3 text-sm">
          <p className="text-xs text-muted">{t("access.intro")}</p>

          {/* Panel session timeout */}
          <div className="flex items-center justify-between gap-2">
            <span className="text-muted">{t("access.sessionTtl")}</span>
            <div className="flex items-center gap-1">
              <NumInput value={ttlMin} onChange={setTtlMin} min={1} max={100000} />
              <span className="text-xs text-muted">{t("access.minutes")}</span>
              <button onClick={saveTtl} disabled={busy === "ttl" || ttlMin <= 0}
                className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-2 py-1 font-medium">
                {t("access.save")}
              </button>
            </div>
          </div>

          {/* LuCI (uhttpd) */}
          <div className="border-t border-border/50 pt-2">
            <div className="text-xs font-medium uppercase tracking-wider text-muted mb-2">{t("access.luci")}</div>
            <div className="flex items-center justify-between gap-2 mb-1">
              <span className="text-muted">HTTP</span>
              <NumInput value={luciHttp} onChange={setLuciHttp} />
            </div>
            <div className="flex items-center justify-between gap-2 mb-1">
              <span className="text-muted">HTTPS</span>
              <NumInput value={luciHttps} onChange={setLuciHttps} />
            </div>
            <div className="flex items-center justify-between gap-2 mb-2">
              <span className="text-muted">{t("access.forceHttps")}</span>
              <input type="checkbox" checked={luciForce} onChange={(e) => setLuciForce(e.target.checked)} className="accent-accent" />
            </div>
            <button onClick={saveLuci} disabled={busy === "luci"}
              className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-2 py-1 font-medium">
              {t("access.save")}
            </button>
          </div>

          {/* SSH (dropbear) */}
          <div className="border-t border-border/50 pt-2">
            <div className="text-xs font-medium uppercase tracking-wider text-muted mb-2">{t("access.ssh")}</div>
            <div className="flex items-center justify-between gap-2 mb-1">
              <span className="text-muted">{t("access.enableSsh")}</span>
              <input type="checkbox" checked={sshEnabled} onChange={(e) => setSshEnabled(e.target.checked)} className="accent-accent" />
            </div>
            <div className="flex items-center justify-between gap-2 mb-2">
              <span className="text-muted">{t("access.sshPort")}</span>
              <NumInput value={Number(sshPort) || 0} onChange={(v) => setSshPort(String(v))} />
            </div>
            <button onClick={saveSsh} disabled={busy === "ssh"}
              className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-2 py-1 font-medium">
              {t("access.save")}
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
