"use client";

import { useTranslations } from "next-intl";
import { type ReactNode, useEffect, useState } from "react";

import { Campo } from "@/components/app/campo";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { AreaTexto, Entrada } from "@/components/formularios/campo";
import { Tabs } from "@/components/navegacion/tabs";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { FilaMetricas, Metrica, type Tono } from "@/components/patrones/metricas";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Avatar } from "@/components/ui/avatar";
import { type HealthStatus, systemApi, tourApi } from "@/lib/auth/auth-api";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { formatearFechaHora } from "@/lib/formato";
import { cargarTodo } from "@/lib/paginar-todo";
import { type HistorialItem, practicasApi } from "@/lib/practices/api";
import { type HistorialSimulacroItem, simulacrosApi } from "@/lib/simulations/api";
import { fetchMiTestimonio, type MiTestimonio, submitTestimonio } from "@/lib/testimonios/api";
import { cn } from "@/lib/utils";
import {
  ChartLineIcon,
  CheckSquareIcon,
  ClockCounterClockwiseIcon,
  GraduationCapIcon,
  HeartbeatIcon,
  IdentificationCardIcon,
  PaperPlaneTiltIcon,
  PencilSimpleIcon,
  PencilSimpleLineIcon,
  PlayIcon,
  SignpostIcon,
  StarIcon,
  TimerIcon,
} from "@phosphor-icons/react";

type Pestana = "perfil" | "actividad" | "opinion" | "plataforma";

const TONO_TESTIMONIO: Record<MiTestimonio["estado"], Tono> = {
  aprobado: "ok",
  pendiente: "aviso",
  rechazado: "error",
};

const ROL_CLAVE: Record<string, string> = {
  usuario: "admin-usuarios-rol-usuario",
  admin: "admin-usuarios-rol-admin",
};

const ESTADO_CLAVE: Record<string, string> = {
  pendiente: "admin-usuarios-estado-pendiente",
  activo: "admin-usuarios-estado-activo",
  rechazado: "admin-usuarios-estado-rechazado",
  inactivo: "admin-usuarios-estado-inactivo",
};

/** Dato de solo lectura (§10 «Dato de solo lectura»): etiqueta + caja `bg-muted`. */
function DatoLectura({ id, etiqueta, valor }: { id: string; etiqueta: string; valor: string }) {
  return (
    <Campo id={id} etiqueta={etiqueta}>
      <Entrada id={id} value={valor || "—"} readOnly tabIndex={-1} className="bg-muted cursor-default focus:ring-0" />
    </Campo>
  );
}

function Seccion({ children, className, id }: { children: ReactNode; className?: string; id?: string }) {
  return (
    <section id={id} className={cn(TARJETA, "flex min-w-0 flex-col", className)}>
      {children}
    </section>
  );
}

/** Estrellas de solo lectura. */
function Estrellas({ valor }: { valor: number }) {
  const t = useTranslations("dashboard");
  return (
    <span className="inline-flex items-center gap-0.5" aria-label={t("perfil-estrellas", { n: valor })}>
      {[1, 2, 3, 4, 5].map((s) => (
        <StarIcon
          key={s}
          aria-hidden
          weight={s <= valor ? "fill" : "regular"}
          className={cn("size-4", s <= valor ? "text-warning-foreground" : "text-texto-terciario/50")}
        />
      ))}
    </span>
  );
}

