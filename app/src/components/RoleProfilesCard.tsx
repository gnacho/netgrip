import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Users } from "lucide-react";
import { api } from "../api";
import type { RoleProfile } from "../types";
import { Card, Pill } from "./Card";

export function RoleProfilesCard() {
  const { t } = useTranslation();
  const [roles, setRoles] = useState<RoleProfile[]>([]);
  const [ports, setPorts] = useState<string[]>([]);
  const [busy, setBusy] = useState<string>();
  const [selectedPort, setSelectedPort] = useState("");
  const [confirmApply, setConfirmApply] = useState<string>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => {
    api.roleProfiles().then((r) => setRoles(r.roles ?? [])).catch(() => {});
    api.switchPorts().then((r) => { if (r.applicable) setPorts(r.ports.map((p) => p.name)); }).catch(() => {});
  }, []);

  const apply = async (roleId: string) => {
    if (!selectedPort) {
      setMsg({ tone: "danger", text: t("roles.selectPort") });
      return;
    }
    if (confirmApply !== roleId) {
      setConfirmApply(roleId);
      return;
    }
    const key = roleId + "-" + selectedPort;
    setBusy(key); setMsg(undefined);
    try {
      await api.applyRoleProfile(roleId, selectedPort);
      setMsg({ tone: "ok", text: t("roles.applied", { port: selectedPort }) });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(undefined); setConfirmApply(undefined); }
  };

  const roleColor = (id: string) => {
    switch (id) {
      case "trusted": return "ok";
      case "iot": return "warn";
      case "guest": return "muted";
      case "camera": return "accent";
      default: return "muted";
    }
  };

  return (
    <Card title={t("roles.title")} icon={Users}>
      <p className="text-xs text-muted mb-3">{t("roles.intro")}</p>

      <div className="mb-3">
        <label className="text-xs text-muted">{t("roles.targetPort")}</label>
        <select value={selectedPort} onChange={(e) => setSelectedPort(e.target.value)}
          className="bg-bg border border-border rounded-lg px-2 py-1 text-sm mt-1 w-full">
          <option value="">{t("roles.selectPort")}</option>
          {ports.map((p) => <option key={p} value={p}>{p}</option>)}
        </select>
      </div>

      <div className="grid grid-cols-2 gap-2">
        {roles.map((role) => {
          const key = role.id + "-" + selectedPort;
          return (
            <button
              key={role.id}
              onClick={() => apply(role.id)}
              disabled={busy === key}
              className="p-2 bg-bg/50 border border-border/50 rounded-lg text-left hover:bg-bg/80 transition-colors disabled:opacity-50"
            >
              <div className="flex items-center gap-2 mb-0.5">
                <Pill tone={roleColor(role.id) as any}>{role.name}</Pill>
                <span className="text-[10px] text-muted">VLAN {role.vid}</span>
                {role.isolated && <span className="text-[10px] text-warn">({t("roles.isolated")})</span>}
              </div>
              <p className="text-[10px] text-muted">{role.description}</p>
              {confirmApply === role.id && selectedPort && (
                <p className="text-[10px] text-warn mt-1">{t("roles.confirmMsg", { port: selectedPort })}</p>
              )}
            </button>
          );
        })}
      </div>

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
