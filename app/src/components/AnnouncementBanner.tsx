import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ExternalLink, Megaphone } from "lucide-react";
import { api } from "../api";
import type { Announcement } from "../types";
import { Banner, Button } from "./ui";

const DISMISS_KEY = "netgrip:announcement.dismissed";
const POLL_MS = 6 * 60 * 60 * 1000;

const pick = (m: Record<string, string> | undefined, lang: string) =>
  (m && (m[lang] || m.en || m.es || "")) || "";

/**
 * Franja de anuncios externos (#382): announcements.json del repo, servido
 * por /api/announcement y refrescado en background cada 6 h. El descarte es
 * por id y persiste en localStorage. Mismo patrón que NetPulse.
 */
export function AnnouncementBanner() {
  const { t, i18n } = useTranslation();
  const [a, setA] = useState<Announcement | null>(null);
  const [dismissed, setDismissed] = useState<string | null>(null);

  const fetchA = useCallback(() => {
    api.announcement()
      .then((json) => setA(json.active && json.announcement ? json.announcement : null))
      .catch(() => { /* sin aviso */ });
  }, []);

  useEffect(() => {
    try {
      setDismissed(localStorage.getItem(DISMISS_KEY));
    } catch { /* sin localStorage: siempre visible */ }
    fetchA();
    const id = window.setInterval(fetchA, POLL_MS);
    return () => window.clearInterval(id);
  }, [fetchA]);

  if (!a || a.id === dismissed) return null;

  const lang = i18n.language?.startsWith("en") ? "en" : "es";
  const title = pick(a.title, lang);
  const body = pick(a.body, lang);
  const urlLabel = pick(a.urlLabel, lang) || t("announcement.link");
  const warn = a.urgency === "warn";

  const dismiss = () => {
    setDismissed(a.id);
    try {
      localStorage.setItem(DISMISS_KEY, a.id);
    } catch { /* sin persistencia: se descarta hasta el refresco */ }
  };

  return (
    <Banner
      tone={warn ? "warn" : "info"}
      icon={Megaphone}
      className="mb-4"
      onDismiss={dismiss}
      action={a.url ? (
        <a href={a.url} target="_blank" rel="noopener noreferrer">
          <Button variant="secondary" size="sm">
            <ExternalLink size={14} aria-hidden="true" /> {urlLabel}
          </Button>
        </a>
      ) : undefined}
    >
      <span className="font-medium">{title}</span>
      {body && <span className="text-muted"> - {body}</span>}
    </Banner>
  );
}
