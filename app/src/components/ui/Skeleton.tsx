/** Skeleton §6.15: bloques shimmer que imitan el contenido final. */
export function Skeleton({ className = "", style }: {
  className?: string;
  style?: React.CSSProperties;
}) {
  return <div aria-hidden="true" style={style} className={`skeleton ${className}`} />;
}

/** Cabecera + N filas, forma de card estándar. */
export function SkeletonRows({ rows = 3 }: { rows?: number }) {
  return (
    <div aria-hidden="true">
      <div className="flex items-center gap-2.5 mb-3">
        <Skeleton className="h-9 w-9 rounded-md" />
        <Skeleton className="h-4 w-28" />
      </div>
      <div className="space-y-2.5">
        {Array.from({ length: rows }).map((_, i) => (
          <Skeleton key={i} className="h-3.5" />
        ))}
      </div>
    </div>
  );
}

/** Rectángulo alto para charts. */
export function SkeletonChart({ height = 160 }: { height?: number }) {
  return (
    <div aria-hidden="true">
      <div className="flex items-center gap-2.5 mb-3">
        <Skeleton className="h-9 w-9 rounded-md" />
        <Skeleton className="h-4 w-28" />
      </div>
      <Skeleton className="w-full" style={{ height }} />
    </div>
  );
}
