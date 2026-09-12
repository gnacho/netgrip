import type { TFunction } from "i18next";
import type { DNSResult, PingResult, SelfTestResult, TCPResult, TracerouteResult } from "../../types";

export interface DiagnosticsResults {
  selftest?: SelfTestResult;
  ping?: PingResult;
  traceroute?: TracerouteResult;
  dns?: DNSResult;
  tcp?: TCPResult;
}

const SEP = "-".repeat(36);

const okText = (t: TFunction, v: boolean) => (v ? t("diagnostics.ok") : t("diagnostics.fail"));

/**
 * Resumen en texto plano de todos los resultados, para copiar en una
 * conversación de soporte. Puro respecto a sus entradas (results + t); sin
 * em dashes, solo guiones normales.
 */
export function buildShareText(r: DiagnosticsResults, t: TFunction): string {
  const out: string[] = [];
  out.push(`${t("diagnostics.shareTitle")} - NetGrip`);
  out.push("=".repeat(36));

  if (r.selftest) {
    out.push("", t("diagnostics.selfTestTitle"), SEP);
    out.push(`${t("diagnostics.gateway")}: ${okText(t, r.selftest.gateway)}`);
    out.push(`${t("diagnostics.wan")}: ${okText(t, r.selftest.wan)}`);
    out.push(`${t("diagnostics.dns")}: ${okText(t, r.selftest.dns)}`);
    out.push(`${t("diagnostics.ntp")}: ${okText(t, r.selftest.ntp)}`);
    out.push(`${t("diagnostics.summary")}: ${r.selftest.all_ok ? t("diagnostics.allGood") : t("diagnostics.someFail")}`);
  }

  if (r.ping) {
    out.push("", `${t("diagnostics.pingTitle")} ${r.ping.host} (${r.ping.count})`, SEP);
    if (r.ping.missing_tool) {
      out.push(t("diagnostics.toolMissing", { tool: r.ping.missing_tool }));
    } else {
      out.push(`${t("diagnostics.sentReceived")}: ${r.ping.sent}/${r.ping.received}`);
      out.push(`${t("diagnostics.loss")}: ${r.ping.loss_pct}%`);
      out.push(`${t("diagnostics.minMaxAvg")}: ${r.ping.min_ms}/${r.ping.avg_ms}/${r.ping.max_ms} ms`);
    }
  }

  if (r.traceroute) {
    out.push("", `${t("diagnostics.tracerouteTitle")} ${r.traceroute.host}`, SEP);
    if (r.traceroute.missing_tool) {
      out.push(t("diagnostics.toolMissing", { tool: r.traceroute.missing_tool }));
    } else {
      for (const h of r.traceroute.hops) {
        const rtts = h.parsed ? h.rtts.map((x) => `${x} ms`).join(" ") : t("diagnostics.unanswered");
        out.push(`${h.hop}  ${h.host}  ${rtts}`);
      }
    }
  }

  if (r.dns) {
    out.push("", `${t("diagnostics.dnsTitle")} ${r.dns.query}`, SEP);
    if (r.dns.missing_tool) {
      out.push(t("diagnostics.toolMissing", { tool: r.dns.missing_tool }));
    } else if (r.dns.answers.length === 0) {
      out.push(t("diagnostics.noAnswers"));
    } else {
      for (const a of r.dns.answers) {
        const type = a.type ? `${a.type} ` : "";
        const ttl = a.ttl ? ` (ttl ${a.ttl})` : "";
        out.push(`${a.name}: ${type}${a.value}${ttl}`);
      }
    }
  }

  if (r.tcp) {
    out.push("", `${t("diagnostics.tcpTitle")} ${r.tcp.host}:${r.tcp.port}`, SEP);
    out.push(r.tcp.open ? t("diagnostics.open") : t("diagnostics.closed"));
    if (!r.tcp.open && r.tcp.error) out.push(r.tcp.error);
  }

  return out.join("\n");
}
