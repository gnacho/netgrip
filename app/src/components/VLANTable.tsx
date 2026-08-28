import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Network, Plus, Trash2 } from "lucide-react";
import { api } from "../api";
import { Card } from "./Card";
import type { VLANProbe, VLANPort } from "../types";

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

  const cellState = (vlan: number, port: string): "empty" | "untagged" | "tagged" => {
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
      setProbe(res.state);
    } catch (e: any) {
      setError(e.message);
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
      setProbe(res.state);
      setNewVid("");
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const deleteVLAN = async (vid: number) => {
    setLoading(true);
    setError("");
    try {
      const res = await api.deleteVlan(vid);
      setProbe(res.state);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
      setConfirmDelete(undefined);
    }
  };

  const cellClass = (state: "empty" | "untagged" | "tagged") => {
    switch (state) {
      case "tagged": return "bg-accent/30 text-accent font-bold";
      case "untagged": return "bg-green-500/20 text-green-400 font-bold";
      default: return "bg-bg/30 text-muted/50";
    }
  };

  const cellLabel = (state: "empty" | "untagged" | "tagged") => {
    switch (state) {
      case "tagged": return "T";
      case "untagged": return "U";
      default: return "-";
    }
  };

  return (
    <Card title={t("vlan.title")} icon={Network}>
      <p className="text-xs text-muted mb-3">{t("vlan.intro")}</p>
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              <th className="text-left px-2 py-1 text-xs text-muted border-b border-border">VLAN</th>
              {probe.ports.map((port) => (
                <th key={port} className="text-center px-1 py-1 text-xs text-muted border-b border-border">{port}</th>
              ))}
              <th className="w-8 border-b border-border" />
            </tr>
          </thead>
          <tbody>
            {probe.vlans.map((vlan) => (
              <tr key={vlan.vid}>
                <td className="px-2 py-1 font-mono text-xs border-b border-border/50">
                  {vlan.vid}
                  {vlan.default && <span className="text-muted ml-1">*</span>}
                </td>
                {probe.ports.map((port) => {
                  const state = cellState(vlan.vid, port);
                  return (
                    <td key={port} className="text-center px-1 py-0.5 border-b border-border/50">
                      <button
                        onClick={() => cycleCell(vlan.vid, port)}
                        disabled={loading || vlan.default}
                        className={`w-full py-1 rounded text-xs transition-colors ${cellClass(state)}
                          ${vlan.default ? "cursor-not-allowed opacity-50" : "cursor-pointer hover:opacity-80"}`}
                      >
                        {cellLabel(state)}
                      </button>
                    </td>
                  );
                })}
                <td className="border-b border-border/50 text-center">
                  {!vlan.default && (
                    confirmDelete === vlan.vid ? (
                      <div className="flex gap-0.5 justify-center">
                        <button onClick={() => deleteVLAN(vlan.vid)} disabled={loading}
                          className="text-xs bg-danger/20 hover:bg-danger/30 rounded px-1.5 py-0.5">
                          {t("vlan.confirmDel")}
                        </button>
                        <button onClick={() => setConfirmDelete(undefined)} className="text-xs text-muted">x</button>
                      </div>
                    ) : (
                      <button onClick={() => setConfirmDelete(vlan.vid)} disabled={loading}
                        className="text-muted hover:text-danger p-1">
                        <Trash2 size={12} />
                      </button>
                    )
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center gap-2 mt-3 pt-3 border-t border-border/50">
        <input
          type="number"
          min={2}
          max={4094}
          value={newVid}
          onChange={(e) => setNewVid(e.target.value)}
          placeholder={t("vlan.newVid")}
          className="w-24 bg-bg border border-border rounded-lg px-2 py-1 text-sm"
        />
        <button onClick={addVLAN} disabled={loading || !newVid}
          className="flex items-center gap-1 text-xs bg-accent/15 text-accent px-2 py-1 rounded-lg hover:bg-accent/25 disabled:opacity-50">
          <Plus size={12} /> {t("vlan.add")}
        </button>
      </div>

      <div className="flex gap-4 mt-2 text-xs text-muted">
        <span><span className="inline-block w-3 h-3 rounded bg-green-500/20 border border-green-400/30 mr-1 align-middle" /> U = {t("vlan.untagged")}</span>
        <span><span className="inline-block w-3 h-3 rounded bg-accent/30 border border-accent/30 mr-1 align-middle" /> T = {t("vlan.tagged")}</span>
        <span><span className="inline-block w-3 h-3 rounded bg-bg/30 border border-border/50 mr-1 align-middle" /> - = {t("vlan.notMember")}</span>
      </div>

      {error && <p className="text-danger text-xs mt-2">{error}</p>}
    </Card>
  );
}
