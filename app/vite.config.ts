import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { gzipSync } from "node:zlib";

// Tracker GoatCounter SOLO en el build de la demo pública:
//   VITE_GC_COUNT=https://stats.netgrip.cloudless.club npm run build
// Los builds normales NO lo llevan: una instalación self-hosted nunca debe
// llamar a casa. Los hits se registran con prefijo /demo en el mismo site
// que la landing ("/" = landing, "/demo/..." = demo).
const gcCount = process.env.VITE_GC_COUNT?.replace(/\/$/, "");

function goatcounterPlugin(): Plugin {
  return {
    name: "netgrip-goatcounter",
    transformIndexHtml(html) {
      if (!gcCount) return html;
      const snippet =
        `    <script>window.goatcounter={path:function(p){return '/demo'+p}}</script>\n` +
        `    <script async data-goatcounter="${gcCount}/count" src="${gcCount}/count.js"></script>\n  </head>`;
      return html.replace("</head>", snippet);
    },
  };
}

// JavaScript and CSS dominate the embedded filesystem. Store only their gzip
// representation; the Go server sends it directly to browsers and inflates it
// only for clients that do not advertise gzip support.
// Se hace en writeBundle y no en generateBundle: el generateBundle de este
// plugin corría antes que el de Vite 8 (vite:build-import-analysis) y borraba
// los chunks antes de que Vite sustituyera el marcador interno
// __VITE_PRELOAD__ de los imports dinámicos. El marcador quedaba literal en
// el .gz y el modo demo explotaba al cargar su chunk lazy. writeBundle corre
// después de TODOS los hooks de generateBundle, con el código ya definitivo.
function gzipEmbeddedAssets(): Plugin {
  let outDir = "";
  return {
    name: "netgrip-gzip-embedded-assets",
    apply: "build",
    enforce: "post",
    configResolved(config) {
      outDir = path.resolve(config.root, config.build.outDir);
    },
    writeBundle(_options, bundle) {
      for (const [fileName, item] of Object.entries(bundle)) {
        if (!fileName.endsWith(".js") && !fileName.endsWith(".css")) continue;
        const source = item.type === "chunk" ? item.code : item.source;
        const target = path.join(outDir, `${fileName}.gz`);
        mkdirSync(path.dirname(target), { recursive: true });
        writeFileSync(target, gzipSync(typeof source === "string" ? source : Buffer.from(source), { level: 9 }));
        rmSync(path.join(outDir, fileName));
      }
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), goatcounterPlugin(), !process.env.VITE_DEMO && gzipEmbeddedAssets()],
  build: {
    outDir: "../internal/server/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://192.168.1.3:8080",
    },
  },
});
