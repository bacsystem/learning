"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";

import { DialogContent } from "@/components/app/dialogo";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { Campo } from "@/components/formularios/campo";
import { Combobox } from "@/components/formularios/combobox";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Chip } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA, ROTULO, TARJETA } from "@/lib/estilos";
import { type IconoComponente, PESO_ICONO } from "@/lib/iconos";
import { inscripcionesApi, type Proceso } from "@/lib/inscripciones/api";
import { type ConfiguracionExamen, simulacrosApi } from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import {
  ArchiveIcon,
  ArrowCounterClockwiseIcon,
  ClockIcon,
  EyeSlashIcon,
  FileTextIcon,
  InfoIcon,
  LockIcon,
  MedalIcon,
  PlayIcon,
  PlayPauseIcon,
  XIcon,
} from "@phosphor-icons/react";

function formatMinutes(secs: number) {
  const m = Math.round(secs / 60);
  return m >= 60 ? `${Math.floor(m / 60)}h ${m % 60}min` : `${m} min`;
}

const CONDITIONS: { icon: IconoComponente; titleKey: string; descKey: string; color: string }[] = [
  {
    icon: LockIcon,
    titleKey: "simulacros-condicion-tiempo-titulo",
    descKey: "simulacros-condicion-tiempo-desc",
    color: "text-warning-foreground",
  },
  {
    icon: EyeSlashIcon,
    titleKey: "simulacros-condicion-feedback-titulo",
    descKey: "simulacros-condicion-feedback-desc",
    color: "text-acento-2-texto dark:text-acento-2",
  },
  {
    icon: ArrowCounterClockwiseIcon,
    titleKey: "simulacros-condicion-cambio-titulo",
    descKey: "simulacros-condicion-cambio-desc",
    color: "text-success-foreground",
  },
  {
    icon: ArchiveIcon,
    titleKey: "simulacros-condicion-historial-titulo",
    descKey: "simulacros-condicion-historial-desc",
    color: "text-muted-foreground",
  },
];

/** Fila del resumen: icono en el color del dato, etiqueta y valor grande. */
function DatoResumen({
  icon: Icon,
  color,
  etiqueta,
  valor,
}: {
  icon: IconoComponente;
  color: string;
  etiqueta: string;
  valor: string | number;
}) {
  return (
    <div className="flex items-center gap-3 py-3">
      <Icon aria-hidden weight={PESO_ICONO} className={cn("size-5 shrink-0", color)} />
      <div className="flex min-w-0 flex-col">
        <span className="text-etiqueta text-muted-foreground">{etiqueta}</span>
        <span className="text-metrica text-foreground tabular-nums">{valor}</span>
      </div>
    </div>
  );
}

