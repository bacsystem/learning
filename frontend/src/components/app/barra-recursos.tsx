"use client";

import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { Menu, MenuContent, MenuGroup, MenuGroupLabel, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { adminConfigApi, type Recurso } from "@/lib/auth/auth-api";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_SECUNDARIA } from "@/lib/estilos";
import { type IconoComponente, PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import {
  ArrowSquareOutIcon,
  BookmarkSimpleIcon,
  BookOpenIcon,
  BriefcaseIcon,
  CalendarIcon,
  CaretDownIcon,
  ChartLineIcon,
  ChatCircleIcon,
  CloudIcon,
  DownloadSimpleIcon,
  EnvelopeIcon,
  FileTextIcon,
  FolderIcon,
  GearSixIcon,
  GlobeIcon,
  GraduationCapIcon,
  HeadsetIcon,
  HouseIcon,
  InfoIcon,
  LinkIcon,
  MapPinIcon,
  NewspaperIcon,
  PlayCircleIcon,
  QuestionIcon,
  ShieldCheckIcon,
  SquaresFourIcon,
  StarIcon,
  UsersIcon,
  VideoIcon,
} from "@phosphor-icons/react";

/**
 * El campo `icono` de un recurso es texto libre (antes guardaba nombres de la plantilla, p. ej. «NiLink»).
 * Se normaliza (sin prefijo «Ni», sin sufijo «Icon», en minúsculas, sin guiones) y se busca aquí; si no hay coincidencia, enlace.
 */
const ICONOS_RECURSO: Record<string, IconoComponente> = {
  link: LinkIcon,
  enlace: LinkIcon,
  globe: GlobeIcon,
  web: GlobeIcon,
  world: GlobeIcon,
  book: BookOpenIcon,
  bookopen: BookOpenIcon,
  libro: BookOpenIcon,
  bookmark: BookmarkSimpleIcon,
  document: FileTextIcon,
  documento: FileTextIcon,
  file: FileTextIcon,
  filetext: FileTextIcon,
  pdf: FileTextIcon,
  folder: FolderIcon,
  carpeta: FolderIcon,
  video: VideoIcon,
  play: PlayCircleIcon,
  youtube: PlayCircleIcon,
  calendar: CalendarIcon,
  calendario: CalendarIcon,
  chart: ChartLineIcon,
  chat: ChatCircleIcon,
  messages: ChatCircleIcon,
  message: ChatCircleIcon,
  whatsapp: ChatCircleIcon,
  email: EnvelopeIcon,
  envelope: EnvelopeIcon,
  mail: EnvelopeIcon,
  cloud: CloudIcon,
  download: DownloadSimpleIcon,
  settings: GearSixIcon,
  graduation: GraduationCapIcon,
  graduationcap: GraduationCapIcon,
  education: GraduationCapIcon,
  headset: HeadsetIcon,
  support: HeadsetIcon,
  soporte: HeadsetIcon,
  house: HouseIcon,
  home: HouseIcon,
  info: InfoIcon,
  question: QuestionIcon,
  help: QuestionIcon,
  ayuda: QuestionIcon,
  map: MapPinIcon,
  location: MapPinIcon,
  news: NewspaperIcon,
  newspaper: NewspaperIcon,
  shield: ShieldCheckIcon,
  star: StarIcon,
  users: UsersIcon,
  user: UsersIcon,
  briefcase: BriefcaseIcon,
  layout: SquaresFourIcon,
  grid: SquaresFourIcon,
};

function iconoRecurso(nombre: string | undefined): IconoComponente {
  if (!nombre) return LinkIcon;
  const clave = nombre
    .trim()
    .replace(/^Ni(?=[A-Z])/, "")
    .replace(/Icon$/, "")
    .toLowerCase()
    .replace(/[-_\s]/g, "");
  return ICONOS_RECURSO[clave] ?? LinkIcon;
}

const esEnlaceWeb = (url: string) => /^https?:\/\//i.test(url);

/** Menú «Recursos» de la barra superior: accesos directos activos de la configuración, en su orden; cada uno abre en pestaña nueva. */
export function BarraRecursos() {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const [recursos, setRecursos] = useState<Recurso[]>([]);

  useEffect(() => {
    if (!token) return;
    adminConfigApi
      .getConfigRecursos(token)
      .then((res) => setRecursos(res.filter((r) => r.activo).sort((a, b) => a.orden - b.orden)))
      .catch(() => setRecursos([]));
  }, [token]);

  const abrir = (url: string) => {
    if (!esEnlaceWeb(url)) return;
    window.open(url, "_blank", "noopener,noreferrer");
  };

  if (recursos.length === 0) return null;

  return (
    <Menu>
      <MenuTrigger
        aria-label={t("barra-recursos")}
        className={cn(ACCION_SECUNDARIA, "max-md:w-control max-md:justify-center max-md:px-0")}
      >
        <SquaresFourIcon className="text-muted-foreground size-4" />
        <span className="hidden md:inline">{t("barra-recursos")}</span>
        <CaretDownIcon weight={PESO_ICONO_FUNCIONAL} className="text-texto-terciario hidden size-3 md:inline" />
      </MenuTrigger>
      <MenuContent side="bottom" align="end" className="w-[min(20rem,calc(100vw-1.5rem))]">
        <MenuGroup>
          <MenuGroupLabel>{t("admin-config-recursos-titulo")}</MenuGroupLabel>
          {recursos.map((r) => {
            const Icono = iconoRecurso(r.icono);
            const valido = esEnlaceWeb(r.url);
            return (
              <MenuItem
                key={r.id}
                disabled={!valido}
                onClick={() => abrir(r.url)}
                title={valido ? r.url : undefined}
                className="items-start gap-2.5 py-2 [&_svg]:size-4"
              >
                <Icono className="mt-0.5" />
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate">{r.titulo}</span>
                  {r.descripcion ? (
                    <span className="text-etiqueta text-muted-foreground line-clamp-2 font-normal">
                      {r.descripcion}
                    </span>
                  ) : null}
                </span>
                <ArrowSquareOutIcon weight={PESO_ICONO_FUNCIONAL} className="mt-0.5 size-3.5! opacity-60" />
              </MenuItem>
            );
          })}
        </MenuGroup>
      </MenuContent>
    </Menu>
  );
}
