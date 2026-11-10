"use client";

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import type { Tono } from "@/components/patrones/metricas";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { Inscripcion, inscripcionesApi, Proceso } from "@/lib/inscripciones/api";
import { cn } from "@/lib/utils";
import { CheckSquareIcon, ClockIcon, GraduationCapIcon, ListChecksIcon, XIcon } from "@phosphor-icons/react";

/** Tipo de proceso: ascenso en el segundo acento, pase a la efectividad en el acento de la paleta. */
const TIPO_CLASE: Record<string, string> = {
  ascenso: "bg-acento-2-suave text-acento-2-texto dark:text-acento-2",
  pase_efectividad: "bg-accent text-accent-foreground",
};

const ESTADO_TONO: Record<Inscripcion["estado"], Tono> = {
  aprobado: "ok",
  pendiente: "aviso",
  rechazado: "error",
};

function ChipTipo({ tipo, etiqueta }: { tipo: string; etiqueta: string }) {
  return (
    <span
      className={cn(
        "text-etiqueta inline-flex shrink-0 items-center rounded-full px-2.5 py-0.5 font-medium whitespace-nowrap",
        TIPO_CLASE[tipo] ?? "bg-secondary text-muted-foreground",
      )}
    >
      {etiqueta}
    </span>
  );
}

function TarjetaProcesoCargando() {
  return (
    <div className={cn(TARJETA, "flex flex-col")}>
      <div className="flex flex-col gap-3 px-4 pt-4 pb-4">
        <div className="flex items-start justify-between gap-3">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-5 w-16 rounded-full" />
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-5 w-20 rounded-full" />
          <Skeleton className="h-5 w-24 rounded-full" />
        </div>
        <Skeleton className="h-3 w-full" />
        <Skeleton className="h-3 w-4/5" />
      </div>
      <div className="border-border/60 mt-auto border-t px-4 py-3">
        <Skeleton className="h-control w-32 rounded-full" />
      </div>
    </div>
  );
}

