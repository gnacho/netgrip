import { useState } from "react";
import type { ActionPhase } from "../ui";

/**
 * Ciclo applying → checking → done/failed para acciones con rollback
 * (ModuleResult). Devuelve la respuesta para que el caller actualice estado.
 */
export function useActionCycle() {
  const [phase, setPhase] = useState<ActionPhase>();
  const [detail, setDetail] = useState<string>();

  const run = async <T extends { status: string; error?: string }>(
    fn: () => Promise<T>,
    verifyMs = 700,
  ): Promise<T | undefined> => {
    setPhase("applying");
    setDetail(undefined);
    try {
      const res = await fn();
      setPhase("checking");
      await new Promise((r) => setTimeout(r, verifyMs));
      if (res.status === "applied") {
        setPhase("done");
      } else {
        setPhase("failed");
        setDetail(res.error);
      }
      return res;
    } catch (e) {
      setPhase("failed");
      setDetail(e instanceof Error ? e.message : String(e));
      return undefined;
    }
  };

  return {
    phase,
    detail,
    busy: phase === "applying" || phase === "checking",
    run,
    clear: () => setPhase(undefined),
  };
}
