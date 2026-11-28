import { useEffect, useState } from "react";

/**
 * Tracks a CSS media query.
 *
 * Needed where the same data has two DOM shapes (a table and a card list) and
 * rendering both would double the work: `hidden` is a paint instruction, so
 * React still builds and diffs the copy nobody can see. On the Clients page
 * that copy was more than half of the nodes on a phone (#815).
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches);

  useEffect(() => {
    const mq = window.matchMedia(query);
    const onChange = () => setMatches(mq.matches);
    setMatches(mq.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}
