import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ExternalLink, Github, Star } from "lucide-react";
import { api } from "../api";
import type { SelfUpdateCheck, UpdateCheck } from "../types";
import { Card, KeyValue } from "../components/ui";
import { Logo } from "../components/ui/illustrations";
import { DEVELOPER, LICENSE, LICENSE_URL, REPO_URL, STARGAZERS_URL, WEBSITE_URL } from "../about";

/** Enlace externo (nueva pestaña), acento y con icono, como en el resto. */
function External({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer"
      className="inline-flex items-center gap-1 text-accent hover:text-accent-hover ring-focus rounded-sm break-all">
      {children}
      <ExternalLink size={12} aria-hidden="true" className="shrink-0" />
    </a>
  );
}

/**
 * Acerca de (#372): identidad de la app, enlaces del proyecto y el CTA de
 * GitHub. La versión de NetGrip viene de /api/selfupdate (igual que la
 * tarjeta de identidad de Sistema) y la de OpenWrt del mismo /api/update
 * que usa la tarjeta de actualización; nada hardcodeado, en demo se
 * muestran las simuladas.
 */
export function AboutPage() {
  const { t } = useTranslation();
  const [check, setCheck] = useState<SelfUpdateCheck>();
  const [owrt, setOwrt] = useState<UpdateCheck>();

  useEffect(() => {
    api.selfUpdateCheck().then(setCheck).catch(() => {});
    api.updateCheck().then(setOwrt).catch(() => {});
  }, []);

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* CTA estrella (#372): lo importante, arriba y bien visible. */}
      <Card index={0} icon={Star} iconTone="warn" title={t("about.starTitle")}>
        <p className="text-small text-muted">{t("about.starDesc")}</p>
        <a href={STARGAZERS_URL} target="_blank" rel="noopener noreferrer"
          className="mt-4 inline-flex items-center justify-center gap-2 rounded-md font-medium ring-focus
            bg-accent text-on-accent hover:bg-accent-hover
            transition-[background-color,color,transform] duration-[var(--dur-fast)] ease-[var(--ease-soft)]
            active:scale-[0.98] h-10 px-4 text-body">
          <Star size={18} aria-hidden="true" />
          {t("about.starButton")}
        </a>
      </Card>

      {/* Identidad: nombre y versión. */}
      <Card index={1}>
        <div className="flex items-center gap-3">
          <span className="text-accent shrink-0"><Logo size={40} /></span>
          <div className="flex-1 min-w-0">
            <p className="text-h2 truncate">{t("app.name")}</p>
            <p className="text-small text-muted mt-0.5">{t("about.description")}</p>
          </div>
          <span className="shrink-0 font-mono text-small text-muted">
            v{check?.current ?? "…"}
          </span>
        </div>
        {owrt?.version_from && (
          <p className="mt-2 text-caption text-faint font-mono">OpenWrt {owrt.version_from}</p>
        )}
      </Card>

      {/* Enlaces del proyecto. */}
      <Card index={2} icon={Github} iconTone="muted" title={t("about.project")}>
        <KeyValue items={[
          { label: t("about.repository"), value: <External href={REPO_URL}>{REPO_URL}</External> },
          { label: t("about.website"), value: <External href={WEBSITE_URL}>{WEBSITE_URL}</External> },
          { label: t("about.developer"), value: DEVELOPER },
          { label: t("about.license"), value: <External href={LICENSE_URL}>{LICENSE}</External> },
        ]} />
      </Card>
    </div>
  );
}
