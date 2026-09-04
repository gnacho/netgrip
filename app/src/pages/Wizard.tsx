import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check } from "lucide-react";
import { api } from "../api";
import type {
  GuestProbe, IoTProbe, ModeProbe, OptionalPackage, WGProbe, WifiUI, WizardState,
  WizardSetupProbe,
} from "../types";
import { Banner, Button, SkeletonRows, Stepper } from "../components/ui";
import { Logo } from "../components/ui/illustrations";
import type { WizardRecord } from "../components/wizard/common";
import {
  DoneStep, ExtraNetStep, ModeStep, PackagesStep, PasswordStep, SetupDependenciesStep, WelcomeStep, WifiStep, WireguardStep,
} from "../components/wizard/steps";

type Step = "welcome" | "setup" | "mode" | "password" | "wifi" | "guest" | "iot" | "wireguard" | "packages" | "done";

const ALL_STEPS: Step[] = ["welcome", "setup", "mode", "password", "wifi", "guest", "iot", "wireguard", "packages", "done"];

/**
 * Wizard de primer arranque (wizard.md): setup guiado estilo GL.iNet, una
 * idea por pantalla, panel lateral de progreso en desktop y barra fina en
 * móvil. Misma API de siempre (wizardState, mode, password, wifi, guestwifi,
 * iotwifi, wireguard, wizardComplete) y misma firma `Wizard({ onDone })`.
 */
