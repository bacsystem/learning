"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";

import { haceCuanto, tipoNotificacion } from "@/components/app/notificaciones-tipo";
import { Skeleton } from "@/components/feedback/skeleton";
import { Tooltip } from "@/components/feedback/tooltip";
import { Tabs } from "@/components/navegacion/tabs";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_SECUNDARIO } from "@/lib/estilos";
import { PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { type Notificacion, notificacionesApi } from "@/lib/notifications/api";
import { fetchMiTestimonio } from "@/lib/testimonios/api";
import { cn } from "@/lib/utils";
import { BellIcon, BellSlashIcon, ChecksIcon, GearSixIcon, XIcon } from "@phosphor-icons/react";

const OPINION_REMINDER_KEY = "opinion_reminder_dismissed";

type ItemNotif = Notificacion & { _virtual?: boolean };
type Pestana = "all" | "sistema" | "user-tab";

const FILTROS: Record<Pestana, (n: ItemNotif) => boolean> = {
  all: () => true,
  sistema: (n) => n.tipo === "sistema" || n.tipo === "mantenimiento" || n.tipo === "alerta",
  "user-tab": (n) => n.tipo === "funcionalidad",
};

/** Campana de la barra superior: contador de no leídas (cada 15 s y al volver a la pestaña), lista por pestañas y recordatorio de opinión. */
export function BarraNotificaciones() {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const [abierto, setAbierto] = useState(false);
  const [silenciado, setSilenciado] = useState(false);
  const [pestana, setPestana] = useState<Pestana>("all");
  const [items, setItems] = useState<Notificacion[]>([]);
  const [noLeidas, setNoLeidas] = useState(0);
  const [cargando, setCargando] = useState(false);
  const [error, setError] = useState(false);
  const [cargadas, setCargadas] = useState(false);
  const [mostrarOpinion, setMostrarOpinion] = useState(false);
  // Ids con un POST de «marcar como leída» en curso: evita el doble envío (clic en la fila y en la X).
  const marcando = useRef(new Set<number>());

  const cargarNoLeidas = useCallback(async () => {
    if (!token) return;
    try {
      const { count } = await notificacionesApi.unreadCount(token);
      setNoLeidas(count);
    } catch {
      /* se reintenta en el siguiente ciclo */
    }
  }, [token]);

  // Contador cada 15 s y al recuperar el foco de la ventana o la pestaña.
  useEffect(() => {
    cargarNoLeidas();
    const id = setInterval(cargarNoLeidas, 15_000);
    const alVolver = () => {
      if (document.visibilityState === "visible") cargarNoLeidas();
    };
    window.addEventListener("focus", alVolver);
    document.addEventListener("visibilitychange", alVolver);
    return () => {
      clearInterval(id);
      window.removeEventListener("focus", alVolver);
      document.removeEventListener("visibilitychange", alVolver);
    };
  }, [cargarNoLeidas]);

  // Si el usuario aún no dejó su testimonio (y no descartó el aviso), se muestra un recordatorio.
  useEffect(() => {
    if (!token) return;
    let descartado = false;
    try {
      descartado = localStorage.getItem(OPINION_REMINDER_KEY) === "true";
    } catch {
      /* almacenamiento bloqueado: se muestra el recordatorio */
    }
    if (descartado) return;
    fetchMiTestimonio(token)
      .then((mio) => setMostrarOpinion(mio === null))
      .catch(() => {});
  }, [token]);

  const cargarItems = useCallback(async () => {
    if (!token) return;
    setCargando(true);
    setError(false);
    try {
      const data = await notificacionesApi.list(token);
      setItems(data);
      // La lista viene con LIMIT 100: el contador sale del endpoint propio, no de contar la lista.
      cargarNoLeidas();
      setCargadas(true);
    } catch {
      setError(true);
    } finally {
      setCargando(false);
    }
  }, [token, cargarNoLeidas]);

  const cambiarAbierto = (siguiente: boolean) => {
    if (siguiente) cargarItems();
    setAbierto(siguiente);
  };

  const marcarLeida = async (id: number) => {
    if (!token || marcando.current.has(id)) return;
    const item = items.find((n) => n.id === id);
    if (!item || item.leida) return;
    marcando.current.add(id);
    try {
      await notificacionesApi.markRead(token, id);
      setItems((prev) => prev.map((n) => (n.id === id ? { ...n, leida: true } : n)));
      // Solo pasó de no leída a leída una vez: se resta una sola vez.
      setNoLeidas((prev) => Math.max(0, prev - 1));
    } catch {
      /* ignorado, como en el original */
    } finally {
      marcando.current.delete(id);
    }
  };

  const marcarTodas = async () => {
    if (!token) return;
    try {
      await notificacionesApi.markAllRead(token);
      setItems((prev) => prev.map((n) => ({ ...n, leida: true })));
      setNoLeidas(0);
    } catch {
      /* ignorado, como en el original */
    }
  };

  const descartarOpinion = () => {
    try {
      localStorage.setItem(OPINION_REMINDER_KEY, "true");
    } catch {
      /* sin almacenamiento: solo se oculta en esta sesión */
    }
    setMostrarOpinion(false);
  };

  const opinion: ItemNotif | null = mostrarOpinion
    ? {
        id: -1,
        titulo: t("notif-opinion-titulo"),
        cuerpo: t("notif-opinion-cuerpo"),
        tipo: "funcionalidad",
        destinatario: "rol:usuario",
        creado_por: undefined,
        created_at: new Date().toISOString(),
        leida: false,
        _virtual: true,
      }
    : null;

  const todos: ItemNotif[] = opinion ? [opinion, ...items] : items;
  const totalNoLeidas = noLeidas + (mostrarOpinion ? 1 : 0);
  const contador = silenciado ? 0 : totalNoLeidas;
  const noLeidasDe = (p: Pestana) => todos.filter((n) => FILTROS[p](n) && !n.leida).length;

  const pestanas = [
    { id: "all" as const, etiqueta: t("notif-pestana-todas") },
    { id: "sistema" as const, etiqueta: t("notif-pestana-sistema") },
    { id: "user-tab" as const, etiqueta: t("notif-pestana-usuarios") },
  ].map((p) => {
    const n = noLeidasDe(p.id);
    return { ...p, contador: n > 0 ? n : undefined };
  });

  const IconoCampana = silenciado ? BellSlashIcon : BellIcon;

  return (
    <Popover open={abierto} onOpenChange={cambiarAbierto}>
      <Tooltip texto={t("notif-titulo")} lado="bottom">
        <PopoverTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              aria-label={contador > 0 ? `${t("notif-titulo")} (${contador})` : t("notif-titulo")}
              className="relative"
            />
          }
        >
          <IconoCampana weight={abierto ? "fill" : undefined} className="size-[18px]" />
          {contador > 0 ? (
            <span
              aria-hidden
              className="bg-primary text-etiqueta text-primary-foreground ring-background pointer-events-none absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full px-1 leading-none font-semibold tabular-nums ring-2"
            >
              {contador > 99 ? "99+" : contador}
            </span>
          ) : null}
        </PopoverTrigger>
      </Tooltip>

      <PopoverContent align="end" className="flex w-[min(24rem,calc(100vw-1.5rem))] flex-col p-0">
        {/* Cabecera */}
        <div className="flex items-center justify-between gap-2 px-4 pt-3.5 pb-1">
          <div className="min-w-0">
            <h2 className="text-titulo text-foreground">{t("notif-titulo")}</h2>
            <p className="text-etiqueta text-texto-terciario tabular-nums">
              {totalNoLeidas > 0 ? t("notif-sin-leer", { n: totalNoLeidas }) : t("notif-sin-notificaciones")}
            </p>
          </div>
          <div className="flex shrink-0 items-center">
            <Tooltip texto={silenciado ? t("notif-activar-sonido") : t("notif-silenciar")}>
              <Button
                variant="ghost"
                size="icon"
                aria-label={silenciado ? t("notif-activar-sonido") : t("notif-silenciar")}
                aria-pressed={silenciado}
                onClick={() => setSilenciado((v) => !v)}
              >
                {silenciado ? <BellSlashIcon className="size-4" /> : <BellIcon className="size-4" />}
              </Button>
            </Tooltip>
            {totalNoLeidas > 0 ? (
              <Tooltip texto={t("notif-marcar-todas")}>
                <Button variant="ghost" size="icon" aria-label={t("notif-marcar-todas")} onClick={marcarTodas}>
                  <ChecksIcon weight={PESO_ICONO_FUNCIONAL} className="size-4" />
                </Button>
              </Tooltip>
            ) : null}
            <Tooltip texto={t("settings")}>
              <Link
                href="/settings"
                aria-label={t("settings")}
                onClick={() => setAbierto(false)}
                className="size-control text-foreground ease-salida hover:bg-muted focus-visible:ring-ring/50 inline-flex items-center justify-center rounded-full transition-colors duration-(--duracion-rapida) outline-none focus-visible:ring-2"
              >
                <GearSixIcon className="size-4" />
              </Link>
            </Tooltip>
          </div>
        </div>

        {/* Pestañas + lista */}
        <Tabs items={pestanas} valor={pestana} onCambio={setPestana} estilo="segmentado" className="px-2 pt-2">
          {(id) => (
            <ListaNotificaciones
              items={todos.filter(FILTROS[id])}
              cargando={cargando && !cargadas}
              error={error && !cargadas}
              onReintentar={cargarItems}
              onMarcarLeida={marcarLeida}
              onDescartarOpinion={descartarOpinion}
              textoVacio={t("notif-sin-notificaciones")}
              textoError={t("comun-error-cargar")}
            />
          )}
        </Tabs>

        {/* Pie */}
        <div className="px-4 pt-1 pb-3.5">
          <Link href="/notificaciones" onClick={() => setAbierto(false)} className={cn(BOTON_SECUNDARIO, "w-full")}>
            {t("notif-ver-todas")}
          </Link>
        </div>
      </PopoverContent>
    </Popover>
  );
}

