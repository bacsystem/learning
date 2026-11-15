"use client";

import { DialogoFormulario } from "../_catalogos/componentes";
import { useFormik } from "formik";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { Campo } from "@/components/app/campo";
import { Entrada } from "@/components/formularios/campo";
import { type OpcionSelect, SelectCampo } from "@/components/formularios/select-campo";
import { claveErrorApi } from "@/lib/errores-api";
import type { Etiqueta } from "@/lib/questions/api";
import {
  ERRORES_ETIQUETA,
  esquemaEtiqueta,
  NOMBRE_ETIQUETA_MAX,
  type ValoresEtiqueta,
} from "@/lib/questions/etiquetas";
import { TagIcon } from "@phosphor-icons/react";

/**
 * Alta y edición de una etiqueta con formik + yup. Se monta al abrir: los valores iniciales no cambian mientras está
 * abierto, y un error del servidor conserva lo escrito.
 */
export function FormularioEtiqueta({
  etiqueta,
  opcionesAsignatura,
  onCerrar,
  onGuardar,
}: {
  /** `null` para crear. */
  etiqueta: Etiqueta | null;
  opcionesAsignatura: readonly OpcionSelect[];
  onCerrar: () => void;
  /** Llama a la API; si rechaza, el error se muestra sin cerrar. */
  onGuardar: (valores: ValoresEtiqueta) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const [errorServidor, setErrorServidor] = useState<string | null>(null);
  const modo = etiqueta ? "edit" : "create";

  const validationSchema = useMemo(
    () =>
      esquemaEtiqueta(
        {
          asignaturaRequerida: t("admin-etiquetas-validacion-asignatura"),
          nombreRequerido: t("admin-etiquetas-validacion-nombre"),
          nombreLongitud: t("admin-etiquetas-validacion-nombre-longitud", { max: NOMBRE_ETIQUETA_MAX }),
        },
        modo,
      ),
    [t, modo],
  );

  const formik = useFormik<ValoresEtiqueta>({
    initialValues: { asignatura_id: etiqueta?.asignatura_id ?? 0, nombre: etiqueta?.nombre ?? "" },
    validationSchema,
    onSubmit: async (valores) => {
      setErrorServidor(null);
      try {
        await onGuardar({ ...valores, nombre: valores.nombre.trim() });
      } catch (e) {
        setErrorServidor(t(claveErrorApi(e, "guardar", ERRORES_ETIQUETA.guardar)));
      }
    },
  });

  const error = (campo: keyof ValoresEtiqueta) => {
    const mensaje = formik.touched[campo] ? formik.errors[campo] : undefined;
    return typeof mensaje === "string" ? mensaje : undefined;
  };

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={TagIcon}
      titulo={etiqueta ? t("admin-editar-etiqueta") : t("admin-nueva-etiqueta")}
      descripcion={etiqueta?.asignatura}
      error={errorServidor}
      pendiente={formik.isSubmitting}
      textoGuardar={t("comun-guardar")}
      textoGuardando={t("comun-guardando")}
      textoCancelar={t("comun-cancelar")}
      onGuardar={formik.submitForm}
      ancho="sm:max-w-md"
    >
      {etiqueta ? null : (
        <Campo
          id="etiqueta-asignatura"
          etiqueta={t("admin-catalogos-col-asignatura")}
          error={error("asignatura_id")}
          className="md:col-span-2"
        >
          <SelectCampo
            id="etiqueta-asignatura"
            required
            placeholder={t("admin-seleccionar-asignatura")}
            opciones={opcionesAsignatura}
            valor={formik.values.asignatura_id ? String(formik.values.asignatura_id) : ""}
            invalido={Boolean(error("asignatura_id"))}
            describidoPor={error("asignatura_id") ? "etiqueta-asignatura-error" : undefined}
            onCambio={(v) => {
              formik.setFieldValue("asignatura_id", Number(v) || 0);
              formik.setFieldTouched("asignatura_id", true, false);
            }}
          />
        </Campo>
      )}
      <Campo
        id="etiqueta-nombre"
        etiqueta={t("admin-catalogos-col-nombre")}
        error={error("nombre")}
        className="md:col-span-2"
      >
        <Entrada
          id="etiqueta-nombre"
          name="nombre"
          required
          autoFocus
          invalido={Boolean(error("nombre"))}
          aria-describedby={error("nombre") ? "etiqueta-nombre-error" : undefined}
          value={formik.values.nombre}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
        />
      </Campo>
    </DialogoFormulario>
  );
}
