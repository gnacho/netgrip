import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Globe } from "lucide-react";
import { api } from "../../api";
import type { RemoteAccess } from "../../types";
import { Banner, Card, SettingRow, SkeletonRows, useToast } from "../ui";

type Key = "ping_wan" | "remote_https" | "remote_ssh";

export function RemoteAccessCard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [probe, setProbe] = useState<RemoteAccess>();
  const [busy, setBusy] = useState<Key>();

  useEffect(() => {
    api.remoteAccess().then(setProbe).catch(() => {});
  }, []);

  const toggle = async (key: Key, value: boolean) => {
    setBusy(key);
    try {
      const res = await api.setRemoteAccess({ [key]: value });
      if (res.status !== "applied") {
        push({ tone: "danger", text: res.error || t("action.failed") });
      }
      setProbe(await api.remoteAccess());
    } catch (err) {
      push({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(undefined);
    }
  };

  const items: { key: Key; label: string; hint: string }[] = [
    { key: "ping_wan", label: t("remote.pingWan"), hint: t("remote.pingWanHint") },
    { key: "remote_https", label: t("remote.https"), hint: t("remote.httpsHint") },
    { key: "remote_ssh", label: t("remote.ssh"), hint: t("remote.sshHint") },
  ];

  if (probe && !probe.applicable) {
    // modo AP: card atenuada con la razón
    return (
      <Card index={index} title={t("remote.title")} icon={Globe} iconTone="muted" className="opacity-70">
        <p className="text-small text-muted">{t("remote.notApplicable")}</p>
      </Card>
    );
  }

  const exposed = !!probe && (probe.remote_https || probe.remote_ssh);

  return (
    <Card index={index} title={t("remote.title")} icon={Globe}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <div className="flex flex-col divide-y divide-border/60">
          {items.map((item) => (
            <SettingRow
              key={item.key}
              title={item.label}
              description={item.hint}
              checked={probe[item.key]}
              busy={busy === item.key}
              onChange={(v) => toggle(item.key, v)}
            />
          ))}
          {exposed && (
            <div className="pt-3">
              <Banner tone="warn">{t("remote.warn")}</Banner>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
