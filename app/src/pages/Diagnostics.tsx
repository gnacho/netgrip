import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy } from "lucide-react";
import { DnsCard, PingCard, SelfTestCard, TcpCard, TracerouteCard } from "../components/diagnostics/cards";
import { buildShareText } from "../components/diagnostics/share";
import { Button, useToast } from "../components/ui";
import type { DNSResult, PingResult, SelfTestResult, TCPResult, TracerouteResult } from "../types";

/**
 * Diagnóstico (issue #305): autodiagnóstico + cuatro pruebas guiadas, con un
 * botón para copiar todos los resultados como texto plano.
 */
export function DiagnosticsPage() {
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

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(summary);
      toast.push({ tone: "ok", text: t("diagnostics.copied") });
    } catch {
      toast.push({ tone: "danger", text: t("diagnostics.copyFailed") });
    }
  };

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <div className="flex items-center justify-between gap-3">
        <p className="text-small text-muted">{t("diagnostics.intro")}</p>
        <Button variant="secondary" icon={Copy} disabled={!hasResults} onClick={copy}>
          {t("diagnostics.share")}
        </Button>
      </div>

      <SelfTestCard onResult={setSelfTest} />

      <div className="grid grid-cols-1 gap-[var(--card-gap)] md:grid-cols-2">
        <PingCard onResult={setPing} />
        <TracerouteCard onResult={setTraceroute} />
        <DnsCard onResult={setDns} />
        <TcpCard onResult={setTcp} />
      </div>
    </div>
  );
}
