import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { RadioTower } from "lucide-react";
import { api, isDemo } from "../../api";
import { Button, Card, Field, HelpTip, Input, Pill, SkeletonRows, Toggle, useToast } from "../ui";
import { CardLoadError } from "../tools/diagnostics";
import { useInstallJob } from "../wizard/common";

/**
 * SNMP (#442): manage the snmpd daemon (net-snmp) from the Advanced section.
 * The package installs on demand through the optional-packages catalog; the
 * card then configures enabled, location/contact, listen address and the two
 * communities (ro mandatory, rw optional). A config managed outside the
 * panel (smux, traps, users...) shows an explicit external state and is
 * never touched.
 */
export function SnmpCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const { begin } = useInstallJob();
  const [probe, setProbe] = useState<import("../../types").SNMPProbe>();
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [installing, setInstalling] = useState(false);

  const [enabled, setEnabled] = useState(false);
  const [location, setLocation] = useState("");
  const [contact, setContact] = useState("");
  const [listen, setListen] = useState("UDP:161");
  const [communityRO, setCommunityRO] = useState("");
  const [communityRW, setCommunityRW] = useState("");

  const applyProbe = useCallback((p: import("../../types").SNMPProbe) => {
    setProbe(p);
    setEnabled(p.enabled);
    setLocation(p.location);
    setContact(p.contact);
    setListen(p.listen || "UDP:161");
    setCommunityRO(p.community_ro ?? "");
    setCommunityRW(p.community_rw ?? "");
  }, []);

  const load = useCallback(async () => {
    setError(false);
    try { applyProbe(await api.snmp()); }
    catch { setError(true); }
  }, [applyProbe]);
  useEffect(() => { load(); }, [load]);

  const install = async () => {
    setInstalling(true);
    try {
      const j = await begin(() => api.wizardPackages(["snmp"]));
      if (j.phase === "done") {
        push({ tone: "ok", text: t("packages.installed") });
        await load();
      } else {
        push({ tone: "danger", text: j.error || t("error.network") });
      }
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setInstalling(false); }
  };

  const save = async (nextEnabled: boolean) => {
    setBusy(true);
    try {
      const r = await api.setSnmp({
        enabled: nextEnabled,
        location, contact, listen,
        community_ro: communityRO.trim(),
        community_rw: communityRW.trim(),
      });
      applyProbe(r.state);
      if (r.rolled_back || r.status === "rolled_back") push({ tone: "danger", text: r.error || t("snmp.rolledBack") });
      else if (r.status === "failed") push({ tone: "danger", text: r.error || t("error.network") });
      else push({ tone: "ok", text: t("snmp.saved") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusy(false); }
  };

  if (error) return <Card index={index} title={t("snmp.title")} icon={RadioTower}><CardLoadError onRetry={() => void load()} /></Card>;
  if (!probe) return <Card index={index} title={t("snmp.title")} icon={RadioTower}><SkeletonRows rows={4} /></Card>;

  if (isDemo() || !probe.installed) {
    return (
      <Card index={index} title={t("snmp.title")} icon={RadioTower}>
        <p className="text-small text-muted">{t("snmp.notInstalled")}</p>
        {!isDemo() && (
          <div className="mt-4">
            <Button variant="secondary" loading={installing} onClick={() => void install()}>
              {t("snmp.install")}
            </Button>
          </div>
        )}
      </Card>
    );
  }

  if (!probe.managed) {
    return (
      <Card index={index} title={t("snmp.title")} icon={RadioTower}>
        <p className="text-small text-muted">{t("snmp.external")}</p>
        <div className="mt-2"><Pill tone="muted">{t("snmp.externalPill")}</Pill></div>
      </Card>
    );
  }

  return (
    <Card
      index={index}
      title={t("snmp.title")}
      icon={RadioTower}
      action={probe.running ? <Pill tone="ok">{t("snmp.running")}</Pill> : undefined}
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <span className="flex items-center gap-2 text-body font-medium text-text">
            {t("snmp.enabled")}
            <HelpTip title={t("snmp.enabled")} body={t("snmp.enabledHint")} />
          </span>
          <div className="sm:shrink-0">
            <Toggle checked={enabled} disabled={busy} onChange={(v) => { setEnabled(v); void save(v); }} label={t("snmp.enabled")} />
          </div>
        </div>

        <Field label={t("snmp.location")} hint={t("snmp.locationHint")}>
          <Input value={location} onChange={(e) => setLocation(e.target.value)} placeholder="Rack 1" />
        </Field>
        <Field label={t("snmp.contact")}>
          <Input value={contact} onChange={(e) => setContact(e.target.value)} placeholder="admin@example.com" />
        </Field>
        <Field label={t("snmp.listen")} hint={t("snmp.listenHint")}>
          <Input value={listen} onChange={(e) => setListen(e.target.value)} placeholder="UDP:161" />
        </Field>
        <Field label={t("snmp.communityRO")} hint={t("snmp.communityROHint")}>
          <Input value={communityRO} onChange={(e) => setCommunityRO(e.target.value)} placeholder="monitors" />
        </Field>
        <Field label={t("snmp.communityRW")} hint={t("snmp.communityRWHint")}>
          <Input value={communityRW} onChange={(e) => setCommunityRW(e.target.value)} placeholder={t("snmp.communityRWPlaceholder")} />
        </Field>

        <div>
          <Button variant="primary" loading={busy} onClick={() => void save(enabled)}>
            {t("snmp.save")}
          </Button>
        </div>
      </div>
    </Card>
  );
}
