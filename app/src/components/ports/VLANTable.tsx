import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Network, Pencil, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { VLANPort, VLANProbe } from "../../types";
import { Banner, Button, Card, ConfirmDialog, Input } from "../ui";
import { compressRanges, portNum } from "../../lib/vlanRanges";

// Los tres estados de una boca en la VLAN cargada en el editor,
// escritos en UCI como: no miembro, sin tag ("lan2:u", con "*": tambien
// es la VLAN de entrada de la boca) y con tag ("lan2:t") (#485).
type CellState = "empty" | "untagged" | "tagged";

const CELL_LABEL: Record<CellState, string> = {
  empty: "-",
  untagged: "U",
  tagged: "T",
};

// "-" azul (no miembro), U verde, T teal: los tres distinguibles a ojo.
const CELL_ACTIVE: Record<CellState, string> = {
  empty: "bg-accent-soft text-accent",
  untagged: "bg-success-soft text-success",
  tagged: "bg-teal-soft text-teal",
};

/** Bocas por banda de la matriz: 52 bocas = 4 bandas de 13, sin scroll. */
const BAND = 13;

/**
 * VLANs (#485), rediseño con el editor centrado en una VLAN: la primera
 * tarjeta resume todas (miembros comprimidos en rangos) y la segunda
 * carga una VID en un editor con la matriz por bandas de 13 bocas:
 * numeros de boca en columnas, fila "Miembro" con control de 3 estados
 * y fila "PVID" con un checkbox por boca. Aplicar envia la membresia
 * completa de UNA VLAN (excluidas las bocas en "-"); el backend rechaza
 * con mensaje claro los conflictos (una boca solo puede ser sin tag en
 * una VLAN) y aqui se muestran tal cual.
 */
