"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { Casilla, Interruptor } from "@/components/formularios/casilla";
import { Combobox } from "@/components/formularios/combobox";
import { GrupoOpciones } from "@/components/formularios/opciones";
import { StepperNumerico } from "@/components/formularios/stepper-numerico";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import { AYUDA_CAMPO, BOTON_PRIMARIO, CONTENEDOR_PAGINA, ETIQUETA_CAMPO, TARJETA } from "@/lib/estilos";
import { inscripcionesApi } from "@/lib/inscripciones/api";
import { MAX_PREGUNTAS_PRACTICA, practicasApi } from "@/lib/practices/api";
import { cn } from "@/lib/utils";
import {
  ClockIcon,
  GearSixIcon,
  ListNumbersIcon,
  PlayIcon,
  StackIcon,
  WarningIcon,
  XIcon,
} from "@phosphor-icons/react";

interface Proceso {
  id: number;
  nombre: string;
}

interface AsignaturaSelection {
  asignaturaId: number;
  nombre: string;
  enabled: boolean;
  numPreguntas: number;
  maxPreguntas: number;
}

export default function NuevaPracticaPage() {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const router = useRouter();

  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [procesoSeleccionado, setProcesoSeleccionado] = useState<Proceso | null>(null);
  const [asignaturas, setAsignaturas] = useState<AsignaturaSelection[]>([]);
  const [modo, setModo] = useState<"inmediato" | "al_final">("al_final");
  const [usarTimer, setUsarTimer] = useState(false);
  const [tiempoMin, setTiempoMin] = useState(30);
  const [error, setError] = useState<string | null>(null);
  const [procesosCargados, setProcesosCargados] = useState(false);
  // H150: a failed load is not "no approved inscripción".
  const [errorProcesos, setErrorProcesos] = useState(false);
  const [loading, setLoading] = useState(false);
  const [conflictDialog, setConflictDialog] = useState<{ open: boolean; practicaId: number | null }>({
    open: false,
    practicaId: null,
  });

  useEffect(() => {
    if (!token) return;
    // Prácticas require an APPROVED inscripción in the proceso (same rule as simulacros).
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
      setAsignaturas([]);
      return;
    }
    catalogsApi
      .listProcesoAsignaturasPublic(token, procesoSeleccionado.id)
      .then((list: ProcesoAsignaturaItem[]) => {
        setAsignaturas(
          list.map((a) => ({
            asignaturaId: a.asignatura_id,
            nombre: a.nombre,
            enabled: false,
            numPreguntas: Math.min(a.numero_preguntas, a.preguntas_disponibles),
            maxPreguntas: a.preguntas_disponibles,
          })),
        );
      })
      .catch(() => {});
  }, [token, procesoSeleccionado]);

  const toggleAsignatura = (id: number) => {
    setAsignaturas((prev) => prev.map((a) => (a.asignaturaId === id ? { ...a, enabled: !a.enabled } : a)));
  };

  const setNum = (id: number, n: number) => {
    setAsignaturas((prev) => prev.map((a) => (a.asignaturaId === id ? { ...a, numPreguntas: n } : a)));
  };

  const clampNum = (id: number) => {
    setAsignaturas((prev) =>
      prev.map((a) =>
        a.asignaturaId === id ? { ...a, numPreguntas: Math.max(1, Math.min(a.maxPreguntas, a.numPreguntas || 1)) } : a,
      ),
    );
  };

  const numError = (a: AsignaturaSelection) => {
    if (!a.enabled) return null;
    if (!a.numPreguntas || a.numPreguntas < 1) return t("practicas-error-min");
    if (a.numPreguntas > a.maxPreguntas) return t("practicas-error-max", { max: a.maxPreguntas });
    return null;
  };

  const hasInvalidCounts = asignaturas.some((a) => a.enabled && numError(a) !== null);

  const handleSubmit = useCallback(async () => {
    if (!token || !procesoSeleccionado) return;
    const selected = asignaturas.filter((a) => a.enabled);
    if (selected.length === 0) {
      setError(t("practicas-seleccionar-asignatura"));
      return;
    }
    const invalid = selected.some((a) => !a.numPreguntas || a.numPreguntas < 1 || a.numPreguntas > a.maxPreguntas);
    if (invalid) {
      setError(t("practicas-error-cantidades"));
      return;
    }
    const totalSeleccionadas = selected.reduce((acc, a) => acc + a.numPreguntas, 0);
    if (totalSeleccionadas > MAX_PREGUNTAS_PRACTICA) {
      setError(t("practicas-error-maximo", { max: MAX_PREGUNTAS_PRACTICA }));
      return;
    }
    setError(null);
    setLoading(true);
    try {
      const practica = await practicasApi.createPractica(token, {
        proceso_id: procesoSeleccionado.id,
        asignaturas: selected.map((a) => ({
          asignatura_id: a.asignaturaId,
          numero_preguntas: a.numPreguntas,
        })),
        modo_calificacion: modo,
        tiempo_limite_seg: usarTimer ? tiempoMin * 60 : null,
      });
      router.push(`/practicas/${practica.id}`);
    } catch (e: unknown) {
      const err = e as { status?: number; practica_id?: number };
      if (err.status === 409 && err.practica_id) {
        setConflictDialog({ open: true, practicaId: err.practica_id });
      } else if (err.status === 403) {
        setError(t("practicas-error-sin-acceso"));
      } else if (err.status === 422) {
        setError(t("practicas-error-preguntas"));
      } else {
        // H33: always the translated key (the server message is Spanish-only).
        setError(t("practicas-error-crear"));
      }
    } finally {
      setLoading(false);
    }
  }, [token, procesoSeleccionado, asignaturas, modo, usarTimer, tiempoMin, router, t]);

  const handleContinueExisting = () => {
    if (conflictDialog.practicaId) router.push(`/practicas/${conflictDialog.practicaId}`);
  };

  const handleAbandonAndCreate = async () => {
    if (!token || !conflictDialog.practicaId) return;
    try {
      await practicasApi.finalizarPractica(token, conflictDialog.practicaId, { abandonar: true });
      setConflictDialog({ open: false, practicaId: null });
      handleSubmit();
    } catch {
      // abandon failed: ignore
    }
  };

  const anySelected = asignaturas.some((a) => a.enabled);
  const totalPreguntas = asignaturas.filter((a) => a.enabled).reduce((sum, a) => sum + a.numPreguntas, 0);

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error && (
        <Alerta
          tono="error"
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setError(null)}>
              <XIcon weight="bold" className="size-4" />
            </Button>
          }
        >
          {error}
        </Alerta>
      )}

      {procesosCargados && errorProcesos && <Alerta tono="error">{t("practicas-error-cargar-procesos")}</Alerta>}

      {procesosCargados && !errorProcesos && procesos.length === 0 && (
        <Alerta tono="info">{t("practicas-sin-inscripcion-aprobada")}</Alerta>
      )}

      <div className="grid grid-cols-1 items-start gap-3 md:grid-cols-12">
        {/* Proceso y asignaturas */}
        <section className={cn(TARJETA, "md:col-span-8")}>
          <CabeceraSeccion
            icon={StackIcon}
            titulo={t("practicas-proceso-titulo")}
            subtitulo={t("practicas-nueva-subtitulo")}
          />
          <div className="flex flex-col gap-5 px-4 pt-2 pb-4">
            <div id="tour-practica-proceso" className="grid gap-1.5">
              <label htmlFor="practica-proceso" className={ETIQUETA_CAMPO}>
                {t("menu-procesos") || "Proceso"}
              </label>
              <Combobox
                vacio={t("comun-sin-resultados")}
                id="practica-proceso"
                items={procesos.map((p) => ({ value: String(p.id), label: p.nombre }))}
                valor={procesoSeleccionado ? String(procesoSeleccionado.id) : null}
                onCambio={(v) => setProcesoSeleccionado(procesos.find((p) => String(p.id) === v) ?? null)}
                placeholder={t("practicas-seleccionar-proceso")}
              />
            </div>

            {asignaturas.length > 0 && (
              <div className="flex flex-col gap-3">
                <div className="grid gap-0.5">
                  <span className={ETIQUETA_CAMPO}>{t("practicas-asignaturas")}</span>
                  <span className={AYUDA_CAMPO}>{t("practicas-asignaturas-hint")}</span>
                </div>

                <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
                  {asignaturas.map((a) => {
                    const errorNum = numError(a);
                    return (
                      <div
                        key={a.asignaturaId}
                        onClick={() => toggleAsignatura(a.asignaturaId)}
                        className={cn(
                          "ease-salida flex cursor-pointer flex-col gap-3 rounded-xl p-3 transition-[background-color,box-shadow] duration-(--duracion-rapida)",
                          a.enabled
                            ? "bg-accent ring-acento-texto dark:ring-acento ring-2"
                            : "bg-card ring-border hover:bg-secondary hover:ring-input ring-1",
                        )}
                      >
                        <div onClick={(e) => e.stopPropagation()}>
                          <Casilla
                            id={`asignatura-${a.asignaturaId}`}
                            etiqueta={a.nombre}
                            checked={a.enabled}
                            onCheckedChange={() => toggleAsignatura(a.asignaturaId)}
                          />
                        </div>

                        <div
                          className="flex flex-wrap items-center gap-2 pl-7"
                          onClick={(e) => e.stopPropagation()}
                          onBlur={() => clampNum(a.asignaturaId)}
                        >
                          <StepperNumerico
                            id={`num-${a.asignaturaId}`}
                            valor={a.numPreguntas}
                            onCambio={(v) => setNum(a.asignaturaId, v ?? 0)}
                            min={1}
                            max={a.maxPreguntas}
                            disabled={!a.enabled}
                            className={cn(errorNum && "border-destructive")}
                          />
                          <span
                            className={cn(
                              "text-etiqueta min-w-0",
                              errorNum
                                ? "text-destructive-texto"
                                : a.enabled
                                  ? "text-muted-foreground"
                                  : "text-texto-terciario",
                            )}
                            role={errorNum ? "alert" : undefined}
                          >
                            {errorNum ?? t("practicas-de-disponibles", { max: a.maxPreguntas })}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>

                {anySelected && (
                  <span className="bg-acento-suave text-etiqueta text-acento-texto dark:text-acento inline-flex w-fit items-center gap-1.5 rounded-full px-3 py-1 font-medium">
                    <ListNumbersIcon aria-hidden weight="duotone" className="size-3.5" />
                    <span className="tabular-nums">{t("practicas-total-preguntas", { total: totalPreguntas })}</span>
                  </span>
                )}
              </div>
            )}
          </div>
        </section>

        {/* Opciones */}
        <div className="flex flex-col gap-3 md:col-span-4">
          <section className={TARJETA}>
            <CabeceraSeccion icon={GearSixIcon} titulo={t("practicas-opciones")} />
            <div className="flex flex-col gap-5 px-4 pt-2 pb-4">
              <fieldset className="grid gap-2">
                <legend className={cn(ETIQUETA_CAMPO, "mb-2")}>{t("practicas-modo")}</legend>
                <GrupoOpciones<"inmediato" | "al_final">
                  nombre="modo"
                  valor={modo}
                  onCambio={setModo}
                  opciones={[
                    { valor: "inmediato", etiqueta: t("practicas-modo-inmediato") },
                    { valor: "al_final", etiqueta: t("practicas-modo-al_final") },
                  ]}
                />
              </fieldset>

              <div className="border-border/60 flex flex-col gap-3 border-t pt-4">
                <Interruptor
                  id="practica-timer"
                  etiqueta={
                    <span className="inline-flex items-center gap-2">
                      <ClockIcon aria-hidden weight="duotone" className="text-muted-foreground size-4" />
                      {t("practicas-tiempo-limite")}
                    </span>
                  }
                  checked={usarTimer}
                  onCheckedChange={setUsarTimer}
                />

                {usarTimer && (
                  <div className="flex items-center gap-2" onBlur={() => setTiempoMin(Math.max(1, tiempoMin || 1))}>
                    <StepperNumerico
                      id="practica-tiempo"
                      valor={tiempoMin}
                      onCambio={(v) => setTiempoMin(v ?? 0)}
                      min={1}
                    />
                    <span className="text-control text-muted-foreground">{t("practicas-min")}</span>
                  </div>
                )}
              </div>
            </div>
          </section>

          <BotonAsync
            id="tour-practica-iniciar"
            pendiente={loading}
            onClick={handleSubmit}
            disabled={!procesoSeleccionado || !anySelected || hasInvalidCounts}
            icon={<PlayIcon weight="fill" className="size-4" />}
            className={cn(BOTON_PRIMARIO, "w-full")}
          >
            {t("practicas-comenzar")}
          </BotonAsync>
        </div>
      </div>

      {/* Práctica en curso */}
      <Dialog
        open={conflictDialog.open}
        onOpenChange={(open) => {
          if (!open) setConflictDialog({ open: false, practicaId: null });
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={WarningIcon}
            tono="aviso"
            titulo={t("practicas-en-curso-titulo")}
            descripcion={t("practicas-en-curso-descripcion")}
          />
          <PieDialogo>
            <Button variant="destructive" onClick={handleAbandonAndCreate}>
              {t("practicas-abandonar")}
            </Button>
            <button type="button" className={BOTON_PRIMARIO} onClick={handleContinueExisting}>
              {t("practicas-continuar")}
            </button>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
