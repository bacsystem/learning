"use client";

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";

import { haceCuanto, tipoNotificacion } from "@/components/app/notificaciones-tipo";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { Tooltip } from "@/components/feedback/tooltip";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_SECUNDARIA, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { type Notificacion, notificacionesApi } from "@/lib/notifications/api";
import { cn } from "@/lib/utils";
import { BellIcon, ChecksIcon, XIcon } from "@phosphor-icons/react";

export default function NotificacionesPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const [items, setItems] = useState<Notificacion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const data = await notificacionesApi.list(token);
      setItems(data);
    } catch {
      setError(t("comun-error-cargar"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    load();
  }, [load]);

  const handleMarkRead = async (id: number) => {
    if (!token) return;
    try {
      await notificacionesApi.markRead(token, id);
      setItems((prev) => prev.map((n) => (n.id === id ? { ...n, leida: true } : n)));
    } catch {
      /* ignorado, como en el original */
    }
  };

  const handleMarkAllRead = async () => {
    if (!token) return;
    try {
      await notificacionesApi.markAllRead(token);
      setItems((prev) => prev.map((n) => ({ ...n, leida: true })));
    } catch {
      /* ignorado, como en el original */
    }
  };

  const unreadCount = items.filter((n) => !n.leida).length;

  return (
    <div className={CONTENEDOR_PAGINA}>
      <section className={TARJETA}>
        <CabeceraSeccion
          icon={BellIcon}
          titulo={t("notif-subtitulo")}
          subtitulo={loading || error ? undefined : t("notif-en-total", { n: items.length })}
          derecha={
            <div className="flex items-center gap-2">
              {unreadCount > 0 ? <Chip>{t("notif-sin-leer", { n: unreadCount })}</Chip> : null}
              {unreadCount > 0 ? (
                <button type="button" onClick={handleMarkAllRead} className={ACCION_SECUNDARIA}>
                  <ChecksIcon weight={PESO_ICONO_FUNCIONAL} className="size-4" />
                  <span className="max-sm:sr-only">{t("notif-marcar-todas")}</span>
                </button>
              ) : null}
            </div>
          }
        />

        <div className="px-2 pt-1 pb-2">
          {loading ? (
            <div className="flex flex-col gap-1" aria-busy="true">
              {[0, 1, 2, 3, 4].map((i) => (
                <div key={i} className="flex items-start gap-3 px-3 py-3">
                  <Skeleton className="size-[18px] rounded-full" />
                  <div className="flex flex-1 flex-col gap-2">
                    <Skeleton className="h-4 w-1/3" />
                    <Skeleton className="h-3.5 w-3/4" />
                    <Skeleton className="h-3 w-20" />
                  </div>
                </div>
              ))}
            </div>
          ) : error ? (
            // Si la carga falla se muestra solo el error con «Reintentar», no el falso «Sin notificaciones».
            <Alerta
              tono="error"
              className="m-2"
              accion={
                <Button variant="outline" size="sm" onClick={load}>
                  {t("comun-reintentar")}
                </Button>
              }
            >
              {error}
            </Alerta>
          ) : items.length === 0 ? (
            <EstadoVacio
              icon={BellIcon}
              titulo={t("notif-sin-notificaciones")}
              className="m-2 border-0 bg-transparent"
            />
          ) : (
            <ul className="divide-border/60 flex flex-col divide-y">
              {items.map((n) => {
                const tipo = tipoNotificacion(n.tipo);
                const Icono = tipo.icono;
                return (
                  <li key={n.id} className="group relative py-0.5">
                    <button
                      type="button"
                      onClick={() => {
                        if (!n.leida) handleMarkRead(n.id);
                      }}
                      className={cn(
                        "ease-salida hover:bg-secondary focus-visible:ring-ring/50 flex w-full items-start gap-3 rounded-lg px-3 py-3 pr-12 text-left transition-colors duration-(--duracion-rapida) outline-none focus-visible:ring-2",
                        !n.leida && "bg-accent/40",
                      )}
                    >
                      <Icono className={cn("mt-0.5 size-[18px] shrink-0", tipo.color)} />
                      <span className="flex min-w-0 flex-1 flex-col gap-1">
                        <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                          <span
                            className={cn(
                              "text-control text-foreground leading-snug",
                              n.leida ? "font-medium" : "font-semibold",
                            )}
                          >
                            {n.titulo}
                          </span>
                          <PillEstado tono={tipo.tono}>{t(tipo.etiqueta)}</PillEstado>
                          {!n.leida ? (
                            <Chip className="bg-accent text-accent-foreground">{t("notif-no-leida")}</Chip>
                          ) : null}
                        </span>
                        <span className="text-cuerpo text-muted-foreground leading-relaxed whitespace-pre-wrap">
                          {n.cuerpo}
                        </span>
                        <span className="text-etiqueta text-texto-terciario">{haceCuanto(n.created_at, t)}</span>
                      </span>
                    </button>
                    {!n.leida ? (
                      <Tooltip texto={t("notif-marcar-leida")} lado="left">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={t("notif-marcar-leida")}
                          onClick={(e) => {
                            e.stopPropagation();
                            handleMarkRead(n.id);
                          }}
                          className="absolute top-3 right-2 opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                        >
                          <XIcon weight={PESO_ICONO_FUNCIONAL} className="size-3.5" />
                        </Button>
                      </Tooltip>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </section>
    </div>
  );
}
