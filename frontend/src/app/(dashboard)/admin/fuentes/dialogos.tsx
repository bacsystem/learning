"use client";

import { DialogoFormulario } from "../_catalogos/componentes";
import { AvisoDuplicado, CasillasProfesiones, type OpcionProfesion } from "./componentes";
import { useFormik } from "formik";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { Campo } from "@/components/app/campo";
import { Alerta } from "@/components/feedback/alerta";
import { AreaTexto, Entrada } from "@/components/formularios/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { ZonaArchivos } from "@/components/formularios/zona-archivos";
import type { NuevaFuente, TipoFuente } from "@/lib/serums-ia/fuentes/api";
import { type FuenteDuplicada, fuenteDuplicada, mensajeErrorFuente } from "@/lib/serums-ia/fuentes/errores";
import {
  ANIO_MAX,
  ANIO_MIN,
  ARCHIVO_MAX_BYTES,
  ENTIDAD_MAX,
  ENTIDAD_MIN,
  ESPECIALIDADES_MAX,
  esquemaEdicion,
  esquemaMarca,
  esquemaMotivo,
  esquemaSubida,
  esquemaVersion,
  type MensajesFuente,
  MOTIVO_MAX,
  MOTIVO_MIN,
  NOTA_MAX,
  NOTA_MIN,
  TIPOS,
  TITULO_MAX,
  TITULO_MIN,
} from "@/lib/serums-ia/fuentes/reglas";
import { ArchiveIcon, FileArrowUpIcon, PencilSimpleIcon, SealCheckIcon, UploadSimpleIcon } from "@phosphor-icons/react";

/** Mensajes de validación traducidos para los esquemas de `reglas.ts`. */
function useMensajes(): MensajesFuente {
  const t = useTranslations("dashboard");
  return useMemo(
    () => ({
      titulo: t("serumsIa.fuentes.validacion.titulo", { min: TITULO_MIN, max: TITULO_MAX }),
      tipo: t("serumsIa.fuentes.validacion.tipo"),
      anio: t("serumsIa.fuentes.validacion.anio", { min: ANIO_MIN, max: ANIO_MAX }),
      especialidadesMax: t("serumsIa.fuentes.validacion.especialidadesMax", { max: ESPECIALIDADES_MAX }),
      archivo: {
        requerido: t("serumsIa.fuentes.validacion.archivoRequerido"),
        extension: t("serumsIa.fuentes.validacion.archivoExtension"),
        tamano: t("serumsIa.fuentes.validacion.archivoTamano", { mb: ARCHIVO_MAX_BYTES / 1024 / 1024 }),
      },
      motivo: t("serumsIa.fuentes.validacion.motivo", { min: MOTIVO_MIN, max: MOTIVO_MAX }),
      entidad: t("serumsIa.fuentes.validacion.entidad", { min: ENTIDAD_MIN, max: ENTIDAD_MAX }),
      nota: t("serumsIa.fuentes.validacion.nota", { min: NOTA_MIN, max: NOTA_MAX }),
    }),
    [t],
  );
}

/** Error de un campo de formik, solo si ya se tocó. */
function errorDe<V>(
  formik: { touched: Partial<Record<keyof V, unknown>>; errors: Partial<Record<keyof V, unknown>> },
  campo: keyof V,
) {
  const e = formik.touched[campo] ? formik.errors[campo] : undefined;
  return typeof e === "string" ? e : undefined;
}

