/**
 * Preferencias de apariencia que elige cada usuario: la paleta (los tres acentos), el tono del modo oscuro y la tipografía.
 * Cada opción es un bloque de tokens en `src/styles/tokens.css`: `html[data-paleta="…"]`, `.dark[data-oscuro="…"]` y
 * `html[data-fuente="…"]`. Las opciones por defecto no tienen bloque: son los tokens de `:root` y `.dark` y la fuente de la app. Claro/oscuro/sistema sigue
 * siendo de next-themes. Este archivo no importa nada: lo usan `ScriptDiseno` (servidor), `AparienciaProvider` (cliente) y los tests.
 */

export type Paleta = { valor: string; etiqueta: string; muestra: [string, string, string] };
/** `muestra`: color de tarjeta; `lienzo` (opcional): color del fondo, para la muestra de dos tonos del selector. */
export type TonoOscuro = { valor: string; etiqueta: string; muestra: string; lienzo?: string };
/** `familia`: el `font-family` CSS de la fuente, para dibujar su muestra «Aa» en el selector. */
export type Fuente = { valor: string; etiqueta: string; familia: string };
export type PreferenciasApariencia = { paleta: string; tonoOscuro: string; fuente: string };

/** Claves de localStorage. */
export const CLAVE_PALETA = "khipu-paleta";
export const CLAVE_TONO_OSCURO = "khipu-oscuro";
export const CLAVE_FUENTE = "khipu-fuente";

export const PALETA_POR_DEFECTO = "lima";
export const TONO_OSCURO_POR_DEFECTO = "medianoche";
export const FUENTE_POR_DEFECTO = "inter";

/** `muestra`: acento, acento 2 y acento 3, en ese orden (los mismos hex que su bloque en tokens.css). */
export const PALETAS: Paleta[] = [
  { valor: "lima", etiqueta: "Lima", muestra: ["#d4f06b", "#cfe2fb", "#f8ec84"] },
  { valor: "indigo", etiqueta: "Índigo", muestra: ["#c9c8ff", "#cdeee9", "#ffdcc2"] },
  { valor: "oceano", etiqueta: "Océano", muestra: ["#9fe3dc", "#d3e4fd", "#ffe7a0"] },
  { valor: "coral", etiqueta: "Coral", muestra: ["#ffc6b3", "#e3dafb", "#fbe39b"] },
  { valor: "violeta", etiqueta: "Violeta", muestra: ["#dacbff", "#c6efdf", "#ffd3e6"] },
];

/** `muestra` y `lienzo`: tarjeta y fondo del tono (Tintado varía con la paleta; la muestra es la de Lima). */
export const TONOS_OSCURO: TonoOscuro[] = [
  { valor: "medianoche", etiqueta: "Medianoche", muestra: "#111a2c", lienzo: "#0a0f1c" },
  { valor: "grafito", etiqueta: "Grafito", muestra: "#1a1b1f", lienzo: "#111214" },
  { valor: "tintado", etiqueta: "Tintado", muestra: "#202420", lienzo: "#0f1110" },
];

/**
 * Fuentes de interfaz. Inter es la fuente de la app (`--font-sans`); las demás las carga la app con `next/font` en
 * `--fuente-<valor>` (ver src/app/layout.tsx) y `html[data-fuente="<valor>"]` la pone en `--font-sans`. Los datos siguen en JetBrains Mono (`--font-mono`).
 */
export const FUENTES: Fuente[] = [
  { valor: "inter", etiqueta: "Inter", familia: "Inter, 'Inter Fallback', sans-serif" },
  {
    valor: "jakarta",
    etiqueta: "Plus Jakarta Sans",
    familia: "var(--fuente-jakarta), 'Plus Jakarta Sans', sans-serif",
  },
  { valor: "manrope", etiqueta: "Manrope", familia: "var(--fuente-manrope), Manrope, sans-serif" },
  { valor: "dmsans", etiqueta: "DM Sans", familia: "var(--fuente-dmsans), 'DM Sans', sans-serif" },
  { valor: "figtree", etiqueta: "Figtree", familia: "var(--fuente-figtree), Figtree, sans-serif" },
];

/** El valor pedido si está entre las opciones; si no, el por defecto. */
export function valorValido(
  valor: string | null | undefined,
  opciones: readonly { valor: string }[],
  porDefecto: string,
): string {
  return valor != null && opciones.some((o) => o.valor === valor) ? valor : porDefecto;
}

/** JSON seguro dentro de `<script>`: escapa `<` para que ningún valor pueda cerrar la etiqueta. */
function jsonEnScript(valor: unknown): string {
  return JSON.stringify(valor).replace(/</g, "\\u003c");
}

/**
 * Código en línea para el `<head>`: pone `data-paleta`, `data-oscuro` y `data-fuente` en `<html>` antes del primer pintado.
 * `inicial` (p. ej. leído del perfil del usuario en el servidor) gana sobre localStorage; un valor desconocido se
 * descarta y se usa lo guardado o, si tampoco vale, el por defecto.
 */
export function codigoScriptApariencia({
  paletas = PALETAS,
  tonosOscuro = TONOS_OSCURO,
  fuentes = FUENTES,
  inicial,
}: {
  paletas?: Paleta[];
  tonosOscuro?: TonoOscuro[];
  fuentes?: Fuente[];
  inicial?: Partial<PreferenciasApariencia>;
} = {}): string {
  const lista = (opciones: readonly { valor: string }[]) => jsonEnScript(opciones.map((o) => o.valor));
  const conocido = (valor: string | undefined, opciones: readonly { valor: string }[]) =>
    valorValido(valor, opciones, "") || null;
  const paletaPorDefecto = valorValido(PALETA_POR_DEFECTO, paletas, paletas[0]?.valor ?? PALETA_POR_DEFECTO);
  const tonoPorDefecto = valorValido(
    TONO_OSCURO_POR_DEFECTO,
    tonosOscuro,
    tonosOscuro[0]?.valor ?? TONO_OSCURO_POR_DEFECTO,
  );
  const fuentePorDefecto = valorValido(FUENTE_POR_DEFECTO, fuentes, fuentes[0]?.valor ?? FUENTE_POR_DEFECTO);
  return (
    `(function(){var d=document.documentElement.dataset;` +
    `function v(c,l,p,i){var x=i;if(!x){try{x=localStorage.getItem(c)}catch(e){}}return l.indexOf(x)>-1?x:p}` +
    `d.paleta=v(${jsonEnScript(CLAVE_PALETA)},${lista(paletas)},${jsonEnScript(paletaPorDefecto)},${jsonEnScript(conocido(inicial?.paleta, paletas))});` +
    `d.oscuro=v(${jsonEnScript(CLAVE_TONO_OSCURO)},${lista(tonosOscuro)},${jsonEnScript(tonoPorDefecto)},${jsonEnScript(conocido(inicial?.tonoOscuro, tonosOscuro))});` +
    `d.fuente=v(${jsonEnScript(CLAVE_FUENTE)},${lista(fuentes)},${jsonEnScript(fuentePorDefecto)},${jsonEnScript(conocido(inicial?.fuente, fuentes))})})()`
  );
}
