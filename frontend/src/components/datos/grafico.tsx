"use client";

import { m } from "motion/react";
import { type ReactNode, type RefObject, useEffect, useId, useRef, useState } from "react";

import { TrazoAnimado } from "@/components/datos/trazo-animado";
import { MovimientoLocal } from "@/components/movimiento";
import { TARJETA } from "@/lib/estilos";
import { RESORTE_SUAVE } from "@/lib/movimiento";
import { cn } from "@/lib/utils";

export type Serie = { nombre: string; valores: number[] };
// Barras: relleno pastel de la paleta (como en el boceto) con un borde de 1 px que sí llega a 3:1 sobre la tarjeta
// (WCAG 1.4.11; el pastel solo queda en 1.3–1.6). Líneas: el color de borde de cada serie, que cumple el mismo 3:1
// (el verde y el ámbar semánticos de chart-2/chart-3 quedaban en 2–2.5 sobre blanco). Lo mide `npm run check:contraste`.
const COLORES_BARRAS = [
  "var(--chart-base)",
  "var(--chart-destacado)",
  "var(--acento-3)",
  "var(--chart-4)",
  "var(--chart-5)",
];
const TRAZOS = [
  "var(--chart-trazo-base)",
  "var(--chart-trazo-destacado)",
  "var(--chart-trazo-3)",
  "var(--chart-trazo-4)",
  "var(--chart-trazo-5)",
];
const COLORES = ["var(--chart-1)", ...TRAZOS.slice(1)];

function escala(max: number, pasos = 4): number[] {
  if (max <= 0) return [0, 1];
  const bruto = max / pasos;
  const mag = 10 ** Math.floor(Math.log10(bruto));
  const paso = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((p) => p >= bruto) ?? bruto;
  const tope = Math.ceil(max / paso) * paso;
  return Array.from({ length: Math.round(tope / paso) + 1 }, (_, i) => i * paso);
}

/** Leyenda en píldoras, a la derecha del título. Solo con ≥ 2 series. */
function Leyenda({
  series,
  colores = COLORES,
  trazos,
}: {
  series: { nombre: string }[];
  colores?: string[];
  trazos?: string[];
}) {
  if (series.length < 2) return null;
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {series.map((s, i) => (
        <span
          key={s.nombre}
          className="bg-secondary text-etiqueta text-foreground inline-flex items-center gap-1.5 rounded-full py-0.5 pr-2.5 pl-2 font-medium"
        >
          <span
            className="size-2.5 rounded-full"
            style={{
              background: colores[i % colores.length],
              boxShadow: trazos ? `inset 0 0 0 1px ${trazos[i % trazos.length]}` : undefined,
            }}
          />
          {s.nombre}
        </span>
      ))}
    </div>
  );
}

/** Título del gráfico con la leyenda a la derecha. */
function Cabecera({ titulo, leyenda }: { titulo?: string; leyenda: ReactNode }) {
  if (!titulo && !leyenda) return null;
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 px-1 pb-2">
      {titulo ? <div className="text-titulo text-foreground">{titulo}</div> : <span />}
      {leyenda}
    </div>
  );
}

/**
 * Ancho real en px del contenedor del SVG. El viewBox usa ese ancho para que el texto de los ejes mida lo que dice
 * (10 px) a cualquier tamaño: con un viewBox fijo el gráfico se estiraba y las etiquetas crecían con él.
 * Arranca en 640 (servidor y primer render) y se ajusta al medir.
 */
function useAnchoReal(): [RefObject<HTMLDivElement | null>, number] {
  const caja = useRef<HTMLDivElement>(null);
  const [ancho, setAncho] = useState(640);
  useEffect(() => {
    const el = caja.current;
    if (!el) return;
    const medir = () => setAncho(Math.max(240, Math.round(el.clientWidth)));
    medir();
    const observador = new ResizeObserver(medir);
    observador.observe(el);
    return () => observador.disconnect();
  }, []);
  return [caja, ancho];
}

