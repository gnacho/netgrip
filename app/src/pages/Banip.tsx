import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowDownToLine, ArrowUpDown, ArrowUpFromLine, Ban, Info, Play, Plus, RefreshCw, Search, ShieldBan, ShieldCheck, Square, Trash2, TriangleAlert } from "lucide-react";
import { api } from "../api";
import type { BanipCatalogFeed, BanipFeed, BanipProbe, BanipSearchResult, BanipStatus } from "../types";
import { Banner, Button, Card, ConfirmDialog, EmptyState, Field, Input, Pill, SegmentedControl, SkeletonRows, useToast } from "../components/ui";

type Tab = "feeds" | "search" | "lists" | "dos";

const fmtInt = new Intl.NumberFormat();

// Preset recomendado (#350): base sensata (~20K IPs) mantenida por la
// comunidad; doh va en cadena de salida porque bloquea DNS-over-HTTPS.
const RECOMMENDED_FEEDS: { name: string; direction: BanipFeed["direction"] }[] = [
  { name: "cinsscore", direction: "" },
  { name: "debl", direction: "" },
  { name: "turris", direction: "" },
  { name: "doh", direction: "out" },
];

/** Pill "Recomendada" para las feeds del preset. */
function RecommendedTag() {
  const { t } = useTranslation();
  return <Pill tone="accent" className="ml-2">{t("banip.feedRecommended")}</Pill>;
}

/**
 * Botón (i) por feed: explica en lenguaje sencillo qué lista es. Usa el
 * diccionario curado (banip.feedInfo.<name>) y cae al descr del catálogo
 * de banIP cuando no hay entrada. Solo las entradas del catálogo añaden
 * dirección por defecto e IPv6 (son los únicos datos que tenemos).
 */
function FeedInfoButton({ name, descr, chain, ipv6, downloadFailed }: { name: string; descr?: string; chain?: BanipFeed["direction"] | ""; ipv6?: boolean; downloadFailed?: boolean }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const body = t(`banip.feedInfo.${name}`, { defaultValue: descr || t("banip.feedInfoFallback") });
  return (
    <span ref={ref} className="relative inline-flex shrink-0 align-middle">
      <button
        type="button"
        aria-expanded={open}
        aria-label={t("banip.feedInfoLabel", { name })}
        onClick={() => setOpen((o) => !o)}
        className="inline-flex h-5 w-5 items-center justify-center rounded-full text-faint hover:text-accent ring-focus transition-colors"
      >
        <Info size={14} />
      </button>
      {open && (
        <span role="tooltip" className="absolute left-1/2 -translate-x-1/2 top-6 z-40 block w-[min(280px,calc(100vw-48px))] rounded-md border border-border bg-surface p-3 text-left shadow-elevated animate-banner-in">
          <span className="block text-small font-semibold mb-1">{name}</span>
          <span className="block text-small text-muted">{body}</span>
          {downloadFailed && (
            <span className="mt-2 flex items-start gap-1.5 text-caption text-warn">
              <TriangleAlert size={12} className="mt-0.5 shrink-0" aria-hidden="true" />
              <span>{t("banip.feedDownloadFailedHint")}</span>
            </span>
          )}
          {chain !== undefined && (
            <span className="mt-2 flex items-center gap-1.5 text-caption text-faint">
              {chain === "in" && <ArrowDownToLine size={12} aria-hidden="true" />}
              {chain === "out" && <ArrowUpFromLine size={12} aria-hidden="true" />}
              {chain === "inout" && <ArrowUpDown size={12} aria-hidden="true" />}
              <span>{chain === "" ? t("banip.dirDefault") : t(chain === "inout" ? "banip.dirBoth" : chain === "in" ? "banip.dirIn" : "banip.dirOut")}</span>
              <span aria-hidden="true">·</span>
              <span>{ipv6 ? t("banip.feedInfoIpv6") : t("banip.feedInfoIpv4Only")}</span>
            </span>
          )}
          <span className="sr-only">{t("common.escToClose")}</span>
        </span>
      )}
    </span>
  );
}

