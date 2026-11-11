"use client";

import { getIn, useFormik } from "formik";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Entrada } from "@/components/formularios/campo";
import { Combobox } from "@/components/formularios/combobox";
import { BotonAsync } from "@/components/patrones/boton-async";
import { Chip } from "@/components/patrones/pill-estado";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, ETIQUETA_CAMPO, OBJETIVO_TACTIL } from "@/lib/estilos";
import type { AreaPesoPayload, ConfiguracionPesos } from "@/lib/serums-ia/pesos/api";
import { mensajeErrorPesos } from "@/lib/serums-ia/pesos/errores";
import {
  aFila,
  aPayload,
  AREAS_MAX,
  COEFICIENTE_MAX,
  COEFICIENTE_MIN,
  esquemaPesos,
  filaNueva,
  type FilaPeso,
  PESO_MAX,
  PESO_MIN,
  PREGUNTAS_AREA_MAX,
  PREGUNTAS_AREA_MIN,
  PREGUNTAS_TOTAL_MAX,
  totales,
  type ValoresPesos,
} from "@/lib/serums-ia/pesos/reglas";
import { cn } from "@/lib/utils";
import { CopySimpleIcon, ScalesIcon, TrashIcon } from "@phosphor-icons/react";

type Campo = "peso_porcentual" | "numero_preguntas" | "coeficiente";
const CAMPOS: readonly Campo[] = ["peso_porcentual", "numero_preguntas", "coeficiente"];
const CLAVE_CAMPO: Record<Campo, string> = {
  peso_porcentual: "serumsIa.pesos.col.peso",
  numero_preguntas: "serumsIa.pesos.col.preguntas",
  coeficiente: "serumsIa.pesos.col.coeficiente",
};
const MODO_TECLADO: Record<Campo, "decimal" | "numeric"> = {
  peso_porcentual: "decimal",
  numero_preguntas: "numeric",
  coeficiente: "decimal",
};

/** Rejilla de cada fila: en celular, nombre arriba y los tres campos debajo; desde `sm`, todo en una línea. */
const REJILLA_FILA = "grid grid-cols-3 gap-2 sm:grid-cols-[minmax(0,1fr)_7rem_7rem_7rem_2.5rem] sm:items-center";

/**
 * Tabla editable de la configuración de (proceso, especialidad) con formik + yup (reglas de PE3). Se monta por par y
 * por guardado: los valores escritos se conservan ante un error del servidor.
 */
