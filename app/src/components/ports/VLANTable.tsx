import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Network, Plus, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { VLAN, VLANPort, VLANProbe } from "../../types";
import { Banner, Button, Card, ConfirmDialog, Input, SkeletonRows } from "../ui";
import { compressRanges, portNum } from "../../lib/vlanRanges";

// Estado de una boca dentro de una VLAN, en el orden del ciclo al hacer
// clic en la celda (#487): "-" (no miembro) -> "U" (sin etiquetar) ->
// "T" (etiquetado) -> "U*" (sin etiquetar + PVID) -> "-". "U*" es unico
// por boca: al ponerlo en una VLAN se quita de la que lo tuviera.
type CellState = "empty" | "untagged" | "tagged" | "pvid";

const CYCLE: CellState[] = ["empty", "untagged", "tagged", "pvid"];

const CELL_LABEL: Record<CellState, string> = {
  empty: "-",
  untagged: "U",
  tagged: "T",
  pvid: "U*",
};

// Mismos tonos que el resto de la app: gris inactivo, verde sin etiquetar,
// teal etiquetado y anillo de acierto para el PVID.
const CELL_CLS: Record<CellState, string> = {
  empty: "bg-surface text-faint border-border hover:bg-surface-2",
  untagged: "bg-success-soft text-success border-success/30 hover:bg-success-soft/60",
  tagged: "bg-teal-soft text-teal border-teal/30 hover:bg-teal-soft/60",
  pvid: "bg-success-soft text-success border-success/60 ring-2 ring-success/40 hover:bg-success-soft/60",
};

/** Bocas por banda de la matriz: 52 bocas = 4 bandas de 13, sin scroll. */
const BAND = 13;

/** Membresia de una VLAN tal como llega del probe (la linea base del diff). */
const stateFromVlan = (vlan: VLAN | undefined, ports: string[]): Record<string, CellState> => {
  const m: Record<string, CellState> = {};
  for (const p of ports) m[p] = "empty";
  for (const vp of vlan?.ports ?? []) {
    m[vp.port] = vp.tagged ? "tagged" : vp.pvid ? "pvid" : "untagged";
  }
  return m;
};

/**
 * Matriz de VLANs (#487), estilo mockup de rediseño: las VLANs son FILAS
 * y las bocas COLUMNAS, en bandas de 13 bocas para que un chasis de 52
 * quepa sin scroll horizontal. Cada celda cicla - -> U -> T -> U* al
 * clic. "Aplicar" calcula el diff por VLAN contra el estado real y envia
 * POST /api/vlans secuenciales solo con las VLANs cambiadas, con progreso
 * y errores limpios; las nuevas (todavia no existentes) se crean al
 * aplicar y las que se quedan vacias no se envian. El borrado es por fila
 * (ConfirmDialog) y la VLAN que lleva la LAN del propio equipo (default)
 * no se edita ni se borra: el backend la rechaza y tocarla puede dejar
 * la sesion de management colgada (#327).
 */