export default function Page() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [misIns, setMisIns] = useState<Inscripcion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Fallo al cargar la lista: se muestra solo el error con «Reintentar», no el falso estado vacío.
  const [errorCarga, setErrorCarga] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<Proceso | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const tipoLabel: Record<string, string> = {
    ascenso: t("procesos-tipo-ascenso"),
    pase_efectividad: t("procesos-tipo-pase-efectividad"),
    admision: t("admin-procesos-tipo-admision"),
  };

  const estadoLabel: Record<Inscripcion["estado"], string> = {
    aprobado: t("procesos-estado-aprobado"),
    pendiente: t("procesos-estado-pendiente"),
    rechazado: t("procesos-estado-rechazado"),
  };

  const estadoPill = (estado: string) => {
    const tono = ESTADO_TONO[estado as Inscripcion["estado"]];
    if (!tono) return null;
    return <PillEstado tono={tono}>{estadoLabel[estado as Inscripcion["estado"]]}</PillEstado>;
  };

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setErrorCarga(null);
    try {
      const [p, i] = await Promise.all([
        inscripcionesApi.listProcesos(token),
        inscripcionesApi.misInscripciones(token),
      ]);
      setProcesos(p);
      setMisIns(i);
    } catch {
      setErrorCarga(t("procesos-error-cargar"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    load();
  }, [load]);

  const inscripcionFor = (procesoId: number) => misIns.find((i) => i.proceso_id === procesoId);

  const inscribirse = async () => {
    if (!token || !confirm) return;
    setSubmitting(true);
    try {
      await inscripcionesApi.inscribir(token, confirm.id);
      setConfirm(null);
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message ?? t("procesos-error-inscribir"));
      setConfirm(null);
    } finally {
      setSubmitting(false);
    }
  };

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

      <section className="flex flex-col gap-3">
        <div className="flex flex-wrap items-end justify-between gap-2 px-1">
          <div className="min-w-0">
            <h2 className="text-titulo text-foreground">{t("procesos-titulo")}</h2>
            <p className="text-control text-muted-foreground">{t("procesos-subtitulo")}</p>
          </div>
          {!loading && procesos.length > 0 && <Chip>{procesos.length}</Chip>}
        </div>

        {loading ? (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3" aria-busy="true">
            {Array.from({ length: 3 }).map((_, i) => (
              <TarjetaProcesoCargando key={i} />
            ))}
          </div>
        ) : errorCarga ? (
          <Alerta
            tono="error"
            accion={
              <Button variant="outline" size="sm" onClick={load}>
                {t("comun-reintentar")}
              </Button>
            }
          >
            {errorCarga}
          </Alerta>
        ) : procesos.length === 0 ? (
          <EstadoVacio icon={ClockIcon} titulo={t("procesos-sin-procesos")} />
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {procesos.map((p) => {
              const ins = inscripcionFor(p.id);
              return (
                <article key={p.id} className={cn(TARJETA, "flex flex-col")}>
                  <div className="flex flex-1 flex-col gap-3 px-4 pt-4 pb-4">
                    <div className="flex items-start justify-between gap-3">
                      <h3 className="text-titulo text-foreground min-w-0">{p.nombre}</h3>
                      <ChipTipo tipo={p.tipo} etiqueta={tipoLabel[p.tipo] ?? p.tipo} />
                    </div>

                    <div className="flex flex-wrap gap-1.5">
                      <Chip>{t("comun-anio", { anio: p.anio })}</Chip>
                      {p.grado && <Chip>{p.grado}</Chip>}
                    </div>

                    {p.descripcion && (
                      <p className="text-control text-muted-foreground leading-relaxed">{p.descripcion}</p>
                    )}
                  </div>

                  <div className="min-h-control border-border/60 box-content flex items-center justify-between gap-2 border-t px-4 py-3">
                    {ins && estadoPill(ins.estado)}
                    {/* A rejected inscription can be re-applied (same row goes back to pendiente). */}
                    {(!ins || ins.estado === "rechazado") && (
                      <button type="button" className={BOTON_PRIMARIO} onClick={() => setConfirm(p)}>
                        <CheckSquareIcon className="size-4" />
                        {t(ins ? "procesos-reinscribirme" : "procesos-inscribirme")}
                      </button>
                    )}
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </section>

      {misIns.length > 0 && (
        <section className={cn(TARJETA, "overflow-hidden")}>
          <div className="flex items-center gap-2.5 px-4 pt-3.5 pb-2">
            <ListChecksIcon aria-hidden className="text-acento-2-texto dark:text-acento-2 size-[18px] shrink-0" />
            <h2 className="text-titulo text-foreground min-w-0 flex-1">{t("procesos-mis-inscripciones")}</h2>
            <Chip>{misIns.length}</Chip>
          </div>
          <ul className="flex flex-col gap-[3px] px-2 pb-2">
            {misIns.map((i) => (
              <li
                key={i.id}
                className="bg-secondary flex flex-wrap items-center justify-between gap-x-4 gap-y-2 rounded-lg px-3 py-2.5"
              >
                <div className="flex min-w-0 items-start gap-2.5">
                  <GraduationCapIcon aria-hidden className="text-muted-foreground mt-0.5 size-4 shrink-0" />
                  <div className="min-w-0">
                    <p className="text-control text-foreground font-medium">{i.proceso_nombre}</p>
                    <p className="text-etiqueta text-texto-terciario tabular-nums">
                      {tipoLabel[i.proceso_tipo] ?? i.proceso_tipo} · {i.proceso_anio}
                    </p>
                    {i.observacion && <p className="text-etiqueta text-destructive-texto mt-1">{i.observacion}</p>}
                  </div>
                </div>
                {estadoPill(i.estado)}
              </li>
            ))}
          </ul>
        </section>
      )}

      <Dialog
        open={!!confirm}
        onOpenChange={(open) => {
          if (!open && !submitting) setConfirm(null);
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={GraduationCapIcon}
            titulo={t("procesos-confirmar-titulo")}
            descripcion={t("procesos-confirmar-pregunta")}
          />
          {confirm && (
            <div className="grid gap-3 px-5 pb-2">
              <div className="bg-secondary rounded-lg px-3.5 py-3">
                <p className="text-control text-foreground font-medium">{confirm.nombre}</p>
                <p className="text-etiqueta text-muted-foreground tabular-nums">
                  {tipoLabel[confirm.tipo] ?? confirm.tipo} · {t("comun-anio", { anio: confirm.anio })}
                </p>
              </div>
              <p className="text-control text-muted-foreground leading-relaxed">{t("procesos-confirmar-nota")}</p>
            </div>
          )}
          <PieDialogo>
            <button type="button" className={BOTON_SECUNDARIO} onClick={() => setConfirm(null)} disabled={submitting}>
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={submitting}
              textoPendiente={t("procesos-inscribiendo")}
              className={BOTON_PRIMARIO}
              onClick={inscribirse}
            >
              {t("comun-confirmar")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
