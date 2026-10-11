import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Server } from "lucide-react";
import { api } from "../../api";
import type { MQTTInfo } from "../../types";
import { Button, Field, SettingRow, StatusDot, useToast } from "../ui";

const STATUS_POLL_MS = 10_000;

/**
 * Contenido del bloque MQTT dentro de "Avisos e integraciones" (#499).
 * Sin Card propia: la envuelve CollapsibleSection. `onStatus` avisa al
 * padre tras cargar y tras guardar para la pill de la cabecera; el
 * sondeo de estado (10 s) solo corre con la sección abierta (`active`).
 */
export function MQTTSettings({ active = true, onStatus }: {
  active?: boolean;
  onStatus?: (enabled: boolean, connected: boolean) => void;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [state, setState] = useState<MQTTInfo>();
  const [host, setHost] = useState("");
  const [port, setPort] = useState(1883);
  const [user, setUser] = useState("");
  const [pass, setPass] = useState("");
  const [nodeId, setNodeId] = useState("");
  const [intervalSec, setIntervalSec] = useState(60);
  const [enabled, setEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  const report = (s: MQTTInfo | undefined) => onStatus?.(!!s?.enabled, !!s?.connected);

  const refresh = () => {
    api.mqtt().then((s) => {
      setState(s);
      setHost((prev) => (prev === "" ? s.host : prev));
      setPort((prev) => (prev === 1883 ? s.port : prev));
      setUser((prev) => (prev === "" ? s.user : prev));
      setNodeId((prev) => (prev === "" ? s.node_id : prev));
      setIntervalSec((prev) => (prev === 60 ? s.interval : prev));
      setEnabled(s.enabled);
      report(s);
    }).catch(() => {});
  };

  useEffect(() => {
    // Carga inicial SIEMPRE: la pill de estado de la cabecera necesita el
    // estado aunque la sección esté plegada. El sondeo (10 s) solo corre
    // con la sección abierta (`active`).
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!active) return;
    const poll = window.setInterval(refresh, STATUS_POLL_MS);
    return () => window.clearInterval(poll);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active]);

  const save = async () => {
    setSaving(true);
    try {
      const s = await api.setMqtt({ enabled, host, port, user, pass, nodeId, interval: intervalSec });
      setState(s);
      setPass("");
      report(s);
      push({ tone: "ok", text: t("mqtt.saved") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setSaving(false);
    }
  };

  // test comprueba los valores del formulario contra el broker sin
  // guardarlos (#409).
  const test = async () => {
    setTesting(true);
    try {
      const r = await api.testMqtt({ enabled, host, port, user, pass, nodeId, interval: intervalSec });
      push(r.ok
        ? { tone: "ok", text: t("mqtt.testOk") }
        : { tone: "danger", text: r.error || t("mqtt.testFail") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setTesting(false);
    }
  };

  const dotTone = !state?.enabled ? "muted" : state.connected ? "ok" : "danger";
  const statusText = !state?.enabled
    ? t("mqtt.idle")
    : state.connected
      ? t("mqtt.connected")
      : t("mqtt.disconnected");

  return (
    <>
      <SettingRow
        title={t("mqtt.enabled")}
        description={t("mqtt.description")}
        checked={enabled}
        onChange={setEnabled}
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-1">
        <Field
          label={t("mqtt.host")}
          icon={Server}
          inputProps={{
            mono: true, value: host, placeholder: "192.168.1.244",
            onChange: (e) => setHost(e.target.value),
          }}
        />
        <Field
          label={t("mqtt.port")}
          inputProps={{
            type: "number", mono: true, value: port,
            onChange: (e) => setPort(Number(e.target.value)),
          }}
        />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-3">
        <Field
          label={t("mqtt.user")}
          inputProps={{
            mono: true, value: user, autoComplete: "off",
            onChange: (e) => setUser(e.target.value),
          }}
        />
        <Field
          label={t("mqtt.pass")}
          hint={state?.configured ? t("mqtt.passSet") : undefined}
          inputProps={{
            type: "password", mono: true, value: pass, autoComplete: "new-password",
            placeholder: state?.configured ? "········" : "",
            onChange: (e) => setPass(e.target.value),
          }}
        />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-3">
        <Field
          label={t("mqtt.nodeId")}
          hint={t("mqtt.nodeIdHint")}
          inputProps={{
            mono: true, value: nodeId, placeholder: "rt3",
            onChange: (e) => setNodeId(e.target.value),
          }}
        />
        <Field
          label={t("mqtt.interval")}
          inputProps={{
            type: "number", mono: true, value: intervalSec,
            onChange: (e) => setIntervalSec(Number(e.target.value)),
          }}
        />
      </div>

      <div className="flex items-center gap-2 mt-3 min-w-0">
        <StatusDot tone={dotTone} live={!!state?.connected} label={statusText} />
        <span className="text-small truncate">{statusText}</span>
      </div>
      {state?.last_error && (
        <p className="text-caption text-danger mt-2 truncate" title={state.last_error}>
          {state.last_error}
        </p>
      )}

      <div className="flex gap-2 mt-3">
        <Button
          size="sm"
          onClick={save}
          loading={saving}
          disabled={!host || (enabled && !state?.configured && !pass)}
        >
          {t("mqtt.save")}
        </Button>
        <Button size="sm" variant="secondary" onClick={test} loading={testing} disabled={!host}>
          {t("mqtt.test")}
        </Button>
      </div>
    </>
  );
}
