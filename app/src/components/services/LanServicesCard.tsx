import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { AppWindow, ExternalLink, Pencil, Plus, Search, Trash2, X } from "lucide-react";
import { api } from "../../api";
import type { LanDiscoverySuggestion, LanService, LanServiceStatus, LanServicesProbe } from "../../types";
import {
  AdvancedDisclosure, Button, ConfirmDialog, EmptyState, Field, IconTile,
  Modal, Pill, SegmentedControl, Skeleton, useToast,
} from "../ui";
import { lanServiceIcon } from "./lanCatalog";

/** URL que abre el servicio desde el navegador del usuario. */
function serviceOpenUrl(s: LanService): string {
  if (s.url) return s.url;
  const scheme = s.scheme || "http";
  const port = s.port && !((scheme === "http" && s.port === 80) || (scheme === "https" && s.port === 443))
    ? `:${s.port}`
    : "";
  return `${scheme}://${s.host}${port}${s.path ?? ""}`;
}

/** "host:port" para mostrar bajo el nombre. */
function serviceTarget(s: LanService): string {
  if (s.url) {
    try {
      return new URL(s.url).host;
    } catch {
      return s.url;
    }
  }
  return `${s.host ?? ""}${s.port ? `:${s.port}` : ""}`;
}

interface FormState {
  id: string;
  name: string;
  kind: string;
  host: string;
  port: string;
  scheme: string;
  path: string;
  url: string;
}

function emptyForm(): FormState {
  return { id: "", name: "", kind: "custom", host: "", port: "", scheme: "http", path: "", url: "" };
}

function toForm(s?: LanService): FormState {
  if (!s) return emptyForm();
  return {
    id: s.id,
    name: s.name,
    kind: s.kind,
    host: s.host ?? "",
    port: s.port ? String(s.port) : "",
    scheme: s.scheme || "http",
    path: s.path ?? "",
    url: s.url ?? "",
  };
}

/** Servicios locales (#303): lista de servicios self-hosted con test de
 *  accesibilidad ejecutado desde el router. Cada tarjeta muestra un pill de
 *  estado y un botón Abrir. El formulario ofrece catálogo o entrada personal. */