/** Tarjeta de cifra clave del resumen. */
function StatCard({ index, label, value, hint }: { index: number; label: string; value: string; hint?: string }) {
  return (
    <Card index={index} className="md:col-span-3">
      <p className="text-small text-muted">{label}</p>
      <p className="text-h2 mt-1 tabular-nums">{value}</p>
      {hint && <p className="text-caption text-faint mt-0.5">{hint}</p>}
    </Card>
  );
}

export function BanipPage() {
  const { t } = useTranslation();
  const { push } = useToast();
  const [probe, setProbe] = useState<BanipProbe>();
  const [st, setSt] = useState<BanipStatus>();
  const [loadError, setLoadError] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("feeds");
  const [confirmInstall, setConfirmInstall] = useState(false);
  const [confirmUninstall, setConfirmUninstall] = useState(false);
  const [confirmReload, setConfirmReload] = useState(false);

  // Carga progresiva (#354): el status ligero pinta la página al instante
  // (mipsle tarda ~9s en el probe completo, dominado por `banip report`);
  // el probe completo llega detrás y rellena stats y tabs.
  const load = useCallback(async () => {
    setLoadError(false);
    api.banipStatus().then(setSt).catch(() => {});
    try {
      setProbe(await api.banip());
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const run = useCallback(async (key: string, fn: () => Promise<{ state?: BanipProbe; error?: string }>) => {
    setBusy(key);
    try {
      const res = await fn();
      if (res.state) setProbe(res.state);
      if (res.error) {
        push({ tone: "danger", text: t("banip.actionFailed"), detail: res.error });
      } else {
        push({ tone: "ok", text: t("banip.actionOk") });
      }
    } catch (e) {
      push({ tone: "danger", text: t("banip.actionFailed"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(null);
      load();
    }
  }, [load, push, t]);

  const report = probe?.report;
  const dosAvailable = !!report?.parsed;
  const enabledFeeds = useMemo(() => probe?.feeds.filter((f) => f.enabled) ?? [], [probe?.feeds]);

  const installDialog = (
    <ConfirmDialog
      open={confirmInstall}
      onClose={() => setConfirmInstall(false)}
      onConfirm={() => { setConfirmInstall(false); setBusy("install"); api.banipInstall().then((p) => { setProbe(p); setSt({ installed: true, enabled: p.enabled, running: p.running, applicable: p.applicable }); push({ tone: "ok", text: t("banip.installOk") }); }).catch((e) => push({ tone: "danger", text: t("banip.actionFailed"), detail: e instanceof Error ? e.message : String(e) })).finally(() => setBusy(null)); }}
      title={t("banip.installConfirmTitle")}
      consequence={t("banip.installConfirmBody")}
      confirmLabel={t("banip.install")}
      busy={busy === "install"}
    />
  );

  const notInstalledView = (
    <>
      <EmptyState
        illustration={<ShieldBan size={120} />}
        title={t("banip.notInstalledTitle")}
        body={t("banip.notInstalledBody")}
        action={<Button variant="primary" onClick={() => setConfirmInstall(true)}>{t("banip.install")}</Button>}
      />
      {installDialog}
    </>
  );

  if (loadError && !probe && !st) {
    return <EmptyState title={t("common.loadError")} action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>} />;
  }

  // Estados decidibles desde el status ligero sin esperar al probe completo.
  if (!probe) {
    if (st && !st.applicable) {
      return <EmptyState illustration={<ShieldBan size={120} />} title={t("banip.notGatewayTitle")} body={t("banip.notGatewayBody")} />;
    }
    if (st && !st.installed) {
      return notInstalledView;
    }
    // Status conocido (instalado) y probe completo en camino: header real +
    // skeletons en las zonas lentas.
    const stActive = !!st && st.enabled && st.running;
    return (
      <div className="flex flex-col gap-[var(--card-gap)]">
        <div className="flex flex-wrap items-center gap-2">
          <Pill tone={stActive ? "ok" : st?.enabled ? "warn" : "muted"} live={stActive}>
            {stActive ? t("banip.stateActive") : st?.enabled ? t("banip.stateStopped") : t("banip.stateDisabled")}
          </Pill>
          <span className="flex-1" />
          <Button variant="primary" size="sm" disabled={busy !== null} onClick={() => setConfirmReload(true)}>
            <RefreshCw size={14} aria-hidden="true" /> {t("banip.reload")}
          </Button>
          <ConfirmDialog
            open={confirmReload}
            onClose={() => setConfirmReload(false)}
            onConfirm={() => { setConfirmReload(false); run("reload", () => api.banipAction("reload")); }}
            title={t("banip.reloadConfirmTitle")}
            consequence={t("banip.reloadNote")}
            confirmLabel={t("banip.reload")}
            busy={busy === "reload"}
          />
        </div>
        <p className="text-small text-muted -mt-1">{t("banip.introDesc")}</p>
        <div className="grid grid-cols-2 md:grid-cols-12 gap-[var(--card-gap)]">
        <div className="md:col-span-3"><SkeletonRows rows={2} /></div>
          <div className="md:col-span-3"><SkeletonRows rows={2} /></div>
          <div className="md:col-span-3"><SkeletonRows rows={2} /></div>
        </div>
        <Card><SkeletonRows rows={5} /></Card>
        <p className="text-caption text-faint">{t("banip.loadingDetails")}</p>
        {installDialog}
      </div>
    );
  }

  // Gateway-only: banIP only makes sense on the router that faces the internet.
  if (!probe.applicable) {
    return <EmptyState illustration={<ShieldBan size={120} />} title={t("banip.notGatewayTitle")} body={t("banip.notGatewayBody")} />;
  }

  if (!probe.installed) {
    return notInstalledView;
  }

  const active = probe.enabled && probe.running;
  const lowMem = probe.mem_available_mb > 0 && probe.mem_available_mb < 256;

  const tabs: { value: Tab; label: string }[] = [
    { value: "feeds", label: t("banip.tabFeeds") },
    { value: "search", label: t("banip.tabSearch") },
    { value: "lists", label: t("banip.tabLists") },
    ...(dosAvailable ? [{ value: "dos" as Tab, label: t("banip.tabDos") }] : []),
  ];

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* Estado + acciones */}
      <div className="flex flex-wrap items-center gap-2">
        <Pill tone={active ? "ok" : probe.enabled ? "warn" : "muted"} live={active}>
          {active ? t("banip.stateActive") : probe.enabled ? t("banip.stateStopped") : t("banip.stateDisabled")}
        </Pill>
        {probe.version && <span className="text-caption text-faint font-mono">banIP {probe.version}</span>}
        <span className="flex-1" />
        {!active && (
          <Button variant="secondary" size="sm" disabled={busy !== null}
            onClick={() => run(probe.enabled ? "start" : "enable", () => api.banipAction(probe.enabled ? "start" : "enable"))}>
            <Play size={14} aria-hidden="true" /> {probe.enabled ? t("banip.start") : t("banip.enable")}
          </Button>
        )}
        <Button variant="primary" size="sm" disabled={busy !== null}
          title={t("banip.reloadHint")}
          onClick={() => setConfirmReload(true)}>
          <RefreshCw size={14} aria-hidden="true" /> {t("banip.reload")}
        </Button>
        <ConfirmDialog
          open={confirmReload}
          onClose={() => setConfirmReload(false)}
          onConfirm={() => { setConfirmReload(false); run("reload", () => api.banipAction("reload")); }}
          title={t("banip.reloadConfirmTitle")}
          consequence={t("banip.reloadNote")}
          confirmLabel={t("banip.reload")}
          busy={busy === "reload"}
        />
        {active && (
          <Button variant="secondary" size="sm" disabled={busy !== null}
            onClick={() => run("stop", () => api.banipAction("stop"))}>
            <Square size={14} aria-hidden="true" /> {t("banip.stop")}
          </Button>
        )}
        <Button variant="ghost" size="sm" className="text-danger hover:text-danger" disabled={busy !== null}
          onClick={() => setConfirmUninstall(true)}>
          <Trash2 size={14} aria-hidden="true" /> {t("banip.uninstall")}
        </Button>
        <ConfirmDialog
          open={confirmUninstall}
          onClose={() => setConfirmUninstall(false)}
          onConfirm={() => {
            setConfirmUninstall(false);
            setBusy("uninstall");
            api.banipUninstall()
              .then((p) => { setProbe(p); push({ tone: "ok", text: t("banip.uninstallOk") }); })
              .catch((e) => push({ tone: "danger", text: t("banip.actionFailed"), detail: e instanceof Error ? e.message : String(e) }))
              .finally(() => setBusy(null));
          }}
          title={t("banip.uninstallConfirmTitle")}
          consequence={t("banip.uninstallConfirmBody")}
          confirmLabel={t("banip.uninstall")}
          busy={busy === "uninstall"}
        />
      </div>

      {/* Explicación sencilla + avisos */}
      <p className="text-small text-muted -mt-1">{t("banip.introDesc")}</p>
      {lowMem && <Banner tone="warn">{t("banip.lowMem", { mb: fmtInt.format(probe.mem_available_mb) })}</Banner>}

      {/* Resumen */}
      <div className="grid grid-cols-2 md:grid-cols-12 gap-[var(--card-gap)]">
        <StatCard index={0} label={t("banip.statIps")} value={report?.parsed ? fmtInt.format(report.total_ips) : "—"}
          hint={report?.parsed ? t("banip.statIpsHint") : undefined} />
        {probe.nft_count && (
          <StatCard index={1} label={t("banip.statPackets")} value={report?.parsed ? fmtInt.format(report.packets_in + report.packets_out) : "—"}
            hint={t("banip.statPacketsHint")} />
        )}
        <StatCard index={2} label={t("banip.statFeeds")} value={`${enabledFeeds.length}`}
          hint={t("banip.statFeedsHint", { total: probe.feeds.length })} />
        {dosAvailable && (
          <StatCard index={3} label={t("banip.statAutobans")} value={fmtInt.format(report.auto_block)}
            hint={t("banip.statAutobansHint", { allow: fmtInt.format(report.auto_allow) })} />
        )}
      </div>

      <SegmentedControl<Tab>
        ariaLabel={t("banip.tabs")}
        value={tab}
        onChange={setTab}
        options={tabs}
      />

      {tab === "feeds" && (
        <FeedsTab probe={probe} busy={busy !== null} onSaved={(p) => { setProbe(p); push({ tone: "ok", text: t("banip.feedsSaved") }); load(); }} onError={(msg) => push({ tone: "danger", text: t("banip.actionFailed"), detail: msg })} />
      )}
      {tab === "search" && <SearchTab onError={(msg) => push({ tone: "danger", text: t("banip.actionFailed"), detail: msg })} />}
      {tab === "lists" && <ListsTab probe={probe} busy={busy !== null} onChanged={(p) => { setProbe(p); load(); }} onError={(msg) => push({ tone: "danger", text: t("banip.actionFailed"), detail: msg })} />}
      {tab === "dos" && dosAvailable && <DosTab probe={probe} />}
    </div>
  );
}

// ── Feeds ──────────────────────────────────────────────────────────────────

function FeedDirectionControl({ feed, onChange }: { feed: BanipFeed; onChange: (d: BanipFeed["direction"]) => void }) {
  const { t } = useTranslation();
  return (
    <SegmentedControl<"" | "in" | "out" | "inout">
      size="sm"
      ariaLabel={t("banip.feedDirection")}
      value={feed.direction}
      onChange={onChange}
      options={[
        { value: "", label: t("banip.dirDefault") },
        { value: "in", label: t("banip.dirIn") },
        { value: "out", label: t("banip.dirOut") },
        { value: "inout", label: t("banip.dirBoth") },
      ]}
    />
  );
}

function FeedsTab({ probe, busy, onSaved, onError }: { probe: BanipProbe; busy: boolean; onSaved: (p: BanipProbe) => void; onError: (msg: string) => void }) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<BanipFeed[]>(probe.feeds);
  const [saving, setSaving] = useState(false);
  useEffect(() => setDraft(probe.feeds), [probe.feeds]);

  const dirty = useMemo(() => JSON.stringify(draft) !== JSON.stringify(probe.feeds), [draft, probe.feeds]);
  const setFeed = (name: string, patch: Partial<BanipFeed>) =>
    setDraft((d) => d.map((f) => (f.name === name ? { ...f, ...patch } : f)));

  const save = async () => {
    setSaving(true);
    try {
      const res = await api.banipSetFeeds({ feeds: draft });
      if (res.error) onError(res.error);
      else onSaved(res.state);
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  const countFor = (name: string) =>
    probe.report?.sets.filter((s) => s.name === `${name}.v4` || s.name === `${name}.v6`).reduce((a, s) => a + s.elements, 0);

  // Catalog feeds not in the draft yet: these are the ones that can be added.
  const draftNames = new Set(draft.map((f) => f.name));
  const available = (probe.catalog ?? []).filter((c) => !draftNames.has(c.name));
  const recommended = (name: string) => RECOMMENDED_FEEDS.some((r) => r.name === name);

  const addFromCatalog = (c: BanipCatalogFeed) =>
    setDraft((d) => [...d, { name: c.name, enabled: true, direction: c.chain, in_catalog: true }]);

  const addRecommended = () =>
    setDraft((d) => {
      const names = new Set(d.map((f) => f.name));
      const extra = RECOMMENDED_FEEDS.filter((r) => !names.has(r.name)).map((r) => ({
        name: r.name,
        enabled: true,
        direction: r.direction,
        in_catalog: true,
      }));
      return [...d, ...extra];
    });

  // Sin feeds configurados y servicio activo: sugerir el preset (#350).
  const suggestRecommended = draft.length === 0 && probe.enabled && probe.running;

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {suggestRecommended && (
        <Banner tone="info"
          action={<Button variant="secondary" size="sm" disabled={busy || saving} onClick={addRecommended}>{t("banip.addRecommended")}</Button>}>
          {t("banip.recommendedBanner")}
        </Banner>
      )}
      <Card icon={ShieldCheck} iconTone="success" title={t("banip.feedsTitle")}
        action={<Button variant="primary" size="sm" disabled={!dirty || busy || saving} onClick={save}>{t("common.save")}</Button>}>
        {draft.length === 0 ? (
          <EmptyState small title={t("banip.feedsEmpty")} />
        ) : (
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-border/60 text-left">
                <th className="pb-2 pr-4">{t("banip.feedName")}</th>
                <th className="pb-2 pr-4 text-right">{t("banip.feedElements")}</th>
                <th className="pb-2 pr-4">{t("banip.feedEnabled")}</th>
                <th className="pb-2 text-right">{t("banip.feedDirection")}</th>
              </tr>
            </thead>
            <tbody>
              {draft.map((f) => {
                const count = countFor(f.name);
                return (
                <tr key={f.name} className="border-b border-border/40 last:border-0">
                  <td className="py-2 pr-4">
                      <span className="inline-flex items-center">
                      <span className="font-mono text-small">{f.name}</span>
                      {recommended(f.name) && <RecommendedTag />}
                      {f.last_download_failed && (
                        <Pill tone="warn" className="ml-2">
                          <TriangleAlert size={12} aria-hidden="true" /> {t("banip.feedDownloadFailed")}
                        </Pill>
                      )}
                      <span className="ml-1.5"><FeedInfoButton name={f.name} chain={f.chain} ipv6={f.ipv6} downloadFailed={f.last_download_failed} /></span>
                      {!f.in_catalog && <span className="ml-1 text-caption text-faint" title={t("banip.notInCatalog")}>·</span>}
                    </span>
                  </td>
                    <td className="py-2 pr-4 text-right text-small tabular-nums">{count !== undefined && count > 0 ? fmtInt.format(count) : "—"}</td>
                    <td className="py-2 pr-4">
                      <input type="checkbox" aria-label={t("banip.feedEnabled")} className="accent-accent"
                        checked={f.enabled} onChange={(e) => setFeed(f.name, { enabled: e.target.checked })} />
                    </td>
                    <td className="py-2 text-right">
                      <div className="inline-flex">
                        <FeedDirectionControl feed={f} onChange={(d) => setFeed(f.name, { direction: d })} />
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
        <p className="text-caption text-faint mt-3">{t("banip.feedsNote")}</p>
      </Card>

      {available.length > 0 && (
        <Card icon={Plus} iconTone="accent" title={t("banip.catalogTitle")}>
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-border/60 text-left">
                <th className="pb-2 pr-4">{t("banip.feedName")}</th>
                <th className="pb-2 pr-4">{t("banip.feedDescr")}</th>
                <th className="pb-2 pr-4 text-right">{t("banip.feedDirection")}</th>
                <th className="pb-2 text-right">{t("banip.catalogAdd")}</th>
              </tr>
            </thead>
            <tbody>
              {available.map((c) => (
                <tr key={c.name} className="border-b border-border/40 last:border-0">
                  <td className="py-2 pr-4">
                    <span className="inline-flex items-center">
                      <span className="font-mono text-small">{c.name}</span>
                      {recommended(c.name) && <RecommendedTag />}
                      <span className="ml-1.5"><FeedInfoButton name={c.name} descr={c.descr} chain={c.chain} ipv6={c.ipv6} /></span>
                      {c.custom && <Pill tone="muted" className="ml-2">{t("banip.feedCustom")}</Pill>}
                      {c.ipv6 && <Pill tone="ok" className="ml-2">{t("banip.feedIpv6")}</Pill>}
                    </span>
                  </td>
                  <td className="py-2 pr-4 text-small text-muted max-w-[280px]">
                    <span className="line-clamp-1" title={c.descr}>{c.descr || "—"}</span>
                  </td>
                  <td className="py-2 pr-4 text-right text-small text-muted">
                    {c.chain ? t(c.chain === "inout" ? "banip.dirBoth" : c.chain === "in" ? "banip.dirIn" : "banip.dirOut") : t("banip.dirDefault")}
                  </td>
                  <td className="py-2 text-right">
                    <Button variant="secondary" size="sm" disabled={busy || saving} onClick={() => addFromCatalog(c)}>
                      <Plus size={14} aria-hidden="true" /> {t("banip.catalogAdd")}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}

// ── Buscar IP ──────────────────────────────────────────────────────────────

function SearchTab({ onError }: { onError: (msg: string) => void }) {
  const { t } = useTranslation();
  const [ip, setIp] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<BanipSearchResult>();

  const search = async () => {
    setBusy(true);
    setResult(undefined);
    try {
      setResult(await api.banipSearch(ip.trim()));
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card icon={Search} iconTone="accent" title={t("banip.searchTitle")}>
      <form className="flex items-end gap-2" onSubmit={(e) => { e.preventDefault(); if (ip.trim()) search(); }}>
        <div className="flex-1">
          <Field label={t("banip.searchLabel")}>
            <Input value={ip} onChange={(e) => setIp(e.target.value)} placeholder="203.0.113.66" />
          </Field>
        </div>
        <Button variant="primary" type="submit" disabled={busy || !ip.trim()}>
          {busy ? t("banip.searching") : t("banip.search")}
        </Button>
      </form>
      {result && (
        <div className="mt-4">
          {result.found ? (
            <div className="flex flex-col gap-1">
              <Pill tone="danger">{t("banip.searchFound")}</Pill>
              <ul className="mt-2">
                {result.sets.map((s) => (
                  <li key={s} className="font-mono text-small py-1">{s}</li>
                ))}
              </ul>
            </div>
          ) : (
            <Pill tone="ok">{t("banip.searchNotFound")}</Pill>
          )}
        </div>
      )}
    </Card>
  );
}

// ── Listas locales ─────────────────────────────────────────────────────────

function ListCard({ list, entries, busy, onChanged, onError }: {
  list: "allowlist" | "blocklist";
  entries: string[];
  busy: boolean;
  onChanged: (p: BanipProbe) => void;
  onError: (msg: string) => void;
}) {
  const { t } = useTranslation();
  const [entry, setEntry] = useState("");
  const [saving, setSaving] = useState(false);
  const isAllow = list === "allowlist";

  const mutate = async (fn: () => Promise<{ state?: BanipProbe; error?: string }>) => {
    setSaving(true);
    try {
      const res = await fn();
      if (res.error) onError(res.error);
      else if (res.state) onChanged(res.state);
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card icon={isAllow ? ShieldCheck : Ban} iconTone={isAllow ? "success" : "danger"} title={t(isAllow ? "banip.allowlist" : "banip.blocklist")}>
      <form className="flex items-end gap-2" onSubmit={(e) => {
        e.preventDefault();
        const value = entry.trim();
        if (!value) return;
        setEntry("");
        mutate(() => api.banipListAdd(list, value));
      }}>
        <div className="flex-1">
          <Field label={t("banip.listAddLabel")}>
            <Input value={entry} onChange={(e) => setEntry(e.target.value)} placeholder={t("banip.listPlaceholder")} />
          </Field>
        </div>
        <Button variant="primary" type="submit" disabled={saving || busy || !entry.trim()}>{t("banip.listAdd")}</Button>
      </form>
      <ul className="mt-3 divide-y divide-border/60">
        {entries.length === 0 && <li className="py-2 text-small text-muted">{t("banip.listEmpty")}</li>}
        {entries.map((e) => (
          <li key={e} className="flex items-center gap-2 py-2">
            <span className="flex-1 min-w-0 font-mono text-small truncate">{e}</span>
            <Button variant="ghost" size="sm" disabled={saving || busy} onClick={() => mutate(() => api.banipListRemove(list, e))}>
              {t("banip.listRemove")}
            </Button>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function ListsTab({ probe, busy, onChanged, onError }: { probe: BanipProbe; busy: boolean; onChanged: (p: BanipProbe) => void; onError: (msg: string) => void }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
      <ListCard list="allowlist" entries={probe.allowlist} busy={busy} onChanged={onChanged} onError={onError} />
      <ListCard list="blocklist" entries={probe.blocklist} busy={busy} onChanged={onChanged} onError={onError} />
    </div>
  );
}

// ── DoS ────────────────────────────────────────────────────────────────────

function DosTab({ probe }: { probe: BanipProbe }) {
  const { t } = useTranslation();
  const dos = probe.report!.dos;
  const rows: { label: string; value: number; limit?: number }[] = [
    { label: t("banip.dosSyn"), value: dos.syn_packets, limit: dos.syn_limit },
    { label: t("banip.dosUdp"), value: dos.udp_packets, limit: dos.udp_limit },
    { label: t("banip.dosIcmp"), value: dos.icmp_packets, limit: dos.icmp_limit },
    { label: t("banip.dosInvalidCt"), value: dos.invalid_ct_packets },
    { label: t("banip.dosInvalidTcp"), value: dos.invalid_tcp_packets },
  ];
  return (
    <Card icon={ShieldBan} iconTone="warn" title={t("banip.dosTitle")}>
      <table className="w-full border-collapse">
        <thead>
          <tr className="border-b border-border/60 text-left">
            <th className="pb-2 pr-4">{t("banip.dosCounter")}</th>
            <th className="pb-2 pr-4 text-right">{t("banip.dosBlocked")}</th>
            <th className="pb-2 text-right">{t("banip.dosLimit")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.label} className="border-b border-border/40 last:border-0">
              <td className="py-2 pr-4 text-small">{r.label}</td>
              <td className="py-2 pr-4 text-right text-small tabular-nums">{fmtInt.format(r.value)}</td>
              <td className="py-2 text-right text-small tabular-nums text-muted">{r.limit ?? "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-caption text-faint mt-3">{t("banip.dosNote")}</p>
    </Card>
  );
}