export function Wizard({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const [wstate, setWstate] = useState<WizardState>();
  const [mode, setMode] = useState<ModeProbe>();
  const [wifiIfaces, setWifiIfaces] = useState<WifiUI[]>([]);
  const [guest, setGuest] = useState<GuestProbe>();
  const [iot, setIot] = useState<IoTProbe>();
  const [wg, setWg] = useState<WGProbe>();
  const [optPkgs, setOptPkgs] = useState<OptionalPackage[]>();
  const [setup, setSetup] = useState<WizardSetupProbe>();
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [step, setStep] = useState<Step>("welcome");
  const [leaving, setLeaving] = useState(false);
  const [record, setRecord] = useState<WizardRecord>({});
  const timer = useRef<number | undefined>(undefined);

  const load = useCallback(async () => {
    setLoadError(false);
    try {
      // api.mode() es opcional: si el backend no lo expone, el wizard sigue.
      const [ws, mp, wf, g, i, w, op, sp] = await Promise.all([
        api.wizardState(),
        api.mode().catch(() => undefined),
        api.wifi(),
        api.guestwifi(),
        api.iotwifi(),
        api.wireguard(),
        api.optionalPackages().catch(() => undefined),
        api.wizardSetup().catch(() => undefined),
      ]);
      setWstate(ws);
      setMode(mp);
      setWifiIfaces(wf.interfaces);
      setGuest(g);
      setIot(i);
      setWg(w);
      setOptPkgs(op?.packages);
      setSetup(sp);
      setLoaded(true);
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => {
    load();
    return () => window.clearTimeout(timer.current);
  }, [load]);

  const finish = useCallback(async () => {
    await api.wizardComplete().catch(() => {});
    onDone();
  }, [onDone]);

  // Lógica condicional por hardware (wizard.md §1): el paso de modo solo si
  // hay elección real; WiFi si hay radios; invitados/IoT solo en gateway;
  // WireGuard solo si está instalado.
  const isGateway = (mode?.mode ?? wstate?.mode) === "router";
  const hasWifi = mode ? mode.has_wifi : wifiIfaces.length > 0;
  const wgInstalled = !!wg?.installed;
  const steps: Step[] = useMemo(
    () =>
      ALL_STEPS.filter((s) => {
        switch (s) {
          case "mode":
            return !!mode && mode.hardware_class === "router";
          case "wifi":
            return hasWifi;
          case "guest":
          case "iot":
            return isGateway;
          case "wireguard":
            return wgInstalled;
          case "packages":
            return !!optPkgs && optPkgs.length > 0;
          default:
            return true;
        }
      }),
    [mode, hasWifi, isGateway, wgInstalled, optPkgs],
  );

  const currentIdx = Math.max(0, steps.indexOf(step));

  // El paso setup siempre aparece en este mockup para poder visualizarlo.

  // Transición entre pasos (wizard.md §2): saliente fade 120ms, entrante
  // fade-up (secuencia, no simultáneos).
  const go = useCallback((target: Step) => {
    setLeaving(true);
    timer.current = window.setTimeout(() => {
      setStep(target);
      setLeaving(false);
    }, 120);
  }, []);

  const next = useCallback(() => {
    const idx = steps.indexOf(step);
    if (idx >= 0 && idx < steps.length - 1) go(steps[idx + 1]);
    else finish();
  }, [steps, step, go, finish]);

  const prev = useCallback(() => {
    const idx = steps.indexOf(step);
    if (idx > 0) go(steps[idx - 1]);
  }, [steps, step, go]);

  const renderStep = () => {
    switch (step) {
      case "welcome":
        return <WelcomeStep onStart={next} onSkipAll={finish} />;
      case "setup":
        return setup ? (
          <SetupDependenciesStep
            probe={setup}
            onBack={prev}
            onDone={() => {
              setSetup((s) => s ? { ...s, groups: s.groups.map((g) => ({ ...g, packages: [] })) } : s);
              next();
            }}
            onSkip={next}
          />
        ) : null;
      case "mode":
        return mode ? (
          <ModeStep
            mode={mode}
            onApplied={(m, chosen) => {
              setMode(m);
              setRecord((r) => ({ ...r, mode: chosen }));
            }}
            onNext={next}
            onBack={prev}
          />
        ) : null;
      case "password":
        return (
          <PasswordStep
            onDone={() => {
              setRecord((r) => ({ ...r, password: true }));
              next();
            }}
            onSkip={next}
            onBack={prev}
          />
        );
      case "wifi":
        return (
          <WifiStep
            ifaces={wifiIfaces}
            guest={guest}
            iot={iot}
            onDone={(ssid, enc) => {
              setRecord((r) => ({ ...r, wifi: { ssid, enc } }));
              next();
            }}
            onSkip={next}
            onBack={prev}
          />
        );
      case "guest":
        return (
          <ExtraNetStep
            kind="guest"
            probe={guest}
            mainSsid={record.wifi?.ssid ?? wifiIfaces[0]?.ssid}
            onApplied={(state, ssid) => {
              setGuest(state as GuestProbe);
              setRecord((r) => (ssid ? { ...r, guest: ssid } : r));
              next();
            }}
            onSkip={next}
            onBack={prev}
          />
        );
      case "iot":
        return (
          <ExtraNetStep
            kind="iot"
            probe={iot}
            mainSsid={record.wifi?.ssid ?? wifiIfaces[0]?.ssid}
            onApplied={(state, ssid) => {
              setIot(state as IoTProbe);
              setRecord((r) => (ssid ? { ...r, iot: ssid } : r));
              next();
            }}
            onSkip={next}
            onBack={prev}
          />
        );
      case "wireguard":
        return wg ? (
          <WireguardStep
            wg={wg}
            onApplied={(state) => {
              setWg(state);
              setRecord((r) => ({ ...r, wg: true }));
              next();
            }}
            onSkip={next}
            onBack={prev}
          />
        ) : null;
      case "packages":
        return optPkgs ? (
          <PackagesStep
            items={optPkgs}
            onSaved={(ids) => {
              setOptPkgs((prev) =>
                prev?.map((p) => (ids.includes(p.id) ? { ...p, installed: true } : p)),
              );
              setRecord((r) => (ids.length > 0 ? { ...r, pkgs: ids } : r));
              next();
            }}
            onSkip={next}
            onBack={prev}
          />
        ) : null;
      case "done":
        return <DoneStep record={record} onFinish={finish} />;
    }
  };

  return (
    <main className="min-h-screen bg-bg lg:flex">
      {/* panel lateral de progreso (desktop ≥1024px, wizard.md §1/§2) */}
      <aside className="hidden lg:flex w-[280px] shrink-0 flex-col border-r border-border bg-surface px-6 py-8 sticky top-0 h-screen overflow-y-auto">
        <div className="flex items-center gap-2.5 text-accent">
          <Logo size={32} />
          <span className="text-h2 text-text">NetGrip</span>
        </div>
        {loaded && !loadError ? (
          <ol className="mt-8">
            {steps.map((s, i) => {
              const done = i < currentIdx;
              const active = i === currentIdx;
              return (
                <li key={s} className="relative flex gap-3 pb-6 last:pb-0">
                  {i < steps.length - 1 && (
                    <span
                      aria-hidden="true"
                      className={`absolute left-[13px] top-7 bottom-0 w-0.5 transition-colors duration-[var(--dur-slow)] ${
                        done ? "bg-ok" : "bg-border"
                      }`}
                    />
                  )}
                  <span
                    aria-hidden="true"
                    className={`relative z-10 flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition-colors duration-200
                      ${done
                        ? "bg-ok-soft text-ok"
                        : active
                          ? "bg-accent-soft text-accent ring-2 ring-accent/30"
                          : "border border-border-strong bg-surface text-faint"}`}
                  >
                    {done ? (
                      <Check size={14} className="animate-banner-in" />
                    ) : (
                      <span className={`h-2 w-2 rounded-full ${active ? "bg-accent" : "bg-border-strong"}`} />
                    )}
                  </span>
                  <span
                    className={`pt-1 text-small ${
                      active ? "text-text font-medium" : done ? "text-muted" : "text-faint"
                    }`}
                  >
                    {t(`wizard.step.${s}`)}
                  </span>
                </li>
              );
            })}
          </ol>
        ) : (
          <div className="mt-10 flex-1">
            <SkeletonRows rows={5} />
          </div>
        )}
        <p className="text-caption text-muted">
          {t("wizard.stepOf", { step: currentIdx + 1, total: steps.length })}
        </p>
      </aside>

      <div className="flex min-h-screen flex-1 flex-col">
        {/* stepper horizontal superior (móvil, §6.14) */}
        <div className="lg:hidden sticky top-0 z-10 border-b border-border bg-bg px-4 py-3">
          <Stepper step={currentIdx + 1} total={steps.length} />
        </div>

        <div className="flex flex-1 items-start lg:items-center justify-center p-5 md:justify-start md:p-8">
          <div
            key={step}
            className={`w-full max-w-[560px] ${
              leaving ? "opacity-0 transition-opacity duration-100" : "animate-fade-up"
            }`}
          >
            {loadError ? (
              <div className="space-y-4">
                <Banner tone="danger">{t("error.network")}</Banner>
                <div className="text-center">
                  <Button variant="secondary" onClick={load}>{t("common.retryNow")}</Button>
                </div>
              </div>
            ) : !loaded ? (
              <SkeletonRows rows={4} />
            ) : (
              renderStep()
            )}
          </div>
        </div>
      </div>
    </main>
  );
}
