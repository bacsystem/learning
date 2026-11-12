"use client";

import { DialogoFormulario } from "../../../../_catalogos/componentes";
import { useOpcionesImportacion } from "./use-opciones-importacion";
import { useFormik } from "formik";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { Campo } from "@/components/app/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { useAuth } from "@/lib/auth/auth-context";
import type { ImportacionCreada } from "@/lib/serums-ia/balotario/api";
import {
  claveErrorBalotario,
  especialidadesDeError,
  pideRecargar,
  valoresDeError,
} from "@/lib/serums-ia/balotario/errores";
import { cuerpoConfirmacion, esquemaImportacion, VALORES_INICIALES } from "@/lib/serums-ia/balotario/reglas";
import { FileArrowDownIcon } from "@phosphor-icons/react";

/** Confirmación de la importación: proceso, profesión y área inicial (provisional). El servidor valida todo de nuevo. */
export function DialogoImportar({
  version,
  onCerrar,
  onConfirmar,
  onConflicto,
}: {
  version: number;
  onCerrar: () => void;
  /** Crea las preguntas; devuelve el resultado o lanza el error de la API. */
  onConfirmar: (cuerpo: ReturnType<typeof cuerpoConfirmacion>) => Promise<ImportacionCreada>;
  /** Un 409 (`version_desactualizada`, `ya_importado`): se cierra el diálogo y se vuelve a analizar. */
  onConflicto: (mensaje: string) => void;
}) {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const [mensaje, setMensaje] = useState<string | null>(null);
  const formik = useFormik({
    initialValues: VALORES_INICIALES,
    validationSchema: useMemo(() => esquemaImportacion(t("serumsIa.balotario.confirmar.requerido")), [t]),
    validateOnMount: true,
    onSubmit: async (v) => {
      setMensaje(null);
      try {
        await onConfirmar(cuerpoConfirmacion(version, v));
      } catch (e) {
        // En `especialidad_no_corresponde` se nombran las profesiones de la fuente para que el admin sepa cuál elegir.
        const profesiones = especialidadesDeError(e)
          .map((id) => opciones.especialidades.find((p) => p.id === id)?.nombre ?? `#${id}`)
          .join(", ");
        const texto = t(claveErrorBalotario(e, t.has), { ...valoresDeError(e), profesiones });
        if (pideRecargar(e)) onConflicto(texto);
        else setMensaje(texto);
      }
    },
  });
  const procesoId = formik.values.proceso_id ? Number(formik.values.proceso_id) : null;
  const opciones = useOpcionesImportacion(token, procesoId);
  const proceso = opciones.procesos.find((p) => p.id === procesoId);
  const campo = (nombre: keyof typeof VALORES_INICIALES) => {
    const e = formik.touched[nombre] ? formik.errors[nombre] : undefined;
    return typeof e === "string" ? e : undefined;
  };

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={FileArrowDownIcon}
      titulo={t("serumsIa.balotario.confirmar.titulo")}
      descripcion={t("serumsIa.balotario.confirmar.descripcion", { version })}
      error={mensaje ?? (opciones.errorOpciones ? t("serumsIa.balotario.confirmar.errorOpciones") : null)}
      pendiente={formik.isSubmitting}
      puedeGuardar={formik.isValid}
      textoGuardar={t("serumsIa.balotario.confirmar.boton")}
      textoGuardando={t("serumsIa.balotario.confirmar.pendiente")}
      textoCancelar={t("serumsIa.balotario.confirmar.cancelar")}
      onGuardar={formik.submitForm}
      ancho="sm:max-w-md"
    >
      <Campo
        id="importar-proceso"
        etiqueta={t("serumsIa.balotario.confirmar.proceso")}
        error={campo("proceso_id")}
        className="md:col-span-2"
      >
        <SelectCampo
          id="importar-proceso"
          nombre="proceso_id"
          opciones={opciones.procesos.map((p) => ({ value: String(p.id), label: `${p.nombre} (${p.anio})` }))}
          valor={formik.values.proceso_id}
          onCambio={(v) => {
            // La profesión y el área dependen del proceso: se vuelven a elegir.
            void formik.setValues({ proceso_id: v, especialidad_id: "", asignatura_id: "" });
          }}
          placeholder={t("serumsIa.balotario.confirmar.procesoPlaceholder")}
          invalido={Boolean(campo("proceso_id"))}
          describidoPor={campo("proceso_id") ? "importar-proceso-error" : undefined}
          disabled={opciones.cargandoOpciones}
        />
      </Campo>
      <Campo
        id="importar-especialidad"
        etiqueta={t("serumsIa.balotario.confirmar.especialidad")}
        error={campo("especialidad_id")}
        className="md:col-span-2"
      >
        <SelectCampo
          id="importar-especialidad"
          nombre="especialidad_id"
          opciones={opciones.especialidades
            .filter((e) => e.institucion_id === proceso?.institucion_id)
            .map((e) => ({ value: String(e.id), label: e.nombre }))}
          valor={formik.values.especialidad_id}
          onCambio={(v) => void formik.setFieldValue("especialidad_id", v)}
          placeholder={t("serumsIa.balotario.confirmar.especialidadPlaceholder")}
          invalido={Boolean(campo("especialidad_id"))}
          describidoPor={campo("especialidad_id") ? "importar-especialidad-error" : undefined}
          disabled={!proceso}
        />
      </Campo>
      <Campo
        id="importar-area"
        etiqueta={t("serumsIa.balotario.confirmar.area")}
        ayuda={
          !proceso
            ? t("serumsIa.balotario.confirmar.areaSinProceso")
            : !opciones.cargandoAreas && opciones.areas.length === 0
              ? t("serumsIa.balotario.confirmar.areaVacia")
              : t("serumsIa.balotario.confirmar.areaAyuda")
        }
        error={campo("asignatura_id")}
        className="md:col-span-2"
      >
        <SelectCampo
          id="importar-area"
          nombre="asignatura_id"
          opciones={opciones.areas.map((a) => ({ value: String(a.asignatura_id), label: a.nombre }))}
          valor={formik.values.asignatura_id}
          onCambio={(v) => void formik.setFieldValue("asignatura_id", v)}
          placeholder={t("serumsIa.balotario.confirmar.areaPlaceholder")}
          invalido={Boolean(campo("asignatura_id"))}
          describidoPor={`importar-area-${campo("asignatura_id") ? "error" : "ayuda"}`}
          disabled={!proceso || opciones.cargandoAreas}
        />
      </Campo>
    </DialogoFormulario>
  );
}
