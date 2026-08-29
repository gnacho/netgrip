import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Camera, CloudOff, Download, GitCompareArrows } from "lucide-react";
import { api } from "../../api";
import type { ConfigDiff, ConfigSnapshot } from "../../types";
import {
  ActionBanner, Banner, Button, Card, ConfirmDialog, EmptyState, Modal, Pill, SkeletonRows,
} from "../ui";
import { IlluShield } from "../ui/illustrations";
import { useActionCycle } from "../wifi/action";
import { backupAge, asApplied, fmtRelDate, lineDiff } from "./shared";

/** Señal visual de antigüedad de la copia (design-rev2 §5): success <24h,
 *  warn >7 días, muted en medio. */
function agePill(t: TFunction, ts: number): { tone: "ok" | "warn" | "muted"; label: string } {
  const age = backupAge(ts);
  if (age.kind === "fresh") return { tone: "ok", label: t("tools.ageFresh") };
  if (age.kind === "old") return { tone: "warn", label: t("tools.ageOld") };
  return { tone: "muted", label: t("tools.ageDays", { count: age.days }) };
}

/**
 * Copias de seguridad (tools.md §2): héroe con última copia + pill
 * "Protegido", botón primario siempre visible, tabla con acciones ghost
 * (Comparar / Exportar / Restaurar / Borrar) y modal de diff.
 */