export function VLANTable() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<VLANProbe>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [selected, setSelected] = useState(""); // "" | "new" | vid
  const [newVid, setNewVid] = useState("");
  const [editVid, setEditVid] = useState<number | null>(null);
  const [editName, setEditName] = useState("");
  const [members, setMembers] = useState<Record<string, CellState>>({});
  const [pvid, setPvid] = useState<Set<string>>(new Set());
  const [nameOverrides, setNameOverrides] = useState<Record<number, string>>({});
  const [confirmDelete, setConfirmDelete] = useState(false);

  const load = () => api.vlans().then(setProbe).catch(() => {});
  useEffect(() => { load(); }, []);

  if (!probe?.applicable) return null;

  // Orden numerico (lan1, lan2, ..., lan52), no lexico (lan1, lan10...).
  const sortedPorts = [...probe.ports].sort((a, b) => {
    const na = portNum(a);
    const nb = portNum(b);
    return (Number.isNaN(na) ? Infinity : na) - (Number.isNaN(nb) ? Infinity : nb);
  });
  const portLabel = (p: string) => {
    const n = portNum(p);
    return Number.isNaN(n) ? p : String(n);
  };
  const bands: string[][] = [];
  for (let i = 0; i < sortedPorts.length; i += BAND) bands.push(sortedPorts.slice(i, i + BAND));

  const vlanByVid = (vid: number) => probe.vlans.find((v) => v.vid === vid);
  const edited = editVid !== null ? vlanByVid(editVid) : undefined;
  const isProtected = edited?.default ?? false;

  // La boca ya es sin tag en otra VLAN: su "*" vive alli y el backend
  // rechazaria repetir el sin tag aqui.
  const untaggedElsewhere = (vid: number, port: string) =>
    probe.vlans.some((v) => v.vid !== vid &&
      v.ports.some((p) => p.port === port && !p.tagged));

  /** Carga la membresia de una VID en el editor (vacía si es nueva). */
  const loadEditor = (vid: number) => {
    const vlan = vlanByVid(vid);
    const m: Record<string, CellState> = {};
    const pv = new Set<string>();
    for (const p of sortedPorts) m[p] = "empty";
    for (const p of vlan?.ports ?? []) {
      m[p.port] = p.tagged ? "tagged" : "untagged";
      if (!p.tagged && p.pvid) pv.add(p.port);
    }
    setEditVid(vid);
    setEditName(nameOverrides[vid] ?? (vlan && vlan.name !== `VLAN ${vid}` ? vlan.name : ""));
    setMembers(m);
    setPvid(pv);
    setError("");
  };

  const handleLoad = () => {
    const vid = selected === "new" ? parseInt(newVid, 10) : parseInt(selected, 10);
    if (!Number.isInteger(vid) || vid < 2 || vid > 4094) {
      setError(t("vlan.invalidVid"));
      return;
    }
    loadEditor(vid);
  };

  const toggleMember = (port: string, state: CellState) => {
    if (editVid === null) return;
    setMembers((prev) => ({ ...prev, [port]: state }));
    setPvid((prev) => {
      const next = new Set(prev);
      // Al pasar a sin tag se lleva la "*" si la boca no la tiene ya en
      // otra VLAN; salir del sin tag la suelta.
      if (state === "untagged") {
        if (!untaggedElsewhere(editVid, port)) next.add(port);
      } else {
        next.delete(port);
      }
      return next;
    });
  };

  const apply = async () => {
    if (editVid === null) return;
    const ports: VLANPort[] = sortedPorts
      .filter((p) => members[p] !== "empty")
      .map((p) => ({
        port: p,
        tagged: members[p] === "tagged",
        pvid: members[p] === "untagged" && pvid.has(p),
      }));
    setLoading(true);
    setError("");
    try {
      const res = await api.setVlan({ vid: editVid, ports });
      if (res.status === "applied") {
        setProbe(res.state);
        const name = editName.trim();
        if (name) setNameOverrides((prev) => ({ ...prev, [editVid]: name }));
        setSelected(String(editVid));
        loadEditor(editVid);
      } else {
        setError(res.error || t("fwd.failed"));
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const deleteVLAN = async () => {
    if (editVid === null) return;
    setLoading(true);
    setError("");
    try {
      const res = await api.deleteVlan(editVid);
      if (res.status === "applied") {
        setProbe(res.state);
        setEditVid(null);
        setMembers({});
        setPvid(new Set());
        setSelected("");
      } else {
        setError(res.error || t("fwd.failed"));
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
      setConfirmDelete(false);
    }
  };

  const rangeOf = (vlanPorts: VLANPort[], pick: (p: VLANPort) => boolean) =>
    compressRanges(vlanPorts.filter(pick).map((p) => portNum(p.port)));

  const summaryName = (v: { vid: number; name: string }) =>
    nameOverrides[v.vid] ?? v.name;

  const segButton = (port: string, state: CellState) => {
    const active = members[port] === state;
    return (
      <button
        key={state}
        type="button"
        onClick={() => toggleMember(port, state)}
        disabled={loading || isProtected}
        aria-pressed={active}
        aria-label={`${portLabel(port)}: ${t("vlan." + state)}`}
        title={`${portLabel(port)}: ${t("vlan." + state)}`}
        className={`w-6 h-6 text-[10px] font-mono font-semibold ring-focus transition-colors duration-[var(--dur-fast)]
          ${state !== "empty" ? "border-l border-border" : ""}
          ${active ? CELL_ACTIVE[state] : "bg-surface text-faint hover:bg-surface-2"}
          ${loading || isProtected ? "cursor-not-allowed opacity-60" : "cursor-pointer"}`}
      >
        {CELL_LABEL[state]}
      </button>
    );
  };

  return (
    <>
      <Card variant="subtle" animate={false} icon={Network} title={t("vlan.title")} help="vlan">
        <Banner tone="warn" className="mb-3">{t("vlan.backupNote")}</Banner>
        <div className="overflow-x-auto rounded-sm border border-border/60">
          <table className="w-full border-collapse text-small">
            <thead>
              <tr className="text-caption text-muted">
                <th className="text-left py-1.5 pl-2 pr-2 font-medium border-b border-border">{t("vlan.colVid")}</th>
                <th className="text-left py-1.5 pr-2 font-medium border-b border-border">{t("vlan.colName")}</th>
                <th className="text-left py-1.5 pr-2 font-medium border-b border-border">{t("vlan.colMembers")}</th>
                <th className="text-left py-1.5 pr-2 font-medium border-b border-border">{t("vlan.colTagged")}</th>
                <th className="text-left py-1.5 pr-2 font-medium border-b border-border">{t("vlan.colUntagged")}</th>
                <th className="text-left py-1.5 pr-2 font-medium border-b border-border">{t("vlan.colPvid")}</th>
              </tr>
            </thead>
            <tbody>
              {probe.vlans.map((v) => (
                <tr key={v.vid} className="border-b border-border/40 last:border-0">
                  <td className="py-1.5 pl-2 pr-2 font-mono font-semibold">{v.vid}</td>
                  <td className="py-1.5 pr-2">{summaryName(v)}</td>
                  <td className="py-1.5 pr-2 font-mono text-caption">{rangeOf(v.ports, () => true)}</td>
                  <td className="py-1.5 pr-2 font-mono text-caption">{rangeOf(v.ports, (p) => p.tagged)}</td>
                  <td className="py-1.5 pr-2 font-mono text-caption">{rangeOf(v.ports, (p) => !p.tagged)}</td>
                  <td className="py-1.5 pr-2 font-mono text-caption">{rangeOf(v.ports, (p) => !p.tagged && !!p.pvid)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {probe.vlans.some((v) => v.default) && (
          <p className="text-caption text-faint mt-2">{t("vlan.protectedNote")}</p>
        )}
      </Card>

      <Card variant="subtle" animate={false} icon={Pencil} title={t("vlan.editorTitle")}>
        <div className="flex flex-wrap items-end gap-2">
          <label className="flex flex-col gap-1">
            <span className="text-caption text-muted">{t("vlan.selectVid")}</span>
            <select
              value={selected}
              onChange={(e) => setSelected(e.target.value)}
              className="h-9 rounded-sm border border-border bg-surface px-2 text-small outline-none focus:border-accent ring-focus"
            >
              <option value="">{t("vlan.optionPick")}</option>
              {probe.vlans.map((v) => (
                <option key={v.vid} value={v.vid}>
                  {v.vid} · {summaryName(v)}
                </option>
              ))}
              <option value="new">{t("vlan.optionNew")}</option>
            </select>
          </label>
          {selected === "new" && (
            <Input
              type="number"
              min={2}
              max={4094}
              value={newVid}
              onChange={(e) => setNewVid(e.target.value)}
              placeholder={t("vlan.newVid")}
              mono
              className="!w-32"
              aria-label={t("vlan.newVid")}
            />
          )}
          <Button size="sm" variant="secondary" onClick={handleLoad} disabled={loading || !selected || (selected === "new" && !newVid)}>
            {t("vlan.load")}
          </Button>
          {editVid !== null && (
            <>
              <label className="flex flex-col gap-1 flex-1 min-w-36">
                <span className="text-caption text-muted">{t("vlan.nameLabel")}</span>
                <Input
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  placeholder={t("vlan.namePlaceholder")}
                  className="!h-9"
                  disabled={loading || isProtected}
                />
              </label>
              <Button size="sm" variant="danger" icon={Trash2} onClick={() => setConfirmDelete(true)}
                disabled={loading || isProtected || !edited}>
                {t("vlan.deleteBtn")}
              </Button>
              <Button size="sm" variant="primary" onClick={apply} disabled={loading || isProtected}>
                {t("vlan.apply")}
              </Button>
            </>
          )}
        </div>

        {editVid === null ? (
          <p className="text-small text-muted mt-3">{t("vlan.notLoaded")}</p>
        ) : (
          <>
            <p className="text-small text-muted mt-3 mb-2">
              {t("vlan.editing", { vid: editVid })}
              {isProtected && <span className="text-warn"> {t("vlan.protectedNote")}</span>}
            </p>
            <div className="flex flex-col gap-2">
              {bands.map((band) => (
                <div key={band[0]} className="rounded-sm border border-border/60 overflow-hidden">
                  <table className="w-full border-collapse text-small table-fixed">
                    <thead>
                      <tr className="text-caption text-muted bg-surface-2">
                        <th className="w-16 text-left py-1 pl-2 pr-2 font-medium border-b border-border/60" />
                        {band.map((p) => (
                          <th key={p} className="py-1 px-0.5 text-center font-mono font-medium border-b border-border/60">
                            {portLabel(p)}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      <tr>
                        <td className="py-1 pl-2 pr-2 text-caption text-muted border-r border-border/60">
                          {t("vlan.rowMember")}
                        </td>
                        {band.map((p) => (
                          <td key={p} className="py-1 px-0.5 text-center">
                            <div role="group" aria-label={`${p} · ${t("vlan.rowMember")}`}
                              className="inline-flex rounded-sm border border-border overflow-hidden">
                              {(["empty", "untagged", "tagged"] as CellState[]).map((s) => segButton(p, s))}
                            </div>
                          </td>
                        ))}
                      </tr>
                      <tr className="border-t border-border/40">
                        <td className="py-1 pl-2 pr-2 text-caption text-muted border-r border-border/60">
                          {t("vlan.rowPvid")}
                        </td>
                        {band.map((p) => (
                          <td key={p} className="py-1 px-0.5 text-center">
                            <input
                              type="checkbox"
                              checked={pvid.has(p)}
                              disabled={loading || isProtected || members[p] !== "untagged"}
                              onChange={() => {
                                setPvid((prev) => {
                                  const next = new Set(prev);
                                  if (next.has(p)) next.delete(p);
                                  else next.add(p);
                                  return next;
                                });
                              }}
                              aria-label={`${portLabel(p)} · ${t("vlan.rowPvid")}`}
                              className="accent-accent h-3.5 w-3.5 cursor-pointer disabled:cursor-not-allowed"
                            />
                          </td>
                        ))}
                      </tr>
                    </tbody>
                  </table>
                </div>
              ))}
            </div>
            <div className="flex flex-wrap gap-x-4 gap-y-1 mt-3 text-caption text-muted">
              <span><span className="inline-block w-3 h-3 rounded-sm bg-accent-soft border border-accent/30 mr-1 align-middle" /> - = {t("vlan.empty")}</span>
              <span><span className="inline-block w-3 h-3 rounded-sm bg-success-soft border border-success/30 mr-1 align-middle" /> U = {t("vlan.untagged")}</span>
              <span><span className="inline-block w-3 h-3 rounded-sm bg-teal-soft border border-teal/30 mr-1 align-middle" /> T = {t("vlan.tagged")}</span>
              <span className="w-full sm:w-auto">{t("vlan.legendPvid")}</span>
            </div>
          </>
        )}

        {error && <p className="text-danger text-caption mt-2">{error}</p>}

        <ConfirmDialog
          open={confirmDelete}
          onClose={() => setConfirmDelete(false)}
          onConfirm={deleteVLAN}
          title={t("vlan.deleteTitle", { vid: editVid ?? "" })}
          consequence={t("vlan.deleteConsequence")}
          confirmLabel={t("vlan.deleteConfirm")}
          busy={loading}
        />
      </Card>
    </>
  );
}