/** Tooltip anclado en horizontal al índice (porcentaje del ancho) y en vertical dentro de la tarjeta, bajo el título. */
function Tooltip({
  x,
  titulo,
  filas,
}: {
  x: number;
  titulo: string;
  filas: { nombre: string; valor: string; color: string }[];
}) {
  return (
    <div
      className="bg-foreground text-etiqueta text-background pointer-events-none absolute top-10 z-10 min-w-28 -translate-x-1/2 rounded-lg px-2.5 py-1.5 shadow-md"
      style={{ left: `clamp(4rem, ${x}%, calc(100% - 4rem))` }}
    >
      <div className="mb-0.5 font-medium">{titulo}</div>
      {filas.map((f) => (
        <div key={f.nombre} className="flex items-center justify-between gap-3">
          <span className="text-background/70 inline-flex items-center gap-1.5">
            <span className="size-2 rounded-sm" style={{ background: f.color }} />
            {f.nombre}
          </span>
          <span className="font-semibold tabular-nums">{f.valor}</span>
        </div>
      ))}
    </div>
  );
}

/**
 * Barras verticales (una o varias series agrupadas): marcas finas con extremo redondeado sobre la línea base, hueco de 2px,
 * rejilla recesiva, etiquetas de eje con la escala real y tooltip por marca. Leyenda solo con ≥ 2 series. Máximo 5 series.
 * Un valor 0 no dibuja marca (solo la franja que responde al cursor). `plano` lo deja sin tarjeta propia, para ir dentro de
 * una sección que ya la tiene; entonces el nombre accesible lo da el título de esa sección (`etiquetadoPor`).
 */
