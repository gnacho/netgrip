import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  CircleCheck, Cpu, Info, KeyRound, Lightbulb, Lock, Network, Router, Shield, Shuffle, Users, Wifi,
} from "lucide-react";
import { api } from "../../api";
import type { GuestProbe, IoTProbe, ModeProbe, ModuleResult, WGProbe, WifiUI } from "../../types";
import {
  ActionBanner, Banner, Button, ConfirmDialog, Field, Input, SegmentedControl, SettingRow,
} from "../ui";
import {
  IlluDevices, IlluFleet, IlluParty, IlluRouter, IlluShield, IlluWifiWaves,
} from "../ui/illustrations";
import { QrBox, useWifiQr } from "../wifi/qr";
import { useActionCycle } from "../wifi/action";
import {
  Reveal, StepFooter, StepShell, StrengthMeter, encLabel, genKey, type WizardRecord,
} from "./common";

/* ── 1. welcome ─────────────────────────────────────────────────────────── */

export function WelcomeStep({ onStart, onSkipAll }: { onStart: () => void; onSkipAll: () => void }) {
  const { t } = useTranslation();
  const [confirmOpen, setConfirmOpen] = useState(false);
  return (
    <>
      <StepShell
        illustration={<IlluRouter size={160} />}
        title={t("wizard.welcome.title")}
        body={t("wizard.welcome.body")}
        footer={
          <div className="mt-8">
            <Button className="w-full" onClick={onStart}>{t("wizard.start")}</Button>
            <div className="mt-4 text-center">
              <button
                type="button"
                onClick={() => setConfirmOpen(true)}
                className="text-small text-muted hover:text-text ring-focus rounded-sm transition-colors duration-[var(--dur-fast)]"
              >
                {t("wizard.skipAll")}
              </button>
            </div>
          </div>
        }
      />
      <ConfirmDialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={onSkipAll}
        title={t("wizard.skipAllTitle")}
        consequence={t("wizard.skipAllConsequence")}
        confirmLabel={t("wizard.skipAllConfirm")}
      />
    </>
  );
}

/* ── 2. mode (solo si el hardware ofrece elección) ──────────────────────── */

type ModeChoice = "router" | "ap" | "switch";

