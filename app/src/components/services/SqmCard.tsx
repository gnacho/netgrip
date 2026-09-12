import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Clapperboard, ExternalLink, Gamepad2, Gauge, Play, Scale, SlidersHorizontal } from "lucide-react";
import { api } from "../../api";
import type { BufferbloatResult, SQMProbe } from "../../types";
import {
  ActionBanner, Button, Card, Field, KeyValue, Pill, SegmentedControl, SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { TechName } from "./shared";

const SQM_PROFILES: { value: string; label: string; icon: typeof Scale }[] = [
  { value: "balanced", label: "sqm.profileBalanced", icon: Scale },
  { value: "gaming", label: "sqm.profileGaming", icon: Gamepad2 },
  { value: "streaming", label: "sqm.profileStreaming", icon: Clapperboard },
  { value: "custom", label: "sqm.profileCustom", icon: SlidersHorizontal },
];

function GradePill({ grade }: { grade: string }) {
  const tone = grade === "A" || grade === "B" ? "ok" : grade === "C" ? "warn" : "danger";
  return <Pill tone={tone}>{grade}</Pill>;
}

function Sparkline({ values }: { values: number[] }) {
  if (values.length < 2) return null;
  const w = 200;
  const h = 32;
  const pad = 2;
  const max = Math.max(...values, 1);
  const step = (w - pad * 2) / (values.length - 1);
  const points = values.map((v, i) => `${pad + i * step},${h - pad - (v / max) * (h - pad * 2)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-8 mt-1" preserveAspectRatio="none">
      <polyline points={points} fill="none" stroke="currentColor" strokeWidth="1.5" className="text-accent" />
    </svg>
  );
}

/**
 * Prioridad de tráfico (SQM/CAKE) — "Que Internet no se atasque"
 * (services.md §7). La jerga vive solo en el caption y en el HelpTip.
 * Sin WAN la card queda atenuada ("Solo en modo router").
 */
export function SqmCard({ probe, onChange, index = 0 }: {
  probe: SQMProbe | undefined;
  onChange: (p: SQMProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [download, setDownload] = useState("");
  const [upload, setUpload] = useState("");
  const [profile, setProfile] = useState<string>("balanced");
  const [latest, setLatest] = useState<BufferbloatResult>();
  const [history, setHistory] = useState<BufferbloatResult[]>([]);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    if (probe) {
      setDownload(probe.download || "");
      setUpload(probe.upload || "");
      setProfile(probe.profile || "balanced");
    }
  }, [probe]);

  useEffect(() => {
    api.bufferbloatHistory().then((r) => {
      const entries = r.entries ?? [];
      setHistory(entries);
      if (entries.length > 0) setLatest(entries[entries.length - 1]);
    }).catch(() => {});
  }, []);

  const runTest = async () => {
    setTesting(true);
    try {
      const result = await api.runBufferbloatTest();
      setLatest(result);
      setHistory((prev) => [...prev, result].slice(-50));
    } catch {
      setDoneMsg(t("sqm.testFailed"));
    } finally {
      setTesting(false);
    }
  };

  const active = probe?.active ?? false;
  const hasWan = probe?.has_wan ?? false;
  const ratesOk = Number(download) > 0 && Number(upload) > 0;
  const luciSqmUrl = `http://${window.location.hostname}/cgi-bin/luci/admin/network/sqm`;

  const profileName = (p: string) => t(SQM_PROFILES.find((o) => o.value === p)?.label ?? "sqm.profileBalanced");
  const profileDesc = (p: string) => t(`${SQM_PROFILES.find((o) => o.value === p)?.label ?? "sqm.profileBalanced"}Desc`);

  const apply = async (enabled: boolean, nextProfile?: string) => {
    setDoneMsg(undefined);
    const prof = nextProfile ?? profile;
    const res = await run(() => api.setSqm(
      enabled ? { enabled, download, upload, profile: prof } : { enabled },
    ));
    if (res) {
      onChange(res.state);
      if (res.status === "applied") {
        setDoneMsg(enabled ? t("sqm.doneOn") : t("sqm.doneOff"));
      } else {
        // Rollback: the probe may keep the same object identity (demo), so
        // resync the selector from the real state.
        setProfile(res.state.profile || "balanced");
      }
    } else {
      onChange(await api.sqm());
    }
  };

  const handleProfileChange = (value: string) => {
    if (busy) return;
    setProfile(value);
    if (active) apply(true, value);
  };

  return (
    <Card index={index} className={probe && !hasWan ? "opacity-70" : ""}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <SettingRow
            icon={Gauge}
            iconTone="accent"
            title={t("sqm.cardTitle")}
            description={t("sqm.desc")}
            help={t("help.sqm.body")}
            helpTitle={t("help.sqm.title")}
            checked={active}
            busy={busy}
            disabled={!hasWan || (!active && !ratesOk)}
            disabledReason={!hasWan ? t("sqm.onlyRouter") : undefined}
            onChange={apply}
            control={
              <span className="flex items-center gap-2">
                <Pill className="max-w-24 sm:max-w-32" tone={probe.running ? "ok" : active ? "warn" : "muted"}>
                  {probe.running ? t("sqm.running") : active ? t("sqm.configured") : t("sqm.off")}
                </Pill>
                <Toggle checked={active} busy={busy} disabled={!hasWan || (!active && !ratesOk)}
                  onChange={apply} label={t("sqm.cardTitle")} />
              </span>
            }
          />
          <TechName>{t("sqm.tech")}</TechName>

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          {hasWan && (
            // Los campos de tasas también se muestran apagado: hacen falta
            // para poder activar (el toggle exige tasas válidas).
            <div className="pt-2 flex flex-col gap-3">
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-eyebrow text-faint">{t("sqm.profileLabel")}</span>
                  <span className="text-caption text-muted">{profileName(profile)}</span>
                </div>
                <SegmentedControl
                  options={SQM_PROFILES.map((o) => ({
                    value: o.value,
                    label: <o.icon size={16} aria-hidden="true" />,
                    title: t(o.label),
                  }))}
                  value={profile}
                  onChange={handleProfileChange}
                  ariaLabel={t("sqm.profileLabel")}
                />
                <p className="text-caption text-muted mt-1">{profileDesc(profile)}</p>
              </div>

              <div>
                <span className="text-eyebrow text-faint">{t("sqm.statusTitle")}</span>
                <KeyValue className="mt-1" items={[
                  { label: t("sqm.queue"), value: probe.qdisc && probe.script ? `${probe.qdisc} · ${probe.script}` : "-", mono: true },
                  { label: t("sqm.profileLabel"), value: profileName(probe.profile || "balanced") },
                ]} />
                <a
                  href={luciSqmUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-small text-accent hover:text-accent-hover ring-focus rounded-sm"
                >
                  {t("sqm.luciLink")}
                  <ExternalLink size={12} aria-hidden="true" />
                </a>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <Field label={t("sqm.download")} mono
                  inputProps={{ value: download, onChange: (e) => setDownload(e.target.value), inputMode: "numeric", disabled: active }} />
                <Field label={t("sqm.upload")} mono
                  inputProps={{ value: upload, onChange: (e) => setUpload(e.target.value), inputMode: "numeric", disabled: active }} />
              </div>
              <div>
                <p className="text-caption text-muted">{t("sqm.rateHelp")}</p>
                <p className="text-caption text-faint mt-0.5 font-mono">{t("sqm.rateExample")}</p>
                <p className="text-caption text-muted mt-1">
                  {t("sqm.bandwidthHint")}{" "}
                  <a href="https://fast.com" target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-0.5 text-accent hover:text-accent-hover">
                    {t("sqm.speedTest")}
                    <ExternalLink size={12} aria-hidden="true" />
                  </a>
                </p>
              </div>
              <div className="pt-2 border-t border-border/60">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-eyebrow text-faint">{t("sqm.bufferbloat")}</span>
                  {latest ? <GradePill grade={latest.grade} /> : <span className="text-caption text-muted">{t("sqm.notTested")}</span>}
                </div>
                {latest && (
                  <div className="grid grid-cols-3 gap-2 text-caption mb-1.5">
                    <div><span className="text-muted">{t("sqm.baseline")}:</span> {latest.baseline_ms} ms</div>
                    <div><span className="text-muted">{t("sqm.loaded")}:</span> {latest.loaded_ms} ms</div>
                    <div><span className="text-muted">{t("sqm.delta")}:</span> {latest.delta_ms} ms</div>
                  </div>
                )}
                <Button variant="secondary" size="sm" onClick={runTest} loading={testing} icon={Play}>
                  {testing ? t("sqm.testing") : t("sqm.runTest")}
                </Button>
                {history.length >= 2 && (
                  <div className="mt-2">
                    <span className="text-[10px] text-muted">{t("sqm.lastTests")}</span>
                    <Sparkline values={history.slice(-10).map((h) => h.delta_ms)} />
                  </div>
                )}
              </div>
            </div>
          )}
        </>
      )}
    </Card>
  );
}
