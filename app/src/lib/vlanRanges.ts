/**
 * Utilidades de rangos para la matriz de VLANs (#485). Las bocas DSA
 * llegan como "lan1".."lan52" y en la UI se resumen por número,
 * comprimiendo consecutivos: 1,2,3,5-8 -> "1-3, 5-8".
 */

/** "lan20" -> 20. Devuelve NaN si el nombre no termina en dígitos. */
export function portNum(port: string): number {
  const m = /(\d+)$/.exec(port);
  return m ? parseInt(m[1], 10) : NaN;
}

/** [1,2,3,5,6,7,8] -> "1-3, 5-8". Vacío -> "-". */
export function compressRanges(nums: number[]): string {
  const sorted = [...new Set(nums)].filter((n) => Number.isFinite(n)).sort((a, b) => a - b);
  if (!sorted.length) return "-";
  const parts: string[] = [];
  let start = sorted[0];
  let prev = sorted[0];
  for (let i = 1; i < sorted.length; i++) {
    const n = sorted[i];
    if (n === prev + 1) {
      prev = n;
      continue;
    }
    parts.push(start === prev ? `${start}` : `${start}-${prev}`);
    start = prev = n;
  }
  parts.push(start === prev ? `${start}` : `${start}-${prev}`);
  return parts.join(", ");
}
