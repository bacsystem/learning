import type { Tono } from "@/components/patrones/metricas";
import type { IconoComponente } from "@/lib/iconos";
import type { NotifTipo } from "@/lib/notifications/api";
import { MonitorIcon, SparkleIcon, WarningOctagonIcon, WrenchIcon } from "@phosphor-icons/react";

/**
 * Presentación de cada tipo de notificación: icono (sin caja, el color dice el significado), color del icono,
 * tono de la pill y clave i18n de su nombre. La comparten la campana de la barra y la página /notificaciones.
 */
export const TIPO_NOTIFICACION: Record<
  NotifTipo,
  { icono: IconoComponente; color: string; tono: Tono; etiqueta: string }
> = {
  sistema: { icono: MonitorIcon, color: "text-info-foreground", tono: "neutro", etiqueta: "admin-notif-tipo-sistema" },
  mantenimiento: {
    icono: WrenchIcon,
    color: "text-warning-foreground",
    tono: "aviso",
    etiqueta: "admin-notif-tipo-mantenimiento",
  },
  funcionalidad: {
    icono: SparkleIcon,
    color: "text-success-foreground",
    tono: "ok",
    etiqueta: "admin-notif-tipo-funcionalidad",
  },
  alerta: {
    icono: WarningOctagonIcon,
    color: "text-destructive-texto",
    tono: "error",
    etiqueta: "admin-notif-tipo-alerta",
  },
};

/** Tipo desconocido (p. ej. uno nuevo del backend): se muestra como «sistema». */
export function tipoNotificacion(tipo: string) {
  return TIPO_NOTIFICACION[tipo as NotifTipo] ?? TIPO_NOTIFICACION.sistema;
}

/** Traductor del namespace `dashboard` (el `t` de `useTranslations("dashboard")`). */
type Traductor = (clave: string, valores?: Record<string, number>) => string;

/** Tiempo relativo corto («Ahora», «Hace 5 min», «Hace 3 h», «Hace 2 días»), traducido con `t`. */
export function haceCuanto(iso: string, t: Traductor): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000;
  if (diff < 60) return t("notif-hace-ahora");
  if (diff < 3600) return t("notif-hace-min", { n: Math.floor(diff / 60) });
  if (diff < 86400) return t("notif-hace-h", { n: Math.floor(diff / 3600) });
  return t("notif-hace-dias", { n: Math.floor(diff / 86400) });
}
