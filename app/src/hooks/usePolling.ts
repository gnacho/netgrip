import { useEffect } from "react";

/**
 * Runs `fn` now and then every `ms`, but only while the document is visible.
 *
 * A panel left open on a phone kept polling and re-rendering the device lists
 * in the background, and kept asking the router for data nobody was looking
 * at. On visibility it refreshes once and carries on, so the list is current
 * again the moment it comes back (#815).
 */
export function usePolling(fn: () => void, ms: number) {
  useEffect(() => {
    let id: ReturnType<typeof setInterval> | undefined;
    const start = () => {
      fn();
      id = setInterval(fn, ms);
    };
    const stop = () => {
      if (id) clearInterval(id);
      id = undefined;
    };
    const onVisibility = () => (document.hidden ? stop() : start());

    start();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [fn, ms]);
}