export default function NuevoSimulacroPage() {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const router = useRouter();

  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [procesoSeleccionado, setProcesoSeleccionado] = useState<Proceso | null>(null);
  const [configuracion, setConfiguracion] = useState<ConfiguracionExamen | null | undefined>(undefined);
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [procesosCargados, setProcesosCargados] = useState(false);
  // H32: a failed load is not "no approved inscripción" (same as prácticas, H150).
  const [errorProcesos, setErrorProcesos] = useState(false);
  const [loading, setLoading] = useState(false);
  const [conflictDialog, setConflictDialog] = useState<{ open: boolean; simulacroId: number | null }>({
    open: false,
    simulacroId: null,
  });

  useEffect(() => {
    if (!token) return;
    // H28: simulacros require an APPROVED inscripción in the proceso (same filter as prácticas).
    Promise.all([inscripcionesApi.listProcesos(token), inscripcionesApi.misInscripciones(token)])
      .then(([lista, inscripciones]) => {
        const aprobados = new Set(inscripciones.filter((i) => i.estado === "aprobado").map((i) => i.proceso_id));
        setProcesos(lista.filter((p) => aprobados.has(p.id)));
        setErrorProcesos(false);
      })
      .catch(() => setErrorProcesos(true))
      .finally(() => setProcesosCargados(true));
  }, [token]);

  useEffect(() => {
    if (!token || !procesoSeleccionado) {
      setConfiguracion(undefined);
      return;
    }
    setLoadingConfig(true);
    simulacrosApi
      .getConfiguracion(token, procesoSeleccionado.id)
      .then((res) => setConfiguracion(res.configuracion))
      .catch(() => setConfiguracion(null))
      .finally(() => setLoadingConfig(false));
  }, [token, procesoSeleccionado]);

  const handleSubmit = useCallback(async () => {
    if (!token || !procesoSeleccionado) return;
    setError(null);
    setLoading(true);
    try {
      const simulacro = await simulacrosApi.createSimulacro(token, {
        proceso_id: procesoSeleccionado.id,
      });
      router.push(`/simulacros/${simulacro.id}`);
    } catch (e: any) {
      if (e.status === 409 && e.simulacro_id) {
        setConflictDialog({ open: true, simulacroId: e.simulacro_id });
      } else if (e.status === 403) {
        setError(t("simulacros-error-sin-acceso"));
      } else if (e.status === 422) {
        setError(t("simulacros-error-preguntas"));
      } else if (e.status === 400) {
        // H33: always the translated key (the server message is Spanish-only).
        // 400 is ErrNoConfig (no active configuración) or a bad payload.
        setError(t("simulacros-error-configuracion"));
      } else {
        setError(t("simulacros-error-crear"));
      }
    } finally {
      setLoading(false);
    }
  }, [token, procesoSeleccionado, router, t]);

  const itemsProcesos = useMemo(
    () =>
      procesos.map((p) => ({
        value: String(p.id),
        label: p.nombre,
        detalle: [p.grado, p.anio].filter(Boolean).join(" · "),
      })),
    [procesos],
  );

  const canStart = !!procesoSeleccionado && !!configuracion && !loadingConfig;
  const closeConflict = () => setConflictDialog({ open: false, simulacroId: null });

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error && (
        <Alerta
          tono="error"
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setError(null)}>
              <XIcon weight="bold" />
            </Button>
          }
        >
          {error}
        </Alerta>
      )}

      {procesosCargados && errorProcesos && <Alerta tono="error">{t("simulacros-error-cargar-procesos")}</Alerta>}

      {procesosCargados && !errorProcesos && procesos.length === 0 && (
        <Alerta tono="info">{t("simulacros-sin-inscripcion-aprobada")}</Alerta>
      )}

      <div className="grid grid-cols-1 items-start gap-3 md:grid-cols-3">
        {/* ── Columna izquierda ── */}
        <div className="flex min-w-0 flex-col gap-3 md:col-span-2">
          {/* Selección del proceso */}
          <section className={TARJETA}>
            <CabeceraSeccion
              icon={FileTextIcon}
              titulo={t("simulacros-seleccionar-proceso")}
              subtitulo={t("simulacros-nuevo-subtitulo")}
            />
            <div className="flex flex-col gap-3 px-4 pt-1 pb-4">
              <Campo id="tour-simulacro-proceso" etiqueta={t("menu-procesos")}>
                <Combobox
                  vacio={t("comun-sin-resultados")}
                  id="tour-simulacro-proceso"
                  items={itemsProcesos}
                  valor={procesoSeleccionado ? String(procesoSeleccionado.id) : null}
                  onCambio={(v) => setProcesoSeleccionado(procesos.find((p) => String(p.id) === v) ?? null)}
                  placeholder={t("simulacros-seleccionar-proceso-placeholder")}
                />
              </Campo>

              {/* Datos del proceso */}
              {procesoSeleccionado && (
                <div className="flex flex-col gap-1.5">
                  <div className="flex flex-wrap gap-1.5">
                    <Chip className="bg-accent text-accent-foreground">
                      {procesoSeleccionado.tipo.replace("_", " ")}
                    </Chip>
                    {procesoSeleccionado.grado && <Chip>{procesoSeleccionado.grado}</Chip>}
                    <Chip>{String(procesoSeleccionado.anio)}</Chip>
                  </div>
                  {procesoSeleccionado.descripcion && (
                    <p className="text-etiqueta text-muted-foreground">{procesoSeleccionado.descripcion}</p>
                  )}
                </div>
              )}
            </div>
          </section>

          {/* Condiciones del examen */}
          <section className={TARJETA}>
            <CabeceraSeccion icon={InfoIcon} titulo={t("simulacros-condiciones-titulo")} />
            <ul className="divide-border/60 flex flex-col divide-y px-4 pb-2">
              {CONDITIONS.map(({ icon: Icon, titleKey, descKey, color }) => (
                <li key={titleKey} className="flex items-start gap-3 py-3">
                  <Icon aria-hidden weight={PESO_ICONO} className={cn("mt-0.5 size-5 shrink-0", color)} />
                  <div className="flex min-w-0 flex-col gap-0.5">
                    <span className="text-control text-foreground font-semibold">{t(titleKey)}</span>
                    <span className="text-etiqueta text-muted-foreground">{t(descKey)}</span>
                  </div>
                </li>
              ))}
            </ul>
          </section>
        </div>

        {/* ── Columna derecha ── */}
        <div className="flex min-w-0 flex-col gap-3 md:sticky md:top-20">
          {/* Resumen de la configuración */}
          <section className={TARJETA}>
            <CabeceraSeccion icon={PlayPauseIcon} titulo={t("simulacros-resumen")} />
            <div className="px-4 pb-4">
              {/* Sin proceso */}
              {!procesoSeleccionado && (
                <div className="flex flex-col items-center gap-2 py-6 text-center">
                  <InfoIcon aria-hidden weight={PESO_ICONO} className="text-texto-terciario size-6" />
                  <p className="text-control text-muted-foreground max-w-56">{t("simulacros-seleccionar-primero")}</p>
                </div>
              )}

              {/* Cargando */}
              {procesoSeleccionado && loadingConfig && (
                <div className="flex flex-col gap-4 py-3">
                  {[1, 2, 3].map((i) => (
                    <div key={i} className="flex items-center gap-3">
                      <Skeleton className="size-5 rounded-full" />
                      <div className="flex flex-1 flex-col gap-1.5">
                        <Skeleton className="h-2.5 w-1/2" />
                        <Skeleton className="h-5 w-1/3" />
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {/* Configuración cargada */}
              {procesoSeleccionado && !loadingConfig && configuracion && (
                <div className="divide-border/60 flex flex-col divide-y">
                  <DatoResumen
                    icon={FileTextIcon}
                    color="text-acento-2-texto dark:text-acento-2"
                    etiqueta={t("simulacros-total-preguntas")}
                    valor={configuracion.total_preguntas}
                  />
                  <DatoResumen
                    icon={ClockIcon}
                    color="text-warning-foreground"
                    etiqueta={t("simulacros-tiempo")}
                    valor={formatMinutes(configuracion.tiempo_total_seg)}
                  />
                  <DatoResumen
                    icon={MedalIcon}
                    color="text-success-foreground"
                    etiqueta={t("simulacros-nota-minima")}
                    valor={`${configuracion.nota_minima.toFixed(1)} / ${configuracion.nota_maxima.toFixed(1)}`}
                  />

                  {configuracion.asignaturas?.length > 0 && (
                    <div className="flex flex-col gap-2 pt-3">
                      <span className={ROTULO}>{t("simulacros-asignaturas")}</span>
                      <ul className="flex flex-col gap-1.5">
                        {configuracion.asignaturas.map((a) => (
                          <li key={a.asignatura_id} className="flex items-center justify-between gap-2">
                            <span className="text-control text-foreground min-w-0 flex-1 truncate">{a.nombre}</span>
                            <span className="text-etiqueta text-muted-foreground shrink-0 tabular-nums">
                              {a.numero_preguntas} / {a.preguntas_disponibles}
                            </span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              )}

              {/* Sin configuración */}
              {procesoSeleccionado && !loadingConfig && configuracion === null && (
                <Alerta tono="aviso">{t("simulacros-sin-configuracion")}</Alerta>
              )}
            </div>
          </section>

          {/* CTA */}
          <BotonAsync
            id="tour-simulacro-iniciar"
            pendiente={loading}
            textoPendiente={t("simulacros-iniciando")}
            icon={<PlayIcon weight="fill" className="size-4" />}
            onClick={handleSubmit}
            disabled={!canStart}
            className={cn(BOTON_PRIMARIO, "w-full")}
          >
            {t("simulacros-iniciar")}
          </BotonAsync>

          <Button variant="ghost" className="text-muted-foreground w-full" onClick={() => router.push("/simulacros")}>
            {t("simulacros-ver-historial")}
          </Button>
        </div>
      </div>

      {/* Simulacro en curso */}
      <Dialog open={conflictDialog.open} onOpenChange={(open) => !open && closeConflict()}>
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={PlayPauseIcon}
            tono="aviso"
            titulo={t("simulacros-en-curso-titulo")}
            descripcion={t("simulacros-en-curso-descripcion")}
          />
          <PieDialogo>
            <button type="button" className={BOTON_SECUNDARIO} onClick={closeConflict}>
              {t("simulacros-cancelar")}
            </button>
            <button
              type="button"
              className={BOTON_PRIMARIO}
              onClick={() => {
                if (conflictDialog.simulacroId) router.push(`/simulacros/${conflictDialog.simulacroId}`);
              }}
            >
              {t("simulacros-continuar")}
            </button>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
