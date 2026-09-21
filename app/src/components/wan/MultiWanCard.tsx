import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Split } from "lucide-react";
import { api } from "../../api";
import type { MultiWanProbe, MultiWanRequest, WanCandidate } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Banner, Button, Card, ConfirmDialog,
  Field, Pill, SegmentedControl, SkeletonRows, StatusDot,
} from "../ui";
import type { PillTone } from "../ui";
import { useActionCycle } from "../wifi/action";
import { InstallProgress, useInstallJob } from "../wizard/common";

const MODES = ["off", "failover", "balance"] as const;
type Mode = (typeof MODES)[number];

/** How long a link has been up, in the same shape the WAN card uses. */
function fmtDur(s: number): string {
  if (!s || s <= 0) return "—";
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

/** The dot on the left: green carries traffic, amber is a healthy standby,
 *  red is down. A link that is up but routing nothing is not a fault. */
function dotTone(c: WanCandidate): "ok" | "warn" | "danger" {
  if (!c.up || c.online === "offline") return "danger";
  return c.active ? "ok" : "warn";
}

/** The one-word verdict. mwan3's opinion wins when it has one: it pings
 *  through the link, which is the difference between "the cable is in" and
 *  "this connection works". */
function stateLabel(c: WanCandidate): { key: string; tone: PillTone } {
  if (!c.up) return { key: "mwan.offline", tone: "danger" };
  if (c.online === "offline") return { key: "mwan.failed", tone: "danger" };
  if (c.active) return { key: "mwan.active", tone: "ok" };
  return { key: "mwan.standby", tone: "muted" };
}

const isValidIp = (v: string) =>
  /^(\d{1,3}\.){3}\d{1,3}$/.test(v) && v.split(".").every((n) => Number(n) <= 255);

function UplinkRow({ c, mode, busy, onPrimary }: {
  c: WanCandidate;
  mode: Mode;
  busy: boolean;
  onPrimary?: (name: string) => void;
}) {
  const { t } = useTranslation();
  const state = stateLabel(c);
  const where = [c.proto, c.port ? t("mwan.viaPort", { port: c.port }) : ""].filter(Boolean).join(" · ");
  return (
    <div className="flex items-center gap-3 py-2.5 flex-wrap">
      <StatusDot tone={dotTone(c)} />
      <div className="min-w-0">
        <div className="text-body font-medium truncate">{c.name}</div>
        <div className="text-caption text-muted truncate">{where}</div>
      </div>
      <span className="font-mono text-caption bg-surface-2 border border-border rounded-sm px-1.5 py-0.5">
        {c.ipv4[0] ?? "—"}
      </span>
      <span className="flex-1" />
      {c.up && <span className="text-caption text-muted tabular-nums">{fmtDur(c.uptime)}</span>}
      {c.metered && <Pill tone="warn">{t("mwan.metered")}</Pill>}
      <Pill tone={state.tone}>
        {c.active && c.share_pct > 0 && c.share_pct < 100
          ? t("mwan.activeShare", { pct: c.share_pct })
          : t(state.key)}
      </Pill>
      {/* Making a connection the main one is one click on its own row.
          It only means anything in failover: while balancing, every
          connection in the pool is equal by definition. */}
      {mode === "failover" && c.primary && <Pill tone="accent">{t("mwan.primary")}</Pill>}
      {mode === "failover" && !c.primary && onPrimary && (
        <Button size="sm" variant="secondary" disabled={busy} onClick={() => onPrimary(c.name)}>
          {t("mwan.makePrimary")}
        </Button>
      )}
    </div>
  );
}

/**
 * Card "Internet connections": every uplink the router has, which one is
 * carrying traffic, and what should happen when one of them dies.
 */
export function MultiWanCard({ probe, onChange, index = 2 }: {
  probe?: MultiWanProbe;
  onChange?: (p: MultiWanProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { job, running, begin } = useInstallJob();
  const { phase, detail, busy, run, clear } = useActionCycle();

  const [mode, setMode] = useState<Mode>("off");
  const [weights, setWeights] = useState<Record<string, number>>({});
  const [pool, setPool] = useState<Record<string, boolean>>({});
  const [track, setTrack] = useState<Record<string, string>>({});
  const [sticky, setSticky] = useState(true);
  const [confirming, setConfirming] = useState(false);
  const [takingOver, setTakingOver] = useState(false);

  // The form follows the router until the user touches it; after an apply
  // the probe comes back and becomes the new starting point.
  useEffect(() => {
    if (!probe) return;
    // "custom" is not one of the choices. Landing on "off" would make the
    // takeover button mean "delete what you have", so a setup we do not
    // recognise starts from failover, which is what it almost always is.
    setMode(
      MODES.includes(probe.mode as Mode)
        ? (probe.mode as Mode)
        : probe.mode === "custom"
          ? "failover"
          : "off",
    );
    setWeights(Object.fromEntries(probe.candidates.map((c) => [c.name, c.weight || 1])));
    setPool(Object.fromEntries(probe.candidates.map((c) => [c.name, probe.mode === "balance" ? c.balance : !c.metered])));
    setTrack(Object.fromEntries(probe.candidates.map((c) => [c.name, c.track.join(", ")])));
    setSticky(probe.mode === "balance" ? probe.sticky : true);
  }, [probe]);

  const parsedTrack = useMemo(() => {
    const out: Record<string, string[]> = {};
    for (const [name, raw] of Object.entries(track)) {
      const ips = raw.split(",").map((v) => v.trim()).filter(Boolean);
      if (ips.length > 0) out[name] = ips;
    }
    return out;
  }, [track]);

  const trackError = useMemo(
    () => Object.entries(parsedTrack).find(([, ips]) => ips.length > 4 || ips.some((ip) => !isValidIp(ip)))?.[0],
    [parsedTrack],
  );

  if (probe && !probe.applicable) return null;

  const install = () => {
    void begin(() => api.wizardPackages(["mwan3"])).then(async () => {
      try {
        onChange?.(await api.multiwan());
      } catch {
        /* the next poll picks it up */
      }
    });
  };

  const apply = (req: MultiWanRequest) => {
    void run(() => api.setMultiwan(req), 3000).then(async (res) => {
      if (res?.status === "applied") onChange?.(res.state);
      else onChange?.(await api.multiwan().catch(() => probe!));
    });
  };

  const applyCurrent = (confirmForeign = false) => {
    setConfirming(false);
    setTakingOver(false);
    apply({
      confirm_foreign: confirmForeign,
      mode,
      // Turning failover on should not move the house onto another line:
      // the connection already carrying traffic stays the main one unless
      // the user says otherwise.
      primary:
        mode === "failover"
          ? probe?.primary_iface ||
            probe?.candidates.find((c) => c.active)?.name ||
            probe?.candidates[0]?.name
          : undefined,
      weights: mode === "balance" ? weights : undefined,
      balance: mode === "balance" ? pool : undefined,
      sticky: mode === "balance" ? sticky : undefined,
      track: parsedTrack,
    });
  };

  const makePrimary = (iface: string) => {
    void run(() => api.setMultiwanPrimary(iface), 3000).then(async (res) => {
      if (res?.status === "applied") onChange?.(res.state);
      else onChange?.(await api.multiwan().catch(() => probe!));
    });
  };

  const modeKey: Record<string, string> = {
    failover: "mwan.modeFailover",
    balance: "mwan.modeBalance",
    custom: "mwan.modeCustom",
    off: "mwan.modeOff",
  };
  const dirty = !!probe && mode !== probe.mode;
  const configurable = !!probe && probe.multi_wan_possible && probe.installed && !probe.foreign;
  // A setup written elsewhere is shown as it is until the user hands it
  // over; there is no halfway state where netgrip owns some of it.
  const foreign = !!probe && probe.multi_wan_possible && probe.installed && probe.foreign;
  // A pool of nothing would take the house offline the moment it applied.
  const emptyPool = mode === "balance" && !Object.values(pool).some(Boolean);

  return (
    <>
      <Card
        index={index}
        icon={Split}
        title={t("mwan.title")}
        help="multiwan"
        action={
          probe && probe.multi_wan_possible && probe.installed ? (
            <Pill tone={probe.mode === "custom" ? "warn" : probe.mode === "off" ? "muted" : "accent"}>
              {t(modeKey[probe.mode] ?? "mwan.modeOff")}
            </Pill>
          ) : undefined
        }
      >
        {!probe ? (
          <SkeletonRows rows={3} />
        ) : (
          <>
            {configurable && (
              <div className="mb-3 space-y-1.5">
                <SegmentedControl
                  ariaLabel={t("mwan.modeLabel")}
                  value={mode}
                  onChange={(v) => setMode(v as Mode)}
                  options={MODES.map((m) => ({ value: m, label: t(modeKey[m]) }))}
                />
                <p className="text-caption text-muted">{t(`mwan.hint.${mode}`)}</p>
              </div>
            )}

            <div className="divide-y divide-border/50">
              {probe.candidates.map((c) => (
                <UplinkRow
                  key={c.name}
                  c={c}
                  mode={configurable && !dirty ? mode : "off"}
                  busy={busy}
                  onPrimary={makePrimary}
                />
              ))}
            </div>

            {!probe.multi_wan_possible && (
              <p className="text-caption text-muted mt-2">{t("mwan.singleBody")}</p>
            )}

            {probe.multi_wan_possible && !probe.installed && (
              <div className="mt-3 flex flex-col gap-2">
                <Banner
                  tone="info"
                  action={
                    <Button size="sm" onClick={install} loading={running}>
                      {t("services.installNow")}
                    </Button>
                  }
                >
                  {t("mwan.installPrompt")}
                </Banner>
                <InstallProgress job={job} />
              </div>
            )}

            {foreign && (
              <Banner
                tone="warn"
                className="mt-3"
                action={
                  <Button size="sm" variant="secondary" disabled={busy} onClick={() => setTakingOver(true)}>
                    {t("mwan.foreignTake")}
                  </Button>
                }
              >
                {t("mwan.foreignBanner", { sections: probe.foreign_sections.join(", ") })}
              </Banner>
            )}

            {configurable && (
              <AdvancedDisclosure label={t("common.advanced")} className="mt-3">
                <div className="space-y-4 pt-1">
                  {mode === "balance" && (
                    <div className="space-y-2">
                      <span className="text-caption text-muted block">{t("mwan.weights")}</span>
                      {probe.candidates.map((c) => {
                        const used = pool[c.name] ?? !c.metered;
                        const total = probe.candidates
                          .filter((o) => pool[o.name] ?? !o.metered)
                          .reduce((n, o) => n + (weights[o.name] ?? 1), 0);
                        const share = used && total > 0 ? Math.round(((weights[c.name] ?? 1) / total) * 100) : 0;
                        return (
                          <div key={c.name} className="flex items-center gap-3 flex-wrap">
                            <label className="flex items-center gap-2 min-w-40">
                              <input
                                type="checkbox"
                                checked={used}
                                onChange={(e) => setPool((p) => ({ ...p, [c.name]: e.target.checked }))}
                                className="accent-accent"
                              />
                              <span className="text-small">{c.name}</span>
                            </label>
                            <input
                              type="range"
                              min={1}
                              max={10}
                              value={weights[c.name] ?? 1}
                              disabled={!used}
                              aria-label={t("mwan.weightAria", { name: c.name })}
                              onChange={(e) => setWeights((wt) => ({ ...wt, [c.name]: Number(e.target.value) }))}
                              className="flex-1 accent-accent disabled:opacity-40"
                            />
                            <span className="w-24 shrink-0 text-right text-caption tabular-nums text-muted">
                              {used ? `${share}%` : t("mwan.weightOff")}
                            </span>
                          </div>
                        );
                      })}
                      {probe.candidates.some((c) => c.metered && (pool[c.name] ?? false)) && (
                        <Banner tone="warn">{t("mwan.meteredWarn")}</Banner>
                      )}

                      {/* The shares describe new connections, not what any
                          one device sees: each device stays put while this
                          is on, which is the thing people notice first. */}
                      <label className="flex items-start gap-2 pt-1">
                        <input
                          type="checkbox"
                          checked={sticky}
                          onChange={(e) => setSticky(e.target.checked)}
                          className="mt-1 accent-accent"
                        />
                        <span className="text-small">
                          {t("mwan.sticky")}
                          <span className="block text-caption text-muted">
                            {sticky ? t("mwan.stickyOnHint") : t("mwan.stickyOffHint")}
                          </span>
                        </span>
                      </label>
                    </div>
                  )}

                  <div className="space-y-2">
                    <span className="text-caption text-muted block">{t("mwan.tracking")}</span>
                    <p className="text-caption text-muted">{t("mwan.trackingHint")}</p>
                    {probe.candidates.map((c) => (
                      <Field
                        key={c.name}
                        label={c.name}
                        mono
                        error={trackError === c.name ? t("mwan.trackInvalid") : undefined}
                        inputProps={{
                          value: track[c.name] ?? "",
                          placeholder: probe.default_track.join(", "),
                          onChange: (e) => setTrack((tr) => ({ ...tr, [c.name]: e.target.value })),
                        }}
                      />
                    ))}
                  </div>
                </div>
              </AdvancedDisclosure>
            )}

            {configurable && (
              <div className="mt-3 flex items-center gap-2 flex-wrap">
                <Button
                  onClick={() => setConfirming(true)}
                  loading={busy}
                  disabled={!!trackError || emptyPool}
                >
                  {t("mwan.apply")}
                </Button>
                {emptyPool && <span className="text-caption text-danger">{t("mwan.needOneBalance")}</span>}
              </div>
            )}

            {phase && (
              <div className="mt-3">
                <ActionBanner
                  phase={phase}
                  text={phase === "done" ? t("mwan.applied") : phase === "failed" ? t("mwan.rolledBack") : undefined}
                  detail={detail}
                  onDone={clear}
                />
              </div>
            )}
          </>
        )}
      </Card>

      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() => applyCurrent(false)}
        title={t("mwan.switchTitle", { mode: t(modeKey[mode]) })}
        consequence={t("mwan.switchConsequence")}
        confirmLabel={t("mwan.switchConfirm")}
        busy={busy}
      />

      <ConfirmDialog
        open={takingOver}
        onClose={() => setTakingOver(false)}
        onConfirm={() => applyCurrent(true)}
        title={t("mwan.foreignTitle")}
        consequence={t("mwan.foreignConsequence", { sections: probe?.foreign_sections.join(", ") ?? "" })}
        confirmLabel={t("mwan.foreignConfirm")}
        busy={busy}
      />
    </>
  );
}
