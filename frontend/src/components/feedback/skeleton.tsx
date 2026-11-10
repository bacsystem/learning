import { cn } from "@/lib/utils";

/** Placeholder de carga con pulso; dale la forma del contenido que reemplaza (`h-4 w-40`, `size-8 rounded-full`…). */
export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden="true" className={cn("bg-secondary animate-pulse rounded-md", className)} />;
}

/** Fila de métricas en carga (mismo layout que `FilaMetricas`). */
export function SkeletonMetricas({ columnas = 4 }: { columnas?: number }) {
  return (
    <section className="border-border bg-card grid grid-cols-2 gap-x-5 gap-y-4 rounded-xl border px-5 py-3 shadow-xs lg:grid-cols-4">
      {Array.from({ length: columnas }).map((_, i) => (
        <div key={i} className="flex flex-col gap-1.5">
          <Skeleton className="h-2.5 w-24" />
          <Skeleton className="h-5 w-16" />
          <Skeleton className="h-2.5 w-32" />
        </div>
      ))}
    </section>
  );
}

/** Tabla en carga: cabecera + N filas de dos líneas. */
export function SkeletonTabla({ filas = 5, columnas = 4 }: { filas?: number; columnas?: number }) {
  return (
    <div className="border-border/90 bg-card overflow-hidden rounded-xl border shadow-xs">
      <div className="border-border/80 bg-muted flex gap-6 border-b px-4 py-2.5">
        {Array.from({ length: columnas }).map((_, i) => (
          <Skeleton key={i} className="h-2.5 w-20" />
        ))}
      </div>
      {Array.from({ length: filas }).map((_, i) => (
        <div key={i} className="border-border/60 flex items-center gap-6 border-b px-4 py-3 last:border-0">
          {Array.from({ length: columnas }).map((_, j) => (
            <div key={j} className="flex flex-1 flex-col gap-1.5">
              <Skeleton className="h-3 w-3/4" />
              <Skeleton className="h-2.5 w-1/2" />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
