"use client";

import { useTranslations } from "next-intl";
import { useId, useState } from "react";

import { cn } from "@/lib/utils";

/** Serie de los gráficos propios del panel: nombre, valores por categoría y color (`var(--chart-N)`). */
export type SerieExtra = { nombre: string; valores: number[]; color: string };

/** Leyenda en píldoras (misma anatomía que la de `datos/grafico`). */
export function LeyendaExtra({
  series,
  className,
}: {
  series: { nombre: string; color: string }[];
  className?: string;
}) {
  return (
    <div className={cn("flex flex-wrap items-center gap-1.5", className)}>
      {series.map((s) => (
        <span
          key={s.nombre}
          className="bg-secondary text-etiqueta text-foreground inline-flex items-center gap-1.5 rounded-full py-0.5 pr-2.5 pl-2 font-medium"
        >
          <span aria-hidden className="size-2.5 rounded-full" style={{ background: s.color }} />
          {s.nombre}
        </span>
      ))}
    </div>
  );
}

const recortar = (texto: string, max = 14) => (texto.length > max ? `${texto.slice(0, max - 1)}…` : texto);

/**
 * Radar de varias series sobre las mismas categorías (≥ 3 ejes para que tenga área; con 2 es una línea).
 * Rejilla recesiva en `--border`, rótulos en texto terciario, áreas tenues con trazo del color de la serie y
 * detalle de la categoría bajo el gráfico al pasar el cursor o enfocar un vértice: ningún dato va solo en el color.
 */
export function GraficoRadar({
  categorias,
  series,
  max,
  etiquetadoPor,
  tamano = 300,
}: {
  categorias: string[];
  series: SerieExtra[];
  max: number;
  etiquetadoPor?: string;
  tamano?: number;
}) {
  const t = useTranslations("dashboard");
  const id = useId();
  const [activo, setActivo] = useState<number | null>(null);
  const n = categorias.length;
  const c = tamano / 2;
  const radio = c - 46;
  const tope = Math.max(1, max);
  const angulo = (i: number) => -Math.PI / 2 + (i * 2 * Math.PI) / n;
  const punto = (i: number, v: number) => {
    const r = (Math.max(0, v) / tope) * radio;
    return [c + r * Math.cos(angulo(i)), c + r * Math.sin(angulo(i))] as const;
  };
  const anillos = [0.25, 0.5, 0.75, 1];

  return (
    <div className="flex flex-col items-center gap-3">
      <svg
        viewBox={`0 0 ${tamano} ${tamano}`}
        className="block h-auto w-full max-w-[320px] overflow-visible"
        role="img"
        aria-labelledby={etiquetadoPor ?? `${id}-t`}
        aria-describedby={`${id}-d`}
        style={{ fontSize: 10 }}
        onMouseLeave={() => setActivo(null)}
      >
        {etiquetadoPor ? null : <title id={`${id}-t`}>{t("dashboard-radar-grafico")}</title>}
        <desc id={`${id}-d`}>
          {categorias
            .map((cat, i) => `${cat}: ${series.map((s) => `${s.nombre} ${s.valores[i] ?? 0}`).join(", ")}`)
            .join("; ")}
        </desc>
        {anillos.map((f) => (
          <polygon
            key={f}
            points={categorias.map((_, i) => punto(i, f * tope).join(",")).join(" ")}
            fill="none"
            stroke="var(--border)"
          />
        ))}
        {categorias.map((cat, i) => {
          const [x, y] = punto(i, tope);
          const [lx, ly] = punto(i, tope * 1.14);
          const ancla = Math.abs(lx - c) < 4 ? "middle" : lx > c ? "start" : "end";
          return (
            <g key={cat}>
              <line x1={c} y1={c} x2={x} y2={y} stroke={activo === i ? "var(--muted-foreground)" : "var(--border)"} />
              <text
                x={lx}
                y={ly + 3}
                textAnchor={ancla}
                fill={activo === i ? "var(--foreground)" : "var(--texto-terciario)"}
                className="font-medium"
              >
                <title>{cat}</title>
                {recortar(cat)}
              </text>
            </g>
          );
        })}
        {series.map((s) => (
          <polygon
            key={s.nombre}
            points={categorias.map((_, i) => punto(i, s.valores[i] ?? 0).join(",")).join(" ")}
            fill={s.color}
            fillOpacity={0.16}
            stroke={s.color}
            strokeWidth={2}
            strokeLinejoin="round"
          />
        ))}
        {series.map((s) =>
          categorias.map((cat, i) => {
            const [x, y] = punto(i, s.valores[i] ?? 0);
            return (
              <circle
                key={`${s.nombre}-${cat}`}
                cx={x}
                cy={y}
                r={activo === i ? 4 : 2.5}
                fill={s.color}
                stroke="var(--card)"
                strokeWidth={1.5}
              />
            );
          }),
        )}
        {categorias.map((cat, i) => {
          const [x, y] = punto(i, tope);
          return (
            <line
              key={`zona-${cat}`}
              x1={c}
              y1={c}
              x2={x}
              y2={y}
              stroke="transparent"
              strokeWidth={22}
              tabIndex={0}
              aria-label={`${cat}: ${series.map((s) => `${s.nombre} ${s.valores[i] ?? 0}`).join(", ")}`}
              className="outline-none"
              onMouseEnter={() => setActivo(i)}
              onFocus={() => setActivo(i)}
              onBlur={() => setActivo(null)}
            />
          );
        })}
      </svg>
      <div
        className="text-etiqueta flex min-h-7 w-full flex-wrap items-center justify-center gap-x-3 gap-y-1"
        aria-live="polite"
      >
        {activo === null ? (
          <LeyendaExtra series={series} className="justify-center" />
        ) : (
          <>
            <span className="text-foreground max-w-full truncate font-semibold">{categorias[activo]}</span>
            {series.map((s) => (
              <span key={s.nombre} className="text-muted-foreground inline-flex items-center gap-1.5">
                <span aria-hidden className="size-2 rounded-full" style={{ background: s.color }} />
                {s.nombre}
                <span className="text-foreground font-semibold tabular-nums">{s.valores[activo] ?? 0}</span>
              </span>
            ))}
          </>
        )}
      </div>
    </div>
  );
}

