import { useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy, ListChecks, Wrench } from "lucide-react";
import { DnsCard, PingCard, SelfTestCard, TcpCard, TracerouteCard } from "../components/diagnostics/cards";
import { SpeedTestCard } from "../components/diagnostics/SpeedTestCard";
import { buildShareText } from "../components/diagnostics/share";
import { CableTestTool, BounceTool, LoopsTool } from "../components/tools/diagnostics";
import { Button, Card, Modal, useToast } from "../components/ui";
import { copyText } from "../lib/clipboard";
import type { EthPort } from "../types";
import type { DNSResult, PingResult, SelfTestResult, TCPResult, TracerouteResult } from "../types";

/**
 * Diagnóstico (#500): UN menú para todo lo diagnóstico - autodiagnóstico,
 * cuatro pruebas de red y las tres herramientas físicas de puertos, todas
 * en tarjetas a ancho completo con columnas internas.
 */
export function DiagnosticsPage({ ethports = [] }: { ethports?: EthPort[] }) {
  const { t } = useTranslation();
  const toast = useToast();
  const [selftest, setSelfTest] = useState<SelfTestResult>();
  const [ping, setPing] = useState<PingResult>();
  const [traceroute, setTraceroute] = useState<TracerouteResult>();
  const [dns, setDns] = useState<DNSResult>();
  const [tcp, setTcp] = useState<TCPResult>();

  const hasResults = !!(selftest || ping || traceroute || dns || tcp);

  const summary = useMemo(
    () => buildShareText({ selftest, ping, traceroute, dns, tcp }, t),
    [selftest, ping, traceroute, dns, tcp, t],
  );

  // Si el navegador rechaza las dos vías de copia (HTTP en LAN sin Clipboard
  // API y execCommand bloqueado), se abre el texto ya seleccionado en vez de
  // dejar al usuario con un error que no le dice cómo salir.
  const [manual, setManual] = useState(false);
  const manualRef = useRef<HTMLTextAreaElement>(null);
  const copy = async () => {
    if (await copyText(summary)) {
      toast.push({ tone: "ok", text: t("diagnostics.copied") });
      return;
    }
    setManual(true);
  };

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <div className="flex justify-end">
        <Button variant="secondary" icon={Copy} disabled={!hasResults} onClick={copy}>
          {t("diagnostics.share")}
        </Button>
      </div>

      <SelfTestCard onResult={setSelfTest} />

      {/* #500: las cuatro pruebas en UNA tarjeta a ancho completo, dos
          columnas internas; el chip (?) de la tarjeta las explica. */}
      <Card title={t("diagnostics.testsTitle")} icon={ListChecks} help="diagnostics">
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 md:gap-5">
          <PingCard onResult={setPing} />
          <TracerouteCard onResult={setTraceroute} />
          <DnsCard onResult={setDns} />
          <TcpCard onResult={setTcp} />
          <div className="md:col-span-2">
            <SpeedTestCard />
          </div>
        </div>
      </Card>

      {/* Herramientas físicas (#500: antes página Herramientas, fusionada
          aquí): una tarjeta, tres columnas internas. */}
      <Modal
        open={manual}
        onClose={() => setManual(false)}
        title={t("diagnostics.copyManualTitle")}
        footer={<Button onClick={() => setManual(false)}>{t("common.close")}</Button>}
      >
        <p className="text-small text-muted mb-2">{t("diagnostics.copyManualBody")}</p>
        <textarea
          ref={manualRef}
          readOnly
          value={summary}
          rows={12}
          autoFocus
          onFocus={() => manualRef.current?.select()}
          className="w-full resize-y rounded-md border border-border bg-surface-2 p-2 font-mono text-caption"
        />
      </Modal>

      <Card title={t("tools.physTitle")} icon={Wrench}>
        <div className="grid grid-cols-1 gap-6 md:grid-cols-3 md:gap-5">
          <CableTestTool />
          <LoopsTool />
          <BounceTool ethports={ethports} />
        </div>
      </Card>
    </div>
  );
}