export function SnapshotsCard() {
  const { t } = useTranslation();
  const [snaps, setSnaps] = useState<ConfigSnapshot[]>();
  const [error, setError] = useState(false);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [failMsg, setFailMsg] = useState<string>();
  const [confirm, setConfirm] = useState<{ kind: "rollback" | "delete"; id: string }>();
  const [diffFrom, setDiffFrom] = useState<string>();
  const [diffPair, setDiffPair] = useState<{ from: string; to: string }>();
  const cycle = useActionCycle();

  const load = useCallback(async () => {
    setError(false);
    try {
      const r = await api.snapshots();
      setSnaps(r.snapshots ?? []);
    } catch {
      setError(true);
    }
  }, []);
  useEffect(() => { load(); }, [load]);

  const create = async () => {
    setCreating(true);
    setFailMsg(undefined);
    try { await api.createSnapshot(); await load(); }
    catch (e) { setFailMsg(e instanceof Error ? e.message : String(e)); }
    finally { setCreating(false); }
  };

  const rollback = (id: string) =>
    cycle.run(async () => asApplied(await api.rollbackSnapshot(id)));

  const del = async (id: string) => {
    setDeleting(true);
    setFailMsg(undefined);
    try { await api.deleteSnapshot(id); await load(); }
    catch (e) { setFailMsg(e instanceof Error ? e.message : String(e)); }
    finally { setDeleting(false); setConfirm(undefined); }
  };

  const compare = (id: string) => {
    if (!diffFrom) setDiffFrom(id);
    else if (diffFrom === id) setDiffFrom(undefined);
    else { setDiffPair({ from: diffFrom, to: id }); setDiffFrom(undefined); }
  };

  const latest = snaps?.[0];

  return (
    <Card
      index={0}
      title={t("tools.snapshotsTitle")}
      icon={Camera}
      help="snapshots"
      action={snaps && snaps.length > 0 ? <Pill tone="ok">{t("tools.protected")}</Pill> : undefined}
    >
      {error ? (
        <EmptyState
          small
          illustration={<CloudOff size={24} />}
          title={t("tools.loadError")}
          action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}
        />
      ) : !snaps ? (
        <SkeletonRows rows={3} />
      ) : snaps.length === 0 ? (
        <EmptyState
          illustration={<IlluShield size={96} />}
          title={t("tools.noSnapshotsTitle")}
          body={t("tools.noSnapshotsBody")}
          action={
            <Button icon={Camera} onClick={create} loading={creating}>
              {creating ? t("tools.creating") : t("tools.createNow")}
            </Button>
          }
        />
      ) : (
        <>
          <p className="text-small text-muted mb-3">
            {t("tools.lastCopy", { when: fmtRelDate(t, latest!.timestamp), count: latest!.configs })}
          </p>
          <div className="flex flex-wrap items-center gap-3">
            <Button icon={Camera} onClick={create} loading={creating}>
              {creating ? t("tools.creating") : t("tools.newSnapshot")}
            </Button>
            {diffFrom && <p className="text-small text-accent">{t("tools.compareHint")}</p>}
          </div>

          <div className="mt-2 divide-y divide-border/60">
            {snaps.map((s, i) => (
              <div key={s.id} style={{ "--i": Math.min(i + 1, 7) } as CSSProperties}
                className="animate-fade-up flex flex-wrap items-center gap-x-2 gap-y-1.5 py-2.5">
                <div className="flex-1 min-w-[140px]">
                  <div className="flex items-center gap-2 min-w-0">
                    <p className="text-body whitespace-nowrap overflow-hidden text-ellipsis" title={fmtRelDate(t, s.timestamp)}>
                      {fmtRelDate(t, s.timestamp)}
                    </p>
                    {(() => { const a = agePill(t, s.timestamp); return <Pill tone={a.tone}>{a.label}</Pill>; })()}
                  </div>
                  <p className="text-caption text-muted">{t("tools.filesCount", { count: s.configs })}</p>
                </div>
                {diffFrom === s.id && <Pill tone="accent">{t("tools.compareBase")}</Pill>}
                <div className="flex items-center">
                  <Button variant="ghost" size="sm" icon={GitCompareArrows} onClick={() => compare(s.id)}>
                    {t("tools.diff")}
                  </Button>
                  <a
                    href={`/api/config/snapshot/export?id=${encodeURIComponent(s.id)}`}
                    download
                    title={t("tools.export")}
                    aria-label={t("tools.export")}
                    className="inline-flex h-9 w-9 items-center justify-center rounded-md text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors"
                  >
                    <Download size={16} aria-hidden="true" />
                  </a>
                  <Button variant="ghost" size="sm" onClick={() => setConfirm({ kind: "rollback", id: s.id })}>
                    {t("tools.rollback")}
                  </Button>
                  <Button variant="ghost" size="sm"
                    onClick={() => setConfirm({ kind: "delete", id: s.id })}>
                    {t("tools.delete")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {failMsg && <Banner tone="danger" className="mt-3" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}

      {cycle.phase && (
        <div className="mt-3">
          <ActionBanner
            phase={cycle.phase}
            text={
              cycle.phase === "applying" ? t("tools.restoreApplying")
                : cycle.phase === "done" ? t("tools.restoreDone")
                  : cycle.phase === "failed" ? t("tools.restoreFailed")
                    : undefined
            }
            detail={cycle.detail}
            onDone={cycle.clear}
          />
        </div>
      )}

      <ConfirmDialog
        open={confirm?.kind === "rollback"}
        onClose={() => setConfirm(undefined)}
        onConfirm={() => { const id = confirm!.id; setConfirm(undefined); rollback(id); }}
        title={t("tools.rollbackConfirmTitle")}
        consequence={t("tools.rollbackConsequence")}
        confirmLabel={t("tools.rollbackGo")}
      />
      <ConfirmDialog
        open={confirm?.kind === "delete"}
        onClose={() => setConfirm(undefined)}
        onConfirm={() => del(confirm!.id)}
        title={t("tools.deleteConfirmTitle")}
        consequence={t("tools.deleteConsequence")}
        confirmLabel={t("tools.deleteGo")}
        busy={deleting}
      />

      {diffPair && <DiffModal pair={diffPair} onClose={() => setDiffPair(undefined)} />}
    </Card>
  );
}

/* ══════════════ Modal de diff (mismo patrón que Drift del Overview) ══════════════ */

function DiffModal({ pair, onClose }: {
  pair: { from: string; to: string };
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [diffs, setDiffs] = useState<ConfigDiff[]>();
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setError(false);
    setDiffs(undefined);
    try {
      const r = await api.snapshotDiff(pair.from, pair.to);
      setDiffs(r.diffs ?? []);
    } catch {
      setError(true);
    }
  }, [pair]);
  useEffect(() => { load(); }, [load]);

  return (
    <Modal open onClose={onClose} title={t("tools.diffModalTitle")} wide>
      {error ? (
        <Banner tone="danger"
          action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}>
          {t("tools.loadError")}
        </Banner>
      ) : !diffs ? (
        <SkeletonRows rows={4} />
      ) : diffs.length === 0 ? (
        <Banner tone="ok">{t("tools.diffEmpty")}</Banner>
      ) : (
        diffs.map((d) => {
          const lines = lineDiff(d.before, d.after);
          const added = lines.filter((l) => l.kind === "added").length;
          const removed = lines.length - added;
          return (
            <div key={d.config} className="mb-3 rounded-md border border-border overflow-hidden">
              <header className="flex items-center gap-2 border-b border-border/60 bg-surface-2 px-3 py-2">
                <span className="font-mono text-small font-medium">{d.config}</span>
                <span className="ml-auto text-caption font-semibold tabular-nums">
                  <span className="text-ok">+{added}</span>
                  {" "}
                  <span className="text-danger">−{removed}</span>
                </span>
              </header>
              <div className="max-h-56 overflow-auto px-3 py-2 font-mono text-caption">
                {lines.map((l, i) => (
                  <div key={i} className={l.kind === "added" ? "text-ok" : "text-danger"}>
                    {l.kind === "added" ? "+" : "−"} {l.text}
                  </div>
                ))}
              </div>
            </div>
          );
        })
      )}
    </Modal>
  );
}