/**
 * Barras horizontales apiladas: una fila por categoría con el rótulo a la izquierda, los segmentos de cada serie sobre una
 * pista `secondary` escalada a `max` y los valores en texto a la derecha.
 */
export function GraficoBarrasApiladas({
  categorias,
  series,
  max,
  etiquetadoPor,
}: {
  categorias: string[];
  series: SerieExtra[];
  max: number;
  etiquetadoPor?: string;
}) {
  const tope = Math.max(1, max);
  const marcas = [0, 0.25, 0.5, 0.75, 1];
  return (
    <div role="group" aria-labelledby={etiquetadoPor} className="flex flex-col gap-3">
      <LeyendaExtra series={series} />
      <ul className="flex flex-col gap-2.5">
        {categorias.map((cat, i) => {
          const resumen = series.map((s) => `${s.nombre}: ${s.valores[i] ?? 0}`).join(" · ");
          return (
            <li
              key={cat}
              className="grid grid-cols-[minmax(0,9rem)_1fr_auto] items-center gap-3 sm:grid-cols-[minmax(0,12rem)_1fr_auto]"
              title={`${cat} — ${resumen}`}
            >
              <span className="text-control text-foreground truncate">{cat}</span>
              <span className="bg-secondary relative flex h-3 overflow-hidden rounded-full" aria-hidden>
                {series.map((s) => {
                  const v = Math.max(0, s.valores[i] ?? 0);
                  return v > 0 ? (
                    <span
                      key={s.nombre}
                      className="h-full first:rounded-l-full last:rounded-r-full"
                      style={{ width: `${(v / tope) * 100}%`, background: s.color }}
                    />
                  ) : null;
                })}
              </span>
              <span className="text-etiqueta text-muted-foreground flex items-center gap-1 tabular-nums">
                <span className="sr-only">{resumen}</span>
                {series.map((s, j) => (
                  <span key={s.nombre} aria-hidden className="inline-flex items-center gap-1">
                    {j > 0 ? <span className="text-texto-terciario ml-1">·</span> : null}
                    <span className="size-1.5 rounded-full" style={{ background: s.color }} />
                    <span className="text-foreground font-semibold">{s.valores[i] ?? 0}</span>
                  </span>
                ))}
              </span>
            </li>
          );
        })}
      </ul>
      <div
        aria-hidden
        className="grid grid-cols-[minmax(0,9rem)_1fr_auto] gap-3 sm:grid-cols-[minmax(0,12rem)_1fr_auto]"
      >
        <span />
        <div className="text-etiqueta text-texto-terciario relative h-4 tabular-nums">
          {marcas.map((f) => (
            <span
              key={f}
              className="absolute -translate-x-1/2 first:translate-x-0 last:-translate-x-full"
              style={{ left: `${f * 100}%` }}
            >
              {Math.round(f * tope)}
            </span>
          ))}
        </div>
        <span className="text-etiqueta invisible tabular-nums">· 00 · 00</span>
      </div>
    </div>
  );
}
