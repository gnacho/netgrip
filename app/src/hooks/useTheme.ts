import { useCallback, useEffect, useState } from "react";

export type ThemeChoice = "light" | "dark" | "auto";
const KEY = "netgrip:theme";

function stored(): ThemeChoice {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark" || v === "auto") return v;
  } catch { /* sin persistencia */ }
  return "auto";
}

function resolvedDark(choice: ThemeChoice): boolean {
  if (choice === "auto") return window.matchMedia("(prefers-color-scheme: dark)").matches;
  return choice === "dark";
}

function applyChoice(choice: ThemeChoice) {
  document.documentElement.classList.toggle("dark", resolvedDark(choice));
}

export function useTheme() {
  const [choice, setChoice] = useState<ThemeChoice>(stored);

  const apply = useCallback((next: ThemeChoice) => {
    applyChoice(next);
    try { localStorage.setItem(KEY, next); } catch { /* sin persistencia */ }
    setChoice(next);
  }, []);

  useEffect(() => {
    if (choice !== "auto") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => applyChoice("auto");
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [choice]);

  // Toggle binario (#158): alterna claro/oscuro. Desde "auto" salta al
  // opuesto del tema efectivo, para que un clic siempre cambie algo visible.
  const toggle = useCallback(() => {
    apply(resolvedDark(choice) ? "light" : "dark");
  }, [choice, apply]);

  return { theme: choice, isDark: resolvedDark(choice), setTheme: apply, toggle };
}
