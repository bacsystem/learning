"use client";

import { m } from "motion/react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useId, useReducer } from "react";

import { FOCO, ROTULO } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import { PESO_ICONO, PESO_ICONO_ACTIVO, PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { RESORTE } from "@/lib/movimiento";
import {
  ESTADO_GRUPOS_INICIAL,
  grupoAbierto,
  guardarGruposAbiertos,
  leerGruposAbiertos,
  reducirGrupos,
} from "@/lib/navegacion";
import { cn } from "@/lib/utils";
import { CaretDownIcon } from "@phosphor-icons/react";

export type ItemNav = {
  /** Sin `href` el ítem se muestra deshabilitado con la etiqueta `pendiente` (patrón "próximamente"). */
  href?: string;
  label: string;
  icon: IconoComponente;
  /** Texto del chip cuando no hay href (por defecto "Pronto"). */
  pendiente?: string;
  /** Tooltip explicando por qué todavía no está disponible. */
  title?: string;
  /** Activo solo si la ruta es exactamente `href` (un «Inicio» en `/admin` no debe marcarse en todas las páginas que cuelgan de él). Por defecto, por prefijo. */
  exacto?: boolean;
};

export type GrupoNav = {
  titulo: string;
  items: ItemNav[];
  /** Con `true` el título es un botón que pliega y despliega el grupo. Sin él, el grupo es fijo, como siempre. */
  colapsable?: boolean;
  /** Un grupo colapsable arranca abierto (por defecto, cerrado; el que contiene la ruta activa se abre siempre). */
  abiertoPorDefecto?: boolean;
  /** Identifica al grupo en el estado recordado. Por defecto, el `titulo`; ponlo si el título se traduce, para que el estado sobreviva al cambio de idioma. */
  id?: string;
};

/** Textos del botón de un grupo colapsable; `{grupo}` se sustituye por su título. */
export type TextosSidebarNav = { expandirGrupo: string; contraerGrupo: string };

export const TEXTOS_SIDEBAR_NAV_ES: TextosSidebarNav = {
  expandirGrupo: "Expandir {grupo}",
  contraerGrupo: "Contraer {grupo}",
};

/** Clave de `localStorage` donde se recuerdan los grupos abiertos (cámbiala si la app aloja más de un sidebar o usa un prefijo propio). */
export const CLAVE_GRUPOS_NAV = "khipu-sidebar-grupos";

// Sin overflow-hidden: recortaría el fondo activo mientras se desliza desde otro ítem. La etiqueta trunca por su cuenta (min-w-0).
// `isolate` deja el fondo activo (-z-10) detrás del icono y del texto.
const ITEM_BASE = "relative isolate flex w-full min-w-0 items-center gap-2.5 rounded-full px-3 py-[7px] text-control";

const idGrupo = (grupo: GrupoNav) => grupo.id ?? grupo.titulo;
const esActivo = (item: ItemNav, pathname: string) =>
  Boolean(item.href) && (item.exacto ? pathname === item.href : pathname.startsWith(item.href!));
const contieneActiva = (grupo: GrupoNav, pathname: string) => grupo.items.some((item) => esActivo(item, pathname));

/** Si un grupo colapsable está abierto cuando nadie lo ha plegado ni desplegado: lo pide el grupo o contiene la ruta activa. */
const abiertoPorOmision = (grupo: GrupoNav, pathname: string) =>
  Boolean(grupo.abiertoPorDefecto) || contieneActiva(grupo, pathname);

/**
 * Navegación del sidebar por grupos; el ítem activo se detecta por prefijo de la ruta actual y su fondo se desliza al cambiar.
 * Un grupo con `colapsable` se pliega desde su título: el que contiene la ruta activa se abre solo y el estado se recuerda en `localStorage`.
 */
export function SidebarNav({
  grupos,
  etiqueta,
  textos: textosParciales,
  claveAlmacenamiento = CLAVE_GRUPOS_NAV,
}: {
  grupos: GrupoNav[];
  etiqueta?: string;
  textos?: Partial<TextosSidebarNav>;
  claveAlmacenamiento?: string;
}) {
  const textos = { ...TEXTOS_SIDEBAR_NAV_ES, ...textosParciales };
  const pathname = usePathname();
  // Un layoutId por instancia: el sidebar de escritorio y el del menú móvil no comparten indicador.
  const idIndicador = useId();
  const idBase = useId();
  // Lo que el usuario eligió (se guarda) va aparte de lo que abrió la ruta (no se guarda); lo demás se deriva en el render, así un grupo que aparece después también arranca bien.
  const [estado, despachar] = useReducer(reducirGrupos, ESTADO_GRUPOS_INICIAL);
  const grupoActivo = grupos.find((g) => g.colapsable && contieneActiva(g, pathname));
  const activo = grupoActivo ? idGrupo(grupoActivo) : undefined;

  // Lo recordado se lee tras montar (en el servidor no hay localStorage), una vez por clave.
  useEffect(() => {
    despachar({ tipo: "recordado", guardado: leerGruposAbiertos(claveAlmacenamiento) });
  }, [claveAlmacenamiento]);

  // Al llegar a una ruta, su grupo se abre aunque estuviera plegado.
  useEffect(() => {
    if (activo) despachar({ tipo: "ruta", id: activo });
  }, [activo]);

  function alternar(id: string, abierto: boolean) {
    const accion = { tipo: "alternar", id, abierto } as const;
    guardarGruposAbiertos(claveAlmacenamiento, reducirGrupos(estado, accion).elegidos);
    despachar(accion);
  }

  return (
    <nav aria-label={etiqueta} className="grid w-full grid-cols-1 gap-4 px-2.5 pt-2 pb-3">
      {grupos.map((grupo, indice) => {
        const items = grupo.items.map((item) => (
          <ItemSidebar key={item.href ?? item.label} item={item} pathname={pathname} idIndicador={idIndicador} />
        ));
        if (!grupo.colapsable) {
          return (
            <div key={grupo.titulo} className="grid w-full grid-cols-1 gap-0.5">
              <span className={cn("truncate px-3 pt-1 pb-1.5", ROTULO)}>{grupo.titulo}</span>
              {items}
            </div>
          );
        }
        const id = idGrupo(grupo);
        const abierto = grupoAbierto(estado, id, abiertoPorOmision(grupo, pathname));
        // El id del DOM va por posición: el del grupo puede ser su título, con espacios, que romperían `aria-controls`.
        const idPanel = `${idBase}-grupo-${indice}`;
        const accion = (abierto ? textos.contraerGrupo : textos.expandirGrupo).replace("{grupo}", grupo.titulo);
        return (
          <div key={grupo.titulo} className="grid w-full grid-cols-1 gap-0.5">
            <button
              type="button"
              aria-expanded={abierto}
              aria-controls={idPanel}
              title={accion}
              onClick={() => alternar(id, abierto)}
              className={cn(
                "ease-salida hover:text-foreground flex min-h-8 w-full min-w-0 items-center justify-between gap-2 rounded-full px-3 pt-1 pb-1.5 text-left transition-colors duration-(--duracion-rapida)",
                FOCO,
                ROTULO,
              )}
            >
              <span className="truncate">{grupo.titulo}</span>
              <CaretDownIcon
                weight={PESO_ICONO_FUNCIONAL}
                aria-hidden
                className={cn(
                  "motion-safe:ease-salida size-3 shrink-0 motion-safe:transition-transform motion-safe:duration-(--duracion-media)",
                  abierto ? "rotate-180" : "",
                )}
              />
            </button>
            {/* El despliegue anima la altura con grid-rows (0fr→1fr), solo si el sistema permite movimiento. Cerrado, `inert` quita sus ítems del teclado y del lector. */}
            <div
              id={idPanel}
              inert={!abierto}
              className={cn(
                "motion-safe:ease-salida grid grid-cols-1 motion-safe:transition-[grid-template-rows] motion-safe:duration-(--duracion-media)",
                abierto ? "grid-rows-[1fr]" : "grid-rows-[0fr]",
              )}
            >
              <div className="-mx-1 min-h-0 overflow-hidden px-1">
                <div className="grid grid-cols-1 gap-0.5">{items}</div>
              </div>
            </div>
          </div>
        );
      })}
    </nav>
  );
}

function ItemSidebar({ item, pathname, idIndicador }: { item: ItemNav; pathname: string; idIndicador: string }) {
  const Icono = item.icon;
  if (!item.href) {
    return (
      <span
        aria-disabled="true"
        title={item.title ?? `${item.label}: próximamente`}
        className={cn(ITEM_BASE, "text-muted-foreground/60 cursor-not-allowed")}
      >
        <Icono weight={PESO_ICONO} className="text-muted-foreground/50 size-[18px] shrink-0" />
        <span className="min-w-0 flex-1 truncate">{item.label}</span>
        <span className="bg-secondary text-etiqueta shrink-0 rounded-full px-2 py-0.5 font-medium whitespace-nowrap">
          {item.pendiente ?? "Pronto"}
        </span>
      </span>
    );
  }
  const active = esActivo(item, pathname);
  return (
    <Link
      href={item.href}
      aria-current={active ? "page" : undefined}
      className={cn(
        ITEM_BASE,
        "ease-salida transition-colors duration-(--duracion-rapida)",
        active
          ? "text-sidebar-primary-foreground font-semibold"
          : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
      )}
    >
      {active ? (
        <m.span
          layoutId={`${idIndicador}-activo`}
          aria-hidden
          transition={RESORTE}
          className="border-acento-borde bg-sidebar-primary absolute inset-0 -z-10 rounded-full border"
        />
      ) : null}
      <Icono
        weight={active ? PESO_ICONO_ACTIVO : PESO_ICONO}
        className={cn("size-[18px] shrink-0", active ? "text-current" : "text-muted-foreground")}
      />
      <span className="min-w-0 flex-1 truncate">{item.label}</span>
    </Link>
  );
}
