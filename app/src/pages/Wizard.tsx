import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, ChevronLeft, ChevronRight, Lock, Router, Shield, Wifi, Users, Cpu, ArrowRight } from "lucide-react";
import { api } from "../api";
import type { WifiUI, WizardState, GuestProbe, IoTProbe, WGProbe } from "../types";

type Step = "welcome" | "password" | "wifi" | "guest" | "iot" | "wireguard" | "done";

const ALL_STEPS: Step[] = ["welcome", "password", "wifi", "guest", "iot", "wireguard", "done"];

export function Wizard({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const [step, setStep] = useState<Step>("welcome");
  const [state, setState] = useState<WizardState>();
  const [wifiIfaces, setWifiIfaces] = useState<WifiUI[]>([]);
  const [guest, setGuest] = useState<GuestProbe>();
  const [iot, setIot] = useState<IoTProbe>();
  const [wg, setWg] = useState<WGProbe>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const isGateway = state?.mode === "router";
  const steps: Step[] = ALL_STEPS.filter(
    (s) => s !== "guest" || isGateway,
  );
  const currentIdx = steps.indexOf(step);

  useEffect(() => {
    (async () => {
      try {
        const [ws, wf, g, i, w] = await Promise.all([
          api.wizardState(),
          api.wifi(),
          api.guestwifi(),
          api.iotwifi(),
          api.wireguard(),
        ]);
        setState(ws);
        setWifiIfaces(wf.interfaces);
        setGuest(g);
        setIot(i);
        setWg(w);
      } catch {
        setError(t("error.load"));
      }
    })();
  }, [t]);

  const finish = useCallback(async () => {
    await api.wizardComplete().catch(() => {});
    onDone();
  }, [onDone]);

  const next = () => {
    const idx = steps.indexOf(step);
    if (idx < steps.length - 1) setStep(steps[idx + 1]);
    else finish();
  };

  const prev = () => {
    const idx = steps.indexOf(step);
    if (idx > 0) setStep(steps[idx - 1]);
  };

  const skip = () => next();

  const progress = steps.length > 1 ? ((currentIdx) / (steps.length - 1)) * 100 : 100;

  const stepIcon = (s: Step) => {
    switch (s) {
      case "welcome": return <Router size={40} className="text-accent" />;
      case "password": return <Lock size={24} />;
      case "wifi": return <Wifi size={24} />;
      case "guest": return <Users size={24} />;
      case "iot": return <Cpu size={24} />;
      case "wireguard": return <Shield size={24} />;
      case "done": return <Check size={40} className="text-green-400" />;
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-lg">
        {step !== "welcome" && step !== "done" && (
          <div className="mb-6">
            <div className="flex items-center justify-between mb-2 text-xs text-muted">
              <span>{t(`wizard.step.${step}`)}</span>
              <span>{currentIdx + 1} / {steps.length}</span>
            </div>
            <div className="h-1.5 bg-card rounded-full overflow-hidden">
              <div
                className="h-full bg-accent rounded-full transition-all duration-300"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        )}

        <div className="bg-card border border-border rounded-2xl p-6 md:p-8">
          {error && <p className="text-danger text-sm mb-4">{error}</p>}

          {step === "welcome" && <WelcomeStep onStart={next} onSkip={finish} t={t} icon={stepIcon("welcome")} />}
          {step === "password" && <PasswordStep onNext={next} onSkip={skip} t={t} icon={stepIcon("password")} loading={loading} setLoading={setLoading} setError={setError} />}
          {step === "wifi" && <WifiStep ifaces={wifiIfaces} onNext={next} onSkip={skip} t={t} icon={stepIcon("wifi")} loading={loading} setLoading={setLoading} setError={setError} />}
          {step === "guest" && <GuestStep guest={guest} onNext={next} onSkip={skip} t={t} icon={stepIcon("guest")} loading={loading} setLoading={setLoading} setError={setError} setGuest={setGuest} />}
          {step === "iot" && <IoTStep iot={iot} onNext={next} onSkip={skip} t={t} icon={stepIcon("iot")} loading={loading} setLoading={setLoading} setError={setError} setIot={setIot} />}
          {step === "wireguard" && <WGStep wg={wg} onNext={next} onSkip={skip} t={t} icon={stepIcon("wireguard")} loading={loading} setLoading={setLoading} setError={setError} setWg={setWg} />}
          {step === "done" && <DoneStep onFinish={finish} t={t} icon={stepIcon("done")} />}

          {step !== "welcome" && step !== "done" && step !== "password" && step !== "wifi" && (
            <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
              <button onClick={prev} className="flex items-center gap-1 text-sm text-muted hover:text-text transition-colors">
                <ChevronLeft size={16} /> {t("wizard.back")}
              </button>
              <button
                onClick={next}
                disabled={loading}
                className="flex items-center gap-1 bg-accent text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-accent/90 disabled:opacity-50 transition-colors"
              >
                {t("wizard.next")} <ChevronRight size={16} />
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function WelcomeStep({ onStart, onSkip, t, icon }: { onStart: () => void; onSkip: () => void; t: (k: string) => string; icon: React.ReactNode }) {
  return (
    <div className="text-center">
      <div className="flex justify-center mb-4">{icon}</div>
      <h1 className="text-2xl font-bold mb-2">{t("wizard.title")}</h1>
      <p className="text-muted mb-8">{t("wizard.intro")}</p>
      <button
        onClick={onStart}
        className="w-full bg-accent text-white py-3 rounded-xl font-medium hover:bg-accent/90 transition-colors mb-3"
      >
        {t("wizard.startSetup")}
      </button>
      <button
        onClick={onSkip}
        className="text-sm text-muted hover:text-text transition-colors"
      >
        {t("wizard.skipToPanel")} <ArrowRight size={14} className="inline ml-1" />
      </button>
    </div>
  );
}

function PasswordStep({ onNext, onSkip, t, icon, loading, setLoading, setError }: {
  onNext: () => void; onSkip: () => void; t: (k: string, opts?: any) => string;
  icon: React.ReactNode; loading: boolean; setLoading: (v: boolean) => void; setError: (v: string) => void;
}) {
  const [current, setCurrent] = useState("");
  const [nextPw, setNextPw] = useState("");
  const [confirm, setConfirm] = useState("");

  const save = async () => {
    setError("");
    if (nextPw !== confirm) { setError(t("security.mismatch")); return; }
    if (nextPw.length < 8) { setError(t("security.tooShort", { count: 8 })); return; }
    setLoading(true);
    try {
      await api.setPassword(current, nextPw);
      onNext();
    } catch (e: any) {
      setError(e.message || t("security.failed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        {icon}
        <div>
          <h2 className="text-lg font-semibold">{t("security.title")}</h2>
          <p className="text-sm text-muted">{t("wizard.passwordHint")}</p>
        </div>
      </div>
      <div className="space-y-3">
        <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)}
          placeholder={t("security.current")} className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm" />
        <input type="password" value={nextPw} onChange={(e) => setNextPw(e.target.value)}
          placeholder={t("security.next")} className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm" />
        <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)}
          placeholder={t("security.confirm")} className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm" />
      </div>
      <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
        <button onClick={onSkip} className="text-sm text-muted hover:text-text transition-colors">
          {t("wizard.skip")}
        </button>
        <button onClick={save} disabled={loading || !current || !nextPw}
          className="flex items-center gap-1 bg-accent text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-accent/90 disabled:opacity-50 transition-colors">
          {loading ? t("ipv6.applying") : t("wizard.next")} <ChevronRight size={16} />
        </button>
      </div>
    </div>
  );
}

function WifiStep({ ifaces, onNext, onSkip, t, icon, loading, setLoading, setError }: {
  ifaces: WifiUI[]; onNext: () => void; onSkip: () => void;
  t: (k: string) => string; icon: React.ReactNode;
  loading: boolean; setLoading: (v: boolean) => void; setError: (v: string) => void;
}) {
  const mainIfaces = ifaces.filter((i) => !i.disabled || true);
  const [edits, setEdits] = useState<Record<string, { ssid: string; key: string }>>(() => {
    const e: Record<string, { ssid: string; key: string }> = {};
    for (const i of mainIfaces) e[i.section] = { ssid: i.ssid, key: "" };
    return e;
  });

  const save = async () => {
    setError("");
    setLoading(true);
    try {
      for (const [section, edit] of Object.entries(edits)) {
        const payload: any = { section };
        if (edit.ssid) payload.ssid = edit.ssid;
        if (edit.key) payload.key = edit.key;
        if (edit.ssid || edit.key) await api.setWifi(payload);
      }
      onNext();
    } catch (e: any) {
      setError(e.message || t("wifi.cancel"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        {icon}
        <div>
          <h2 className="text-lg font-semibold">{t("wifi.title")}</h2>
          <p className="text-sm text-muted">{t("wizard.wifiHint")}</p>
        </div>
      </div>
      <div className="space-y-4">
        {mainIfaces.map((iface) => (
          <div key={iface.section} className="space-y-2">
            <p className="text-sm font-medium">{iface.band === "2g" ? t("wifi.band24") : t("wifi.band5")}</p>
            <input
              value={edits[iface.section]?.ssid ?? iface.ssid}
              onChange={(e) => setEdits((p) => ({ ...p, [iface.section]: { ...p[iface.section], ssid: e.target.value } }))}
              placeholder={t("wifi.ssid")}
              className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm"
            />
            <input
              type="password"
              value={edits[iface.section]?.key ?? ""}
              onChange={(e) => setEdits((p) => ({ ...p, [iface.section]: { ...p[iface.section], key: e.target.value } }))}
              placeholder={t("wifi.key") + (iface.has_key ? " (keep current)" : "")}
              className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm"
            />
          </div>
        ))}
      </div>
      <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
        <button onClick={onSkip} className="text-sm text-muted hover:text-text transition-colors">
          {t("wizard.skip")}
        </button>
        <button onClick={save} disabled={loading}
          className="flex items-center gap-1 bg-accent text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-accent/90 disabled:opacity-50 transition-colors">
          {loading ? t("ipv6.applying") : t("wizard.next")} <ChevronRight size={16} />
        </button>
      </div>
    </div>
  );
}

function GuestStep({ guest, onNext, onSkip, t, icon, loading, setLoading, setError, setGuest: setGuestState }: {
  guest?: GuestProbe; onNext: () => void; onSkip: () => void;
  t: (k: string) => string; icon: React.ReactNode;
  loading: boolean; setLoading: (v: boolean) => void; setError: (v: string) => void;
  setGuest: (g: GuestProbe) => void;
}) {
  const [enabled, setEnabled] = useState(false);
  const [ssid, setSsid] = useState(guest?.ssid ?? "Guest");
  const [key, setKey] = useState("");

  const save = async () => {
    setError("");
    setLoading(true);
    try {
      const cfg: any = { enabled };
      if (enabled) { cfg.ssid = ssid; if (key) cfg.key = key; }
      const res = await api.setGuestwifi(cfg);
      setGuestState(res.state);
      onNext();
    } catch (e: any) {
      setError(e.message || t("guest.failed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        {icon}
        <div>
          <h2 className="text-lg font-semibold">{t("guest.title")}</h2>
          <p className="text-sm text-muted">{t("guest.scope")}</p>
        </div>
      </div>
      <label className="flex items-center gap-3 mb-4 cursor-pointer">
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)}
          className="w-5 h-5 rounded border-border accent-accent" />
        <span className="text-sm font-medium">{t("guest.toggle")}</span>
      </label>
      {enabled && (
        <div className="space-y-2">
          <input value={ssid} onChange={(e) => setSsid(e.target.value)}
            placeholder={t("wifi.ssid")} className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm" />
          <input type="password" value={key} onChange={(e) => setKey(e.target.value)}
            placeholder={t("wifi.key")} className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm" />
        </div>
      )}
      <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
        <button onClick={onSkip} className="text-sm text-muted hover:text-text transition-colors">
          {t("wizard.skip")}
        </button>
        <button onClick={save} disabled={loading}
          className="flex items-center gap-1 bg-accent text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-accent/90 disabled:opacity-50 transition-colors">
          {loading ? t("ipv6.applying") : t("wizard.next")} <ChevronRight size={16} />
        </button>
      </div>
    </div>
  );
}

function IoTStep({ iot, onNext, onSkip, t, icon, loading, setLoading, setError, setIot: setIotState }: {
  iot?: IoTProbe; onNext: () => void; onSkip: () => void;
  t: (k: string) => string; icon: React.ReactNode;
  loading: boolean; setLoading: (v: boolean) => void; setError: (v: string) => void;
  setIot: (i: IoTProbe) => void;
}) {
  const [enabled, setEnabled] = useState(false);
  const [ssid, setSsid] = useState(iot?.ssid ?? "IoT");
  const [key, setKey] = useState("");
  const [band, setBand] = useState("2g");

  const save = async () => {
    setError("");
    setLoading(true);
    try {
      const cfg: any = { enabled };
      if (enabled) { cfg.ssid = ssid; cfg.band = band; if (key) cfg.key = key; }
      const res = await api.setIotwifi(cfg);
      setIotState(res.state);
      onNext();
    } catch (e: any) {
      setError(e.message || t("iot.failed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        {icon}
        <div>
          <h2 className="text-lg font-semibold">{t("iot.title")}</h2>
          <p className="text-sm text-muted">{t("iot.banner")}</p>
        </div>
      </div>
      <label className="flex items-center gap-3 mb-4 cursor-pointer">
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)}
          className="w-5 h-5 rounded border-border accent-accent" />
        <span className="text-sm font-medium">{t("iot.toggle")}</span>
      </label>
      {enabled && (
        <div className="space-y-2">
          <input value={ssid} onChange={(e) => setSsid(e.target.value)}
            placeholder={t("wifi.ssid")} className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm" />
          <input type="password" value={key} onChange={(e) => setKey(e.target.value)}
            placeholder={t("iot.key")} className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm" />
          <select value={band} onChange={(e) => setBand(e.target.value)}
            className="w-full bg-bg border border-border rounded-lg px-3 py-2 text-sm">
            <option value="2g">{t("wifi.band24")}</option>
            <option value="5g">{t("wifi.band5")}</option>
            <option value="both">{t("iot.both")}</option>
          </select>
        </div>
      )}
      <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
        <button onClick={onSkip} className="text-sm text-muted hover:text-text transition-colors">
          {t("wizard.skip")}
        </button>
        <button onClick={save} disabled={loading}
          className="flex items-center gap-1 bg-accent text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-accent/90 disabled:opacity-50 transition-colors">
          {loading ? t("ipv6.applying") : t("wizard.next")} <ChevronRight size={16} />
        </button>
      </div>
    </div>
  );
}

function WGStep({ wg, onNext, onSkip, t, icon, loading, setLoading, setError, setWg: setWgState }: {
  wg?: WGProbe; onNext: () => void; onSkip: () => void;
  t: (k: string) => string; icon: React.ReactNode;
  loading: boolean; setLoading: (v: boolean) => void; setError: (v: string) => void;
  setWg: (w: WGProbe) => void;
}) {
  const [enabled, setEnabled] = useState(false);

  const save = async () => {
    setError("");
    if (!enabled) { onNext(); return; }
    setLoading(true);
    try {
      const res = await api.setWireguard("enable");
      setWgState(res.state);
      onNext();
    } catch (e: any) {
      setError(e.message || t("wg.failed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        {icon}
        <div>
          <h2 className="text-lg font-semibold">{t("wg.title")}</h2>
          <p className="text-sm text-muted">{t("wizard.wgHint")}</p>
        </div>
      </div>
      {!wg?.installed ? (
        <p className="text-sm text-muted bg-bg rounded-lg p-3">{t("wizard.wgNotInstalled")}</p>
      ) : (
        <label className="flex items-center gap-3 cursor-pointer">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)}
            className="w-5 h-5 rounded border-border accent-accent" />
          <span className="text-sm font-medium">{t("wizard.wgEnable")}</span>
        </label>
      )}
      <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
        <button onClick={onSkip} className="text-sm text-muted hover:text-text transition-colors">
          {t("wizard.skip")}
        </button>
        <button onClick={save} disabled={loading}
          className="flex items-center gap-1 bg-accent text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-accent/90 disabled:opacity-50 transition-colors">
          {loading ? t("ipv6.applying") : t("wizard.next")} <ChevronRight size={16} />
        </button>
      </div>
    </div>
  );
}

function DoneStep({ onFinish, t, icon }: { onFinish: () => void; t: (k: string) => string; icon: React.ReactNode }) {
  return (
    <div className="text-center">
      <div className="flex justify-center mb-4">{icon}</div>
      <h2 className="text-xl font-bold mb-2">{t("wizard.doneTitle")}</h2>
      <p className="text-muted mb-6">{t("wizard.doneBody")}</p>
      <button
        onClick={onFinish}
        className="w-full bg-accent text-white py-3 rounded-xl font-medium hover:bg-accent/90 transition-colors"
      >
        {t("wizard.goToPanel")}
      </button>
    </div>
  );
}
