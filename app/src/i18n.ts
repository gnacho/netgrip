import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import es from "./locales/es";
import en from "./locales/en";

const stored = localStorage.getItem("netgrip-lang");
const browser = navigator.language.startsWith("es") ? "es" : "en";

i18n.use(initReactI18next).init({
  resources: { es: { translation: es }, en: { translation: en } },
  lng: stored || browser,
  fallbackLng: "en",
  interpolation: { escapeValue: false },
});

i18n.on("languageChanged", (lng) => localStorage.setItem("netgrip-lang", lng));

export default i18n;
