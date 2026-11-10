"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import { Kbd } from "@/components/ui/kbd";
import { cn } from "@/lib/utils";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

/** Miga por prefijo de ruta: la primera que coincida con `pathname.startsWith(prefijo)` gana. */
export type Miga = { prefijo: string; seccion: string; pagina: string };

/** Pill de estado del entorno/servicio para la zona izquierda. */
export type PillEntorno = {
  texto: string;
  tono: "ok" | "aviso" | "neutro" | "oscuro";
  title?: string;
  deshabilitado?: boolean;
};

// Las recetas de acción viven en lib/estilos.ts (también se usan fuera de la top bar); se re-exportan aquí para no romper
// a quien ya las importa desde este archivo.
export { ACCION_PRINCIPAL, ACCION_SECUNDARIA } from "@/lib/estilos";

function Pill({ pill }: { pill: PillEntorno }) {
  const tonos = {
    ok: "bg-success text-success-foreground",
    aviso: "bg-warning text-warning-foreground",
    neutro: "bg-card text-muted-foreground",
    oscuro: "bg-foreground text-background",
  } as const;
  const puntos = {
    ok: "bg-success-solid",
    aviso: "bg-warning-solid",
    neutro: "bg-muted-foreground/40",
    oscuro: "bg-success-solid",
  } as const;
  return (
    <span
      title={pill.title}
      className={cn(
        "text-etiqueta hidden shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 font-semibold whitespace-nowrap lg:inline-flex",
        tonos[pill.tono],
        pill.deshabilitado && "opacity-60",
      )}
    >
      <span className={cn("size-1.5 rounded-full", puntos[pill.tono])} />
      {pill.texto}
    </span>
  );
}

/**
 * Barra superior pegajosa (h-14). Izquierda: `menuMovil` (hamburguesa), miga (es el h1 de la página) y pills.
 * Derecha: buscador opcional, y `acciones` (máximo dos secundarias + una principal; el resto va en diálogos).
 */
export function TopBar({
  migas,
  pills = [],
  menuMovil,
  buscador,
  acciones,
  etiquetaUbicacion = "Ubicación",
}: {
  migas: Miga[];
  pills?: PillEntorno[];
  menuMovil?: ReactNode;
  /** `{ placeholder, onClick }` o `false` para ocultarlo; sin `onClick` se muestra deshabilitado ("próximamente"). */
  buscador?: { placeholder: string; atajo?: string; onClick?: () => void; title?: string } | false;
  acciones?: ReactNode;
  /** Nombre accesible de la miga (`aria-label` de su `nav`). */
  etiquetaUbicacion?: string;
}) {
  const pathname = usePathname();
  const miga = migas.find((m) => pathname.startsWith(m.prefijo));

  return (
    <header className="bg-background/80 sticky top-0 z-20 flex h-14 shrink-0 items-center justify-between gap-3 px-3 backdrop-blur md:px-4">
      <div className="flex min-w-0 items-center gap-3">
        {menuMovil ? <div className="md:hidden">{menuMovil}</div> : null}

        {miga ? (
          <nav
            aria-label={etiquetaUbicacion}
            className="text-control flex min-w-0 shrink items-center gap-1.5 overflow-hidden whitespace-nowrap"
          >
            {/* En el teléfono solo la página: la sección y la página no entraban junto a la acción principal y la página quedaba en «C…». */}
            <span className="text-texto-terciario hidden sm:inline">{miga.seccion}</span>
            <span className="text-texto-terciario/50 hidden sm:inline">/</span>
            <h1 className="font-heading text-titulo text-foreground truncate">{miga.pagina}</h1>
          </nav>
        ) : null}

        {pills.map((p) => (
          <Pill key={p.texto} pill={p} />
        ))}
      </div>

      <div className="flex shrink-0 items-center gap-2.5">
        {buscador ? (
          <div className="relative hidden w-72 lg:block" title={buscador.title}>
            <MagnifyingGlassIcon
              weight="bold"
              className="text-texto-terciario pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
            />
            <input
              readOnly
              disabled={!buscador.onClick}
              onClick={buscador.onClick}
              placeholder={buscador.placeholder}
              className="h-control bg-card text-control text-foreground ring-border placeholder:text-texto-terciario w-full rounded-full pr-12 pl-9 ring-1 disabled:cursor-not-allowed"
            />
            {buscador.atajo ? <Kbd className="absolute top-1/2 right-2 -translate-y-1/2">{buscador.atajo}</Kbd> : null}
          </div>
        ) : null}
        {/*
          `contents` (no un `{acciones}` suelto) a propósito: si `acciones` es un elemento de un componente
          cliente creado por un Server Component padre (el caso de la demo, ver ejemplo/layout.tsx) y queda
          como uno más de los hijos estáticos de este div, React 19 en dev tira "Each child in a list should
          have a unique key prop" apuntando a TopBar/el layout, aunque nada de la lista tenga de verdad más de
          un elemento. Aislarlo como único hijo de su propio contenedor lo evita; `display:contents` no añade
          caja, así que el `gap-2.5` del padre se comporta igual que si `acciones` estuviera suelto aquí.
        */}
        <div className="contents">{acciones}</div>
      </div>
    </header>
  );
}
