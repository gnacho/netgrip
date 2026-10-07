/* ==========================================================================
   NetGrip - Landing · app.js
   ========================================================================== */
const DEMO_URL     = "https://demo.netgrip.cloudless.club";
const REPO_URL     = "https://github.com/gnacho/netgrip";
const NETPULSE_URL = "https://netpulse.cloudless.club";

/* ---------- Screenshots del hero por tema ---------- */
const HERO_SHOT = {
  light: "assets/shot-overview-light.webp",
  dark:  "assets/shot-overview-dark.webp"
};

/* ==========================================================================
   Diccionario i18n - ES (defecto) / EN
   Tokens disponibles en los valores: {DEMO_URL} {REPO_URL} {NETPULSE_URL}
   ========================================================================== */
const I18N = {
  es: {
    "a11y.skip": "Saltar al contenido",
    "a11y.theme": "Cambiar entre tema claro y oscuro",
    "a11y.star": "Dale una estrella a NetGrip en GitHub",
    "a11y.menu": "Abrir o cerrar el menú de navegación",
    "a11y.close": "Cerrar la imagen ampliada",
    "a11y.prev": "Ver la captura anterior",
    "a11y.next": "Ver la captura siguiente",
    "a11y.lightbox": "Imagen ampliada de NetGrip",

    "nav.features": "Funcionalidades",
    "nav.what": "Qué es",
    "nav.shots": "Capturas",
    "nav.community": "Comunidad",
    "nav.github": "GitHub",
    "nav.demo": "Demo",
    "nav.star": "Star",

    "hero.eyebrow": "Software libre · AGPL · Hecho por la comunidad",
    "hero.title": "OpenWrt para todos.",
    "hero.sub": "Tu router con OpenWrt puede hacer de todo: VPN, WiFi de invitados, priorizar videollamadas… NetGrip lo convierte en algo que cualquiera de casa puede usar: sin manuales, sin miedo a romper nada.",
    "hero.ctaDemo": "Probar la demo",
    "hero.ctaGithub": "Ver en GitHub",
    "hero.micro": "La demo es de juguete: datos de ejemplo, nada se aplica de verdad.",
    "hero.shotAlt": "Panel Overview de NetGrip en modo claro: salud del router, tráfico en vivo y CPU por núcleos",
    "hero.shotAltDark": "Panel Overview de NetGrip en modo oscuro: salud del router, tráfico en vivo y CPU por núcleos",

    "strip.gpl.t": "100% libre (AGPL)",
    "strip.gpl.d": "Para siempre. Sin versión «pro».",
    "strip.local.t": "Sin cuentas ni nube",
    "strip.local.d": "Todo ocurre en tu router, en tu red.",
    "strip.zero.t": "Cero funciones nuevas",
    "strip.zero.d": "Solo una interfaz amable sobre OpenWrt.",
    "strip.community.t": "Community-driven",
    "strip.community.d": "Las prioridades las marca la comunidad.",

    "what.eyebrow": "Qué es",
    "what.title": "Toda la potencia de OpenWrt, sin la curva de aprendizaje.",
    "what.p1": "OpenWrt es increíblemente capaz: VPN, cortafuegos, priorización de tráfico, redes separadas… está todo ahí dentro.",
    "what.p2": "Sus herramientas clásicas, como LuCI, son potentes y completas, pero están pensadas para gente técnica. NetGrip es una capa amable encima: las mismas funciones, el mismo router, explicadas en tu idioma.",
    "what.cardTitle": "¿Qué añade NetGrip? Nada. Y eso es lo bueno.",
    "what.cardText": "No inventa funciones: todo lo que ves ya lo puede hacer OpenWrt. NetGrip solo lo hace entendible: textos claros, ayudas en cada ajuste y gráficos que se entienden de un vistazo.",
    "what.listTitle": "Amable =",
    "what.li1": "Ayudas contextuales en cada ajuste",
    "what.li2": "Confirmaciones con deshacer",
    "what.li3": "Gráficos en vez de tablas",
    "what.li4": "Asistente de primer arranque",
    "what.netpulse": "También hace de agente de <a href=\"{NETPULSE_URL}\" target=\"_blank\" rel=\"noopener\">NetPulse</a>: el mismo router reporta métricas, eventos WiFi y clientes sin instalar nada más.",
    "what.footprint": "Ligero de verdad: unos 10 MB en disco y ~15 MB de RAM en uso, medidos en un router ARM64.",

    "feat.eyebrow": "Funcionalidades",
    "feat.title": "Todo lo que tu router ya hace, en amable.",
    "feat.f1.t": "WiFi sin líos",
    "feat.f1.d": "Redes de invitados e IoT con dos toques, QR incluido.",
    "feat.f2.t": "VPN con WireGuard",
    "feat.f2.d": "Entra a tu casa desde fuera escaneando un código QR.",
    "feat.f3.t": "Quién usa tu red",
    "feat.f3.d": "Dispositivos, consumo y tráfico en vivo, con gráficos.",
    "feat.f4.t": "Cortafuegos visual",
    "feat.f4.d": "Zonas y reglas explicadas con un diagrama, no con tablas.",
    "feat.f5.t": "Adiós al lag (SQM)",
    "feat.f5.d": "Videollamadas fluidas aunque alguien suba 200 fotos.",
    "feat.f6.t": "Un nombre para tu casa (DDNS)",
    "feat.f6.d": "Acceso remoto sin pelearte con IPs que cambian.",
    "feat.f7.t": "Copias y plantillas",
    "feat.f7.d": "Snapshots antes de tocar nada, y plantillas listas para usar.",
    "feat.f8.t": "Toda la red con NetPulse",
    "feat.f8.d": "¿Varios routers? Nuestra app hermana los gestiona en conjunto.",
    "feat.f9.t": "VLANs y puertos",
    "feat.f9.d": "Redes por cable separadas, con los puertos reales de tu placa y la de acceso protegida.",
    "feat.f10.t": "CPU núcleo a núcleo",
    "feat.f10.d": "El promedio engaña: mira el núcleo más cargado y los paquetes que se pierden.",
    "feat.f11.t": "Diagnóstico que explica",
    "feat.f11.d": "Ping, traceroute y DNS con resultados legibles, y un autochequeo que dice el porqué.",
    "feat.f12.t": "Qué tienes abierto a internet",
    "feat.f12.d": "Todo lo visible desde fuera, forwards y listeners incluidos; lo creado fuera, en solo lectura.",
    "feat.f13.t": "Home Assistant y MQTT",
    "feat.f13.d": "El router se descubre solo en Home Assistant: sensores de estado y tráfico, interruptores que actúan (WiFi de invitados, banIP, IPv6, SQM) y botón de reinicio. Desactivado por defecto.",
    "feat.f14.t": "Protección y baneo de IPs",
    "feat.f14.d": "banIP integrado: feeds de listas negras, listas locales y un monitor que bloquea lo que insiste, con aviso cuando la RAM aprieta.",
    "feat.f15.t": "Tu router, hablando con asistentes de IA",
    "feat.f15.d": "Endpoint MCP con 10 herramientas de solo lectura (estado, clientes, WiFi, WAN, SNMP) para que un asistente lea tu router en nuestro vocabulario. Desactivado por defecto, con token propio.",
    "feat.f16.t": "Modo avanzado opt-in",
    "feat.f16.d": "VLANs, SNMP, storm control y ACLs de MAC tras un check en Ajustes: potencia real fuera de vista hasta que la pides, y un menú que solo muestra lo que tu equipo tiene.",
    "feat.f17.t": "WiFi que se ajusta solo",
    "feat.f17.d": "Canal recomendado a partir de la utilización real del aire, aplicable en un clic, y horarios semanales por red: la de invitados, fuera de madrugada.",

    "shots.eyebrow": "Capturas",
    "shots.title": "Se ve tan bien como funciona.",
    "shots.cap1": "VLANs por puerto con su matriz de etiquetado, estadísticas y agregación LAG.",
    "shots.cap2": "Bloqueo de anuncios, cortafuegos visual, DDNS y uso por dispositivo: cada servicio en su tarjeta.",
    "shots.cap3": "WiFi de invitados e IoT con QR listo para compartir.",
    "shots.cap4": "Autodiagnóstico que dice qué falla y por qué, más ping, traceroute, DNS y puertos.",
    "shots.cap5": "El modo oscuro también es cosa seria.",
    "shots.note": "Capturas reales del modo demo.",
    "shots.alt1": "Pantalla de puertos de NetGrip: tabla de VLANs por puerto con etiquetado U/T, estadísticas y LAG",
    "shots.alt2": "Pantalla de servicios de NetGrip: bloqueo de anuncios, cortafuegos visual, DDNS y consumo por dispositivo",
    "shots.alt3": "Pantalla de WiFi de NetGrip con redes de invitados e IoT y códigos QR",
    "shots.alt4": "Pantalla de diagnóstico de NetGrip con autochequeo explicado y herramientas ping, traceroute, DNS y puerto",
    "shots.alt5": "Panel Overview de NetGrip en modo oscuro con gráficos de tráfico",

    "comm.eyebrow": "Comunidad",
    "comm.title": "Libre hoy, libre siempre.",
    "comm.p": "NetGrip es AGPL y lo seguirá siendo: sin planes de pago, sin funciones capadas, sin sorpresas. El rumbo lo decide la comunidad en GitHub: issues, ideas, traducciones y código.",
    "comm.c1.t": "Reporta o pide",
    "comm.c1.d": "Abre un issue con ese fallo que te persigue o la función que echas de menos.",
    "comm.c2.t": "Traduce",
    "comm.c2.d": "NetGrip ya habla español e inglés. Añade tu idioma y ayuda a más gente.",
    "comm.c3.t": "Programa",
    "comm.c3.d": "React en el front, Go en el router. Los PRs son bienvenidos, grandes o pequeños.",
    "comm.github": "Échale un ojo en GitHub",
    "comm.netpulse": "¿Tienes varios routers? <a href=\"{NETPULSE_URL}\" target=\"_blank\" rel=\"noopener\">NetPulse</a>, nuestra app hermana, los gestiona en conjunto.",
    "comm.star": "¿Te sirve NetGrip? <a href=\"https://github.com/gnacho/netgrip/stargazers\" target=\"_blank\" rel=\"noopener\">Una estrella en GitHub</a> ayuda mucho y no cuesta nada.",

    "cta.title": "¿Le das una vuelta?",
    "cta.demo": "Probar la demo",
    "cta.github": "Ver en GitHub",
    "cta.micro": "5 minutos en la demo, sin instalar nada.",

    "footer.tag": "NetGrip es software libre (AGPL) hecho por su comunidad.",
    "footer.disclaimer": "OpenWrt es una marca de su proyecto; NetGrip no está afiliado oficialmente.",

    "meta.title": "NetGrip - OpenWrt para todos",
    "meta.description": "NetGrip es un panel amable y software libre (AGPL) para routers OpenWrt. Toda la potencia de tu router, al alcance de cualquiera."
  },

  en: {
    "a11y.skip": "Skip to content",
    "a11y.theme": "Switch between light and dark theme",
    "a11y.star": "Star NetGrip on GitHub",
    "a11y.menu": "Open or close the navigation menu",
    "a11y.close": "Close the enlarged image",
    "a11y.prev": "View the previous screenshot",
    "a11y.next": "View the next screenshot",
    "a11y.lightbox": "Enlarged NetGrip image",

    "nav.features": "Features",
    "nav.what": "What it is",
    "nav.shots": "Screenshots",
    "nav.community": "Community",
    "nav.github": "GitHub",
    "nav.demo": "Demo",
    "nav.star": "Star",

    "hero.eyebrow": "Free software · AGPL · Built by the community",
    "hero.title": "OpenWrt for everyone.",
    "hero.sub": "Your OpenWrt router can do it all: VPN, guest Wi-Fi, smooth video calls… NetGrip turns it into something anyone at home can use: no manuals, no fear of breaking things.",
    "hero.ctaDemo": "Try the demo",
    "hero.ctaGithub": "View on GitHub",
    "hero.micro": "The demo is a playground: sample data, nothing is applied for real.",
    "hero.shotAlt": "NetGrip Overview dashboard in light mode: router health, live traffic and per-core CPU",
    "hero.shotAltDark": "NetGrip Overview dashboard in dark mode: router health, live traffic and per-core CPU",

    "strip.gpl.t": "100% free (AGPL)",
    "strip.gpl.d": "Forever. No “pro” version.",
    "strip.local.t": "No accounts, no cloud",
    "strip.local.d": "Everything happens on your router, on your network.",
    "strip.zero.t": "Zero new features",
    "strip.zero.d": "Just a friendly interface on top of OpenWrt.",
    "strip.community.t": "Community-driven",
    "strip.community.d": "The community sets the priorities.",

    "what.eyebrow": "What it is",
    "what.title": "All the power of OpenWrt, without the learning curve.",
    "what.p1": "OpenWrt is incredibly capable: VPN, firewall, traffic shaping, separate networks… it's all in there.",
    "what.p2": "Its classic tools, like LuCI, are powerful and complete, but built for technical folks. NetGrip is a friendly layer on top: same features, same router, explained in your language.",
    "what.cardTitle": "What does NetGrip add? Nothing. And that's the point.",
    "what.cardText": "It doesn't invent features: everything you see, OpenWrt can already do. NetGrip just makes it understandable: clear wording, help on every setting, and charts you can read at a glance.",
    "what.listTitle": "Friendly =",
    "what.li1": "Contextual help on every setting",
    "what.li2": "Confirmations with undo",
    "what.li3": "Charts instead of tables",
    "what.li4": "First-run setup wizard",
    "what.netpulse": "It doubles as a <a href=\"{NETPULSE_URL}\" target=\"_blank\" rel=\"noopener\">NetPulse</a> agent: the same router reports metrics, WiFi events and clients, with nothing extra to install.",
    "what.footprint": "Genuinely light: about 10 MB on disk and ~15 MB of RAM in use (measured on an ARM64 router).",

    "feat.eyebrow": "Features",
    "feat.title": "Everything your router already does, made friendly.",
    "feat.f1.t": "Wi-Fi without the fuss",
    "feat.f1.d": "Guest and IoT networks in two taps, QR code included.",
    "feat.f2.t": "VPN with WireGuard",
    "feat.f2.d": "Get into your home from anywhere by scanning a QR code.",
    "feat.f3.t": "Who's on your network",
    "feat.f3.d": "Devices, usage and live traffic, with charts.",
    "feat.f4.t": "Visual firewall",
    "feat.f4.d": "Zones and rules explained with a diagram, not tables.",
    "feat.f5.t": "Bye-bye lag (SQM)",
    "feat.f5.d": "Smooth video calls even while someone uploads 200 photos.",
    "feat.f6.t": "A name for your home (DDNS)",
    "feat.f6.d": "Remote access without fighting ever-changing IPs.",
    "feat.f7.t": "Backups & templates",
    "feat.f7.d": "Snapshots before you touch anything, plus ready-made templates.",
    "feat.f8.t": "Your whole network with NetPulse",
    "feat.f8.d": "Multiple routers? Our sister app manages them together.",
    "feat.f9.t": "VLANs and ports",
    "feat.f9.d": "Separate wired networks, your board's real port list, and the one you reach the router on protected.",
    "feat.f10.t": "CPU, core by core",
    "feat.f10.d": "The average lies: see the busiest core and the packets it is dropping.",
    "feat.f11.t": "Diagnostics that explain",
    "feat.f11.d": "Ping, traceroute and DNS with readable results, and a self-test that says why.",
    "feat.f12.t": "What's open to the Internet",
    "feat.f12.d": "Everything visible from outside, forwards and listeners included; what was made elsewhere stays read-only.",
    "feat.f13.t": "Home Assistant and MQTT",
    "feat.f13.d": "The router discovers itself in Home Assistant: state and traffic sensors, switches that act (guest Wi-Fi, banIP, IPv6, SQM) and a reboot button. Off by default.",
    "feat.f14.t": "IP protection and banning",
    "feat.f14.d": "banIP built in: blocklist feeds, local allow/block lists and a monitor that blocks persistent abusers, with a heads-up when RAM runs short.",
    "feat.f15.t": "Your router, talking to AI assistants",
    "feat.f15.d": "An MCP endpoint with 10 read-only tools (status, clients, Wi-Fi, WAN, SNMP) so an assistant can read your router in our vocabulary. Off by default, with its own token.",
    "feat.f16.t": "Opt-in advanced mode",
    "feat.f16.d": "VLANs, SNMP, storm control and MAC ACLs behind one settings toggle: real power out of sight until you ask, and a menu that only shows what your device has.",
    "feat.f17.t": "Wi-Fi that tunes itself",
    "feat.f17.d": "A channel recommendation built on real airtime utilization, one click to apply, plus weekly per-network schedules: the guest network, off at 3 am.",

    "shots.eyebrow": "Screenshots",
    "shots.title": "Looks as good as it works.",
    "shots.cap1": "Per-port VLANs with their tagging matrix, statistics and link aggregation.",
    "shots.cap2": "Ad blocking, a visual firewall, DDNS and per-device usage: every service gets its own card.",
    "shots.cap3": "Guest and IoT Wi-Fi with a QR code ready to share.",
    "shots.cap4": "A self-test that says what fails and why, plus ping, traceroute, DNS and port checks.",
    "shots.cap5": "Dark mode means business too.",
    "shots.note": "Real screenshots from demo mode.",
    "shots.alt1": "NetGrip ports screen: per-port VLAN table with U/T tagging, statistics and LAG",
    "shots.alt2": "NetGrip services screen: ad blocking, visual firewall, DDNS and per-device usage",
    "shots.alt3": "NetGrip Wi-Fi screen with guest and IoT networks and QR codes",
    "shots.alt4": "NetGrip diagnostics screen with an explained self-test plus ping, traceroute, DNS and port tools",
    "shots.alt5": "NetGrip Overview dashboard in dark mode with traffic charts",

    "comm.eyebrow": "Community",
    "comm.title": "Free today, free forever.",
    "comm.p": "NetGrip is AGPL and will stay that way: no paid plans, no crippled features, no surprises. The community steers the project on GitHub: issues, ideas, translations and code.",
    "comm.c1.t": "Report or request",
    "comm.c1.d": "Open an issue about that bug that keeps chasing you, or the feature you miss.",
    "comm.c2.t": "Translate",
    "comm.c2.d": "NetGrip already speaks Spanish and English. Add your language and help more people.",
    "comm.c3.t": "Code",
    "comm.c3.d": "React up front, Go on the router. PRs welcome, big or small.",
    "comm.github": "Check it out on GitHub",
    "comm.netpulse": "Running several routers? <a href=\"{NETPULSE_URL}\" target=\"_blank\" rel=\"noopener\">NetPulse</a>, our sister app, manages them together.",
    "comm.star": "Does NetGrip help you? <a href=\"https://github.com/gnacho/netgrip/stargazers\" target=\"_blank\" rel=\"noopener\">A star on GitHub</a> helps a lot and costs nothing.",

    "cta.title": "Take it for a spin?",
    "cta.demo": "Try the demo",
    "cta.github": "View on GitHub",
    "cta.micro": "5 minutes in the demo, nothing to install.",

    "footer.tag": "NetGrip is free software (AGPL) made by its community.",
    "footer.disclaimer": "OpenWrt is a trademark of its project; NetGrip is not officially affiliated.",

    "meta.title": "NetGrip - OpenWrt for everyone",
    "meta.description": "NetGrip is a friendly, free software (AGPL) dashboard for OpenWrt routers. All the power of your router, within anyone's reach."
  }
};

