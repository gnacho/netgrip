import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link2, Plus, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { LAGEntry, LAGProbe } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Button, Card, ConfirmDialog, Field, Input,
  Pill, SegmentedControl,
} from "../ui";
import { useActionCycle } from "../wifi/action";

type LagMode = "802.3ad" | "active-backup" | "balance-rr";

/**
 * LAG / agregado de enlaces (issue #73): agrupa varias bocas en un solo
 * enlace (LACP). Solo en hardware con bridge y 2+ bocas físicas.
 */
export function LagCard({ index = 2 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<LAGProbe>();
  const [delTarget, setDelTarget] = useState<LAGEntry>();
  const { phase, detail, busy, run, clear } = useActionCycle();

  const [name, setName] = useState("lag0");
  const [mode, setMode] = useState<LagMode>("802.3ad");
  const [selected, setSelected] = useState<string[]>([]);

  const load = useCallback(() => {
    api.lag().then(setProbe).catch(() => {});
  }, []);

  useEffect(() => { load(); }, [load]);

  if (probe && !probe.applicable) return null;
  if (!probe) return null;

  const togglePort = (port: string) => {
    setSelected((prev) =>
      prev.includes(port) ? prev.filter((p) => p !== port) : [...prev, port],
    );
  };

  const create = async () => {
    const res = await run(() => api.setLag({ name: name.trim(), mode, slaves: selected }));
    if (res) {
      setProbe(res.state);
      setSelected([]);
    }
  };

  const remove = async (lag: LAGEntry) => {
    const res = await run(() => api.deleteLag(lag.name));
    if (res) setProbe(res.state);
    setDelTarget(undefined);
  };

  const modeLabel = (m: string) =>
    m === "802.3ad" ? t("lag.modeLacp") : m === "active-backup" ? t("lag.modeBackup") : t("lag.modeRoundRobin");

  const canCreate = /^[a-z][a-z0-9_]{0,12}$/.test(name.trim()) && selected.length >= 2;

  return (
    <Card index={index} icon={Link2} title={t("lag.title")} help="lag">
      {phase && (
        <div className="mb-2">
          <ActionBanner phase={phase} detail={detail} onDone={clear} />
        </div>
      )}

      {(probe.lags ?? []).length === 0 ? (
        <p className="text-small text-muted">{t("lag.empty")}</p>
      ) : (
        <div className="divide-y divide-border/60">
          {(probe.lags ?? []).map((lag) => (
            <div key={lag.name} className="flex items-center gap-3 py-2.5">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-medium">{lag.name}</span>
                  <Pill tone={lag.up ? "ok" : "muted"}>{lag.up ? t("lag.up") : t("lag.down")}</Pill>
                  <Pill tone="accent">{modeLabel(lag.mode)}</Pill>
                </div>
                <p className="text-small text-muted mt-0.5">
                  {lag.slaves.join(" + ")}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                className="text-danger hover:text-danger hover:bg-danger/10"
                disabled={busy}
                onClick={() => setDelTarget(lag)}
                aria-label={t("lag.delete", { name: lag.name })}
              >
                <Trash2 size={16} />
              </Button>
            </div>
          ))}
        </div>
      )}

      <AdvancedDisclosure label={t("lag.add")} className="mt-1">
        <div className="flex flex-col gap-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Field label={t("lag.nameLabel")}>
              <Input value={name} onChange={(e) => setName(e.target.value)} mono />
            </Field>
            <div className="flex flex-col gap-1.5">
              <span className="text-caption text-muted">{t("lag.modeLabel")}</span>
              <SegmentedControl<LagMode>
                ariaLabel={t("lag.modeLabel")}
                value={mode}
                onChange={setMode}
                options={[
                  { value: "802.3ad", label: t("lag.modeLacp"), title: t("lag.modeLacpHint") },
                  { value: "active-backup", label: t("lag.modeBackup"), title: t("lag.modeBackupHint") },
                  { value: "balance-rr", label: t("lag.modeRoundRobin"), title: t("lag.modeRoundRobinHint") },
                ]}
              />
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <span className="text-caption text-muted">
              {t("lag.portsLabel")} ({selected.length}/{probe.free_ports.length})
            </span>
            <div className="flex flex-wrap gap-2">
              {probe.free_ports.map((port) => {
                const on = selected.includes(port);
                return (
                  <button
                    key={port}
                    type="button"
                    onClick={() => togglePort(port)}
                    aria-pressed={on}
                    className={`rounded-md border px-2.5 py-1.5 text-small font-medium ring-focus transition-colors duration-[var(--dur-fast)] ${
                      on
                        ? "border-accent bg-accent-soft text-accent"
                        : "border-border bg-surface text-muted hover:text-text hover:bg-surface-2"
                    }`}
                  >
                    {port}
                  </button>
                );
              })}
            </div>
          </div>

          <p className="text-caption text-muted">{t("lag.warn")}</p>
          {!probe.installed && <p className="text-caption text-muted">{t("lag.installHint")}</p>}

          <Button onClick={create} loading={busy} disabled={!canCreate}>
            <Plus size={16} /> {t("lag.add")}
          </Button>
        </div>
      </AdvancedDisclosure>

      <ConfirmDialog
        open={!!delTarget}
        title={t("lag.delete", { name: delTarget?.name ?? "" })}
        consequence={t("lag.deleteBody", { ports: delTarget?.slaves.join(", ") ?? "" })}
        confirmLabel={t("lag.deleteConfirm")}
        busy={busy}
        onClose={() => setDelTarget(undefined)}
        onConfirm={() => delTarget && remove(delTarget)}
      />
    </Card>
  );
}