function ListaNotificaciones({
  items,
  cargando,
  error,
  onReintentar,
  onMarcarLeida,
  onDescartarOpinion,
  textoVacio,
  textoError,
}: {
  items: ItemNotif[];
  cargando: boolean;
  error: boolean;
  onReintentar: () => void;
  onMarcarLeida: (id: number) => void;
  onDescartarOpinion: () => void;
  textoVacio: string;
  textoError: string;
}) {
  const t = useTranslations("dashboard");

  if (cargando) {
    return (
      <div className="flex flex-col gap-1 px-2 pb-2" aria-busy="true">
        {[0, 1, 2].map((i) => (
          <div key={i} className="flex items-start gap-3 px-2 py-2.5">
            <Skeleton className="size-4 rounded-full" />
            <div className="flex flex-1 flex-col gap-1.5">
              <Skeleton className="h-3.5 w-2/3" />
              <Skeleton className="h-3 w-full" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (error && items.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 px-4 pt-2 pb-6 text-center">
        <p className="text-control text-destructive-texto">{textoError}</p>
        <Button variant="outline" size="sm" onClick={onReintentar}>
          {t("comun-reintentar")}
        </Button>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="text-muted-foreground flex flex-col items-center gap-2 px-4 pt-4 pb-8 text-center">
        <BellIcon className="text-texto-terciario size-6" />
        <p className="text-control">{textoVacio}</p>
      </div>
    );
  }

  return (
    <ul className="flex max-h-80 flex-col gap-0.5 overflow-y-auto px-0 pb-2">
      {items.map((n) => {
        const tipo = tipoNotificacion(n.tipo);
        const Icono = tipo.icono;
        const etiquetaAccion = n._virtual ? t("notif-descartar") : t("notif-marcar-leida");
        return (
          <li key={n.id} className="group relative">
            <button
              type="button"
              onClick={() => {
                if (n._virtual) {
                  onDescartarOpinion();
                  window.location.href = "/settings#opinion";
                  return;
                }
                if (!n.leida) onMarcarLeida(n.id);
              }}
              className={cn(
                "ease-salida hover:bg-secondary focus-visible:ring-ring/50 flex w-full items-start gap-3 rounded-lg px-2.5 py-2.5 pr-10 text-left transition-colors duration-(--duracion-rapida) outline-none focus-visible:ring-2",
                !n.leida && "bg-accent/40",
              )}
            >
              <Icono className={cn("mt-0.5 size-4 shrink-0", tipo.color)} />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span
                  className={cn("text-control text-foreground leading-snug", n.leida ? "font-medium" : "font-semibold")}
                >
                  {n.titulo}
                </span>
                <span className="text-cuerpo text-muted-foreground line-clamp-2 leading-snug">{n.cuerpo}</span>
                <span className="text-etiqueta text-texto-terciario">
                  {t(tipo.etiqueta)} · {n._virtual ? t("notif-hace-ahora") : haceCuanto(n.created_at, t)}
                </span>
              </span>
              {!n.leida ? <span aria-hidden className="bg-primary mt-1.5 size-1.5 shrink-0 rounded-full" /> : null}
            </button>
            {!n.leida ? (
              <Tooltip texto={etiquetaAccion} lado="left">
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={etiquetaAccion}
                  onClick={(e) => {
                    e.stopPropagation();
                    if (n._virtual) onDescartarOpinion();
                    else onMarcarLeida(n.id);
                  }}
                  className="absolute top-1.5 right-1 opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                >
                  <XIcon weight={PESO_ICONO_FUNCIONAL} className="size-3.5" />
                </Button>
              </Tooltip>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}
