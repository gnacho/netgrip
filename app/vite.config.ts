import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

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

export default defineConfig({
  plugins: [react(), tailwindcss(), goatcounterPlugin()],
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
