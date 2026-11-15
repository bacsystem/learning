import type { GrupoNav } from "@/components/nav/sidebar-nav";
import type { IconoComponente } from "@/lib/iconos";
import {
  BellIcon,
  BookmarksIcon,
  BooksIcon,
  BuildingsIcon,
  ChartLineIcon,
  ClipboardTextIcon,
  ClockCountdownIcon,
  CoinsIcon,
  GearSixIcon,
  GraduationCapIcon,
  HeartbeatIcon,
  HouseIcon,
  ListChecksIcon,
  ListMagnifyingGlassIcon,
  MedalIcon,
  NotePencilIcon,
  PlusCircleIcon,
  ScalesIcon,
  ShieldCheckIcon,
  SignpostIcon,
  SlidersHorizontalIcon,
  SparkleIcon,
  StarIcon,
  StethoscopeIcon,
  TagIcon,
  TimerIcon,
  TreeStructureIcon,
  UserCircleIcon,
  UsersIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";

/** Un destino de la navegación. `etiqueta` es una clave de i18n del namespace `dashboard`. */
export type DestinoNav = { href: string; etiqueta: string; icono: IconoComponente; exacto?: boolean };
/**
 * Grupo del sidebar; `soloAdmin` lo oculta a quien no es administrador. `colapsable` lo pliega desde su título y exige
 * un `id` estable (no se traduce), con el que el sidebar recuerda si está abierto aunque cambie el idioma.
 */
export type GrupoDestinos = {
  id?: string;
  titulo: string;
  soloAdmin?: boolean;
  colapsable?: boolean;
  items: DestinoNav[];
};

export const NAVEGACION: GrupoDestinos[] = [
  {
    titulo: "menu-inicio",
    items: [
      { href: "/dashboards/analytics", etiqueta: "menu-inicio", icono: HouseIcon },
      { href: "/procesos", etiqueta: "menu-procesos", icono: GraduationCapIcon },
      { href: "/profesion", etiqueta: "menu-profesion", icono: StethoscopeIcon },
    ],
  },
  {
    titulo: "menu-practicas",
    items: [
      { href: "/practicas", etiqueta: "menu-practicas-historial", icono: NotePencilIcon, exacto: true },
      { href: "/practicas/nueva", etiqueta: "menu-practicas-nueva", icono: PlusCircleIcon },
    ],
  },
  {
    titulo: "menu-simulacros",
    items: [
      { href: "/simulacros", etiqueta: "menu-simulacros-historial", icono: TimerIcon, exacto: true },
      { href: "/simulacros/nueva", etiqueta: "menu-simulacros-nuevo", icono: PlusCircleIcon },
    ],
  },
  {
    id: "personas",
    titulo: "menu-admin-grupo-personas",
    soloAdmin: true,
    colapsable: true,
    items: [
      { href: "/admin/dashboard", etiqueta: "menu-admin-dashboard", icono: ChartLineIcon },
      { href: "/admin/usuarios", etiqueta: "menu-admin-usuarios", icono: UsersIcon },
      { href: "/admin/personas", etiqueta: "menu-admin-personas", icono: UsersThreeIcon },
      { href: "/admin/inscripciones", etiqueta: "menu-admin-inscripciones", icono: ClipboardTextIcon },
    ],
  },
  {
    id: "contenido",
    titulo: "menu-admin-grupo-contenido",
    soloAdmin: true,
    colapsable: true,
    items: [
      { href: "/admin/preguntas", etiqueta: "menu-admin-preguntas", icono: ShieldCheckIcon },
      { href: "/admin/fuentes", etiqueta: "menu-admin-fuentes", icono: BooksIcon },
      { href: "/admin/asignaturas", etiqueta: "menu-admin-asignaturas", icono: ListChecksIcon },
      { href: "/admin/temas", etiqueta: "menu-admin-temas", icono: TreeStructureIcon },
      { href: "/admin/pesos", etiqueta: "menu-admin-pesos", icono: ScalesIcon },
      { href: "/admin/etiquetas", etiqueta: "menu-admin-etiquetas", icono: TagIcon },
      { href: "/admin/ia", etiqueta: "menu-admin-ia", icono: SparkleIcon },
      { href: "/admin/configuracion-examen", etiqueta: "menu-admin-configuracion-examen", icono: ClockCountdownIcon },
    ],
  },
  {
    id: "catalogos",
    titulo: "menu-admin-grupo-catalogos",
    soloAdmin: true,
    colapsable: true,
    items: [
      { href: "/admin/instituciones", etiqueta: "menu-admin-instituciones", icono: BuildingsIcon },
      { href: "/admin/grados", etiqueta: "menu-admin-grados", icono: MedalIcon },
      { href: "/admin/especialidades", etiqueta: "menu-admin-especialidades", icono: BookmarksIcon },
      { href: "/admin/procesos", etiqueta: "menu-admin-procesos", icono: SignpostIcon },
    ],
  },
  {
    id: "sistema",
    titulo: "menu-admin-grupo-sistema",
    soloAdmin: true,
    colapsable: true,
    items: [
      { href: "/admin/ingresos", etiqueta: "menu-admin-ingresos", icono: CoinsIcon },
      { href: "/admin/testimonios", etiqueta: "menu-admin-testimonios", icono: StarIcon },
      { href: "/admin/notificaciones", etiqueta: "menu-admin-notificaciones", icono: BellIcon },
      { href: "/admin/audit-log", etiqueta: "menu-admin-audit-log", icono: ListMagnifyingGlassIcon },
      { href: "/admin/salud", etiqueta: "saludServicios.menu", icono: HeartbeatIcon },
      {
        href: "/admin/configuracion",
        etiqueta: "menu-admin-configuracion",
        icono: SlidersHorizontalIcon,
        exacto: true,
      },
    ],
  },
];

/** Grupos del sidebar para el rol dado: los de `soloAdmin` se ocultan a quien no es admin y los títulos y etiquetas se traducen con `t`. */
export function gruposDelSidebar(
  esAdmin: boolean,
  t: (clave: string) => string,
  grupos: GrupoDestinos[] = NAVEGACION,
): GrupoNav[] {
  return grupos
    .filter((g) => !g.soloAdmin || esAdmin)
    .map((g) => ({
      id: g.id,
      colapsable: g.colapsable,
      titulo: t(g.titulo),
      items: g.items.map((i) => ({ href: i.href, label: t(i.etiqueta), icon: i.icono, exacto: i.exacto })),
    }));
}

/** Rutas sin ítem propio en el sidebar, para la miga de la barra superior. */
export const RUTAS_EXTRA: { prefijo: string; seccion: string; pagina: string }[] = [
  { prefijo: "/admin/personas/import", seccion: "menu-admin-grupo-personas", pagina: "menu-admin-personas-import" },
  { prefijo: "/admin/preguntas/import", seccion: "menu-admin-grupo-contenido", pagina: "menu-admin-preguntas" },
  { prefijo: "/admin/preguntas/new", seccion: "menu-admin-grupo-contenido", pagina: "menu-admin-preguntas" },
  { prefijo: "/settings", seccion: "menu-settings", pagina: "menu-settings" },
  { prefijo: "/notificaciones", seccion: "menu-inicio", pagina: "menu-admin-notificaciones" },
];

export const ICONO_PERFIL = UserCircleIcon;
export const ICONO_AJUSTES = GearSixIcon;
