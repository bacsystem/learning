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
  StarIcon,
  TagIcon,
  TimerIcon,
  TreeStructureIcon,
  UploadSimpleIcon,
  UserCircleIcon,
  UsersIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";

/** Un destino de la navegación. `etiqueta` es una clave de i18n del namespace `dashboard`. */
export type DestinoNav = { href: string; etiqueta: string; icono: IconoComponente; exacto?: boolean };
/** Grupo del sidebar; `soloAdmin` lo oculta a quien no es administrador. */
export type GrupoDestinos = { titulo: string; soloAdmin?: boolean; items: DestinoNav[] };

export const NAVEGACION: GrupoDestinos[] = [
  {
    titulo: "menu-inicio",
    items: [
      { href: "/dashboards/analytics", etiqueta: "menu-inicio", icono: HouseIcon },
      { href: "/procesos", etiqueta: "menu-procesos", icono: GraduationCapIcon },
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
    titulo: "menu-admin",
    soloAdmin: true,
    items: [
      { href: "/admin/dashboard", etiqueta: "menu-admin-dashboard", icono: ChartLineIcon },
      { href: "/admin/usuarios", etiqueta: "menu-admin-usuarios", icono: UsersIcon },
      { href: "/admin/personas", etiqueta: "menu-admin-personas", icono: UsersThreeIcon, exacto: true },
      { href: "/admin/personas/import", etiqueta: "menu-admin-personas-import", icono: UploadSimpleIcon },
      { href: "/admin/inscripciones", etiqueta: "menu-admin-inscripciones", icono: ClipboardTextIcon },
      { href: "/admin/ingresos", etiqueta: "menu-admin-ingresos", icono: CoinsIcon },
      { href: "/admin/testimonios", etiqueta: "menu-admin-testimonios", icono: StarIcon },
      { href: "/admin/audit-log", etiqueta: "menu-admin-audit-log", icono: ListMagnifyingGlassIcon },
    ],
  },
  {
    titulo: "menu-gestion",
    soloAdmin: true,
    items: [
      { href: "/admin/instituciones", etiqueta: "menu-admin-instituciones", icono: BuildingsIcon },
      { href: "/admin/grados", etiqueta: "menu-admin-grados", icono: MedalIcon },
      { href: "/admin/especialidades", etiqueta: "menu-admin-especialidades", icono: BookmarksIcon },
      { href: "/admin/asignaturas", etiqueta: "menu-admin-asignaturas", icono: ListChecksIcon },
      { href: "/admin/temas", etiqueta: "menu-admin-temas", icono: TreeStructureIcon },
      { href: "/admin/pesos", etiqueta: "menu-admin-pesos", icono: ScalesIcon },
      { href: "/admin/fuentes", etiqueta: "menu-admin-fuentes", icono: BooksIcon },
      { href: "/admin/procesos", etiqueta: "menu-admin-procesos", icono: SignpostIcon },
      { href: "/admin/preguntas", etiqueta: "menu-admin-preguntas", icono: ShieldCheckIcon },
      { href: "/admin/etiquetas", etiqueta: "menu-admin-etiquetas", icono: TagIcon },
      { href: "/admin/configuracion-examen", etiqueta: "menu-admin-configuracion-examen", icono: ClockCountdownIcon },
      { href: "/admin/notificaciones", etiqueta: "menu-admin-notificaciones", icono: BellIcon },
      {
        href: "/admin/configuracion",
        etiqueta: "menu-admin-configuracion",
        icono: SlidersHorizontalIcon,
        exacto: true,
      },
    ],
  },
];

/** Rutas sin ítem propio en el sidebar, para la miga de la barra superior. */
export const RUTAS_EXTRA: { prefijo: string; seccion: string; pagina: string }[] = [
  { prefijo: "/admin/preguntas/import", seccion: "menu-gestion", pagina: "menu-admin-preguntas" },
  { prefijo: "/admin/preguntas/new", seccion: "menu-gestion", pagina: "menu-admin-preguntas" },
  { prefijo: "/settings", seccion: "menu-settings", pagina: "menu-settings" },
  { prefijo: "/notificaciones", seccion: "menu-inicio", pagina: "menu-admin-notificaciones" },
];

export const ICONO_PERFIL = UserCircleIcon;
export const ICONO_AJUSTES = GearSixIcon;
