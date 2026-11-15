import type { Traductor } from "@/components/app/textos-khipu";
import type { Notificacion } from "@/lib/notifications/api";

/**
 * Plantillas de las notificaciones personales con `clave` (docs/api/plataforma-avisos-idioma.md §2.3): se traducen al
 * idioma actual con `parametros` y llevan a una pantalla. `requeridos` son los parámetros que el texto necesita; si
 * falta alguno se muestra el `titulo`/`cuerpo` guardados.
 */
const PLANTILLAS = {
  inscripcion_aprobada: {
    titulo: "notif-plantilla-inscripcion-aprobada-titulo",
    cuerpo: "notif-plantilla-inscripcion-aprobada-cuerpo",
    requeridos: ["proceso", "institucion"],
    destino: "/procesos",
  },
  inscripcion_rechazada: {
    titulo: "notif-plantilla-inscripcion-rechazada-titulo",
    cuerpo: "notif-plantilla-inscripcion-rechazada-cuerpo",
    requeridos: ["proceso", "institucion", "motivo"],
    destino: "/procesos",
  },
} as const;

type ClaveConocida = keyof typeof PLANTILLAS;

const plantillaDe = (n: Pick<Notificacion, "clave">) =>
  n.clave && Object.hasOwn(PLANTILLAS, n.clave) ? PLANTILLAS[n.clave as ClaveConocida] : null;

/** Solo valores de texto o número: el resto del JSON no se interpola. */
function parametrosDe(n: Pick<Notificacion, "parametros">): Record<string, string | number> {
  return Object.fromEntries(
    Object.entries(n.parametros ?? {}).filter(([, v]) => typeof v === "string" || typeof v === "number"),
  ) as Record<string, string | number>;
}

/**
 * Título y cuerpo de una notificación: por `clave` en el idioma actual (con `parametros`) o, sin plantilla o con
 * parámetros incompletos, los guardados. Es siempre texto: `motivo` nunca se interpreta como HTML.
 */
export function textoNotificacion(
  n: Pick<Notificacion, "clave" | "parametros" | "titulo" | "cuerpo">,
  t: Traductor,
): { titulo: string; cuerpo: string } {
  const plantilla = plantillaDe(n);
  const parametros = parametrosDe(n);
  if (!plantilla || !plantilla.requeridos.every((p) => Object.hasOwn(parametros, p))) {
    return { titulo: n.titulo, cuerpo: n.cuerpo };
  }
  return { titulo: t(plantilla.titulo, parametros), cuerpo: t(plantilla.cuerpo, parametros) };
}

/** Pantalla a la que lleva una notificación con plantilla; `null` si solo se marca como leída. */
export function destinoNotificacion(n: Pick<Notificacion, "clave">): string | null {
  return plantillaDe(n)?.destino ?? null;
}