/* ========================================================================== */
(function () {
  "use strict";
  document.documentElement.classList.add("js");

  const root = document.documentElement;
  const LS_THEME = "netgrip-web-theme";
  const LS_LANG = "netgrip-web-lang";

  /* ---------- URLs desde constantes ---------- */
  const URLS = { demo: DEMO_URL, repo: REPO_URL, netpulse: NETPULSE_URL };
  document.querySelectorAll("[data-url]").forEach((el) => {
    const u = URLS[el.getAttribute("data-url")];
    if (u) el.setAttribute("href", u);
  });

  /* ---------- Tema ---------- */
  const themeToggle = document.getElementById("theme-toggle");
  const heroShot = document.getElementById("hero-shot");

  function currentTheme() {
    return root.getAttribute("data-theme") === "dark" ? "dark" : "light";
  }

  function applyTheme(theme, persist) {
    root.setAttribute("data-theme", theme);
    if (persist) {
      try { localStorage.setItem(LS_THEME, theme); } catch (e) { /* noop */ }
    }
    updateHeroShot();
  }

  function updateHeroShot() {
    if (!heroShot) return;
    const theme = currentTheme();
    const src = HERO_SHOT[theme];
    if (heroShot.getAttribute("src") !== src) heroShot.setAttribute("src", src);
    const lang = currentLang();
    const altKey = theme === "dark" ? "hero.shotAltDark" : "hero.shotAlt";
    if (I18N[lang] && I18N[lang][altKey]) heroShot.setAttribute("alt", I18N[lang][altKey]);
  }

  themeToggle.addEventListener("click", () => {
    applyTheme(currentTheme() === "dark" ? "light" : "dark", true);
  });

  /* Si el usuario no ha elegido tema, seguir al sistema */
  try {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    mq.addEventListener("change", (e) => {
      let saved = null;
      try { saved = localStorage.getItem(LS_THEME); } catch (err) { /* noop */ }
      if (saved !== "light" && saved !== "dark") {
        applyTheme(e.matches ? "dark" : "light", false);
      }
    });
  } catch (e) { /* noop */ }

  /* ---------- Idioma ---------- */
  const langButtons = document.querySelectorAll(".lang-btn");

  function currentLang() {
    return root.getAttribute("lang") === "en" ? "en" : "es";
  }

  function translate(str) {
    return str
      .replaceAll("{DEMO_URL}", DEMO_URL)
      .replaceAll("{REPO_URL}", REPO_URL)
      .replaceAll("{NETPULSE_URL}", NETPULSE_URL);
  }

  function applyLang(lang, persist) {
    const dict = I18N[lang] || I18N.es;
    root.setAttribute("lang", lang);
    if (persist) {
      try { localStorage.setItem(LS_LANG, lang); } catch (e) { /* noop */ }
    }

    document.querySelectorAll("[data-i18n]").forEach((el) => {
      const key = el.getAttribute("data-i18n");
      if (dict[key] !== undefined) el.innerHTML = translate(dict[key]);
    });
    document.querySelectorAll("[data-i18n-alt]").forEach((el) => {
      const key = el.getAttribute("data-i18n-alt");
      if (dict[key] !== undefined) el.setAttribute("alt", dict[key]);
    });
    document.querySelectorAll("[data-i18n-aria]").forEach((el) => {
      const key = el.getAttribute("data-i18n-aria");
      if (dict[key] !== undefined) el.setAttribute("aria-label", dict[key]);
    });

    document.title = dict["meta.title"];
    const metaDesc = document.querySelector('meta[name="description"]');
    if (metaDesc) metaDesc.setAttribute("content", dict["meta.description"]);

    langButtons.forEach((btn) => {
      const active = btn.getAttribute("data-lang") === lang;
      btn.classList.toggle("is-active", active);
      btn.setAttribute("aria-pressed", active ? "true" : "false");
    });

    updateHeroShot();
  }

  langButtons.forEach((btn) => {
    btn.addEventListener("click", () => applyLang(btn.getAttribute("data-lang"), true));
  });

  /* Idioma inicial: ES por defecto; solo se cambia si el usuario lo guardó */
  let initialLang = "es";
  try {
    const saved = localStorage.getItem(LS_LANG);
    if (saved === "es" || saved === "en") initialLang = saved;
  } catch (e) { /* noop */ }
  if (initialLang !== "es") applyLang(initialLang, false);
  else applyLang("es", false); // asegura estado de botones y meta

  /* ---------- Header con blur al scroll ---------- */
  const header = document.getElementById("header");
  function onScroll() {
    header.classList.toggle("scrolled", window.scrollY > 12);
  }
  window.addEventListener("scroll", onScroll, { passive: true });
  onScroll();

  /* ---------- Nav móvil hamburguesa ---------- */
  const hamburger = document.getElementById("hamburger");
  const nav = document.getElementById("nav");

  function closeNav() {
    header.classList.remove("nav-open");
    hamburger.setAttribute("aria-expanded", "false");
  }
  hamburger.addEventListener("click", () => {
    const open = header.classList.toggle("nav-open");
    hamburger.setAttribute("aria-expanded", open ? "true" : "false");
  });
  nav.querySelectorAll("a").forEach((a) => a.addEventListener("click", closeNav));
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") closeNav();
  });
  window.addEventListener("resize", () => {
    if (window.innerWidth > 860) closeNav();
  });

  /* ---------- Reveal on scroll (IntersectionObserver + stagger) ---------- */
  const revealEls = document.querySelectorAll(".reveal");
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  if (reducedMotion || !("IntersectionObserver" in window)) {
    revealEls.forEach((el) => el.classList.add("in"));
  } else {
    /* Stagger: pequeño retardo a los hermanos consecutivos dentro del mismo grid */
    document.querySelectorAll(".strip-grid, .feat-grid, .comm-grid, .what-list ul").forEach((group) => {
      group.querySelectorAll(".reveal").forEach((el, i) => {
        el.style.setProperty("--reveal-delay", `${Math.min(i * 70, 350)}ms`);
      });
    });

    const io = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          entry.target.classList.add("in");
          io.unobserve(entry.target);
        }
      });
    }, { threshold: 0.12, rootMargin: "0px 0px -40px 0px" });

    revealEls.forEach((el) => io.observe(el));
  }

  /* ---------- Lightbox: capturas a tamano real (1440px) ---------- */
  const lightbox = document.getElementById("lightbox");
  if (lightbox) {
    const lbImg = document.getElementById("lightbox-img");
    const lbCaption = document.getElementById("lightbox-caption");
    const lbCloseBtn = document.getElementById("lightbox-close");
    const lbPrev = document.getElementById("lightbox-prev");
    const lbNext = document.getElementById("lightbox-next");
    const shots = Array.from(document.querySelectorAll(".browser img"));
    let lbIndex = -1;
    let lbLastFocus = null;

    function lbShow(i) {
      if (!shots.length) return;
      lbIndex = (i + shots.length) % shots.length;
      const img = shots[lbIndex];
      lbImg.src = img.src;
      lbImg.alt = img.alt || "";
      const fig = img.closest("figure");
      const cap = fig && fig.querySelector("figcaption");
      lbCaption.textContent = cap ? cap.textContent : "";
    }

    function lbOpen(i) {
      lbLastFocus = document.activeElement;
      lbShow(i);
      lightbox.hidden = false;
      requestAnimationFrame(() => lightbox.classList.add("open"));
      document.body.style.overflow = "hidden";
      lbCloseBtn.focus();
    }

    function lbCloseNow() {
      lightbox.classList.remove("open");
      document.body.style.overflow = "";
      window.setTimeout(() => { lightbox.hidden = true; }, reducedMotion ? 0 : 180);
      if (lbLastFocus && typeof lbLastFocus.focus === "function") lbLastFocus.focus();
      lbIndex = -1;
    }

    shots.forEach((img, i) => {
      img.addEventListener("click", () => lbOpen(i));
      img.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") { e.preventDefault(); lbOpen(i); }
      });
    });

    lbCloseBtn.addEventListener("click", lbCloseNow);
    document.getElementById("lightbox-backdrop").addEventListener("click", lbCloseNow);
    lbPrev.addEventListener("click", () => lbShow(lbIndex - 1));
    lbNext.addEventListener("click", () => lbShow(lbIndex + 1));
    lightbox.addEventListener("keydown", (e) => {
      if (e.key === "Escape") lbCloseNow();
      else if (e.key === "ArrowLeft") lbShow(lbIndex - 1);
      else if (e.key === "ArrowRight") lbShow(lbIndex + 1);
    });
  }

  // Contador de estrellas del repo (fail-soft: si la API limita, el botón sigue funcionando)
  (async () => {
    try {
      const r = await fetch("https://api.github.com/repos/gnacho/netgrip", { headers: { Accept: "application/vnd.github+json" } });
      if (!r.ok) return;
      const d = await r.json();
      if (typeof d.stargazers_count !== "number") return;
      document.querySelectorAll(".star-count[data-n]").forEach((el) => {
        el.textContent = d.stargazers_count;
        el.hidden = false;
      });
    } catch { /* sin red o API limitada */ }
  })();
})();
