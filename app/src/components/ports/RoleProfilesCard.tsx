import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Users } from "lucide-react";
import { api } from "../../api";
import type { RoleProfile } from "../../types";
import { Card, ConfirmDialog, Pill } from "../ui";
import type { PillTone } from "../ui";

/** Perfiles por rol (lan.md §5, vive en Puertos en el código real). */
export function RoleProfilesCard() {
  const { t } = useTranslation();
  const [roles, setRoles] = useState<RoleProfile[]>([]);
  const [ports, setPorts] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [selectedPort, setSelectedPort] = useState("");
  const [confirmRole, setConfirmRole] = useState<RoleProfile>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => {
    api.roleProfiles().then((r) => setRoles(r.roles ?? [])).catch(() => {});
    api.switchPorts().then((r) => { if (r.applicable) setPorts(r.ports.map((p) => p.name)); }).catch(() => {});
  }, []);

  if (roles.length === 0) return null;

  const apply = async (role: RoleProfile) => {
    setBusy(true); setMsg(undefined);
    try {
      await api.applyRoleProfile(role.id, selectedPort);
      setMsg({ tone: "ok", text: t("roles.applied", { port: selectedPort }) });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusy(false); }
  };

  const roleColor = (id: string): PillTone => {
    switch (id) {
      case "trusted": return "ok";
      case "iot": return "warn";
      case "guest": return "muted";
      case "camera": return "accent";
      default: return "muted";
    }
  };

  return (
    <Card variant="subtle" animate={false} icon={Users} title={t("roles.title")} help="roles">
      <p className="text-small text-muted mb-3">{t("roles.intro")}</p>

      <div className="mb-3">
        <label className="text-small text-muted block mb-1">{t("roles.targetPort")}</label>
        <select value={selectedPort} onChange={(e) => setSelectedPort(e.target.value)}
          className="w-full h-10 rounded-sm border border-border bg-surface-2 px-3 text-body outline-none focus:border-accent ring-focus">
          <option value="">{t("roles.selectPort")}</option>
          {ports.map((p) => <option key={p} value={p}>{p}</option>)}
        </select>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        {roles.map((role) => (
          <button
            key={role.id}
            type="button"
            onClick={() => {
              if (!selectedPort) {
                setMsg({ tone: "danger", text: t("roles.selectPort") });
                return;
              }
              setConfirmRole(role);
            }}
            disabled={busy}
            className="p-3 bg-surface border border-border rounded-md text-left
              hover:bg-surface-2 hover:border-accent transition-colors duration-[var(--dur-fast)] ring-focus
              disabled:opacity-50 disabled:pointer-events-none"
          >
            <div className="flex items-center gap-2 mb-1 flex-wrap">
              <Pill tone={roleColor(role.id)}>{role.name}</Pill>
              <span className="text-caption text-muted font-mono">VLAN {role.vid}</span>
              {role.isolated && <Pill tone="warn">{t("roles.isolatedPill")}</Pill>}
            </div>
            <p className="text-caption text-muted">{role.description}</p>
          </button>
        ))}
      </div>

      {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}

      <ConfirmDialog
        open={!!confirmRole}
        onClose={() => setConfirmRole(undefined)}
        onConfirm={() => { const r = confirmRole; setConfirmRole(undefined); if (r) apply(r); }}
        title={t("roles.applyTitle", { role: confirmRole?.name ?? "", port: selectedPort })}
        consequence={t("roles.applyConsequence", { vid: confirmRole?.vid ?? "" })}
        confirmLabel={t("roles.applyConfirm")}
      />
    </Card>
  );
}
