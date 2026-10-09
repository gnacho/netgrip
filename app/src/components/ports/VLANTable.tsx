import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Network, Plus, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { SwitchProbe, VLANProbe, VLANPort } from "../../types";
import { Banner, Button, Card, ConfirmDialog, Input } from "../ui";

// Los tres estados de un puerto en una VLAN, escritos en UCI como:
// no miembro, sin tag (con "*": tambien es la VLAN por defecto del puerto,
// "lan2:u*") y con tag ("lan2:t"). Se ciclan con un clic en vez de un
// desplegable: asi caben 52 puertos como filas (#485).
type CellState = "empty" | "untagged" | "tagged";

const CYCLE: Record<CellState, CellState> = {
  empty: "untagged",
  untagged: "tagged",
  tagged: "empty",
};

const CELL_LABEL: Record<CellState, string> = {
  empty: "-",
  untagged: "U",
  tagged: "T",
};

/**
 * VLANs (#485): matriz transpuesta, mismo estilo que la tabla STP. Las
 * filas son los puertos (con su estado de enlace) y las columnas las VLANs;
 * la cabecera queda fija al hacer scroll vertical. La marca "*" de PVID es
 * unica por puerto: al fijar "sin tag" en una VLAN se lleva la marca si el
 * puerto no tenia ninguna.
 */
