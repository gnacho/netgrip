import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Pencil } from "lucide-react";
import { api } from "../../api";
import type { PortRoleEntry, PortRolesProbe } from "../../types";
import { ActionBanner, Button, ConfirmDialog, Field, HelpTip, Input, Modal, Pill } from "../ui";
import { useActionCycle } from "../wifi/action";
import { RollbackConfirmModal, useRollbackConfirm } from "../rollback";

/**
 * Rol y MAC de la boca seleccionada (#662), dentro de la banda de detalle del
 * chasis: al pinchar un puerto, su configuración aparece ahí y no en una
 * tarjeta aparte. El rol no se guarda en ningún campo, se deduce de la
 * configuración (bridge de la LAN y device de la WAN), así que la sección
 * enseña el estado real y lo cambia moviendo el puerto de un sitio al otro.
 * Quitar un puerto del bridge puede cortar la ruta de gestión, por eso cada
 * cambio pide confirmación y abre la ventana de auto-reversión del servidor.
 */
export function PortRoleSection({ entry, applicable, canAssignWan, onProbe }: {
  entry?: PortRoleEntry;
  applicable: boolean;
  canAssignWan: boolean;
  /** Devuelve el probe nuevo (y el confirm si el write abrió ventana). */
  onProbe: (probe: PortRolesProbe, confirm?: { id: string; expires: number }) => void;
}) {
  const { t } = useTranslation();
  const [roleTarget, setRoleTarget] = useState<"lan" | "wan">();
  const [macValue, setMacValue] = useState<string>();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const rc = useRollbackConfirm();

  if (!applicable || !entry) return null;

  const isWan = entry.role === "wan";
  const roleTone: "accent" | "ok" | "muted" =
    entry.role === "wan" ? "accent" : entry.role === "lan" ? "ok" : "muted";
  const roleLabel =
    entry.role === "wan" ? t("portRoles.wan") : entry.role === "lan" ? t("portRoles.lan") : t("portRoles.unused");

  const applyRole = async () => {
    if (!roleTarget) return;
    const res = await run(() => api.setPortRole(entry.name, roleTarget));
    if (res) onProbe(res.state, res.confirm);
    setRoleTarget(undefined);
  };

  const applyMac = async () => {
    if (macValue === undefined) return;
    const res = await run(() => api.setPortMac(entry.name, macValue.trim()));
    if (res) onProbe(res.state, res.confirm);
    setMacValue(undefined);
  };

  return (
    <div className="mt-3 rounded-md border border-border bg-surface-2 p-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="text-caption text-muted">{t("portRoles.roleLabel")}</span>
        <Pill tone={roleTone}>{roleLabel}</Pill>
        <HelpTip title={t("help.portRoles.title")} body={t("help.portRoles.body")} />
        <span className="font-mono text-small">{entry.mac}</span>
        {entry.mac_override && <Pill tone="muted">{t("portRoles.customMac")}</Pill>}
        <span className="ml-auto flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            disabled={busy}
            onClick={() => setMacValue(entry.mac_override)}
            aria-label={t("portRoles.editMac", { port: entry.name })}
          >
            <Pencil size={14} />
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || (!isWan && !canAssignWan)}
            onClick={() => setRoleTarget(isWan ? "lan" : "wan")}
          >
            {isWan ? t("portRoles.makeLan") : t("portRoles.makeWan")}
          </Button>
        </span>
      </div>

      <p className="text-caption text-muted mt-2">
        {canAssignWan ? t("portRoles.footnote") : t("portRoles.apNote")}
      </p>

      {phase && (
        <div className="mt-2">
          <ActionBanner phase={phase} detail={detail} onDone={clear} />
        </div>
      )}

      <ConfirmDialog
        open={!!roleTarget}
        busy={busy}
        onClose={() => setRoleTarget(undefined)}
        onConfirm={applyRole}
        title={t("portRoles.confirmTitle", { port: entry.name })}
        consequence={
          roleTarget === "wan"
            ? t("portRoles.confirmWan", { port: entry.name, vlan: entry.vlan_suffix ? entry.vlan_suffix.slice(1) : "" })
            : t("portRoles.confirmLan", { port: entry.name })
        }
        confirmLabel={roleTarget === "wan" ? t("portRoles.makeWan") : t("portRoles.makeLan")}
      />

      <Modal
        open={macValue !== undefined}
        onClose={() => setMacValue(undefined)}
        title={t("portRoles.macTitle", { port: entry.name })}
        footer={
          <div className="flex w-full justify-between gap-2">
            <Button variant="ghost" disabled={busy} onClick={() => setMacValue("")}>
              {t("portRoles.factory")}
            </Button>
            <div className="flex gap-2">
              <Button variant="ghost" onClick={() => setMacValue(undefined)}>
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
          <Field
            label={t("portRoles.macLabel")}
            hint={t("portRoles.macHint", { mac: entry.factory_mac })}
          >
            <Input
              mono
              value={macValue ?? ""}
              placeholder={entry.factory_mac}
              onChange={(e) => setMacValue(e.target.value)}
            />
          </Field>
          <p className="text-small text-muted">{t("portRoles.macBridgeNote")}</p>
        </div>
      </Modal>

      <RollbackConfirmModal pending={rc.pending} seconds={rc.seconds} onKeep={rc.keep} onRevert={rc.revert} />
    </div>
  );
}