export function GraficoBarras({
  categorias,
  series,
  formato = (v) => v.toLocaleString("en-US"),
  alto = 200,
  titulo,
  destacado,
  plano = false,
  etiquetadoPor,
  etiqueta,
  descripcion,
  rotuloVisible,
  tituloDeCategoria,
  propiedadesDeCategoria,
  className,
}: {
  categorias: string[];
  series: Serie[];
  formato?: (v: number) => string;
  alto?: number;
  titulo?: string;
  /** Índice de la categoría a resaltar con el acento de la paleta (solo con una serie). */
  destacado?: number;
  /** Sin tarjeta ni relleno propios: para un gráfico dentro de otra tarjeta. */
  plano?: boolean;
  /** `id` del elemento que nombra al gráfico (el título de la sección que lo contiene). */
  etiquetadoPor?: string;
  /** Nombre accesible cuando no hay `titulo` ni `etiquetadoPor` (traducido por quien lo usa). */
  etiqueta?: string;
  /** Resumen para lectores de pantalla («120 comprobantes en 30 días; el pico, el 12»): va en `<desc>`. */
  descripcion?: string;
  /** Qué categorías llevan rótulo en el eje; por defecto, todas. Con muchas (los días de un mes) no caben en un teléfono. */
  rotuloVisible?: (indice: number) => boolean;
  /** El texto de cada categoría en su `<title>` y en el tooltip («5 oct: 3 comprobantes · S/ 1,180.00»); por defecto, la categoría. */
  tituloDeCategoria?: (indice: number) => string;
  /** Atributos extra del grupo de cada categoría (`data-*`, para pruebas o analítica). */
  propiedadesDeCategoria?: (indice: number) => Record<`data-${string}`, string | number>;
  className?: string;
}) {
  const id = useId();
  const [hover, setHover] = useState<number | null>(null);
  const [caja, ancho] = useAnchoReal();
  const mg = { top: 8, right: 8, bottom: 24, left: 44 };
  const w = ancho - mg.left - mg.right;
  const h = alto - mg.top - mg.bottom;
  const ticks = escala(Math.max(...series.flatMap((s) => s.valores), 0));
  const max = ticks[ticks.length - 1];
  const grupo = w / categorias.length;
  const barra = Math.max(4, (grupo * 0.62 - 4 * (series.length - 1)) / series.length);
  const y = (v: number) => mg.top + h - (v / max) * h;
  const nombreDe = (i: number) => tituloDeCategoria?.(i) ?? categorias[i];
  const nombrado = etiquetadoPor ?? `${id}-t`;

  return (
    <div className={cn(!plano && cn(TARJETA, "px-4 py-3.5"), "relative", className)}>
      <Cabecera titulo={titulo} leyenda={<Leyenda series={series} colores={COLORES_BARRAS} trazos={TRAZOS} />} />
      <div ref={caja} className="w-full">
        <MovimientoLocal>
          <svg
            viewBox={`0 0 ${ancho} ${alto}`}
            width={ancho}
            height={alto}
            role="img"
            aria-labelledby={nombrado}
            aria-describedby={descripcion ? `${id}-d` : undefined}
            className="block max-w-full overflow-visible"
            style={{ fontSize: 10 }}
          >
            {etiquetadoPor ? null : <title id={`${id}-t`}>{titulo ?? etiqueta ?? "Gráfico de barras"}</title>}
            {descripcion ? <desc id={`${id}-d`}>{descripcion}</desc> : null}
            {ticks.map((t) => (
              <g key={t}>
                <line x1={mg.left} x2={ancho - mg.right} y1={y(t)} y2={y(t)} stroke="var(--border)" />
                <text
                  x={mg.left - 6}
                  y={y(t) + 3}
                  textAnchor="end"
                  fill="var(--texto-terciario)"
                  className="tabular-nums"
                >
                  {formato(t)}
                </text>
              </g>
            ))}
            {categorias.map((c, i) => {
              const x0 = mg.left + i * grupo + (grupo - (barra * series.length + 4 * (series.length - 1))) / 2;
              return (
                <g
                  key={c}
                  onMouseEnter={() => setHover(i)}
                  onMouseLeave={() => setHover(null)}
                  {...propiedadesDeCategoria?.(i)}
                >
                  {tituloDeCategoria ? <title>{nombreDe(i)}</title> : null}
                  <rect
                    x={mg.left + i * grupo}
                    y={mg.top}
                    width={grupo}
                    height={h}
                    fill={hover === i ? "var(--muted)" : "transparent"}
                  />
                  {series.map((s, j) => {
                    const v = s.valores[i] ?? 0;
                    // Sin valor no hay marca: el extremo redondeado dejaba una joroba sobre la línea base.
                    if (v <= 0) return null;
                    const bx = x0 + j * (barra + 4);
                    const by = y(v);
                    const bh = Math.max(0, mg.top + h - by);
                    // El radio no pasa del alto de la barra: una barra baja no asoma por encima de su valor.
                    const r = Math.min(9, barra / 2, bh);
                    // Crece desde la línea base; cada categoría entra 30 ms después de la anterior. scaleY es una transformación:
                    // con «reducir movimiento» MotionConfig la deja en su estado final.
                    return (
                      <m.path
                        key={s.nombre}
                        d={`M${bx},${mg.top + h} v${-Math.max(0, bh - r)} a${r},${r} 0 0 1 ${r},${-r} h${barra - 2 * r} a${r},${r} 0 0 1 ${r},${r} v${Math.max(0, bh - r)} z`}
                        fill={
                          series.length === 1
                            ? i === destacado
                              ? "var(--chart-destacado)"
                              : "var(--chart-base)"
                            : COLORES_BARRAS[j % COLORES_BARRAS.length]
                        }
                        stroke={
                          series.length === 1
                            ? i === destacado
                              ? "var(--chart-trazo-destacado)"
                              : "var(--chart-trazo-base)"
                            : TRAZOS[j % TRAZOS.length]
                        }
                        strokeWidth={1}
                        style={{ originY: 1 }}
                        initial={{ scaleY: 0 }}
                        animate={{ scaleY: 1 }}
                        transition={{ ...RESORTE_SUAVE, delay: i * 0.03 }}
                      >
                        {tituloDeCategoria ? null : <title>{`${c}: ${formato(v)}`}</title>}
                      </m.path>
                    );
                  })}
                  {rotuloVisible && !rotuloVisible(i) ? null : (
                    <text
                      x={mg.left + i * grupo + grupo / 2}
                      y={alto - 8}
                      textAnchor="middle"
                      fill="var(--texto-terciario)"
                    >
                      {c}
                    </text>
                  )}
                </g>
              );
            })}
          </svg>
        </MovimientoLocal>
      </div>
      {hover != null ? (
        <Tooltip
          x={((mg.left + hover * grupo + grupo / 2) / ancho) * 100}
          titulo={nombreDe(hover)}
          filas={series.map((s, j) => ({
            nombre: s.nombre,
            valor: formato(s.valores[hover] ?? 0),
            color:
              series.length === 1
                ? hover === destacado
                  ? "var(--chart-destacado)"
                  : "var(--chart-base)"
                : COLORES_BARRAS[j % COLORES_BARRAS.length],
          }))}
        />
      ) : null}
    </div>
  );
}