export function VLANTable() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<VLANProbe>();
  const [swProbe, setSwProbe] = useState<SwitchProbe>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [newVid, setNewVid] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<number>();

  const load = () => api.vlans().then(setProbe).catch(() => {});
  useEffect(() => { load(); }, []);
  useEffect(() => {
    api.switchPorts().then(setSwProbe).catch(() => {});
  }, []);

  if (!probe?.applicable) return null;

  const linkByName = new Map<string, boolean>();
  if (swProbe?.applicable) for (const p of swProbe.ports) linkByName.set(p.name, p.oper_up);

  const membership = (vid: number, port: string) =>
    probe.vlans.find((v) => v.vid === vid)?.ports.find((p) => p.port === port);

  const cellState = (vid: number, port: string): CellState => {
    const p = membership(vid, port);
    if (!p) return "empty";
    return p.tagged ? "tagged" : "untagged";
  };

  // El puerto ya es miembro sin tag de otra VLAN: su marca "*" vive alli.
  const untaggedElsewhere = (vid: number, port: string) =>
    probe.vlans.some((v) => v.vid !== vid &&
      v.ports.some((p) => p.port === port && !p.tagged));

  const setCell = async (vid: number, port: string) => {
    const vlan = probe.vlans.find((v) => v.vid === vid);
    if (!vlan) return;
    const next = CYCLE[cellState(vid, port)];

    const others = vlan.ports.filter((p) => p.port !== port);
    const newPorts: VLANPort[] =
      next === "empty"
        ? others
        : [...others, {
            port,
            tagged: next === "tagged",
            // Al marcar "sin tag" se lleva la "*" si el puerto no tenia
            // ninguna; si ya la tiene en otra VLAN el backend lo rechaza
            // con un mensaje claro (un puerto solo puede ser sin tag en
            // una VLAN).
            pvid: next === "untagged" && !untaggedElsewhere(vid, port),
          }];

    setLoading(true);
    setError("");
    try {
      const res = await api.setVlan({ vid, ports: newPorts });
      if (res.status === "applied") setProbe(res.state);
      else setError(res.error || t("fwd.failed"));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const addVLAN = async () => {
    const vid = parseInt(newVid, 10);
    if (!vid || vid < 2 || vid > 4094) {
      setError(t("vlan.invalidVid"));
      return;
    }
    setLoading(true);
    setError("");
    try {
      const res = await api.setVlan({ vid, ports: [] });
      if (res.status === "applied") {
        setProbe(res.state);
        setNewVid("");
      } else setError(res.error || t("fwd.failed"));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const deleteVLAN = async (vid: number) => {
    setLoading(true);
    setError("");
    try {
      const res = await api.deleteVlan(vid);
      if (res.status === "applied") setProbe(res.state);
      else setError(res.error || t("fwd.failed"));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
      setConfirmDelete(undefined);
    }
  };

  const cellClass = (state: CellState) => {
    switch (state) {
      case "tagged": return "bg-accent-soft text-accent font-semibold";
      case "untagged": return "bg-success-soft text-success font-semibold";
      default: return "bg-fill text-faint";
    }
  };

  return (
    <Card variant="subtle" animate={false} icon={Network} title={t("vlan.title")} help="vlan">
      <Banner tone="warn" className="mb-3">{t("vlan.backupNote")}</Banner>

      <div className="max-h-[65vh] overflow-auto rounded-sm border border-border/60">
        <table className="w-full border-collapse text-small">
          <thead>
            <tr className="text-caption text-muted">
              <th className="sticky left-0 top-0 z-20 bg-surface-2 text-left py-1.5 pl-2 pr-2 font-medium border-b border-r border-border">
                {t("vlan.colPort")}
              </th>
              {linkByName.size > 0 && (
                <th className="sticky top-0 z-10 bg-surface-2 text-left py-1.5 pr-2 font-medium border-b border-border">
                  {t("vlan.colLink")}
                </th>
              )}
              {probe.vlans.map((vlan) => (
                <th key={vlan.vid}
                  className="sticky top-0 z-10 bg-surface-2 px-1 py-1.5 text-center font-mono font-medium border-b border-border">
                  <span className="inline-flex items-center gap-1">
                    <span>{vlan.vid}</span>
                    {vlan.default && <span className="text-faint">*</span>}
                    {!vlan.default && (
                      <button
                        type="button"
                        onClick={() => setConfirmDelete(vlan.vid)}
                        disabled={loading}
                        aria-label={`${t("vlan.confirmDel")} ${vlan.vid}`}
                        className="text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm"
                      >
                        <Trash2 size={12} />
                      </button>
                    )}
                  </span>
                  {vlan.name && (
                    <span className="block font-sans font-normal text-faint text-[10px] truncate max-w-16" title={vlan.name}>
                      {vlan.name}
                    </span>
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {probe.ports.map((port) => (
              <tr key={port} className="border-b border-border/40 last:border-0">
                <td className="sticky left-0 z-10 bg-surface-2 py-1 pl-2 pr-2 font-mono border-r border-border/60">
                  {port}
                </td>
                {linkByName.size > 0 && (
                  <td className="py-1 pr-2">
                    <span
                      role="img"
                      aria-label={linkByName.get(port) ? t("ports.up") : t("ports.down")}
                      title={linkByName.get(port) ? t("ports.up") : t("ports.down")}
                      className={`inline-block h-2 w-2 rounded-full ${linkByName.get(port) ? "bg-ok" : "bg-border-strong"}`}
                    />
                  </td>
                )}
                {probe.vlans.map((vlan) => {
                  const state = cellState(vlan.vid, port);
                  const m = membership(vlan.vid, port);
                  const label = CELL_LABEL[state] + (state === "untagged" && m?.pvid ? "*" : "");
                  return (
                    <td key={vlan.vid} className="text-center px-0.5 py-1">
                      <button
                        type="button"
                        onClick={() => setCell(vlan.vid, port)}
                        disabled={loading || vlan.default}
                        aria-label={`VLAN ${vlan.vid} · ${port}: ${t("vlan." + state)}`}
                        title={`${vlan.vid} · ${port}: ${t("vlan." + state)}`}
                        className={`w-9 h-6 rounded-sm text-caption font-mono text-center ring-focus transition-colors duration-[var(--dur-fast)]
                          ${cellClass(state)} ${vlan.default ? "cursor-not-allowed opacity-60" : "cursor-pointer hover:opacity-80"}`}
                      >
                        {label}
                      </button>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-end gap-2 mt-3 pt-3 border-t border-border/50">
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
        <Button size="sm" variant="secondary" icon={Plus} onClick={addVLAN} disabled={loading || !newVid}>
          {t("vlan.add")}
        </Button>
      </div>

      <div className="flex flex-wrap gap-x-4 gap-y-1 mt-3 text-caption text-muted">
        <span><span className="inline-block w-3 h-3 rounded-sm bg-success-soft border border-success/30 mr-1 align-middle" /> U* = {t("vlan.untaggedPvid")}</span>
        <span><span className="inline-block w-3 h-3 rounded-sm bg-success-soft border border-success/30 mr-1 align-middle" /> U = {t("vlan.untagged")}</span>
        <span><span className="inline-block w-3 h-3 rounded-sm bg-accent-soft border border-accent/30 mr-1 align-middle" /> T = {t("vlan.tagged")}</span>
        <span><span className="inline-block w-3 h-3 rounded-sm bg-fill border border-border mr-1 align-middle" /> - = {t("vlan.notMember")}</span>
        <span className="w-full sm:w-auto">{t("vlan.defaultNote")}</span>
      </div>

      {error && <p className="text-danger text-caption mt-2">{error}</p>}

      <ConfirmDialog
        open={confirmDelete !== undefined}
        onClose={() => setConfirmDelete(undefined)}
        onConfirm={() => { if (confirmDelete !== undefined) deleteVLAN(confirmDelete); }}
        title={t("vlan.deleteTitle", { vid: confirmDelete ?? "" })}
        consequence={t("vlan.deleteConsequence")}
        confirmLabel={t("vlan.deleteConfirm")}
        busy={loading}
      />
    </Card>
  );
}