export function VLANTable() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<VLANProbe>();
  const [draft, setDraft] = useState<Record<number, Record<string, CellState>>>({});
  const [newVid, setNewVid] = useState("");
  const [error, setError] = useState("");
  const [errors, setErrors] = useState<string[]>([]);
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [applying, setApplying] = useState(false);
  const [progress, setProgress] = useState<{ vid: number; done: number; total: number } | null>(null);

  useEffect(() => {
    api.vlans().then((p) => {
      setProbe(p);
      const d: Record<number, Record<string, CellState>> = {};
      for (const v of p.vlans) d[v.vid] = stateFromVlan(v, p.ports);
      setDraft(d);
    }).catch(() => {});
  }, []);

  if (probe && !probe.applicable) return null;

  // Orden numerico (lan1, lan2, ..., lan52), no lexico (lan1, lan10...).
  const sortedPorts = [...(probe?.ports ?? [])].sort((a, b) => {
    const na = portNum(a);
    const nb = portNum(b);
    return (Number.isNaN(na) ? Infinity : na) - (Number.isNaN(nb) ? Infinity : nb);
  });
  const bands: string[][] = [];
  for (let i = 0; i < sortedPorts.length; i += BAND) bands.push(sortedPorts.slice(i, i + BAND));
  const portLabel = (p: string) => {
    const n = portNum(p);
    return Number.isNaN(n) ? p : String(n);
  };

  const vids = Object.keys(draft).map(Number).sort((a, b) => a - b);
  const vlanByVid = (vid: number) => probe?.vlans.find((v) => v.vid === vid);

  /** La fila cambia respecto al estado real (diff por VLAN al aplicar). */
  const isChanged = (vid: number) => {
    const row = draft[vid];
    const base = stateFromVlan(vlanByVid(vid), sortedPorts);
    return sortedPorts.some((p) => row[p] !== base[p]);
  };
  const changedVids = vids.filter(isChanged);

  /** Bocas con mas de una VLAN sin etiquetar: el backend las rechazaria. */
  const conflictPorts = sortedPorts.filter((p) => {
    let n = 0;
    for (const vid of vids) {
      const s = draft[vid]?.[p];
      if (s === "untagged" || s === "pvid") n++;
    }
    return n > 1;
  });

  const cycle = (vid: number, port: string) => {
    if (applying || vlanByVid(vid)?.default) return;
    setError("");
    setDraft((prev) => {
      const row = { ...prev[vid] };
      row[port] = CYCLE[(CYCLE.indexOf(row[port]) + 1) % CYCLE.length];
      const out = { ...prev, [vid]: row };
      if (row[port] === "pvid") {
        // Unico PVID por boca: la quita de la VLAN que la tuviera.
        for (const other of Object.keys(out)) {
          if (Number(other) !== vid && out[Number(other)][port] === "pvid") {
            out[Number(other)] = { ...out[Number(other)], [port]: "untagged" };
          }
        }
      }
      return out;
    });
  };

  const add = () => {
    const vid = parseInt(newVid, 10);
    if (!Number.isInteger(vid) || vid < 2 || vid > 4094) {
      setError(t("vlan.invalidVid"));
      return;
    }
    if (draft[vid] !== undefined) {
      setError(t("vlan.addExists", { vid }));
      return;
    }
    const row: Record<string, CellState> = {};
    for (const p of sortedPorts) row[p] = "empty";
    setDraft((prev) => ({ ...prev, [vid]: row }));
    setNewVid("");
    setError("");
  };

  const payload = (vid: number): VLANPort[] =>
    sortedPorts
      .filter((p) => draft[vid][p] !== "empty")
      .map((p) => ({
        port: p,
        tagged: draft[vid][p] === "tagged",
        pvid: draft[vid][p] === "pvid",
      }));

  const apply = async () => {
    // Las nuevas sin ningun miembro no se envian: no aportan nada.
    const changes = changedVids.filter((vid) => vlanByVid(vid) || payload(vid).length > 0);
    if (!changes.length || !probe) return;
    setApplying(true);
    setError("");
    setErrors([]);
    let current = probe;
    for (let i = 0; i < changes.length; i++) {
      const vid = changes[i];
      setProgress({ vid, done: i, total: changes.length });
      try {
        const res = await api.setVlan({ vid, ports: payload(vid) });
        if (res.status === "applied") {
          current = res.state;
          // Linea base actualizada para esta VLAN: la fila deja de estar
          // "pendiente" pero el resto del borrador se conserva.
          setDraft((prev) => ({
            ...prev,
            [vid]: stateFromVlan(res.state.vlans.find((v) => v.vid === vid), sortedPorts),
          }));
        } else {
          setErrors((prev) => [...prev, res.error || t("fwd.failed")]);
        }
      } catch (e) {
        setErrors((prev) => [...prev, e instanceof Error ? e.message : String(e)]);
      }
    }
    setProbe(current);
    setProgress((p) => (p ? { ...p, done: p.total } : null));
    setApplying(false);
  };

  const deleteVLAN = async () => {
    const vid = confirmDelete;
    if (vid === null) return;
    setDeleting(true);
    setError("");
    try {
      const res = await api.deleteVlan(vid);
      if (res.status === "applied") {
        setProbe(res.state);
        setDraft((prev) => {
          const next = { ...prev };
          delete next[vid];
          return next;
        });
      } else {
        setError(res.error || t("fwd.failed"));
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setDeleting(false);
      setConfirmDelete(null);
    }
  };

  if (!probe) {
    return (
      <Card variant="subtle" animate={false} icon={Network} title={t("vlan.title")} help="vlan">
        <SkeletonRows rows={3} />
      </Card>
    );
  }

  return (
    <Card variant="subtle" animate={false} icon={Network} title={t("vlan.title")} help="vlan">
      <Banner tone="warn" className="mb-3">{t("vlan.backupNote")}</Banner>

      {/* Leyenda del ciclo de celda. */}
      <div className="flex flex-wrap gap-x-4 gap-y-1 mb-3 text-caption text-muted">
        {(Object.keys(CELL_LABEL) as CellState[]).map((s) => (
          <span key={s} className="inline-flex items-center gap-1.5">
            <span className={`inline-flex w-5 h-5 items-center justify-center rounded-sm border text-[10px] font-mono font-semibold ${CELL_CLS[s]}`}>
              {CELL_LABEL[s]}
            </span>
            {t("vlan." + s)}
          </span>
        ))}
        <span className="w-full sm:w-auto">{t("vlan.legendPvid")}</span>
      </div>

      {/* Bandas de bocas: cada una con su cabecera y las mismas filas de
          VLANs, para 52 bocas en 4 bandas de 13 sin scroll horizontal. */}
      <div className="flex flex-col gap-2">
        {bands.map((band) => (
          <div key={band[0]} className="rounded-sm border border-border/60 overflow-x-auto">
            <table className="w-full border-collapse text-small table-fixed">
              <thead>
                <tr className="text-caption text-muted bg-surface-2">
                  <th className="w-24 text-left py-1 pl-2 pr-2 font-medium border-b border-border/60">{t("vlan.colVid")}</th>
                  {band.map((p) => (
                    <th key={p} className="py-1 px-0.5 text-center font-mono font-medium border-b border-border/60">
                      {portLabel(p)}
                    </th>
                  ))}
                  <th className="w-36 text-left py-1 pl-2 font-medium border-b border-border/60">{t("vlan.colName")}</th>
                  <th className="w-9 border-b border-border/60" />
                </tr>
              </thead>
              <tbody>
                {vids.map((vid) => {
                  const vlan = vlanByVid(vid);
                  const isDefault = vlan?.default ?? false;
                  return (
                    <tr key={vid} className="border-b border-border/40 last:border-0">
                      <td className="py-1 pl-2 pr-2 font-mono font-semibold whitespace-nowrap">
                        {vid}
                        {isDefault && (
                          <span className="ml-1.5 px-1.5 py-0.5 rounded-full bg-fill text-faint text-[10px] font-semibold align-middle">
                            {t("vlan.defaultTag")}
                          </span>
                        )}
                        {!isDefault && isChanged(vid) && (
                          <span
                            className="inline-block ml-1.5 h-1.5 w-1.5 rounded-full bg-accent align-middle"
                            title={t("vlan.modified")}
                            aria-label={t("vlan.modified")}
                          />
                        )}
                      </td>
                      {band.map((p) => (
                        <td key={p} className="py-1 px-0.5 text-center">
                          <button
                            type="button"
                            onClick={() => cycle(vid, p)}
                            disabled={applying || isDefault}
                            aria-label={`${portLabel(p)} · ${t("vlan." + draft[vid][p])}`}
                            title={`${portLabel(p)} · ${t("vlan." + draft[vid][p])}`}
                            className={`w-7 h-7 text-[11px] font-mono font-semibold rounded-sm border transition-colors duration-[var(--dur-fast)] ring-focus
                              ${CELL_CLS[draft[vid][p]]}
                              ${applying || isDefault ? "cursor-not-allowed opacity-60" : "cursor-pointer"}`}
                          >
                            {CELL_LABEL[draft[vid][p]]}
                          </button>
                        </td>
                      ))}
                      <td className="py-1 pl-2 pr-1 truncate text-caption text-muted" title={vlan?.name ?? ""}>
                        {vlan?.name ?? "-"}
                      </td>
                      <td className="py-1 pr-2 text-center">
                        {!isDefault && vlan && (
                          <button
                            type="button"
                            onClick={() => setConfirmDelete(vid)}
                            disabled={applying}
                            aria-label={t("vlan.deleteTitle", { vid })}
                            title={t("vlan.deleteTitle", { vid })}
                            className="inline-flex w-7 h-7 items-center justify-center rounded-sm text-faint hover:text-danger hover:bg-danger-soft ring-focus transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                          >
                            <Trash2 size={14} />
                          </button>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ))}
      </div>

      {conflictPorts.length > 0 && (
        <Banner tone="warn" className="mt-3">
          {t("vlan.conflictWarn", { ports: compressRanges(conflictPorts.map(portNum)) })}
        </Banner>
      )}

      {/* Alta de VLAN: la fila se crea en el borrador y el POST llega al
          aplicar (el backend crea la seccion bridge-vlan al no existir). */}
      <div className="flex flex-wrap items-center gap-2 mt-3">
        <Input
          type="number"
          min={2}
          max={4094}
          value={newVid}
          onChange={(e) => setNewVid(e.target.value)}
          placeholder={t("vlan.addVid")}
          mono
          className="!w-32"
          aria-label={t("vlan.addVid")}
        />
        <Button size="sm" variant="secondary" icon={Plus} onClick={add} disabled={applying || !newVid}>
          {t("vlan.add")}
        </Button>
      </div>

      {/* Aplicar: POST secuenciales de las VLANs con diff, con progreso. */}
      <div className="flex flex-wrap items-center gap-3 mt-3">
        <Button size="sm" variant="primary" onClick={apply} disabled={applying || changedVids.length === 0} loading={applying}>
          {t("vlan.apply")}{changedVids.length > 0 ? ` (${changedVids.length})` : ""}
        </Button>
        {progress && (
          <span className="text-caption text-muted" role="status">
            {progress.done >= progress.total
              ? t("vlan.applyDone", { count: progress.total })
              : t("vlan.applying", { vid: progress.vid, done: progress.done + 1, total: progress.total })}
          </span>
        )}
      </div>

      {error && <p className="text-danger text-caption mt-2">{error}</p>}
      {errors.map((e, i) => (
        <p key={i} className="text-danger text-caption mt-1">{e}</p>
      ))}

      <ConfirmDialog
        open={confirmDelete !== null}
        onClose={() => setConfirmDelete(null)}
        onConfirm={deleteVLAN}
        title={t("vlan.deleteTitle", { vid: confirmDelete ?? "" })}
        consequence={t("vlan.deleteConsequence")}
        confirmLabel={t("vlan.deleteConfirm")}
        busy={deleting}
      />
    </Card>
  );
}
