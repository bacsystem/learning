"use client";

import { DialogoFormulario } from "../_catalogos/componentes";
import { useFormik } from "formik";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { AreaTexto, Campo, Entrada } from "@/components/formularios/campo";
import { Casilla } from "@/components/formularios/casilla";
import { AYUDA_CAMPO, ETIQUETA_CAMPO } from "@/lib/estilos";
import type { EspecialidadTema, Tema, TemaPayload } from "@/lib/serums-ia/temas/api";
import { claveErrorTema, mensajeErrorTema } from "@/lib/serums-ia/temas/errores";
import {
  aPayload,
  DESCRIPCION_MAX,
  ESPECIALIDADES_MAX,
  esquemaTema,
  NOMBRE_MAX,
  NOMBRE_MIN,
  type ValoresTema,
} from "@/lib/serums-ia/temas/reglas";
import { TreeStructureIcon } from "@phosphor-icons/react";

/** Errores del servidor que se muestran bajo el campo del nombre en vez de en el aviso del diálogo. */
const ERRORES_DEL_NOMBRE = new Set(["serumsIa.errores.tema_duplicado"]);

function valoresIniciales(tema: Tema | null): ValoresTema {
  return {
    nombre: tema?.nombre ?? "",
    descripcion: tema?.descripcion ?? "",
    especialidad_ids: tema?.especialidades.map((e) => e.id) ?? [],
  };
}

/**
 * Alta y edición de un tema con formik + yup (reglas de §1 del contrato). Se monta al abrir: los valores iniciales no
 * cambian mientras está abierto, y un error del servidor conserva lo escrito.
 */
export function FormularioTema({
  tema,
  asignatura,
  especialidades,
  onCerrar,
  onGuardar,
}: {
  /** `null` para crear. */
  tema: Tema | null;
  asignatura: string;
  /** Especialidades activas que se pueden asociar. */
  especialidades: EspecialidadTema[];
  onCerrar: () => void;
  /** Llama a la API; si rechaza, el error se muestra sin cerrar. */
  onGuardar: (payload: TemaPayload) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const [errorServidor, setErrorServidor] = useState<string | null>(null);

  const validationSchema = useMemo(
    () =>
      esquemaTema({
        nombreRequerido: t("serumsIa.temas.validacion.nombreRequerido"),
        nombreLongitud: t("serumsIa.temas.validacion.nombreLongitud", { min: NOMBRE_MIN, max: NOMBRE_MAX }),
        descripcionLongitud: t("serumsIa.temas.validacion.descripcionLongitud", { max: DESCRIPCION_MAX }),
        especialidadesMax: t("serumsIa.temas.validacion.especialidadesMax", { max: ESPECIALIDADES_MAX }),
      }),
    [t],
  );

  const formik = useFormik<ValoresTema>({
    initialValues: valoresIniciales(tema),
    validationSchema,
    onSubmit: async (valores, { setFieldError }) => {
      setErrorServidor(null);
      try {
        await onGuardar(aPayload(valores));
      } catch (e) {
        const mensaje = mensajeErrorTema(e, t, t("comun-error-guardar"));
        const clave = claveErrorTema(e);
        if (clave && ERRORES_DEL_NOMBRE.has(clave)) setFieldError("nombre", mensaje);
        else setErrorServidor(mensaje);
      }
    },
  });

  // Las ya asociadas siguen visibles aunque la especialidad ya no esté activa, para poder quitarlas.
  const opciones = useMemo(() => {
    const ids = new Set(especialidades.map((e) => e.id));
    return [...especialidades, ...(tema?.especialidades.filter((e) => !ids.has(e.id)) ?? [])];
  }, [especialidades, tema]);

  const error = (campo: keyof ValoresTema) => {
    const mensaje = formik.touched[campo] ? formik.errors[campo] : undefined;
    return typeof mensaje === "string" ? mensaje : undefined;
  };

  function alternar(id: number, marcada: boolean) {
    const actuales = formik.values.especialidad_ids;
    formik.setFieldValue("especialidad_ids", marcada ? [...actuales, id] : actuales.filter((x) => x !== id));
    formik.setFieldTouched("especialidad_ids", true, false);
  }

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={TreeStructureIcon}
      titulo={tema ? t("serumsIa.temas.editar") : t("serumsIa.temas.nuevo")}
      descripcion={asignatura}
      error={errorServidor}
      pendiente={formik.isSubmitting}
      textoGuardar={t("comun-guardar")}
      textoGuardando={t("comun-guardando")}
      textoCancelar={t("comun-cancelar")}
      onGuardar={formik.submitForm}
    >
      <Campo
        id="tema-nombre"
        etiqueta={t("serumsIa.temas.campo.nombre")}
        error={error("nombre")}
        className="md:col-span-2"
      >
        <Entrada
          id="tema-nombre"
          name="nombre"
          required
          autoFocus
          invalido={Boolean(error("nombre"))}
          aria-describedby={error("nombre") ? "tema-nombre-error" : undefined}
          value={formik.values.nombre}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
        />
      </Campo>
      <Campo
        id="tema-descripcion"
        etiqueta={t("serumsIa.temas.campo.descripcion")}
        ayuda={t("serumsIa.temas.campo.descripcionAyuda", {
          n: [...formik.values.descripcion].length,
          max: DESCRIPCION_MAX,
        })}
        error={error("descripcion")}
        className="md:col-span-2"
      >
        <AreaTexto
          id="tema-descripcion"
          name="descripcion"
          invalido={Boolean(error("descripcion"))}
          aria-describedby={error("descripcion") ? "tema-descripcion-error" : "tema-descripcion-ayuda"}
          value={formik.values.descripcion}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
        />
      </Campo>
      <fieldset className="grid gap-1.5 md:col-span-2" aria-describedby="tema-especialidades-ayuda">
        <legend className={ETIQUETA_CAMPO}>{t("serumsIa.temas.campo.especialidades")}</legend>
        {error("especialidad_ids") ? (
          <p id="tema-especialidades-ayuda" role="alert" className="text-control text-destructive">
            {error("especialidad_ids")}
          </p>
        ) : (
          <p id="tema-especialidades-ayuda" className={AYUDA_CAMPO}>
            {t("serumsIa.temas.campo.especialidadesAyuda")}
          </p>
        )}
        {opciones.length > 0 ? (
          <div className="border-border grid max-h-56 gap-2 overflow-y-auto rounded-lg border p-3 sm:grid-cols-2">
            {opciones.map((e) => (
              <Casilla
                key={e.id}
                id={`tema-especialidad-${e.id}`}
                name="especialidad_ids"
                etiqueta={e.nombre}
                checked={formik.values.especialidad_ids.includes(e.id)}
                onCheckedChange={(v) => alternar(e.id, v)}
              />
            ))}
          </div>
        ) : (
          <p className={AYUDA_CAMPO}>{t("serumsIa.temas.campo.sinEspecialidades")}</p>
        )}
      </fieldset>
    </DialogoFormulario>
  );
}
