"use client";

import { cn } from "@/lib/utils";
import { Avatar as AvatarPrimitive } from "@base-ui/react/avatar";

const TAMANOS = {
  xs: "size-5 text-etiqueta",
  sm: "size-6 text-etiqueta",
  default: "size-8 text-etiqueta",
  lg: "size-10 text-control",
  xl: "size-12 text-titulo",
} as const;

const TONOS = {
  primary: "bg-primary text-primary-foreground",
  secondary: "bg-secondary text-foreground/80",
  accent: "bg-accent text-primary",
} as const;

/** "Ana Torres" → "AT" (nombre y apellido); "ana@acme.pe" o "ana" → "AN" (2 primeras letras). */
export function iniciales(texto: string): string {
  const local = texto.includes("@") ? (texto.split("@")[0] ?? texto) : texto;
  const partes = local.trim().split(/\s+/);
  if (partes.length > 1 && partes[0] && partes[1]) return (partes[0][0]! + partes[1][0]!).toUpperCase();
  return local.slice(0, 2).toUpperCase();
}

/**
 * Avatar de usuario/empresa (`base-ui/avatar`): muestra `src` si carga, iniciales de `nombre` si no hay imagen o falla.
 * `rounded-full` fijo, como marca la convención de radios del kit (§4). Para el chip compacto y cuadrado del propio
 * sidebar (`size-6 rounded-md` dentro de una tarjeta de disparador) sigue siendo válido el patrón ad-hoc de
 * `MenuUsuario`/`SelectorContexto`; este `Avatar` es para el resto de la app (listas, fichas, `AvatarGroup`).
 */
export function Avatar({
  nombre,
  src,
  tamano = "default",
  tono = "secondary",
  className,
}: {
  /** Nombre o correo; da las iniciales de respaldo y el `alt` de la imagen. */
  nombre: string;
  src?: string | null;
  tamano?: keyof typeof TAMANOS;
  tono?: keyof typeof TONOS;
  className?: string;
}) {
  return (
    <AvatarPrimitive.Root
      className={cn(
        "inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full font-bold select-none",
        TAMANOS[tamano],
        TONOS[tono],
        className,
      )}
    >
      {src ? <AvatarPrimitive.Image src={src} alt={nombre} className="size-full object-cover" /> : null}
      <AvatarPrimitive.Fallback className="flex items-center justify-center">
        {iniciales(nombre)}
      </AvatarPrimitive.Fallback>
    </AvatarPrimitive.Root>
  );
}

/** Fila de avatares superpuestos (asignados a un documento, participantes de un hilo). `max` recorta y suma "+N". */
export function AvatarGroup({
  nombres,
  max = 4,
  tamano = "sm",
  className,
}: {
  nombres: string[];
  max?: number;
  tamano?: keyof typeof TAMANOS;
  className?: string;
}) {
  const visibles = nombres.slice(0, max);
  const resto = nombres.length - visibles.length;
  return (
    <div className={cn("flex -space-x-2", className)}>
      {visibles.map((nombre, i) => (
        <Avatar
          key={`${nombre}-${i}`}
          nombre={nombre}
          tamano={tamano}
          tono={i % 2 === 0 ? "secondary" : "accent"}
          className="ring-card ring-2"
        />
      ))}
      {resto > 0 ? (
        <div
          className={cn(
            "bg-muted text-muted-foreground ring-card inline-flex shrink-0 items-center justify-center rounded-full font-bold ring-2",
            TAMANOS[tamano],
          )}
        >
          +{resto}
        </div>
      ) : null}
    </div>
  );
}