/** Selector del `.md` con textos traducidos. El error de tamaño o extensión se ve al elegirlo, antes de enviar. */
function CampoArchivo({
  archivo,
  onCambio,
  error,
}: {
  archivo: File | null;
  onCambio: (f: File | null) => void;
  error?: string;
}) {
  const t = useTranslations("dashboard");
  return (
    <div className="grid gap-1.5 md:col-span-2">
      <ZonaArchivos
        id="fuente-archivo"
        archivo={archivo}
        onCambio={onCambio}
        accept=".md,text/markdown"
        titulo={t("serumsIa.fuentes.campo.archivoTitulo")}
        ayuda={t("serumsIa.fuentes.campo.archivoAyuda", { mb: ARCHIVO_MAX_BYTES / 1024 / 1024 })}
        etiquetaQuitar={t("serumsIa.fuentes.campo.archivoQuitar")}
      />
      {error ? (
        <p role="alert" className="text-control text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}

/** Resultado de un envío rechazado: aviso de duplicado o texto del error. */
function useErrorEnvio() {
  const t = useTranslations("dashboard");
  const [duplicada, setDuplicada] = useState<FuenteDuplicada | null>(null);
  const [mensaje, setMensaje] = useState<string | null>(null);
  return {
    duplicada,
    mensaje,
    limpiar() {
      setDuplicada(null);
      setMensaje(null);
    },
    mostrar(e: unknown) {
      const dup = fuenteDuplicada(e);
      setDuplicada(dup);
      setMensaje(dup ? null : mensajeErrorFuente(e, t, t("comun-error-guardar")));
    },
  };
}

// ── S1: subir una fuente ──────────────────────────────────────────────────────

export function DialogoSubida({
  profesiones,
  procesos,
  onCerrar,
  onSubir,
}: {
  profesiones: OpcionProfesion[];
  procesos: { id: number; nombre: string }[];
  onCerrar: () => void;
  onSubir: (fuente: NuevaFuente) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const mensajes = useMensajes();
  const envio = useErrorEnvio();
  const formik = useFormik({
    initialValues: {
      archivo: null as File | null,
      titulo: "",
      tipo: "" as TipoFuente | "",
      especialidad_ids: [] as number[],
      anio: "",
      proceso_id: "",
    },
    validationSchema: useMemo(() => esquemaSubida(mensajes), [mensajes]),
    validateOnMount: true,
    onSubmit: async (v) => {
      envio.limpiar();
      try {
        await onSubir({
          archivo: v.archivo as File,
          titulo: v.titulo,
          tipo: v.tipo as TipoFuente,
          especialidadIds: v.especialidad_ids,
          anio: v.anio ? Number(v.anio) : null,
          procesoId: v.proceso_id ? Number(v.proceso_id) : null,
        });
      } catch (e) {
        envio.mostrar(e);
      }
    },
  });
  const err = (c: keyof typeof formik.values) => errorDe(formik, c);

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={UploadSimpleIcon}
      titulo={t("serumsIa.fuentes.subir.titulo")}
      error={envio.mensaje}
      pendiente={formik.isSubmitting}
      puedeGuardar={formik.isValid}
      textoGuardar={t("serumsIa.fuentes.subir.boton")}
      textoGuardando={t("serumsIa.fuentes.subiendo")}
      textoCancelar={t("comun-cancelar")}
      onGuardar={formik.submitForm}
      ancho="sm:max-w-2xl"
    >
      <div className="md:col-span-2">
        <Alerta tono="aviso">{t("serumsIa.fuentes.subir.avisoDatos")}</Alerta>
      </div>
      <CampoArchivo
        archivo={formik.values.archivo}
        onCambio={(f) => {
          formik.setFieldValue("archivo", f);
          formik.setFieldTouched("archivo", true, false);
        }}
        error={err("archivo")}
      />
      <Campo
        id="fuente-titulo"
        etiqueta={t("serumsIa.fuentes.campo.titulo")}
        error={err("titulo")}
        className="md:col-span-2"
      >
        <Entrada
          id="fuente-titulo"
          name="titulo"
          value={formik.values.titulo}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
          invalido={Boolean(err("titulo"))}
        />
      </Campo>
      <Campo id="fuente-tipo" etiqueta={t("serumsIa.fuentes.campo.tipo")} error={err("tipo")}>
        <SelectCampo
          id="fuente-tipo"
          placeholder={t("serumsIa.fuentes.campo.tipoElegir")}
          opciones={TIPOS.map((x) => ({ value: x, label: t(`serumsIa.fuentes.tipo.${x}`) }))}
          valor={formik.values.tipo}
          onCambio={(v) => {
            formik.setFieldValue("tipo", v);
            formik.setFieldTouched("tipo", true, false);
          }}
        />
      </Campo>
      <Campo id="fuente-anio" etiqueta={t("serumsIa.fuentes.campo.anio")} error={err("anio")}>
        <Entrada
          id="fuente-anio"
          name="anio"
          inputMode="numeric"
          value={formik.values.anio}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
          invalido={Boolean(err("anio"))}
        />
      </Campo>
      <Campo id="fuente-proceso" etiqueta={t("serumsIa.fuentes.campo.proceso")} className="md:col-span-2">
        <SelectCampo
          id="fuente-proceso"
          opciones={[
            { value: "", label: t("serumsIa.fuentes.campo.procesoNinguno") },
            ...procesos.map((p) => ({ value: String(p.id), label: p.nombre })),
          ]}
          valor={formik.values.proceso_id}
          onCambio={(v) => formik.setFieldValue("proceso_id", v)}
        />
      </Campo>
      <CasillasProfesiones
        id="fuente-profesiones"
        opciones={profesiones}
        valor={formik.values.especialidad_ids}
        onCambio={(ids) => formik.setFieldValue("especialidad_ids", ids)}
        error={typeof formik.errors.especialidad_ids === "string" ? formik.errors.especialidad_ids : undefined}
      />
      {envio.duplicada ? (
        <div className="md:col-span-2">
          <AvisoDuplicado duplicada={envio.duplicada} />
        </div>
      ) : null}
    </DialogoFormulario>
  );
}

// ── S2: nueva versión ─────────────────────────────────────────────────────────

export function DialogoVersion({
  titulo,
  onCerrar,
  onSubir,
}: {
  titulo: string;
  onCerrar: () => void;
  onSubir: (archivo: File) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const mensajes = useMensajes();
  const envio = useErrorEnvio();
  const formik = useFormik({
    initialValues: { archivo: null as File | null },
    validationSchema: useMemo(() => esquemaVersion(mensajes), [mensajes]),
    validateOnMount: true,
    onSubmit: async (v) => {
      envio.limpiar();
      try {
        await onSubir(v.archivo as File);
      } catch (e) {
        envio.mostrar(e);
      }
    },
  });

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={FileArrowUpIcon}
      titulo={t("serumsIa.fuentes.version.titulo")}
      descripcion={titulo}
      error={envio.mensaje}
      pendiente={formik.isSubmitting}
      puedeGuardar={formik.isValid}
      textoGuardar={t("serumsIa.fuentes.version.boton")}
      textoGuardando={t("serumsIa.fuentes.subiendo")}
      textoCancelar={t("comun-cancelar")}
      onGuardar={formik.submitForm}
    >
      <p className="text-control text-muted-foreground md:col-span-2">{t("serumsIa.fuentes.version.ayuda")}</p>
      <CampoArchivo
        archivo={formik.values.archivo}
        onCambio={(f) => {
          formik.setFieldValue("archivo", f);
          formik.setFieldTouched("archivo", true, false);
        }}
        error={errorDe(formik, "archivo")}
      />
      {envio.duplicada ? (
        <div className="md:col-span-2">
          <AvisoDuplicado duplicada={envio.duplicada} />
        </div>
      ) : null}
    </DialogoFormulario>
  );
}

// ── S7: editar título y profesiones ───────────────────────────────────────────

export function DialogoEdicion({
  titulo,
  especialidadIds,
  profesiones,
  onCerrar,
  onGuardar,
}: {
  titulo: string;
  especialidadIds: number[];
  profesiones: OpcionProfesion[];
  onCerrar: () => void;
  onGuardar: (titulo: string, especialidadIds: number[]) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const mensajes = useMensajes();
  const envio = useErrorEnvio();
  const formik = useFormik({
    initialValues: { titulo, especialidad_ids: especialidadIds },
    validationSchema: useMemo(() => esquemaEdicion(mensajes), [mensajes]),
    validateOnMount: true,
    onSubmit: async (v) => {
      envio.limpiar();
      try {
        await onGuardar(v.titulo, v.especialidad_ids);
      } catch (e) {
        envio.mostrar(e);
      }
    },
  });
  const err = errorDe(formik, "titulo");

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={PencilSimpleIcon}
      titulo={t("serumsIa.fuentes.editar.titulo")}
      error={envio.mensaje}
      pendiente={formik.isSubmitting}
      puedeGuardar={formik.isValid}
      textoGuardar={t("comun-guardar")}
      textoGuardando={t("comun-guardando")}
      textoCancelar={t("comun-cancelar")}
      onGuardar={formik.submitForm}
      ancho="sm:max-w-xl"
    >
      <p className="text-control text-muted-foreground md:col-span-2">{t("serumsIa.fuentes.editar.ayuda")}</p>
      <Campo id="editar-titulo" etiqueta={t("serumsIa.fuentes.campo.titulo")} error={err} className="md:col-span-2">
        <Entrada
          id="editar-titulo"
          name="titulo"
          value={formik.values.titulo}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
          invalido={Boolean(err)}
        />
      </Campo>
      <CasillasProfesiones
        id="editar-profesiones"
        opciones={profesiones}
        valor={formik.values.especialidad_ids}
        onCambio={(ids) => formik.setFieldValue("especialidad_ids", ids)}
        error={typeof formik.errors.especialidad_ids === "string" ? formik.errors.especialidad_ids : undefined}
      />
    </DialogoFormulario>
  );
}

// ── S5 retirar y S6 desmarcar: diálogo con motivo ─────────────────────────────

export function DialogoMotivo({
  titulo,
  descripcion,
  detalle,
  textoConfirmar,
  onCerrar,
  onConfirmar,
}: {
  titulo: string;
  descripcion: string;
  /** Texto destacado bajo la descripción (p. ej. los contenidos que se marcarán «en revisión»). */
  detalle?: string;
  textoConfirmar: string;
  onCerrar: () => void;
  onConfirmar: (motivo: string) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const mensajes = useMensajes();
  const envio = useErrorEnvio();
  const formik = useFormik({
    initialValues: { motivo: "" },
    validationSchema: useMemo(() => esquemaMotivo(mensajes), [mensajes]),
    validateOnMount: true,
    onSubmit: async (v) => {
      envio.limpiar();
      try {
        await onConfirmar(v.motivo);
      } catch (e) {
        envio.mostrar(e);
      }
    },
  });
  const err = errorDe(formik, "motivo");

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={ArchiveIcon}
      titulo={titulo}
      descripcion={descripcion}
      error={envio.mensaje}
      pendiente={formik.isSubmitting}
      puedeGuardar={formik.isValid}
      textoGuardar={textoConfirmar}
      textoGuardando={t("comun-procesando")}
      textoCancelar={t("comun-cancelar")}
      onGuardar={formik.submitForm}
      ancho="sm:max-w-md"
    >
      {detalle ? (
        <div className="md:col-span-2">
          <Alerta tono="aviso">{detalle}</Alerta>
        </div>
      ) : null}
      <Campo id="fuente-motivo" etiqueta={t("serumsIa.fuentes.campo.motivo")} error={err} className="md:col-span-2">
        <AreaTexto
          id="fuente-motivo"
          name="motivo"
          value={formik.values.motivo}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
          invalido={Boolean(err)}
        />
      </Campo>
    </DialogoFormulario>
  );
}

// ── S6 marcar como oficial verificada ─────────────────────────────────────────

export function DialogoMarca({
  titulo,
  onCerrar,
  onConfirmar,
}: {
  titulo: string;
  onCerrar: () => void;
  onConfirmar: (entidad: string, nota: string) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  const mensajes = useMensajes();
  const envio = useErrorEnvio();
  const formik = useFormik({
    initialValues: { entidad_emisora: "", nota: "" },
    validationSchema: useMemo(() => esquemaMarca(mensajes), [mensajes]),
    validateOnMount: true,
    onSubmit: async (v) => {
      envio.limpiar();
      try {
        await onConfirmar(v.entidad_emisora, v.nota);
      } catch (e) {
        envio.mostrar(e);
      }
    },
  });
  const errEntidad = errorDe(formik, "entidad_emisora");
  const errNota = errorDe(formik, "nota");

  return (
    <DialogoFormulario
      open
      onCerrar={onCerrar}
      icon={SealCheckIcon}
      titulo={t("serumsIa.fuentes.marcar.titulo")}
      descripcion={titulo}
      error={envio.mensaje}
      pendiente={formik.isSubmitting}
      puedeGuardar={formik.isValid}
      textoGuardar={t("serumsIa.fuentes.marcar.boton")}
      textoGuardando={t("comun-procesando")}
      textoCancelar={t("comun-cancelar")}
      onGuardar={formik.submitForm}
      ancho="sm:max-w-lg"
    >
      <div className="md:col-span-2">
        <Alerta tono="info">{t("serumsIa.fuentes.marcar.efecto")}</Alerta>
      </div>
      <Campo
        id="fuente-entidad"
        etiqueta={t("serumsIa.fuentes.campo.entidad")}
        error={errEntidad}
        className="md:col-span-2"
      >
        <Entrada
          id="fuente-entidad"
          name="entidad_emisora"
          value={formik.values.entidad_emisora}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
          invalido={Boolean(errEntidad)}
        />
      </Campo>
      <Campo id="fuente-nota" etiqueta={t("serumsIa.fuentes.campo.nota")} error={errNota} className="md:col-span-2">
        <AreaTexto
          id="fuente-nota"
          name="nota"
          value={formik.values.nota}
          onChange={formik.handleChange}
          onBlur={formik.handleBlur}
          invalido={Boolean(errNota)}
        />
      </Campo>
    </DialogoFormulario>
  );
}