export function ModeStep({ mode, onApplied, onNext, onBack }: {
  mode: ModeProbe;
  onApplied: (m: ModeProbe, chosen: "router" | "ap") => void;
  onNext: () => void;
  onBack: () => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [choice, setChoice] = useState<ModeChoice>(
    mode.hardware_class === "switch" ? "switch" : mode.mode,
  );
  const [fails, setFails] = useState(0);

  /* Tono del sistema de acentos por opción (design-rev2 §1): router = accent
   * (interacción primaria), AP = teal (WiFi/red), switch = violet. */
  const options: { id: ModeChoice; icon: typeof Router; title: string; desc: string; tone: "accent" | "teal" | "violet" }[] = [
    { id: "router", icon: Router, title: t("wizard.mode.router"), desc: t("wizard.mode.routerDesc"), tone: "accent" },
    { id: "ap", icon: Wifi, title: t("wizard.mode.ap"), desc: t("wizard.mode.apDesc"), tone: "teal" },
    ...(mode.hardware_class === "switch"
      ? [{ id: "switch" as const, icon: Network, title: t("wizard.mode.switch"), desc: t("wizard.mode.switchDesc"), tone: "violet" as const }]
      : []),
  ];

  const SEL = {
    accent: { card: "border-accent bg-accent-soft", radio: "border-accent", dot: "bg-accent", icon: "text-accent" },
    teal: { card: "border-teal bg-teal-soft", radio: "border-teal", dot: "bg-teal", icon: "text-teal" },
    violet: { card: "border-violet bg-violet-soft", radio: "border-violet", dot: "bg-violet", icon: "text-violet" },
  } as const;

  const unchanged =
    choice === mode.mode || (choice === "switch" && mode.hardware_class === "switch");

  const save = async () => {
    if (unchanged) { onNext(); return; }
    if (choice === "switch") return; // la API solo expone router/ap
    const res = await run(() => api.setMode(choice));
    if (res?.status === "applied") {
      onApplied(res.state, choice);
      onNext();
    } else {
      setFails((f) => f + 1);
    }
  };

  return (
    <StepShell
      illustration={<IlluFleet size={140} />}
      title={t("wizard.mode.title")}
      footer={
        <>
          {phase && (
            <div className="mt-4">
              <ActionBanner phase={phase} detail={detail} onDone={clear} />
            </div>
          )}
          <StepFooter onBack={onBack} onNext={save} busy={busy} fails={fails}
            onSkip={fails >= 2 ? onNext : undefined} />
        </>
      }
    >
      <div className="space-y-3" role="radiogroup" aria-label={t("wizard.mode.title")}>
        {options.map((o) => {
          const selected = choice === o.id;
          const tone = SEL[o.tone];
          return (
            <button
              key={o.id}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => setChoice(o.id)}
              disabled={busy}
              className={`flex w-full items-start gap-3 rounded-lg border p-4 text-left ring-focus
                transition-colors duration-[var(--dur-fast)] disabled:opacity-60
                ${selected ? tone.card : "border-border bg-surface hover:bg-surface-2"}`}
            >
              <span
                aria-hidden="true"
                className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-2 transition-colors duration-[var(--dur-fast)]
                  ${selected ? tone.radio : "border-border-strong"}`}
              >
                {selected && <span className={`h-2.5 w-2.5 rounded-full ${tone.dot}`} />}
              </span>
              <span className="min-w-0">
                <span className="flex items-center gap-2 text-body font-medium">
                  <o.icon size={16} className={selected ? tone.icon : "text-faint"} aria-hidden="true" />
                  {o.title}
                </span>
                <span className="mt-0.5 block text-small text-muted">{o.desc}</span>
              </span>
            </button>
          );
        })}
      </div>
    </StepShell>
  );
}

/* ── 3. password ────────────────────────────────────────────────────────── */

export function PasswordStep({ onDone, onSkip, onBack }: {
  onDone: () => void;
  onSkip: () => void;
  onBack: () => void;
}) {
  const { t } = useTranslation();
  const [current, setCurrent] = useState("");
  const [nextPw, setNextPw] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<{ next?: string; confirm?: string }>({});
  const [fatal, setFatal] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [fails, setFails] = useState(0);

  const save = async () => {
    const errs: { next?: string; confirm?: string } = {};
    if (nextPw.length < 8) errs.next = t("security.tooShort", { count: 8 });
    else if (nextPw !== confirm) errs.confirm = t("security.mismatch");
    setErrors(errs);
    if (errs.next || errs.confirm) return;
    setBusy(true);
    setFatal(undefined);
    try {
      await api.setPassword(current, nextPw);
      onDone();
    } catch (e) {
      setFatal(e instanceof Error ? e.message : t("security.failed"));
      setFails((f) => f + 1);
    } finally {
      setBusy(false);
    }
  };

  return (
    <StepShell
      illustration={<IlluShield size={96} />}
      title={t("wizard.password.title")}
      body={t("wizard.password.body")}
      footer={
        <>
          {fatal && <Banner tone="danger" className="mt-4">{fatal}</Banner>}
          <StepFooter
            onBack={onBack}
            onNext={save}
            busy={busy}
            nextDisabled={!current || !nextPw || !confirm}
            onSkip={onSkip}
            fails={fails}
          />
        </>
      }
    >
      <div className="space-y-4">
        <Field
          label={t("security.current")}
          icon={Lock}
          inputProps={{
            type: "password",
            value: current,
            onChange: (e) => setCurrent(e.target.value),
            autoComplete: "current-password",
            disabled: busy,
          }}
        />
        <Field label={t("security.next")} error={errors.next}>
          <Input
            type="password"
            icon={Lock}
            value={nextPw}
            error={!!errors.next}
            onChange={(e) => setNextPw(e.target.value)}
            autoComplete="new-password"
            disabled={busy}
          />
          <StrengthMeter password={nextPw} />
        </Field>
        <Field
          label={t("security.confirm")}
          icon={Lock}
          error={errors.confirm}
          inputProps={{
            type: "password",
            value: confirm,
            onChange: (e) => setConfirm(e.target.value),
            autoComplete: "new-password",
            disabled: busy,
          }}
        />
      </div>
    </StepShell>
  );
}

/* ── 4. wifi (misma clave en todas las radios principales) ──────────────── */

export function WifiStep({ ifaces, guest, iot, onDone, onSkip, onBack }: {
  ifaces: WifiUI[];
  guest?: GuestProbe;
  iot?: IoTProbe;
  onDone: (ssid: string, enc: string) => void;
  onSkip: () => void;
  onBack: () => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();

  // Redes principales = las que no son de invitados ni IoT (mismo criterio que Wifi.tsx).
  const main = useMemo(() => {
    const secondary = [...(guest?.ifaces ?? []), ...(iot?.ifaces ?? [])];
    const secondarySsids = [guest?.ssid, iot?.ssid].filter(Boolean);
    const m = ifaces.filter((i) => !secondary.includes(i.ifname) && !secondarySsids.includes(i.ssid));
    return m.length > 0 ? m : ifaces;
  }, [ifaces, guest, iot]);

  const [ssid, setSsid] = useState(main[0]?.ssid ?? "");
  const [key, setKey] = useState("");
  const [keyError, setKeyError] = useState<string>();
  const [fails, setFails] = useState(0);

  const keepKey = main.length > 0 && main.every((i) => i.has_key);
  const enc = main[0]?.encryption ?? "sae-mixed";
  const inUse = main.some((i) => i.ssid);
  const qr = useWifiQr(ssid, key, enc === "none" ? "none" : "sae-mixed", 96);

  const save = async () => {
    setKeyError(undefined);
    if (main.length === 0) { onSkip(); return; }
    if (key && key.length < 8) {
      setKeyError(t("security.tooShort", { count: 8 }));
      return;
    }
    if (!key && !keepKey) {
      setKeyError(t("security.tooShort", { count: 8 }));
      return;
    }
    const cleanSsid = ssid.trim();
    const res = await run(async () => {
      let last;
      for (const i of main) {
        const r = await api.setWifi({ section: i.section, ssid: cleanSsid, ...(key ? { key } : {}) });
        if (r.status !== "applied") return r;
        last = r;
      }
      return last!;
    });
    if (res?.status === "applied") {
      onDone(cleanSsid, encLabel(enc));
    } else {
      setFails((f) => f + 1);
    }
  };

  return (
    <StepShell
      illustration={<IlluWifiWaves size={120} />}
      title={t("wizard.wifi.title")}
      body={t("wizard.wifi.body")}
      footer={
        <>
          {phase && (
            <div className="mt-4">
              <ActionBanner phase={phase} detail={detail} onDone={clear} />
            </div>
          )}
          <StepFooter
            onBack={onBack}
            onNext={save}
            busy={busy}
            nextDisabled={!ssid.trim()}
            onSkip={onSkip}
            fails={fails}
          />
        </>
      }
    >
      <div className="space-y-4">
        {inUse && <Banner tone="warn">{t("wizard.wifi.consequence")}</Banner>}
        <Field
          label={t("wifi.ssid")}
          icon={Wifi}
          inputProps={{
            value: ssid,
            onChange: (e) => setSsid(e.target.value),
            disabled: busy || main.length === 0,
            maxLength: 32,
          }}
        />
        <div>
          <Field
            label={t("wifi.key")}
            icon={KeyRound}
            error={keyError}
            hint={keyError ? undefined : keepKey ? t("wizard.wifi.keepKey") : undefined}
            inputProps={{
              type: "password",
              value: key,
              onChange: (e) => setKey(e.target.value),
              disabled: busy || main.length === 0,
              autoComplete: "off",
            }}
          />
          <Button
            variant="ghost"
            size="sm"
            icon={Shuffle}
            className="mt-1.5"
            onClick={() => setKey(genKey())}
            disabled={busy || main.length === 0}
          >
            {t("wifi.generateKey")}
          </Button>
        </div>
        {qr && (
          <div className="flex items-center gap-4 rounded-lg border border-border bg-surface-2 p-4 animate-fade-up">
            <QrBox data={qr} size={96} alt={ssid} />
            <p className="text-small text-muted">{t("wizard.wifi.qrCaption")}</p>
          </div>
        )}
      </div>
    </StepShell>
  );
}

/* ── 5/6. guest + iot (red extra con toggle) ────────────────────────────── */

export function ExtraNetStep({ kind, probe, mainSsid, onApplied, onSkip, onBack }: {
  kind: "guest" | "iot";
  probe?: GuestProbe | IoTProbe;
  mainSsid?: string;
  onApplied: (state: GuestProbe | IoTProbe, ssid?: string) => void;
  onSkip: () => void;
  onBack: () => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [enabled, setEnabled] = useState(probe?.active ?? false);
  const [ssid, setSsid] = useState(
    () => probe?.ssid || (mainSsid ? mainSsid + t(`wizard.${kind}.ssidSuffix`) : ""),
  );
  const [key, setKey] = useState("");
  const [band, setBand] = useState<string>(("band" in (probe ?? {}) ? (probe as IoTProbe).band : "") || "2g");
  const [keyError, setKeyError] = useState<string>();
  const [fails, setFails] = useState(0);

  const wasActive = probe?.active ?? false;

  const save = async () => {
    setKeyError(undefined);
    if (!enabled && !wasActive) { onSkip(); return; } // nada que cambiar
    if (enabled && key && key.length < 8) {
      setKeyError(t("security.tooShort", { count: 8 }));
      return;
    }
    const cleanSsid = ssid.trim();
    const res = await run((): Promise<ModuleResult<GuestProbe | IoTProbe>> =>
      kind === "guest"
        ? api.setGuestwifi({ enabled, ...(enabled ? { ssid: cleanSsid, ...(key ? { key } : {}) } : {}) })
        : api.setIotwifi({ enabled, ...(enabled ? { ssid: cleanSsid, band, ...(key ? { key } : {}) } : {}) }));
    if (res?.status === "applied") {
      onApplied(res.state, enabled ? cleanSsid : undefined);
    } else {
      setFails((f) => f + 1);
    }
  };

  return (
    <StepShell
      illustration={<IlluDevices size={160} />}
      title={t(`wizard.${kind}.title`)}
      body={t(`wizard.${kind}.body`)}
      footer={
        <>
          {phase && (
            <div className="mt-4">
              <ActionBanner phase={phase} detail={detail} onDone={clear} />
            </div>
          )}
          <StepFooter
            onBack={onBack}
            onNext={save}
            busy={busy}
            nextDisabled={enabled && !ssid.trim()}
            onSkip={onSkip}
            fails={fails}
          />
        </>
      }
    >
      <div className="rounded-lg border border-border bg-surface p-4">
        <SettingRow
          icon={kind === "guest" ? Users : Lightbulb}
          title={t(`wizard.${kind}.toggle`)}
          description={t(`wizard.${kind}.toggleDesc`)}
          checked={enabled}
          busy={busy}
          onChange={setEnabled}
        />
        <Reveal open={enabled}>
          <div className="space-y-4 pt-2 pb-1">
            <Field
              label={t("wifi.ssid")}
              icon={kind === "guest" ? Users : Cpu}
              inputProps={{
                value: ssid,
                onChange: (e) => setSsid(e.target.value),
                disabled: busy,
                maxLength: 32,
              }}
            />
            <div>
              <Field
                label={t("wifi.key")}
                icon={KeyRound}
                error={keyError}
                inputProps={{
                  type: "password",
                  value: key,
                  onChange: (e) => setKey(e.target.value),
                  disabled: busy,
                  autoComplete: "off",
                }}
              />
              <Button
                variant="ghost"
                size="sm"
                icon={Shuffle}
                className="mt-1.5"
                onClick={() => setKey(genKey())}
                disabled={busy}
              >
                {t("wifi.generateKey")}
              </Button>
            </div>
            {kind === "iot" && (
              <SegmentedControl
                ariaLabel={t("iot.title")}
                value={band}
                onChange={setBand}
                options={[
                  { value: "2g", label: t("wifi.band24") },
                  { value: "5g", label: t("wifi.band5") },
                  { value: "both", label: t("iot.both") },
                ]}
              />
            )}
          </div>
        </Reveal>
      </div>
    </StepShell>
  );
}

/* ── 7. wireguard (solo si está instalado) ──────────────────────────────── */

export function WireguardStep({ wg, onApplied, onSkip, onBack }: {
  wg: WGProbe;
  onApplied: (state: WGProbe) => void;
  onSkip: () => void;
  onBack: () => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [enabled, setEnabled] = useState(wg.active);
  const [fails, setFails] = useState(0);

  const save = async () => {
    if (!enabled || wg.active) { onSkip(); return; } // nada que cambiar / nunca desactivamos
    const res = await run(() => api.setWireguard("enable"));
    if (res?.status === "applied") {
      onApplied(res.state);
    } else {
      setFails((f) => f + 1);
    }
  };

  return (
    <StepShell
      illustration={<IlluShield size={96} />}
      title={t("wizard.wg.title")}
      body={t("wizard.wg.body")}
      footer={
        <>
          {phase && (
            <div className="mt-4">
              <ActionBanner phase={phase} detail={detail} onDone={clear} />
            </div>
          )}
          <StepFooter onBack={onBack} onNext={save} busy={busy} onSkip={onSkip} fails={fails} />
        </>
      }
    >
      <div className="rounded-lg border border-border bg-surface p-4">
        <SettingRow
          icon={Shield}
          title={t("wizard.wg.toggle")}
          description={t("wizard.wg.toggleDesc")}
          checked={enabled}
          busy={busy}
          onChange={setEnabled}
        />
        <Reveal open={enabled && !wg.active}>
          <p className="flex items-start gap-2 pt-1 pb-1 text-small text-muted">
            <Info size={16} className="mt-0.5 shrink-0 text-accent" aria-hidden="true" />
            {t("wizard.wg.after")}
          </p>
        </Reveal>
      </div>
    </StepShell>
  );
}

/* ── 8. done ────────────────────────────────────────────────────────────── */

export function DoneStep({ record, onFinish }: { record: WizardRecord; onFinish: () => void }) {
  const { t } = useTranslation();
  const rows: string[] = [];
  if (record.mode) {
    rows.push(t(record.mode === "router" ? "wizard.summary.modeRouter" : "wizard.summary.modeAp"));
  }
  if (record.password) rows.push(t("wizard.summary.password"));
  if (record.wifi) rows.push(t("wizard.summary.wifi", { ssid: record.wifi.ssid, enc: record.wifi.enc }));
  if (record.guest) rows.push(t("wizard.summary.guest", { ssid: record.guest }));
  if (record.iot) rows.push(t("wizard.summary.iot", { ssid: record.iot }));
  if (record.wg) rows.push(t("wizard.summary.wg"));

  return (
    <StepShell
      illustration={<IlluParty size={120} />}
      title={t("wizard.done.title")}
      body={rows.length > 0 ? t("wizard.done.body") : t("wizard.doneBody")}
      footer={
        <div className="mt-8">
          <Banner tone="info">{t("wizard.done.backup")}</Banner>
          <Button className="w-full mt-5" onClick={onFinish}>{t("wizard.done.go")}</Button>
        </div>
      }
    >
      {rows.length > 0 && (
        <ul className="mx-auto max-w-[420px] space-y-2.5 rounded-lg border border-border bg-surface p-4">
          {rows.map((r) => (
            <li key={r} className="flex items-start gap-2.5 text-body">
              <CircleCheck size={18} className="mt-0.5 shrink-0 text-ok" aria-hidden="true" />
              <span>{r}</span>
            </li>
          ))}
        </ul>
      )}
    </StepShell>
  );
}
