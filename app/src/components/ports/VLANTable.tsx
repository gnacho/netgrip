import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Network, Plus, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { VLANProbe, VLANPort } from "../../types";
import { Banner, Button, Card, ConfirmDialog, Input } from "../ui";

type CellState = "empty" | "untagged" | "tagged";

/** VLANs (lan.md §5, vive en Puertos en el código real). Bajo Opciones avanzadas. */
export function VLANTable() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<VLANProbe>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [newVid, setNewVid] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<number>();

  const load = () => api.vlans().then(setProbe).catch(() => {});
  useEffect(() => { load(); }, []);

  if (!probe?.applicable) return null;

  const cellState = (vlan: number, port: string): CellState => {
    const v = probe.vlans.find((v) => v.vid === vlan);
    if (!v) return "empty";
    const p = v.ports.find((p) => p.port === port);
    if (!p) return "empty";
    return p.tagged ? "tagged" : "untagged";
  };

  const cycleCell = async (vid: number, port: string) => {
    const current = cellState(vid, port);
    const vlan = probe.vlans.find((v) => v.vid === vid);
    if (!vlan) return;

    let newPorts: VLANPort[];
    switch (current) {
      case "empty":
        newPorts = [...vlan.ports, { port, tagged: false }];
        break;
      case "untagged":
        newPorts = vlan.ports.map((p) =>
          p.port === port ? { ...p, tagged: true } : p
        );
        break;
      case "tagged":
        newPorts = vlan.ports.filter((p) => p.port !== port);
        break;
    }

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

  const cellLabel = (state: CellState) => {
    switch (state) {
      case "tagged": return "T";
      case "untagged": return "U";
      default: return "–";
    }
  };

  return (
    <Card variant="subtle" animate={false} icon={Network} title={t("vlan.title")} help="vlan">
      <Banner tone="warn" className="mb-3">{t("vlan.backupNote")}</Banner>

      <div className="overflow-x-auto">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className="text-left px-2 py-1.5 text-caption text-muted border-b border-border">VLAN</th>
              {probe.ports.map((port) => (
                <th key={port} className="text-center px-1 py-1.5 text-caption text-muted font-mono border-b border-border">{port}</th>
              ))}
              <th className="w-8 border-b border-border" />
            </tr>
          </thead>
          <tbody>
            {(probe.vlans ?? []).map((vlan) => (
              <tr key={vlan.vid}>
                <td className="px-2 py-1 font-mono text-small border-b border-border/50">
                  {vlan.vid}
                  {vlan.name && <span className="text-muted ml-1.5 font-sans text-caption">{vlan.name}</span>}
                  {vlan.default && <span className="text-muted ml-1">*</span>}
                </td>
                {probe.ports.map((port) => {
                  const state = cellState(vlan.vid, port);
                  return (
                    <td key={port} className="text-center px-1 py-0.5 border-b border-border/50">
                      <button
                        type="button"
                        onClick={() => cycleCell(vlan.vid, port)}
                        disabled={loading || vlan.default}
                        title={`${vlan.vid} · ${port}: ${state === "tagged" ? t("vlan.tagged") : state === "untagged" ? t("vlan.untagged") : t("vlan.notMember")}`}
                        className={`w-full py-1 rounded-sm text-caption font-mono transition-colors duration-[var(--dur-fast)] ring-focus ${cellClass(state)}
                          ${vlan.default ? "cursor-not-allowed opacity-50" : "cursor-pointer hover:opacity-80"}`}
                      >
                        {cellLabel(state)}
                      </button>
                    </td>
                  );
                })}
                <td className="border-b border-border/50 text-center">
                  {!vlan.default && (
                    <button
                      type="button"
                      onClick={() => setConfirmDelete(vlan.vid)}
                      disabled={loading}
                      aria-label={`${t("vlan.confirmDel")} ${vlan.vid}`}
                      className="text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1"
                    >
                      <Trash2 size={14} />
                    </button>
                  )}
                </td>
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
        <span><span className="inline-block w-3 h-3 rounded-sm bg-success-soft border border-success/30 mr-1 align-middle" /> U = {t("vlan.untagged")}</span>
        <span><span className="inline-block w-3 h-3 rounded-sm bg-accent-soft border border-accent/30 mr-1 align-middle" /> T = {t("vlan.tagged")}</span>
        <span><span className="inline-block w-3 h-3 rounded-sm bg-fill border border-border mr-1 align-middle" /> – = {t("vlan.notMember")}</span>
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
