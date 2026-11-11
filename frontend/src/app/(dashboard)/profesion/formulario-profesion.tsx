"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { SelectCampo } from "@/components/formularios/select-campo";
import { DialogoConfirmacion } from "@/components/navegacion/dialogo-confirmacion";
import { BotonAsync } from "@/components/patrones/boton-async";
import { ACCION_PRINCIPAL } from "@/lib/estilos";
import { apiErrorCode } from "@/lib/http/api-error";
import type { OpcionesProfesion, Profesion } from "@/lib/serums-ia/profesion/api";
import { mensajeErrorProfesion } from "@/lib/serums-ia/profesion/errores";
import {
  cambiarProceso,
  especialidadesDe,
  puedeGuardar,
  type Seleccion,
  seleccionInicial,
} from "@/lib/serums-ia/profesion/seleccion";
import { ArrowsClockwiseIcon } from "@phosphor-icons/react";

/**
 * Selector de proceso y profesión (F1 → F3). Abre con la sugerencia del servidor; no guarda nada hasta que el usuario
 * pulsa «Guardar». Si ya tenía otra profesión, el servidor pide confirmar el reinicio (409) y se reenvía con la
 * confirmación.
 */
export function FormularioProfesion({
  opciones,
  activa,
  onGuardar,
}: {
  opciones: OpcionesProfesion;
  /** Perfil activo, o `null` si aún no eligió. */
  activa: Profesion | null;
  /** Llama a F3; si rechaza, el error se muestra sin perder la selección. */
  onGuardar: (seleccion: { procesoId: number; especialidadId: number }, confirmarReinicio: boolean) => Promise<void>;
}) {
  const t = useTranslations("dashboard");
  // `null` = el usuario aún no tocó nada: se muestra la selección inicial (derivada, no copiada a un estado).
  const [elegida, setElegida] = useState<Seleccion | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmando, setConfirmando] = useState(false);

  const { procesos } = opciones;
  const seleccion = elegida ?? seleccionInicial(opciones);
  const especialidades = especialidadesDe(procesos, seleccion.procesoId);
  const parActivo = activa ? { procesoId: activa.proceso.id, especialidadId: activa.especialidad.id } : null;
  const sugerida = !activa && elegida === null && opciones.preseleccion !== null;

  const nombreProceso = (id: number | null) => procesos.find((p) => p.id === id)?.nombre ?? "";
  const nombreEspecialidad = (id: number | null) => especialidades.find((e) => e.id === id)?.nombre ?? "";

  async function guardar(confirmarReinicio: boolean): Promise<string | null> {
    if (seleccion.procesoId === null || seleccion.especialidadId === null) return null;
    setError(null);
    try {
      await onGuardar({ procesoId: seleccion.procesoId, especialidadId: seleccion.especialidadId }, confirmarReinicio);
      setElegida(null);
      return null;
    } catch (e) {
      if (!confirmarReinicio && apiErrorCode(e) === "confirmacion_requerida") {
        setConfirmando(true);
        return null;
      }
      const mensaje = mensajeErrorProfesion(e, t, t("comun-error-guardar"));
      if (confirmarReinicio) return mensaje;
      setError(mensaje);
      return null;
    }
  }

  async function enviar() {
    setGuardando(true);
    try {
      await guardar(false);
    } finally {
      setGuardando(false);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      {sugerida ? <Alerta tono="info">{t("serumsIa.profesion.sugerida")}</Alerta> : null}
      <div className="grid gap-3 sm:grid-cols-2">
        <SelectCampo
          id="profesion-proceso"
          etiqueta={t("serumsIa.profesion.campo.proceso")}
          placeholder={t("serumsIa.profesion.campo.procesoElegir")}
          opciones={procesos.map((p) => ({ value: String(p.id), label: `${p.nombre} (${p.anio})` }))}
          valor={seleccion.procesoId ? String(seleccion.procesoId) : ""}
          onCambio={(v) => setElegida(cambiarProceso(procesos, seleccion, Number(v) || null))}
          disabled={procesos.length === 1}
        />
        <SelectCampo
          id="profesion-especialidad"
          etiqueta={t("serumsIa.profesion.campo.especialidad")}
          placeholder={t("serumsIa.profesion.campo.especialidadElegir")}
          opciones={especialidades.map((e) => ({
            value: String(e.id),
            label: e.pesos === "estimada" ? t("serumsIa.profesion.campo.conEstimados", { nombre: e.nombre }) : e.nombre,
          }))}
          valor={seleccion.especialidadId ? String(seleccion.especialidadId) : ""}
          onCambio={(v) => setElegida({ ...seleccion, especialidadId: Number(v) || null })}
          disabled={seleccion.procesoId === null}
        />
      </div>
      {error ? <Alerta tono="error">{error}</Alerta> : null}
      <div className="flex justify-end">
        <BotonAsync
          pendiente={guardando}
          textoPendiente={t("comun-guardando")}
          disabled={!puedeGuardar(seleccion, parActivo)}
          onClick={enviar}
          className={ACCION_PRINCIPAL}
        >
          {activa ? t("serumsIa.profesion.cambiar") : t("serumsIa.profesion.guardar")}
        </BotonAsync>
      </div>

      <DialogoConfirmacion
        open={confirmando}
        onOpenChange={setConfirmando}
        icon={ArrowsClockwiseIcon}
        titulo={t("serumsIa.profesion.confirmar.titulo")}
        descripcion={
          activa
            ? t("serumsIa.profesion.confirmar.texto", {
                actual: `${activa.especialidad.nombre} · ${activa.proceso.nombre}`,
                nueva: `${nombreEspecialidad(seleccion.especialidadId)} · ${nombreProceso(seleccion.procesoId)}`,
              })
            : t("serumsIa.profesion.confirmar.textoGenerico")
        }
        textoConfirmar={t("serumsIa.profesion.confirmar.boton")}
        textoCancelar={t("comun-cancelar")}
        textoPendiente={t("comun-guardando")}
        etiquetaCerrar={t("comun-cerrar")}
        onConfirmar={() => guardar(true)}
      />
    </div>
  );
}