export default function Settings() {
  const { user, token, updateUser } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const [simHistorial, setSimHistorial] = useState<HistorialSimulacroItem[]>([]);
  const [pracHistorial, setPracHistorial] = useState<HistorialItem[]>([]);
  const [loading, setLoading] = useState(true);
  // El historial no se pudo cargar: las métricas muestran «—» y un aviso en vez de ceros que parecen datos reales.
  const [errorMetricas, setErrorMetricas] = useState(false);
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [healthLoading, setHealthLoading] = useState(true);
  const [tourResetting, setTourResetting] = useState(false);
  const [pestana, setPestana] = useState<Pestana>("perfil");

  const [miTestimonio, setMiTestimonio] = useState<MiTestimonio | null | undefined>(undefined);
  const [submitCal, setSubmitCal] = useState<number | null>(null);
  const [hoverCal, setHoverCal] = useState<number | null>(null);
  const [submitText, setSubmitText] = useState("");
  const [submitLoading, setSubmitLoading] = useState(false);
  const [submitSuccess, setSubmitSuccess] = useState(false);
  const [submitError, setSubmitError] = useState("");

  // `/settings#opinion` (enlace de la landing y recordatorio de la campana) abre la pestaña de opinión, también
  // cuando ya se está en /settings: ahí el enlace solo cambia el hash, sin recargar la página.
  useEffect(() => {
    const abrirSiOpinion = () => {
      if (window.location.hash === "#opinion") setPestana("opinion");
    };
    abrirSiOpinion();
    window.addEventListener("hashchange", abrirSiOpinion);
    return () => window.removeEventListener("hashchange", abrirSiOpinion);
  }, []);

  const handleRestartTour = async () => {
    if (!token) return;
    setTourResetting(true);
    try {
      await tourApi.reset(token);
      updateUser({ tour_completado: false });
    } catch (e: unknown) {
      toast.error((e as { message?: string }).message ?? t("perfil-tour-error"));
    } finally {
      setTourResetting(false);
    }
  };

  useEffect(() => {
    if (!token) return;
    Promise.all([
      cargarTodo((q) => simulacrosApi.getHistorial(token, q)),
      cargarTodo((q) => practicasApi.listHistorial(token, q)),
    ])
      .then(([sim, prac]) => {
        setSimHistorial(sim);
        setPracHistorial(prac);
        setErrorMetricas(false);
      })
      .catch(() => setErrorMetricas(true))
      .finally(() => setLoading(false));
  }, [token]);

  useEffect(() => {
    systemApi
      .getHealth()
      .then(setHealth)
      .catch(() => setHealth({ status: "unhealthy" }))
      .finally(() => setHealthLoading(false));
  }, []);

  useEffect(() => {
    if (!token) return;
    fetchMiTestimonio(token)
      .then(setMiTestimonio)
      .catch(() => setMiTestimonio(null));
  }, [token]);

  const handleSubmitOpinion = async () => {
    if (!token || !submitCal || submitText.trim().length < 10) return;
    setSubmitLoading(true);
    setSubmitError("");
    try {
      await submitTestimonio(token, { contenido: submitText.trim(), calificacion: submitCal });
      setSubmitSuccess(true);
      fetchMiTestimonio(token)
        .then((t) => {
          if (t) setMiTestimonio(t);
        })
        .catch(() => {});
    } catch {
      setSubmitError(t("perfil-opinion-error"));
    } finally {
      setSubmitLoading(false);
    }
  };

  const openEdit = () => {
    if (miTestimonio) {
      setSubmitCal(miTestimonio.calificacion);
      setSubmitText(miTestimonio.contenido);
    } else {
      setSubmitCal(null);
      setSubmitText("");
    }
    setSubmitSuccess(false);
    setSubmitError("");
  };

  if (!user) return null;

  const fullName = [user.nombres, user.apellido_paterno, user.apellido_materno].filter(Boolean).join(" ");
  // Valores del backend → claves existentes de admin/usuarios; un valor desconocido se muestra tal cual.
  const rolTexto = Object.hasOwn(ROL_CLAVE, user.rol) ? t(ROL_CLAVE[user.rol]) : user.rol;
  const estadoTexto = Object.hasOwn(ESTADO_CLAVE, user.estado) ? t(ESTADO_CLAVE[user.estado]) : user.estado;

  // Simulacro stats
  const simCompletados = simHistorial.filter((s) => s.estado === "completado");
  const simAprobados = simCompletados.filter((s) => s.aprobado).length;
  const simNotaPromedio =
    simCompletados.length > 0
      ? (simCompletados.reduce((sum, s) => sum + s.nota_centesimal, 0) / simCompletados.length).toFixed(1)
      : "—";
  const simMejorNota =
    simCompletados.length > 0 ? Math.max(...simCompletados.map((s) => s.nota_centesimal)).toFixed(1) : "—";

  // Práctica stats
  const pracCompletadas = pracHistorial.filter((p) => p.estado === "completada");
  const pracPromedio =
    pracCompletadas.length > 0
      ? Math.round(pracCompletadas.reduce((sum, p) => sum + p.porcentaje, 0) / pracCompletadas.length)
      : null;

  const recentSims = simHistorial.slice(0, 5);
  const textoValido = submitText.trim().length >= 10;
  const calMostrada = hoverCal ?? submitCal;
  const healthOk = health?.status === "ok";

  const valorMetrica = (valor: ReactNode) =>
    loading ? <Skeleton className="my-1 h-5 w-16" /> : errorMetricas ? "—" : valor;
  const metricasListas = !loading && !errorMetricas;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {/* Cabecera del perfil */}
      <section
        className={cn(
          TARJETA,
          "flex flex-col items-center gap-4 px-5 py-5 text-center sm:flex-row sm:items-center sm:text-start",
        )}
      >
        <Avatar
          nombre={`${user.nombres} ${user.apellido_paterno}`}
          tamano="xl"
          tono="accent"
          className="text-titulo-pagina size-16"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <div className="min-w-0">
            <p className="text-titulo-pagina text-foreground">{fullName}</p>
            <p className="text-control text-muted-foreground">@{user.username}</p>
          </div>
          <div className="flex flex-wrap justify-center gap-1.5 sm:justify-start">
            <Chip className="bg-accent text-accent-foreground">{rolTexto}</Chip>
            <PillEstado tono={user.estado === "activo" ? "ok" : "neutro"}>{estadoTexto}</PillEstado>
            {user.grado && <Chip>{user.grado}</Chip>}
            {user.especialidad && <Chip>{user.especialidad}</Chip>}
          </div>
        </div>
        {user.institucion && (
          <div className="text-control text-muted-foreground flex min-w-0 items-center gap-2 sm:max-w-[40%] sm:self-start">
            <GraduationCapIcon aria-hidden className="size-4 shrink-0" />
            <span className="min-w-0 truncate">{user.institucion}</span>
          </div>
        )}
      </section>

      {/* Métricas */}
      {errorMetricas ? <Alerta tono="error">{t("metricas-error-cargar")}</Alerta> : null}
      <FilaMetricas>
        <Metrica
          etiqueta={t("perfil-simulacros-realizados")}
          icono={TimerIcon}
          tono="acento"
          ayuda={loading ? "…" : errorMetricas ? null : t("perfil-aprobados-sub", { count: simAprobados })}
        >
          {valorMetrica(simCompletados.length)}
        </Metrica>
        <Metrica
          etiqueta={t("perfil-nota-promedio")}
          icono={ChartLineIcon}
          tono="acento"
          sufijo={metricasListas && simNotaPromedio !== "—" ? "/ 100" : undefined}
          ayuda={metricasListas && simMejorNota !== "—" ? t("perfil-mejor-nota", { nota: simMejorNota }) : null}
        >
          {valorMetrica(simNotaPromedio)}
        </Metrica>
        <Metrica etiqueta={t("perfil-practicas-completadas")} icono={PencilSimpleIcon} tono="acento" ayuda={null}>
          {valorMetrica(pracCompletadas.length)}
        </Metrica>
        <Metrica
          etiqueta={t("perfil-promedio-practicas")}
          icono={CheckSquareIcon}
          tono="acento"
          visual={metricasListas && pracPromedio !== null ? { tipo: "barra", valor: pracPromedio } : undefined}
          ayuda={null}
        >
          {valorMetrica(pracPromedio !== null ? `${pracPromedio}%` : "—")}
        </Metrica>
      </FilaMetricas>

      <Tabs<Pestana>
        valor={pestana}
        onCambio={setPestana}
        items={[
          { id: "perfil", etiqueta: t("perfil-pestana-datos") },
          { id: "actividad", etiqueta: t("perfil-pestana-actividad") },
          { id: "opinion", etiqueta: t("perfil-pestana-opinion") },
          { id: "plataforma", etiqueta: t("perfil-pestana-plataforma") },
        ]}
      >
        {(id) => {
          if (id === "perfil")
            return (
              <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                <Seccion>
                  <CabeceraSeccion icon={IdentificationCardIcon} titulo={t("perfil-info-personal")} />
                  <div className="grid gap-4 px-4 pt-2 pb-4 sm:grid-cols-2">
                    <DatoLectura id="perfil-nombres" etiqueta={t("perfil-nombres")} valor={user.nombres} />
                    <DatoLectura id="perfil-usuario" etiqueta={t("perfil-usuario")} valor={user.username} />
                    <DatoLectura
                      id="perfil-apellido-paterno"
                      etiqueta={t("perfil-apellido-paterno")}
                      valor={user.apellido_paterno}
                    />
                    <DatoLectura
                      id="perfil-apellido-materno"
                      etiqueta={t("perfil-apellido-materno")}
                      valor={user.apellido_materno}
                    />
                    <DatoLectura id="perfil-estado" etiqueta={t("perfil-estado")} valor={estadoTexto} />
                  </div>
                </Seccion>
                <Seccion>
                  <CabeceraSeccion icon={GraduationCapIcon} titulo={t("perfil-info-academica")} />
                  <div className="grid gap-4 px-4 pt-2 pb-4 sm:grid-cols-2">
                    <DatoLectura id="perfil-institucion" etiqueta={t("perfil-institucion")} valor={user.institucion} />
                    <DatoLectura id="perfil-rol" etiqueta={t("perfil-rol")} valor={rolTexto} />
                    <DatoLectura id="perfil-grado" etiqueta={t("perfil-grado")} valor={user.grado} />
                    <DatoLectura
                      id="perfil-especialidad"
                      etiqueta={t("perfil-especialidad")}
                      valor={user.especialidad}
                    />
                  </div>
                </Seccion>
              </div>
            );

          if (id === "actividad")
            return (
              <Seccion>
                <CabeceraSeccion icon={ClockCounterClockwiseIcon} titulo={t("perfil-ultimos-simulacros")} />
                {loading ? (
                  <div className="flex flex-col gap-[3px] px-2 pb-2" aria-busy="true">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <div
                        key={i}
                        className="bg-secondary flex items-center justify-between gap-4 rounded-lg px-3 py-2.5"
                      >
                        <div className="flex flex-col gap-1.5">
                          <Skeleton className="bg-card h-3 w-48" />
                          <Skeleton className="bg-card h-2.5 w-24" />
                        </div>
                        <Skeleton className="bg-card h-5 w-24 rounded-full" />
                      </div>
                    ))}
                  </div>
                ) : errorMetricas ? (
                  <div className="px-4 pb-4">
                    <Alerta tono="error">{t("metricas-error-cargar")}</Alerta>
                  </div>
                ) : recentSims.length === 0 ? (
                  <div className="px-4 pb-4">
                    <EstadoVacio
                      icon={TimerIcon}
                      titulo={t("perfil-sin-simulacros")}
                      className="border-0 bg-transparent py-10"
                    />
                  </div>
                ) : (
                  <ul className="flex flex-col gap-[3px] px-2 pb-2">
                    {recentSims.map((sim) => (
                      <li
                        key={sim.id}
                        className="bg-secondary flex flex-wrap items-center justify-between gap-x-4 gap-y-2 rounded-lg px-3 py-2.5"
                      >
                        <div className="min-w-0">
                          <p className="text-control text-foreground truncate font-medium">
                            {t("perfil-simulacro-numero", { n: sim.numero })} — {sim.proceso}
                          </p>
                          <p className="text-etiqueta text-texto-terciario tabular-nums">
                            {formatearFechaHora(sim.fecha)}
                          </p>
                        </div>
                        <div className="flex shrink-0 items-center gap-3">
                          <p className="text-control text-foreground font-semibold tabular-nums">
                            {sim.nota_centesimal.toFixed(1)}
                            <span className="text-texto-terciario font-normal"> / 100</span>
                          </p>
                          <PillEstado tono={sim.aprobado ? "ok" : "error"}>
                            {sim.aprobado ? t("simulacros-aprobado-si") : t("simulacros-aprobado-no")}
                          </PillEstado>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </Seccion>
            );

          if (id === "opinion")
            return (
              <Seccion id="opinion">
                <CabeceraSeccion
                  icon={StarIcon}
                  titulo={t("perfil-opinion-titulo")}
                  subtitulo={t("perfil-opinion-subtitulo")}
                />
                <div className="flex flex-col gap-4 px-4 pt-2 pb-4">
                  {miTestimonio === undefined ? (
                    <div className="flex flex-col gap-2" aria-busy="true">
                      <Skeleton className="h-4 w-28" />
                      <Skeleton className="h-3 w-3/4" />
                    </div>
                  ) : submitSuccess ? (
                    <Alerta tono="ok">{t("perfil-opinion-enviada")}</Alerta>
                  ) : (
                    <>
                      {miTestimonio && (
                        <div className="bg-secondary flex flex-col gap-2 rounded-lg px-3.5 py-3">
                          <div className="flex flex-wrap items-center gap-3">
                            <Estrellas valor={miTestimonio.calificacion} />
                            <PillEstado tono={TONO_TESTIMONIO[miTestimonio.estado] ?? "neutro"} className="capitalize">
                              {t(`perfil-testimonio-estado-${miTestimonio.estado}`)}
                            </PillEstado>
                          </div>
                          <p className="text-cuerpo text-muted-foreground leading-relaxed">
                            &ldquo;{miTestimonio.contenido}&rdquo;
                          </p>
                        </div>
                      )}

                      <div className="flex flex-col gap-4">
                        <div className="grid gap-1.5">
                          <span id="opinion-calificacion" className="text-etiqueta text-foreground font-medium">
                            {t("perfil-opinion-calificacion")}
                          </span>
                          <div
                            role="radiogroup"
                            aria-labelledby="opinion-calificacion"
                            className="flex items-center gap-1"
                            onMouseLeave={() => setHoverCal(null)}
                          >
                            {[1, 2, 3, 4, 5].map((star) => {
                              const activa = calMostrada !== null && star <= calMostrada;
                              return (
                                <button
                                  key={star}
                                  type="button"
                                  role="radio"
                                  aria-checked={submitCal === star}
                                  aria-label={t("perfil-estrellas", { n: star })}
                                  onClick={() => setSubmitCal(star)}
                                  onMouseEnter={() => setHoverCal(star)}
                                  onFocus={() => setHoverCal(star)}
                                  onBlur={() => setHoverCal(null)}
                                  className="size-control ease-salida hover:bg-secondary focus-visible:ring-ring/50 grid place-items-center rounded-full transition-colors duration-(--duracion-rapida) outline-none focus-visible:ring-2"
                                >
                                  <StarIcon
                                    aria-hidden
                                    weight={activa ? "fill" : "regular"}
                                    className={cn(
                                      "size-6",
                                      activa ? "text-warning-foreground" : "text-texto-terciario/60",
                                    )}
                                  />
                                </button>
                              );
                            })}
                          </div>
                        </div>

                        <Campo
                          id="opinion-comentario"
                          etiqueta={t("perfil-opinion-comentario")}
                          ayuda={
                            <span
                              className={cn(
                                "tabular-nums",
                                submitText.length > 0 && !textoValido && "text-warning-foreground",
                              )}
                            >
                              {t("perfil-opinion-contador", { n: submitText.length, max: 300, min: 10 })}
                            </span>
                          }
                        >
                          <AreaTexto
                            id="opinion-comentario"
                            rows={3}
                            maxLength={300}
                            value={submitText}
                            onChange={(e) => setSubmitText(e.target.value.slice(0, 300))}
                            aria-describedby="opinion-comentario-ayuda"
                            className="max-h-40"
                          />
                        </Campo>

                        {submitError && <Alerta tono="error">{submitError}</Alerta>}

                        <div className="flex flex-wrap items-center gap-2">
                          {miTestimonio && !submitCal && (
                            <button type="button" className={BOTON_SECUNDARIO} onClick={openEdit}>
                              <PencilSimpleLineIcon className="size-4" />
                              {t("perfil-opinion-editar")}
                            </button>
                          )}
                          <BotonAsync
                            pendiente={submitLoading}
                            textoPendiente={t("perfil-opinion-enviando")}
                            disabled={!submitCal || !textoValido}
                            icon={<PaperPlaneTiltIcon className="size-4" />}
                            className={BOTON_PRIMARIO}
                            onClick={handleSubmitOpinion}
                          >
                            {miTestimonio ? t("perfil-opinion-actualizar") : t("perfil-opinion-enviar")}
                          </BotonAsync>
                        </div>
                      </div>
                    </>
                  )}
                </div>
              </Seccion>
            );

          return (
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
              <Seccion>
                <CabeceraSeccion
                  icon={SignpostIcon}
                  titulo={t("perfil-tour-titulo")}
                  subtitulo={t("perfil-tour-subtitulo")}
                />
                <div className="mt-auto px-4 pt-2 pb-4">
                  <BotonAsync
                    pendiente={tourResetting}
                    textoPendiente={t("perfil-tour-reiniciando")}
                    icon={<PlayIcon className="size-4" />}
                    className={BOTON_SECUNDARIO}
                    onClick={handleRestartTour}
                  >
                    {t("perfil-tour-repetir")}
                  </BotonAsync>
                </div>
              </Seccion>
              <Seccion>
                <CabeceraSeccion icon={HeartbeatIcon} titulo={t("perfil-sistema-titulo")} />
                <div className="mt-auto flex flex-wrap items-center gap-3 px-4 pt-2 pb-4">
                  {healthLoading ? (
                    <Skeleton className="h-6 w-48 rounded-full" />
                  ) : (
                    <>
                      <span className="text-control text-foreground font-medium">{t("perfil-sistema-api")}</span>
                      <PillEstado tono={healthOk ? "ok" : "error"}>
                        {healthOk ? t("perfil-sistema-conectado") : t("perfil-sistema-sin-conexion")}
                      </PillEstado>
                      {health?.version && <Chip className="font-mono">v{health.version}</Chip>}
                    </>
                  )}
                </div>
              </Seccion>
            </div>
          );
        }}
      </Tabs>
    </div>
  );
}
