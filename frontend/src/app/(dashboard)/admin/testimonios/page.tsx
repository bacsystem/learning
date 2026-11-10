"use client";
import { useTranslations } from "next-intl";
import { useEffect, useMemo, useRef, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { Tabs } from "@/components/navegacion/tabs";
import { BotonAsync } from "@/components/patrones/boton-async";
import type { Tono } from "@/components/patrones/metricas";
import { PieTabla } from "@/components/patrones/pie-tabla";
import { PillEstado } from "@/components/patrones/pill-estado";
import { Avatar } from "@/components/ui/avatar";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { formatearFechaHora } from "@/lib/formato";
import { fetchTestimoniosAdmin, type TestimonioAdmin, updateTestimonioEstado } from "@/lib/testimonios/api";
import { cn } from "@/lib/utils";
import { ChatCircleTextIcon, CheckCircleIcon, StarIcon, XCircleIcon } from "@phosphor-icons/react";

type TabValue = "todos" | "pendiente" | "aprobado" | "rechazado";

const TABS: { id: TabValue; clave: string }[] = [
  { id: "pendiente", clave: "admin-testimonios-tab-pendientes" },
  { id: "aprobado", clave: "admin-testimonios-tab-aprobados" },
  { id: "rechazado", clave: "admin-testimonios-tab-rechazados" },
  { id: "todos", clave: "simulacros-todos" },
];

const POR_PAGINA_INICIAL = 20;

const ESTADO_TONO: Record<string, Tono> = {
  pendiente: "aviso",
  aprobado: "ok",
  rechazado: "error",
};

const ESTADO_CLAVE: Record<string, string> = {
  pendiente: "admin-testimonios-estado-pendiente",
  aprobado: "admin-testimonios-estado-aprobado",
  rechazado: "admin-testimonios-estado-rechazado",
};

function Estrellas({ valor }: { valor: number }) {
  const t = useTranslations("dashboard");
  return (
    <div className="flex items-center gap-0.5" aria-label={t("admin-testimonios-estrellas", { n: valor })} role="img">
      {[1, 2, 3, 4, 5].map((s) => (
        <StarIcon
          key={s}
          aria-hidden
          weight="fill"
          className={cn("size-3.5", s <= valor ? "text-warning-foreground" : "text-muted-foreground/25")}
        />
      ))}
    </div>
  );
}

function TarjetaCargando() {
  return (
    <div className={cn(TARJETA, "flex flex-col gap-3 p-4")}>
      <div className="flex items-center gap-3">
        <Skeleton className="size-8 rounded-full" />
        <Skeleton className="h-4 w-48" />
      </div>
      <Skeleton className="h-3 w-24" />
      <Skeleton className="h-3 w-full" />
      <Skeleton className="h-3 w-2/3" />
    </div>
  );
}

export default function TestimoniosAdminPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const [tab, setTab] = useState<TabValue>("pendiente");
  const [pagina, setPagina] = useState(1);
  const [porPagina, setPorPagina] = useState(POR_PAGINA_INICIAL);
  // Se incrementa para volver a pedir la página actual (tras aprobar o rechazar).
  const [recarga, setRecarga] = useState(0);
  const [rows, setRows] = useState<TestimonioAdmin[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [actingIds, setActingIds] = useState<Set<number>>(new Set());
  // Al cambiar de pestaña o página rápido, solo cuenta la última petición.
  const loadReq = useRef(0);

  const tabs = useMemo(() => TABS.map(({ id, clave }) => ({ id, etiqueta: t(clave) })), [t]);

  useEffect(() => {
    if (!token) return;
    const reqId = ++loadReq.current;
    setLoadError("");
    setLoading(true);
    fetchTestimoniosAdmin(token, tab === "todos" ? "" : tab, pagina, porPagina)
      .then((resp) => {
        if (reqId !== loadReq.current) return;
        // Si se vació la última página (p. ej. al moderar su único testimonio), retrocede una.
        if (resp.data.length === 0 && pagina > 1) {
          setPagina(Math.max(1, Math.ceil(resp.total / porPagina)));
          return;
        }
        setRows(resp.data);
        setTotal(resp.total);
      })
      .catch(() => {
        if (reqId !== loadReq.current) return;
        setRows([]);
        setLoadError(t("admin-testimonios-error-cargar"));
      })
      .finally(() => {
        if (reqId === loadReq.current) setLoading(false);
      });
  }, [tab, token, pagina, porPagina, recarga, t]);

  const ultimaPagina = Math.max(1, Math.ceil(total / porPagina));

  async function handleEstado(id: number, estado: "aprobado" | "rechazado") {
    if (!token || actingIds.has(id)) return;
    setActingIds((prev) => new Set(prev).add(id));
    try {
      await updateTestimonioEstado(token, id, estado);
      // Vuelve a pedir la página: así el total y las tarjetas siguen cuadrando con el servidor.
      setRecarga((n) => n + 1);
      toast.ok(estado === "aprobado" ? t("admin-testimonios-aprobado") : t("admin-testimonios-rechazado"));
    } catch {
      toast.error(t("admin-testimonios-error-estado"));
    } finally {
      setActingIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }
  }

  return (
    <div className={CONTENEDOR_PAGINA}>
      <p className="text-cuerpo text-muted-foreground">{t("admin-testimonios-subtitulo")}</p>

      <Tabs
        items={tabs}
        valor={tab}
        onCambio={(v) => {
          setTab(v);
          setPagina(1);
          setRows([]);
        }}
        estilo="segmentado"
      >
        {() => (
          <div className="flex flex-col gap-3">
            {loadError ? <Alerta tono="error">{loadError}</Alerta> : null}

            {loading && rows.length === 0 ? (
              <>
                <TarjetaCargando />
                <TarjetaCargando />
                <TarjetaCargando />
              </>
            ) : rows.length === 0 ? (
              loadError ? null : (
                <EstadoVacio icon={ChatCircleTextIcon} titulo={t("admin-testimonios-vacio-titulo")}>
                  {t("admin-testimonios-vacio")}
                </EstadoVacio>
              )
            ) : (
              <>
                {rows.map((row) => {
                  const ocupado = actingIds.has(row.id);
                  return (
                    <article key={row.id} className={cn(TARJETA, "flex flex-wrap justify-between gap-4 p-4")}>
                      <div className="flex min-w-52 flex-1 gap-3">
                        <Avatar nombre={row.nombre_completo} tono="accent" />
                        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                            <h3 className="text-titulo text-foreground">{row.nombre_completo}</h3>
                            {row.grado ? (
                              <span className="text-etiqueta text-muted-foreground">{row.grado}</span>
                            ) : null}
                            <PillEstado tono={ESTADO_TONO[row.estado] ?? "neutro"}>
                              {ESTADO_CLAVE[row.estado] ? t(ESTADO_CLAVE[row.estado]) : row.estado}
                            </PillEstado>
                          </div>
                          <Estrellas valor={row.calificacion} />
                          <p className="text-cuerpo text-foreground leading-relaxed">
                            {t("admin-testimonios-cita", { contenido: row.contenido })}
                          </p>
                          <p className="text-etiqueta text-texto-terciario">
                            @{row.username} · <span className="tabular-nums">{formatearFechaHora(row.created_at)}</span>
                          </p>
                        </div>
                      </div>

                      {row.estado === "pendiente" ? (
                        <div className="flex items-start gap-2">
                          <BotonAsync
                            pendiente={ocupado}
                            className={BOTON_SECUNDARIO}
                            icon={<XCircleIcon className="text-destructive-texto size-4" />}
                            onClick={() => handleEstado(row.id, "rechazado")}
                            title={t("admin-testimonios-rechazar")}
                          >
                            {t("admin-testimonios-rechazar")}
                          </BotonAsync>
                          <BotonAsync
                            pendiente={ocupado}
                            className={BOTON_PRIMARIO}
                            icon={<CheckCircleIcon className="size-4" />}
                            onClick={() => handleEstado(row.id, "aprobado")}
                            title={t("admin-testimonios-aprobar")}
                          >
                            {t("admin-testimonios-aprobar")}
                          </BotonAsync>
                        </div>
                      ) : null}
                    </article>
                  );
                })}
                <PieTabla
                  desde={(pagina - 1) * porPagina + 1}
                  hasta={(pagina - 1) * porPagina + rows.length}
                  total={total}
                  unidad={t("admin-testimonios-unidad")}
                  porPagina={porPagina}
                  onPorPagina={(n) => {
                    setPorPagina(n);
                    setPagina(1);
                  }}
                  pagina={Math.min(pagina, ultimaPagina)}
                  ultimaPagina={ultimaPagina}
                  onPagina={setPagina}
                />
              </>
            )}
          </div>
        )}
      </Tabs>
    </div>
  );
}