export function FormularioPesos({
  configuracion,
  asignaturas,
  onGuardar,
}: {
  configuracion: ConfiguracionPesos;
  /** Asignaturas activas que se pueden añadir. */
  asignaturas: { id: number; nombre: string }[];
  /** Llama a PE3; si rechaza, el error se muestra sin perder lo escrito. */
  onGuardar: (areas: AreaPesoPayload[]) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const [errorServidor, setErrorServidor] = useState<string | null>(null);

  const validationSchema = useMemo(
    () =>
      esquemaPesos({
        areasMin: t("serumsIa.pesos.validacion.areasMin"),
        areasMax: t("serumsIa.pesos.validacion.areasMax", { max: AREAS_MAX }),
        peso: t("serumsIa.pesos.validacion.peso", { min: PESO_MIN, max: PESO_MAX }),
        preguntas: t("serumsIa.pesos.validacion.preguntas", { min: PREGUNTAS_AREA_MIN, max: PREGUNTAS_AREA_MAX }),
        coeficiente: t("serumsIa.pesos.validacion.coeficiente", { min: COEFICIENTE_MIN, max: COEFICIENTE_MAX }),
        repetidas: t("serumsIa.pesos.validacion.repetidas"),
        limitePreguntas: t("serumsIa.pesos.validacion.limitePreguntas", { max: PREGUNTAS_TOTAL_MAX }),
        sumaPesos: t("serumsIa.pesos.validacion.sumaPesos"),
      }),
    [t],
  );

  const formik = useFormik<ValoresPesos>({
    initialValues: { areas: configuracion.areas.map(aFila) },
    validationSchema,
    validateOnMount: true,
    onSubmit: async ({ areas }) => {
      setErrorServidor(null);
      try {
        await onGuardar(aPayload(areas));
      } catch (e) {
        setErrorServidor(mensajeErrorPesos(e, t, t("comun-error-guardar")));
      }
    },
  });

  const areas = formik.values.areas;
  const resumen = totales(areas);
  const nombres = new Map(areas.map((a) => [a.asignatura_id, a.nombre]));
  const disponibles = asignaturas
    .filter((a) => !nombres.has(a.id))
    .map((a) => ({ value: String(a.id), label: a.nombre }));

  function establecer(areasNuevas: FilaPeso[]) {
    formik.setFieldValue("areas", areasNuevas);
  }

  function anadir(valor: string | null) {
    const asignatura = asignaturas.find((a) => String(a.id) === valor);
    if (asignatura) establecer([...areas, filaNueva(asignatura.id, asignatura.nombre)]);
  }

  function errorDe(i: number, campo: Campo): string | undefined {
    const ruta = `areas.${i}.${campo}`;
    const mensaje = getIn(formik.touched, ruta) ? getIn(formik.errors, ruta) : undefined;
    return typeof mensaje === "string" ? mensaje : undefined;
  }

  // Los errores del conjunto (repetidas, total y suma) se muestran en vivo, sin esperar a enviar.
  const avisos = [
    resumen.repetidas.length > 0 &&
      t("serumsIa.pesos.validacion.repetidasNombres", {
        nombres: resumen.repetidas.map((id) => nombres.get(id)).join(", "),
      }),
    !resumen.preguntasValidas && t("serumsIa.pesos.validacion.limitePreguntas", { max: PREGUNTAS_TOTAL_MAX }),
    areas.length > 0 &&
      !resumen.sumaValida &&
      t("serumsIa.pesos.validacion.sumaPesosActual", { suma: resumen.sumaPesos }),
  ].filter((x): x is string => typeof x === "string");

  return (
    <form onSubmit={formik.handleSubmit} noValidate className="flex flex-col gap-3">
      {areas.length === 0 ? (
        <EstadoVacio
          icon={ScalesIcon}
          titulo={t("serumsIa.pesos.vacio.titulo")}
          accion={
            configuracion.plantilla.length > 0 ? (
              <button
                type="button"
                onClick={() => establecer(configuracion.plantilla.map(aFila))}
                className={ACCION_SECUNDARIA}
              >
                <CopySimpleIcon className="size-4" />
                {t("serumsIa.pesos.usarPlantilla", { n: configuracion.plantilla.length })}
              </button>
            ) : undefined
          }
        >
          {t("serumsIa.pesos.vacio.texto")}
        </EstadoVacio>
      ) : (
        <div className="grid gap-1.5">
          <div
            className={cn(REJILLA_FILA, "text-etiqueta text-texto-terciario px-1 font-medium max-sm:hidden")}
            aria-hidden
          >
            <span>{t("serumsIa.pesos.col.area")}</span>
            {CAMPOS.map((c) => (
              <span key={c} className="text-center">
                {t(CLAVE_CAMPO[c])}
              </span>
            ))}
            <span />
          </div>
          <ul className="grid gap-1.5">
            {areas.map((a, i) => (
              <li key={a.asignatura_id} className="bg-secondary grid gap-2 rounded-lg p-2 sm:py-1 sm:pl-3">
                <div className={REJILLA_FILA}>
                  <div className="col-span-3 flex items-center justify-between gap-2 sm:col-span-1">
                    <span className="text-control text-foreground truncate font-medium" title={a.nombre}>
                      {a.nombre}
                    </span>
                    <button
                      type="button"
                      onClick={() => establecer(areas.filter((_, j) => j !== i))}
                      aria-label={t("serumsIa.pesos.quitar", { nombre: a.nombre })}
                      title={t("serumsIa.pesos.quitar", { nombre: a.nombre })}
                      className={cn(OBJETIVO_TACTIL, ACCION_SECUNDARIA, "px-2.5 sm:hidden")}
                    >
                      <TrashIcon className="size-4" />
                    </button>
                  </div>
                  {CAMPOS.map((campo) => {
                    const id = `peso-${a.asignatura_id}-${campo}`;
                    const error = errorDe(i, campo);
                    return (
                      <div key={campo} className="grid gap-1">
                        <label htmlFor={id} className={cn(ETIQUETA_CAMPO, "sm:sr-only")}>
                          {t(CLAVE_CAMPO[campo])}
                        </label>
                        <Entrada
                          id={id}
                          name={`areas.${i}.${campo}`}
                          inputMode={MODO_TECLADO[campo]}
                          autoComplete="off"
                          aria-label={t("serumsIa.pesos.campoDe", { campo: t(CLAVE_CAMPO[campo]), nombre: a.nombre })}
                          aria-describedby={error ? `${id}-error` : undefined}
                          invalido={Boolean(error)}
                          className="text-center font-mono tabular-nums"
                          value={a[campo]}
                          onChange={formik.handleChange}
                          onBlur={formik.handleBlur}
                        />
                      </div>
                    );
                  })}
                  <button
                    type="button"
                    onClick={() => establecer(areas.filter((_, j) => j !== i))}
                    aria-label={t("serumsIa.pesos.quitar", { nombre: a.nombre })}
                    title={t("serumsIa.pesos.quitar", { nombre: a.nombre })}
                    className={cn(OBJETIVO_TACTIL, ACCION_SECUNDARIA, "justify-center px-2.5 max-sm:hidden")}
                  >
                    <TrashIcon className="size-4" />
                  </button>
                </div>
                {CAMPOS.map((campo) => {
                  const error = errorDe(i, campo);
                  return error ? (
                    <p
                      key={campo}
                      id={`peso-${a.asignatura_id}-${campo}-error`}
                      role="alert"
                      className="text-control text-destructive"
                    >
                      {error}
                    </p>
                  ) : null;
                })}
              </li>
            ))}
          </ul>
        </div>
      )}

      {areas.length < AREAS_MAX ? (
        <div className="flex flex-wrap items-center gap-2">
          <label htmlFor="pesos-anadir" className={ETIQUETA_CAMPO}>
            {t("serumsIa.pesos.anadir")}
          </label>
          <Combobox
            id="pesos-anadir"
            className="w-full sm:w-72"
            items={disponibles}
            valor={null}
            onCambio={anadir}
            placeholder={t("serumsIa.pesos.anadirPlaceholder")}
            vacio={t("serumsIa.pesos.anadirVacio")}
            etiquetaAbrir={t("serumsIa.pesos.anadir")}
          />
        </div>
      ) : null}

      <div className="flex flex-wrap items-center gap-2" aria-live="polite">
        <Chip
          className={cn(
            areas.length > 0 &&
              (resumen.sumaValida ? "bg-success text-success-foreground" : "bg-warning text-warning-foreground"),
          )}
        >
          {t("serumsIa.pesos.sumaPesos", { suma: resumen.sumaPesos })}
        </Chip>
        <Chip className={cn(!resumen.preguntasValidas && "bg-warning text-warning-foreground")}>
          {t("serumsIa.pesos.totalPreguntas", { n: resumen.sumaPreguntas, max: PREGUNTAS_TOTAL_MAX })}
        </Chip>
      </div>

      {avisos.length > 0 ? (
        <Alerta tono="aviso">
          {avisos.length === 1 ? (
            avisos[0]
          ) : (
            <ul className="list-disc pl-4">
              {avisos.map((a) => (
                <li key={a}>{a}</li>
              ))}
            </ul>
          )}
        </Alerta>
      ) : null}
      {errorServidor ? <Alerta tono="error">{errorServidor}</Alerta> : null}

      <div className="flex justify-end">
        <BotonAsync
          type="submit"
          pendiente={formik.isSubmitting}
          textoPendiente={t("comun-guardando")}
          disabled={areas.length === 0 || avisos.length > 0}
          className={ACCION_PRINCIPAL}
        >
          {t("comun-guardar")}
        </BotonAsync>
      </div>
    </form>
  );
}