/** Líneas (una o varias series): trazo de 2px, marcador ≥ 8px al pasar el cursor, crosshair y tooltip con todas las series. */
export function GraficoLineas({
  categorias,
  series,
  formato = (v) => v.toLocaleString("en-US"),
  alto = 200,
  titulo,
  className,
}: {
  categorias: string[];
  series: Serie[];
  formato?: (v: number) => string;
  alto?: number;
  titulo?: string;
  className?: string;
}) {
  const id = useId();
  const [hover, setHover] = useState<number | null>(null);
  const [caja, ancho] = useAnchoReal();
  const mg = { top: 8, right: 12, bottom: 24, left: 44 };
  const w = ancho - mg.left - mg.right;
  const h = alto - mg.top - mg.bottom;
  const ticks = escala(Math.max(...series.flatMap((s) => s.valores), 0));
  const max = ticks[ticks.length - 1];
  const x = (i: number) => mg.left + (categorias.length === 1 ? w / 2 : (i / (categorias.length - 1)) * w);
  const y = (v: number) => mg.top + h - (v / max) * h;

  return (
    <div className={cn(TARJETA, "relative px-4 py-3.5", className)}>
      <Cabecera titulo={titulo} leyenda={<Leyenda series={series} />} />
      <div ref={caja} className="w-full">
        <MovimientoLocal>
          <svg
            viewBox={`0 0 ${ancho} ${alto}`}
            width={ancho}
            height={alto}
            role="img"
            aria-labelledby={`${id}-t`}
            className="block max-w-full overflow-visible"
            style={{ fontSize: 10 }}
            onMouseLeave={() => setHover(null)}
          >
            <title id={`${id}-t`}>{titulo ?? "Gráfico de líneas"}</title>
            {ticks.map((t) => (
              <g key={t}>
                <line x1={mg.left} x2={ancho - mg.right} y1={y(t)} y2={y(t)} stroke="var(--border)" />
                <text
                  x={mg.left - 6}
                  y={y(t) + 3}
                  textAnchor="end"
                  fill="var(--texto-terciario)"
                  className="tabular-nums"
                >
                  {formato(t)}
                </text>
              </g>
            ))}
            {hover != null ? (
              <line
                x1={x(hover)}
                x2={x(hover)}
                y1={mg.top}
                y2={mg.top + h}
                stroke="var(--muted-foreground)"
                strokeOpacity="0.4"
              />
            ) : null}
            {series.map((s, j) => (
              <TrazoAnimado
                key={s.nombre}
                d={s.valores.map((v, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ")}
                stroke={COLORES[j % COLORES.length]}
                retraso={j * 0.08}
              />
            ))}
            {hover != null
              ? series.map((s, j) => (
                  <circle
                    key={s.nombre}
                    cx={x(hover)}
                    cy={y(s.valores[hover] ?? 0)}
                    r="4"
                    fill={COLORES[j % COLORES.length]}
                    stroke="var(--card)"
                    strokeWidth="2"
                  />
                ))
              : null}
            {categorias.map((c, i) => (
              <g key={c}>
                <rect
                  x={x(i) - w / categorias.length / 2}
                  y={mg.top}
                  width={w / categorias.length}
                  height={h}
                  fill="transparent"
                  onMouseEnter={() => setHover(i)}
                />
                <text x={x(i)} y={alto - 8} textAnchor="middle" fill="var(--texto-terciario)">
                  {c}
                </text>
              </g>
            ))}
          </svg>
        </MovimientoLocal>
      </div>
      {hover != null ? (
        <Tooltip
          x={(x(hover) / ancho) * 100}
          titulo={categorias[hover]}
          filas={series.map((s, j) => ({
            nombre: s.nombre,
            valor: formato(s.valores[hover] ?? 0),
            color: COLORES[j % COLORES.length],
          }))}
        />
      ) : null}
    </div>
  );
}

export type SerieAnillo = { nombre: string; valor: number; detalle?: ReactNode };
const COLORES_ANILLOS = ["var(--acento-2)", "var(--acento)", "var(--acento-3)", "var(--chart-4)", "var(--chart-5)"];

/**
 * Anillos concéntricos (hasta 5): cada serie es un porcentaje 0–100 en un anillo, de afuera hacia adentro, con los
 * pasteles de la paleta y su leyenda al lado. `centro` va en el medio (p. ej. el total). Los valores van también en texto
 * (leyenda y `<title>`), así que el pastel no carga solo la información.
 */
export function GraficoAnillos({
  series,
  centro,
  titulo,
  tamano = 132,
  className,
}: {
  series: SerieAnillo[];
  centro?: ReactNode;
  titulo?: string;
  tamano?: number;
  className?: string;
}) {
  const id = useId();
  const grosor = Math.max(8, Math.round(tamano / 11));
  const hueco = 3;
  const anillos = series.slice(0, 5);
  return (
    <div className={cn(TARJETA, "grid gap-3 px-4 py-3.5", className)}>
      {titulo ? <div className="text-titulo text-foreground px-1">{titulo}</div> : null}
      <div className="flex items-center gap-4">
        <div className="relative shrink-0" style={{ width: tamano, height: tamano }}>
          <MovimientoLocal>
            <svg
              width={tamano}
              height={tamano}
              viewBox={`0 0 ${tamano} ${tamano}`}
              role="img"
              aria-labelledby={`${id}-t`}
              className="-rotate-90"
            >
              <title id={`${id}-t`}>{anillos.map((a) => `${a.nombre}: ${a.valor}%`).join(", ")}</title>
              {anillos.map((a, i) => {
                const radio = tamano / 2 - grosor / 2 - i * (grosor + hueco);
                const v = Math.max(0, Math.min(100, a.valor)) / 100;
                return (
                  <g key={a.nombre}>
                    <circle
                      cx={tamano / 2}
                      cy={tamano / 2}
                      r={radio}
                      fill="none"
                      stroke="var(--secondary)"
                      strokeWidth={grosor}
                    />
                    <m.circle
                      cx={tamano / 2}
                      cy={tamano / 2}
                      r={radio}
                      fill="none"
                      stroke={COLORES_ANILLOS[i % COLORES_ANILLOS.length]}
                      strokeWidth={grosor}
                      strokeLinecap="round"
                      initial={{ pathLength: 0 }}
                      animate={{ pathLength: v }}
                      transition={{ ...RESORTE_SUAVE, delay: i * 0.08 }}
                    />
                  </g>
                );
              })}
            </svg>
          </MovimientoLocal>
          {centro ? (
            <div className="text-titulo text-foreground absolute inset-0 grid place-items-center font-semibold tabular-nums">
              {centro}
            </div>
          ) : null}
        </div>
        <ul className="grid min-w-0 flex-1 gap-2">
          {anillos.map((a, i) => (
            <li key={a.nombre} className="text-control grid grid-cols-[auto_1fr_auto] items-center gap-x-2">
              <span
                className="size-2 rounded-full"
                style={{ background: COLORES_ANILLOS[i % COLORES_ANILLOS.length] }}
              />
              <span className="text-foreground min-w-0">{a.nombre}</span>
              <span className="text-foreground font-semibold tabular-nums">{a.valor}%</span>
              {a.detalle ? (
                <span className="text-etiqueta text-texto-terciario col-start-2 col-end-4 truncate">{a.detalle}</span>
              ) : null}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