export function LanServicesCard() {
  const { t } = useTranslation();
  const toast = useToast();
  const [probe, setProbe] = useState<LanServicesProbe>();
  const [loadError, setLoadError] = useState(false);
  const [editing, setEditing] = useState<LanService | undefined>();
  const [showAdd, setShowAdd] = useState(false);
  const [toRemove, setToRemove] = useState<LanServiceStatus>();
  const [suggestions, setSuggestions] = useState<LanDiscoverySuggestion[]>([]);
  const [discovering, setDiscovering] = useState(false);

  const load = useCallback(async () => {
    setLoadError(false);
    try {
      setProbe(await api.lanServices());
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const services = probe?.services ?? [];
  const loading = probe === undefined && !loadError;
  const empty = !loading && !loadError && services.length === 0;

  const doRemove = async () => {
    if (!toRemove) return;
    try {
      await api.deleteLanService(toRemove.id);
      setProbe((prev) => prev && { ...prev, services: prev.services.filter((s) => s.id !== toRemove.id) });
      toast.push({ tone: "ok", text: t("lanservices.removedOk", { name: toRemove.name }) });
      setToRemove(undefined);
    } catch (e) {
      toast.push({ tone: "danger", text: t("common.loadError"), detail: e instanceof Error ? e.message : String(e) });
    }
  };

  const discover = useCallback(async () => {
    setDiscovering(true);
    try {
      const res = await api.discoverLanServices();
      const found = res.suggestions ?? [];
      setSuggestions(found);
      if (found.length === 0) {
        toast.push({ tone: "info", text: t("lanservices.noResults") });
      }
    } catch (e) {
      toast.push({ tone: "danger", text: t("common.loadError"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setDiscovering(false);
    }
  }, [t, toast]);

  const addSuggestion = async (s: LanDiscoverySuggestion) => {
    const name = t(`lanservices.kind.${s.kind}`);
    try {
      await api.upsertLanService({
        id: "",
        name,
        kind: s.kind,
        host: s.host,
        port: s.port,
        scheme: s.scheme,
        path: s.path ?? "",
        enabled: true,
      });
      setSuggestions((prev) => prev.filter((x) => !(x.host === s.host && x.port === s.port && x.kind === s.kind)));
      toast.push({ tone: "ok", text: t("lanservices.savedOk", { name }) });
      load();
    } catch (e) {
      toast.push({ tone: "danger", text: t("common.loadError"), detail: e instanceof Error ? e.message : String(e) });
    }
  };

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {loadError ? (
        <EmptyState
          small
          illustration={<AppWindow size={24} />}
          title={t("common.loadError")}
          action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}
        />
      ) : loading ? (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
        </div>
      ) : empty ? (
        <EmptyState
          illustration={<AppWindow size={140} />}
          title={t("lanservices.emptyTitle")}
          body={t("lanservices.emptyBody")}
          action={<Button icon={Plus} onClick={() => setShowAdd(true)}>{t("lanservices.addFirst")}</Button>}
        />
      ) : (
        <div className="flex flex-col gap-[var(--card-gap)]">
          {services.map((s, i) => (
            <ServiceRow
              key={s.id}
              s={s}
              index={i}
              onEdit={() => { setEditing(s); }}
              onRemove={() => setToRemove(s)}
            />
          ))}
          <button
            type="button"
            onClick={() => setShowAdd(true)}
            style={{ "--i": Math.min(services.length, 7), animationDelay: `${services.length * 70}ms` } as CSSProperties}
            className="animate-fade-up rounded-lg border border-dashed border-border-strong p-4
              flex items-center justify-center gap-2 text-muted hover:text-accent hover:border-accent
              transition-colors duration-[var(--dur-fast)] ring-focus"
          >
            <Plus size={20} aria-hidden="true" />
            <span className="text-body font-medium">{t("lanservices.add")}</span>
          </button>
        </div>
      )}

      {!loadError && !loading && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <div className="flex items-center justify-between">
            <Button variant="secondary" size="sm" icon={Search} loading={discovering} onClick={discover}>
              {discovering ? t("lanservices.discovering") : t("lanservices.discover")}
            </Button>
          </div>
          {suggestions.length > 0 && (
            <div className="rounded-lg border border-border bg-surface p-3 flex flex-col gap-2 animate-fade-up">
              <div className="flex items-center justify-between">
                <h3 className="text-caption font-medium text-muted">
                  {t("lanservices.suggestionsTitle", { count: suggestions.length })}
                </h3>
                <button
                  type="button"
                  onClick={() => setSuggestions([])}
                  aria-label={t("lanservices.dismiss")}
                  className="text-muted hover:text-text ring-focus rounded-sm"
                >
                  <X size={16} aria-hidden="true" />
                </button>
              </div>
              {suggestions.map((s) => (
                <div key={`${s.host}-${s.port}-${s.kind}`} className="flex items-center gap-2.5">
                  <IconTile icon={lanServiceIcon(s.kind)} tone="accent" size={28} />
                  <div className="flex-1 min-w-0">
                    <p className="text-small truncate">{t(`lanservices.kind.${s.kind}`)}</p>
                    <p className="text-caption text-muted font-mono">
                      {s.host}:{s.port}
                      {s.ip && s.ip !== s.host ? ` · ${s.ip}` : ""}
                    </p>
                  </div>
                  <Button size="sm" variant="secondary" icon={Plus} onClick={() => addSuggestion(s)}>
                    {t("lanservices.addSuggestion")}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <ServiceModal
        open={showAdd || editing !== undefined}
        service={editing}
        catalog={probe?.catalog ?? []}
        hosts={probe?.hosts ?? []}
        onClose={() => { setShowAdd(false); setEditing(undefined); }}
        onSaved={(name) => {
          setShowAdd(false);
          setEditing(undefined);
          toast.push({ tone: "ok", text: t("lanservices.savedOk", { name }) });
          load();
        }}
      />

      <ConfirmDialog
        open={!!toRemove}
        onClose={() => setToRemove(undefined)}
        onConfirm={doRemove}
        title={t("lanservices.removeTitle", { name: toRemove?.name ?? "" })}
        consequence={t("lanservices.removeConsequence")}
        confirmLabel={t("lanservices.removeConfirm")}
      />
    </div>
  );
}

function ServiceRow({ s, index, onEdit, onRemove }: {
  s: LanServiceStatus;
  index: number;
  onEdit: () => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const open = () => window.open(serviceOpenUrl(s), "_blank", "noopener,noreferrer");

  const pill = !s.ok
    ? (s.error === "disabled"
      ? <Pill tone="muted">{t("lanservices.statusOff")}</Pill>
      : <span title={s.error}><Pill tone="danger">{t("lanservices.statusFail")}</Pill></span>)
    : <Pill tone="ok">{t("lanservices.statusOk")}</Pill>;

  const detail = s.ok
    ? [s.latency_ms != null ? `${s.latency_ms} ms` : "", s.http_status ? `HTTP ${s.http_status}` : ""].filter(Boolean).join(" · ")
    : "";

  return (
    <article
      style={{ "--i": Math.min(index, 7), animationDelay: `${index * 70}ms` } as CSSProperties}
      className="animate-fade-up rounded-lg border border-border bg-surface p-4 shadow-card flex flex-col"
    >
      <div className="flex items-start gap-2.5">
        <IconTile icon={lanServiceIcon(s.kind)} tone="accent" />
        <div className="flex-1 min-w-0">
          <h2 className="text-h2 truncate">{s.name}</h2>
          <p className="text-caption text-muted font-mono">
            {serviceTarget(s)}
            {s.resolved_ip && s.resolved_ip !== s.host ? ` · ${s.resolved_ip}` : ""}
          </p>
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          {pill}
          {detail && <span className="text-caption text-muted">{detail}</span>}
        </div>
      </div>

      {s.error && s.error !== "disabled" && (
        <AdvancedDisclosure label={t("lanservices.technicalDetail")} className="mt-1">
          <pre className="max-h-32 overflow-auto rounded-sm bg-surface-2 border border-border p-2 font-mono text-caption whitespace-pre-wrap">
            {s.error}
          </pre>
        </AdvancedDisclosure>
      )}

      <div className="flex-1 min-h-3" aria-hidden="true" />
      <div className="pt-3 border-t border-border/60 flex flex-wrap items-center gap-1">
        <Button size="sm" icon={ExternalLink} onClick={open} className="max-sm:h-11">
          {t("lanservices.open")}
        </Button>
        <Button variant="ghost" size="sm" icon={Pencil} onClick={onEdit} className="max-sm:h-11">
          {t("lanservices.edit")}
        </Button>
        <Button variant="ghost" size="sm" icon={Trash2} onClick={onRemove}
          aria-label={t("lanservices.removeTitle", { name: s.name })}
          className="ml-auto max-sm:h-11">
          {t("lanservices.remove")}
        </Button>
      </div>
    </article>
  );
}

function ServiceModal({ open, service, catalog, hosts, onClose, onSaved }: {
  open: boolean;
  service?: LanService;
  catalog: import("../../types").LanServiceCatalogEntry[];
  hosts: import("../../types").LanHost[];
  onClose: () => void;
  onSaved: (name: string) => void;
}) {
  const { t } = useTranslation();
  const [form, setForm] = useState<FormState>(emptyForm());
  const [urlMode, setUrlMode] = useState(false);
  const [busy, setBusy] = useState(false);
  const [fail, setFail] = useState<string>();

  useEffect(() => {
    if (open) {
      setForm(toForm(service));
      setUrlMode(!!service?.url);
      setFail(undefined);
      setBusy(false);
    }
  }, [open, service]);

  const set = (patch: Partial<FormState>) => setForm((f) => ({ ...f, ...patch }));

  const pickKind = (kind: string) => {
    if (kind === "custom") {
      set({ kind });
      return;
    }
    const entry = catalog.find((c) => c.kind === kind);
    set({
      kind,
      name: form.name.trim() === "" ? t(`lanservices.kind.${kind}`) : form.name,
      scheme: entry?.scheme ?? form.scheme,
      port: entry ? String(entry.port) : form.port,
      path: entry?.path ?? form.path,
    });
  };

  const close = () => { setBusy(false); onClose(); };

  const submit = async () => {
    const name = form.name.trim();
    const svc: LanService = urlMode
      ? { id: form.id, name, kind: form.kind, url: form.url.trim(), enabled: true }
      : {
          id: form.id,
          name,
          kind: form.kind,
          host: form.host.trim(),
          port: parseInt(form.port, 10) || undefined,
          scheme: form.scheme || "http",
          path: form.path.trim(),
          enabled: true,
        };
    setBusy(true);
    setFail(undefined);
    try {
      await api.upsertLanService(svc);
      onSaved(name);
    } catch (e) {
      setFail(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const valid = form.name.trim().length > 0
    && (urlMode ? form.url.trim().length > 0 : form.host.trim().length > 0 && parseInt(form.port, 10) > 0);

  const kindOptions = [
    ...catalog.map((c) => ({ value: c.kind, label: t(`lanservices.kind.${c.kind}`) })),
    { value: "custom", label: t("lanservices.kindCustom") },
  ];

  return (
    <Modal
      open={open}
      onClose={close}
      wide
      title={service ? t("lanservices.editTitle") : t("lanservices.addTitle")}
      footer={
        <>
          <Button variant="ghost" onClick={close}>{t("common.cancel")}</Button>
          <Button onClick={submit} disabled={!valid} loading={busy}>
            {busy ? t("lanservices.saving") : t("lanservices.save")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <label className="block">
          <span className="mb-1.5 flex items-center gap-1 text-caption font-medium text-muted">
            {t("lanservices.kindLabel")}
          </span>
          <select
            value={form.kind}
            onChange={(e) => pickKind(e.target.value)}
            className="w-full h-[var(--input-h)] rounded-[10px] border border-transparent bg-fill px-3 text-body outline-none
              hover:border-border-strong focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-accent)]
              transition-[background-color,border-color,box-shadow] duration-[var(--dur-fast)]"
          >
            {kindOptions.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </label>

        <Field
          label={t("lanservices.nameLabel")}
          inputProps={{
            value: form.name,
            onChange: (e) => set({ name: e.target.value }),
            placeholder: t("lanservices.namePlaceholder"),
            autoFocus: true,
          }}
        />

        <SegmentedControl
          ariaLabel={t("lanservices.modeLabel")}
          value={urlMode ? "url" : "host"}
          onChange={(v) => setUrlMode(v === "url")}
          options={[
            { value: "host", label: t("lanservices.modeHost") },
            { value: "url", label: t("lanservices.modeUrl") },
          ]}
        />

        {urlMode ? (
          <Field
            label={t("lanservices.urlLabel")}
            hint={t("lanservices.urlHint")}
            mono
            inputProps={{
              value: form.url,
              onChange: (e) => set({ url: e.target.value }),
              placeholder: "https://home.example.com",
            }}
          />
        ) : (
          <>
            <Field
              label={t("lanservices.hostLabel")}
              hint={t("lanservices.hostHint")}
              mono
              inputProps={{
                value: form.host,
                onChange: (e) => set({ host: e.target.value }),
                placeholder: t("lanservices.hostPlaceholder"),
                list: "lanservices-hosts",
              }}
            />
            <datalist id="lanservices-hosts">
              {hosts.map((h) => (
                <option key={`${h.name}-${h.ip}`} value={h.name}>{h.ip}</option>
              ))}
            </datalist>
            <div className="grid grid-cols-2 gap-3">
              <Field
                label={t("lanservices.schemeLabel")}
                inputProps={{
                  value: form.scheme,
                  onChange: (e) => set({ scheme: e.target.value }),
                }}
              />
              <Field
                label={t("lanservices.portLabel")}
                mono
                inputProps={{
                  value: form.port,
                  onChange: (e) => set({ port: e.target.value.replace(/[^0-9]/g, "") }),
                  inputMode: "numeric",
                  placeholder: "8096",
                }}
              />
            </div>
            <Field
              label={t("lanservices.pathLabel")}
              hint={t("lanservices.pathHint")}
              mono
              inputProps={{
                value: form.path,
                onChange: (e) => set({ path: e.target.value }),
                placeholder: "/web",
              }}
            />
          </>
        )}

        {fail && (
          <div className="rounded-md bg-danger-soft px-3.5 py-3 animate-banner-in" role="alert">
            <p className="text-small text-danger">{t("lanservices.formError")}</p>
            <AdvancedDisclosure label={t("lanservices.technicalDetail")} className="mt-1">
              <pre className="max-h-32 overflow-auto rounded-sm bg-surface/60 border border-border p-2 font-mono text-caption whitespace-pre-wrap">
                {fail}
              </pre>
            </AdvancedDisclosure>
          </div>
        )}
      </div>
    </Modal>
  );
}
