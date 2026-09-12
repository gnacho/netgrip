import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ImageUp, KeyRound, ShieldCheck, Timer, Type } from "lucide-react";
import { api } from "../../api";
import type { CaptivePortalProbe } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Button, Field, Pill, SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "./action";

const IMAGE_EXTS = ["png", "jpg", "jpeg", "gif", "webp"];

/**
 * Portal cautivo del WiFi de invitados (issue #309), renderizado dentro de la
 * tarjeta Guest WiFi. El código de acceso y el HTML personalizado son
 * write-only: la sonda devuelve solo su presencia, no su contenido.
 */
export function CaptivePortalPanel({ gateway }: { gateway: boolean }) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [probe, setProbe] = useState<CaptivePortalProbe>();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [title, setTitle] = useState("");
  const [message, setMessage] = useState("");
  const [accessCode, setAccessCode] = useState("");
  const [sessionMinutes, setSessionMinutes] = useState("0");
  const [customHtml, setCustomHtml] = useState("");
  const [imageErr, setImageErr] = useState<string>();
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    api.captiveportal().then(setProbe).catch(() => {});
  }, []);

  useEffect(() => {
    if (probe) {
      setTitle(probe.title || "");
      setMessage(probe.message || "");
      setSessionMinutes(String(probe.session_minutes || 0));
    }
  }, [probe]);

  if (!gateway) return null;

  const active = probe?.active ?? false;
  const applicable = probe?.applicable ?? false;
  const minutes = Number(sessionMinutes);
  const sessionOk = Number.isFinite(minutes) && minutes >= 0;
  const codeOk = accessCode === "" || /^[A-Za-z0-9_-]{4,32}$/.test(accessCode);

  const apply = async (enabled: boolean, fields?: Record<string, unknown>) => {
    setDoneMsg(undefined);
    const res = await run(() => api.setCaptiveportal(enabled
      ? { enabled, title, message, access_code: accessCode, session_minutes: minutes, custom_html: customHtml, ...fields }
      : { enabled },
    ));
    if (res) {
      setProbe(res.state);
      if (res.status === "applied") setDoneMsg(enabled ? t("captiveportal.doneOn") : t("captiveportal.doneOff"));
    } else {
      setProbe(await api.captiveportal().catch(() => probe));
    }
  };

  const save = () => apply(true);

  const onFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    const ext = file.name.split(".").pop()?.toLowerCase() ?? "";
    if (!IMAGE_EXTS.includes(ext)) {
      setImageErr(t("captiveportal.imageBadExt"));
      return;
    }
    if (file.size === 0 || file.size > 512 * 1024) {
      setImageErr(t("captiveportal.imageTooBig"));
      return;
    }
    const reader = new FileReader();
    reader.onload = async () => {
      const base64 = String(reader.result ?? "").split(",")[1] ?? "";
      if (!base64) return;
      setImageErr(undefined);
      const res = await run(() => api.uploadCaptivePortalImage(base64, ext));
      if (res) {
        setProbe(res.state);
        if (res.status === "applied") setDoneMsg(t("captiveportal.imageSaved"));
      }
    };
    reader.readAsDataURL(file);
  };

  return (
    <div className="mt-2 border-t border-border/60 pt-2">
      {!probe ? (
        <SkeletonRows rows={2} />
      ) : (
        <>
          <SettingRow
            icon={ShieldCheck}
            iconTone="accent"
            title={t("captiveportal.cardTitle")}
            description={t("captiveportal.desc")}
            help={t("help.captiveportal.body")}
            helpTitle={t("help.captiveportal.title")}
            checked={active}
            busy={busy}
            disabled={!applicable}
            disabledReason={!applicable ? t("captiveportal.onlyGuest") : undefined}
            onChange={apply}
            control={
              <span className="flex items-center gap-2">
                {active && (
                  <Pill tone={probe.running ? "ok" : "warn"}>{probe.running ? t("captiveportal.running") : t("captiveportal.configured")}</Pill>
                )}
                <Toggle checked={active} busy={busy} disabled={!applicable} onChange={apply} label={t("captiveportal.cardTitle")} />
              </span>
            }
          />

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          {applicable && active && (
            <div className="pt-2 flex flex-col gap-3">
              <Field label={t("captiveportal.titleLabel")} icon={Type}
                inputProps={{ value: title, onChange: (e) => setTitle(e.target.value), maxLength: 64, placeholder: t("captiveportal.titleHint") }} />

              <label className="block">
                <span className="mb-1.5 flex items-center gap-1 text-caption font-medium text-muted">
                  {t("captiveportal.messageLabel")}
                </span>
                <textarea
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  maxLength={512}
                  rows={3}
                  placeholder={t("captiveportal.messageHint")}
                  className="w-full rounded-[10px] border border-transparent bg-fill px-3 py-2 text-body outline-none
                    placeholder:text-faint transition-[background-color,border-color,box-shadow] duration-[var(--dur-fast)]
                    hover:border-border-strong focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-accent)]"
                />
              </label>

              <Field label={t("captiveportal.codeLabel")} icon={KeyRound} hint={t("captiveportal.codeHint")}
                error={!codeOk ? t("captiveportal.codeBad") : undefined}
                inputProps={{ value: accessCode, onChange: (e) => setAccessCode(e.target.value), maxLength: 32, placeholder: t("captiveportal.codePlaceholder"), autoComplete: "off" }} />

              <Field label={t("captiveportal.sessionLabel")} icon={Timer} hint={t("captiveportal.sessionHint")}
                error={!sessionOk ? t("captiveportal.sessionBad") : undefined}
                inputProps={{ value: sessionMinutes, onChange: (e) => setSessionMinutes(e.target.value), inputMode: "numeric", mono: true }} />

              <div>
                <span className="mb-1.5 flex items-center gap-1 text-caption font-medium text-muted">
                  {t("captiveportal.imageLabel")}
                </span>
                <div className="flex items-center gap-2">
                  <input ref={fileRef} type="file" accept=".png,.jpg,.jpeg,.gif,.webp" onChange={onFile} className="hidden" />
                  <Button variant="secondary" size="sm" icon={ImageUp} disabled={busy} onClick={() => fileRef.current?.click()}>
                    {probe.has_image ? t("captiveportal.imageReplace") : t("captiveportal.imageUpload")}
                  </Button>
                  {probe.has_image && <Pill tone="ok">{t("captiveportal.imageSet")}</Pill>}
                </div>
                {imageErr ? <p className="mt-1 text-caption text-danger">{imageErr}</p>
                  : <p className="mt-1 text-caption text-muted">{t("captiveportal.imageHint")}</p>}
              </div>

              <AdvancedDisclosure label={t("captiveportal.customTitle")}>
                <p className="text-caption text-muted mb-2">{t("captiveportal.customHint")}</p>
                <textarea
                  value={customHtml}
                  onChange={(e) => setCustomHtml(e.target.value)}
                  rows={6}
                  placeholder={t("captiveportal.customPlaceholder")}
                  className="w-full rounded-[10px] border border-transparent bg-fill px-3 py-2 text-small font-mono outline-none
                    placeholder:text-faint transition-[background-color,border-color,box-shadow] duration-[var(--dur-fast)]
                    hover:border-border-strong focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-accent)]"
                />
              </AdvancedDisclosure>

              <div className="flex gap-2">
                <Button size="sm" loading={busy} disabled={!sessionOk || !codeOk} onClick={save}>
                  {t("captiveportal.save")}
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
