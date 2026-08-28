import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Download, Plus, RefreshCw, Server, Trash2 } from "lucide-react";
import { api } from "../api";
import type { FleetNodeStatus } from "../types";
import { Card } from "../components/Card";

export function FleetPage() {
  const { t } = useTranslation();
  const [nodes, setNodes] = useState<FleetNodeStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [checking, setChecking] = useState(false);

  const load = async () => {
    try {
      const data = await api.fleet();
      setNodes(data.nodes ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleAdd = async (node: { id: string; name: string; address: string; password: string }) => {
    await api.addFleetNode(node);
    setShowAdd(false);
    load();
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t("fleet.confirmDelete"))) return;
    await api.deleteFleetNode(id);
    load();
  };

  const handleCheck = async (id: string) => {
    setChecking(true);
    try {
      const status = await api.checkFleetNode(id);
      setNodes((prev) => prev.map((n) => (n.id === id ? status : n)));
    } catch {
      // ignore
    } finally {
      setChecking(false);
    }
  };

  const handleCheckAll = async () => {
    setChecking(true);
    try {
      const data = await api.checkAllFleet();
      setNodes(data.nodes ?? []);
    } catch {
      // ignore
    } finally {
      setChecking(false);
    }
  };

  const handleUpdate = async (id: string) => {
    if (!confirm(t("fleet.confirmUpdate"))) return;
    try {
      await api.updateFleetNode(id);
      setNodes((prev) => prev.map((n) => (n.id === id ? { ...n, update_available: false } : n)));
    } catch {
      // ignore
    }
  };

  if (loading) return <p className="text-muted">{t("fleet.loading")}</p>;

  return (
    <div className="flex flex-col gap-4">
      <Card title={t("fleet.title")} icon={Server}>
        <p className="text-xs text-muted mb-3">{t("fleet.intro")}</p>

        <div className="flex items-center gap-2 mb-3">
          <button onClick={handleCheckAll} disabled={checking || nodes.length === 0}
            className="px-3 py-1.5 bg-accent/15 hover:bg-accent/25 border border-accent/30 rounded text-xs disabled:opacity-50">
            {checking ? t("fleet.checking") : t("fleet.checkAll")}
          </button>
          <button onClick={() => setShowAdd(true)}
            className="px-3 py-1.5 bg-card border border-border rounded text-xs hover:bg-card/80">
            <Plus size={14} className="inline mr-1" />{t("fleet.addNode")}
          </button>
        </div>

        {nodes.length === 0 ? (
          <p className="text-sm text-muted">{t("fleet.empty")}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-muted border-b border-border">
                  <th className="text-left py-2 px-2">{t("fleet.name")}</th>
                  <th className="text-left py-2 px-2">{t("fleet.address")}</th>
                  <th className="text-left py-2 px-2">{t("fleet.version")}</th>
                  <th className="text-left py-2 px-2">{t("fleet.status")}</th>
                  <th className="text-right py-2 px-2">{t("fleet.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((node) => (
                  <tr key={node.id} className="border-b border-border/50 hover:bg-card/50">
                    <td className="py-2 px-2 font-medium">{node.name}</td>
                    <td className="py-2 px-2 text-xs text-muted">{node.address}</td>
                    <td className="py-2 px-2 text-xs">
                      {node.reachable ? `${node.current_version}` : "-"}
                      {node.update_available && (
                        <span className="text-accent ml-1">({t("fleet.updateAvailable")})</span>
                      )}
                    </td>
                    <td className="py-2 px-2">
                      {node.error ? (
                        <span className="text-danger text-xs">{node.error}</span>
                      ) : node.reachable ? (
                        <span className="text-success text-xs flex items-center gap-1">
                          <Check size={12} />{t("fleet.online")}
                        </span>
                      ) : (
                        <span className="text-muted text-xs">-</span>
                      )}
                    </td>
                    <td className="py-2 px-2 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <button onClick={() => handleCheck(node.id)} disabled={checking}
                          className="p-1 text-muted hover:text-text disabled:opacity-50" title={t("fleet.check")}>
                          <RefreshCw size={14} />
                        </button>
                        {node.update_available && (
                          <button onClick={() => handleUpdate(node.id)}
                            className="p-1 text-accent hover:text-accent/80" title={t("fleet.update")}>
                            <Download size={14} />
                          </button>
                        )}
                        <button onClick={() => handleDelete(node.id)}
                          className="p-1 text-muted hover:text-danger" title={t("fleet.delete")}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {showAdd && <AddNodeDialog onAdd={handleAdd} onCancel={() => setShowAdd(false)} />}
    </div>
  );
}

function AddNodeDialog({ onAdd, onCancel }: {
  onAdd: (node: { id: string; name: string; address: string; password: string }) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [password, setPassword] = useState("");

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-card border border-border rounded-lg p-4 w-full max-w-md">
        <h3 className="text-sm font-semibold mb-3">{t("fleet.addNode")}</h3>
        <div className="flex flex-col gap-2 mb-4">
          <input value={id} onChange={(e) => setId(e.target.value)} placeholder={t("fleet.nodeId")}
            className="px-3 py-2 bg-bg border border-border rounded text-sm" />
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("fleet.nodeName")}
            className="px-3 py-2 bg-bg border border-border rounded text-sm" />
          <input value={address} onChange={(e) => setAddress(e.target.value)} placeholder={t("fleet.nodeAddress")}
            className="px-3 py-2 bg-bg border border-border rounded text-sm" />
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t("fleet.nodePassword")}
            className="px-3 py-2 bg-bg border border-border rounded text-sm" />
        </div>
        <div className="flex justify-end gap-2">
          <button onClick={onCancel} className="px-3 py-1.5 text-sm text-muted hover:text-text">
            {t("fleet.cancel")}
          </button>
          <button onClick={() => onAdd({ id, name, address, password })} disabled={!id || !name || !address}
            className="px-3 py-1.5 bg-accent text-white rounded text-sm disabled:opacity-50">
            {t("fleet.add")}
          </button>
        </div>
      </div>
    </div>
  );
}
