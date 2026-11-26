import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable, Pencil } from "lucide-react";
import { api } from "../../api";
import type { PortRoleEntry, PortRolesProbe } from "../../types";
import {
  ActionBanner, Button, Card, ConfirmDialog, Field, Input, Modal, Pill,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { RollbackConfirmModal, useRollbackConfirm } from "../rollback";

type RoleTarget = { port: PortRoleEntry; role: "lan" | "wan" };

/**
 * Rol de cada boca y su MAC (#662), el equivalente al selector de puerto
 * WAN/LAN de GL.iNet. El rol no se guarda en ningún campo: se deduce de la
 * configuración (bridge de la LAN y device de la WAN), así que la tarjeta
 * enseña el estado real y lo cambia moviendo el puerto de un sitio al otro.
 * Quitar un puerto del bridge puede cortar la ruta de gestión, por eso cada
 * cambio pide confirmación y abre la ventana de auto-reversión del servidor.
 */
export function PortRolesCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<PortRolesProbe>();
  const [roleTarget, setRoleTarget] = useState<RoleTarget>();
  const [macEdit, setMacEdit] = useState<{ port: PortRoleEntry; value: string }>();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const rc = useRollbackConfirm();

  const load = useCallback(() => {
    api.portRoles().then(setProbe).catch(() => {});
  }, []);
  useEffect(() => { load(); }, [load]);

  const applyRole = async () => {
    if (!roleTarget) return;
    const res = await run(() => api.setPortRole(roleTarget.port.name, roleTarget.role));
    if (res) {
      setProbe(res.state);
      rc.arm(res.confirm);
    }
    setRoleTarget(undefined);
  };

  const applyMac = async () => {
    if (!macEdit) return;
    const res = await run(() => api.setPortMac(macEdit.port.name, macEdit.value.trim()));
    if (res) {
      setProbe(res.state);
      rc.arm(res.confirm);
    }
    setMacEdit(undefined);
  };

  if (!probe?.applicable) return null;

  const roleTone = (role: string): "accent" | "ok" | "muted" =>
    role === "wan" ? "accent" : role === "lan" ? "ok" : "muted";
  const roleLabel = (role: string) =>
    role === "wan" ? t("portRoles.wan") : role === "lan" ? t("portRoles.lan") : t("portRoles.unused");

  return (
    <Card index={index} icon={Cable} title={t("portRoles.title")} help="portRoles">
      {phase && (
        <div className="mb-2">
          <ActionBanner phase={phase} detail={detail} onDone={clear} />
        </div>
      )}

      {!probe.can_assign_wan && (
        <p className="text-small text-muted mb-2">{t("portRoles.apNote")}</p>
      )}

      <div className="divide-y divide-border/60">
        {(probe.ports ?? []).map((port) => {
          const isWan = port.role === "wan";
          const canMove = isWan || probe.can_assign_wan;
          return (
            <div key={port.name} className="flex items-center gap-3 py-2.5">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-medium">{port.name}</span>
                  <Pill tone={roleTone(port.role)}>{roleLabel(port.role)}</Pill>
                  {port.speed_mbps > 0 && <Pill tone="muted">{port.speed_mbps} Mb/s</Pill>}
                  {!port.link_up && <Pill tone="muted">{t("portRoles.noLink")}</Pill>}
                </div>
                <p className="text-small text-muted mt-0.5 font-mono">{port.mac}</p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                disabled={busy}
                onClick={() => setMacEdit({ port, value: port.mac_override })}
                aria-label={t("portRoles.editMac", { port: port.name })}
              >
                <Pencil size={16} />
              </Button>
              <Button
                variant="secondary"
                size="sm"
                disabled={busy || !canMove}
                onClick={() => setRoleTarget({ port, role: isWan ? "lan" : "wan" })}
              >
                {isWan ? t("portRoles.makeLan") : t("portRoles.makeWan")}
              </Button>
            </div>
          );
        })}
      </div>

      <p className="text-caption text-muted mt-3">{t("portRoles.footnote")}</p>

      <ConfirmDialog
        open={!!roleTarget}
        busy={busy}
        onClose={() => setRoleTarget(undefined)}
        onConfirm={applyRole}
        title={t("portRoles.confirmTitle", { port: roleTarget?.port.name ?? "" })}
        consequence={
          roleTarget?.role === "wan"
            ? t("portRoles.confirmWan", {
                port: roleTarget?.port.name ?? "",
                vlan: roleTarget?.port.vlan_suffix ? roleTarget.port.vlan_suffix.slice(1) : "",
              })
            : t("portRoles.confirmLan", { port: roleTarget?.port.name ?? "" })
        }
        confirmLabel={roleTarget?.role === "wan" ? t("portRoles.makeWan") : t("portRoles.makeLan")}
      />

      <Modal
        open={!!macEdit}
        onClose={() => setMacEdit(undefined)}
        title={t("portRoles.macTitle", { port: macEdit?.port.name ?? "" })}
        footer={
          <div className="flex justify-between gap-2">
            <Button
              variant="ghost"
              disabled={busy}
              onClick={() => setMacEdit((m) => (m ? { ...m, value: "" } : m))}
            >
              {t("portRoles.factory")}
            </Button>
            <div className="flex gap-2">
              <Button variant="ghost" onClick={() => setMacEdit(undefined)}>
                {t("common.cancel")}
              </Button>
              <Button variant="primary" loading={busy} onClick={applyMac}>
                {t("common.save")}
              </Button>
            </div>
          </div>
        }
      >
        <div className="flex flex-col gap-3">
          <Field label={t("portRoles.macLabel")} hint={t("portRoles.macHint", { mac: macEdit?.port.factory_mac ?? "" })}>
            <Input
              mono
              value={macEdit?.value ?? ""}
              placeholder={macEdit?.port.factory_mac ?? ""}
              onChange={(e) => setMacEdit((m) => (m ? { ...m, value: e.target.value } : m))}
            />
          </Field>
          <p className="text-small text-muted">{t("portRoles.macBridgeNote")}</p>
        </div>
      </Modal>

      <RollbackConfirmModal
        pending={rc.pending}
        seconds={rc.seconds}
        onKeep={rc.keep}
        onRevert={() => { rc.revert(); load(); }}
      />
    </Card>
  );
}
