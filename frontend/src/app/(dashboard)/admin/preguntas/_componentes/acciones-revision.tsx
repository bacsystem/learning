"use client";

import { PillEstadoPregunta } from "./estado-pregunta";
import { useTranslations } from "next-intl";
import { useId, useState } from "react";

import { Campo } from "@/components/app/campo";
import { AreaTexto } from "@/components/formularios/campo";
import { Casilla } from "@/components/formularios/casilla";
import { BotonAsync } from "@/components/patrones/boton-async";
import { Chip } from "@/components/patrones/pill-estado";
import { BOTON_DESTRUCTIVO, BOTON_PRIMARIO, BOTON_SECUNDARIO, TARJETA } from "@/lib/estilos";
import type { CambiarEstadoPayload, Pregunta } from "@/lib/questions/api";
import { claveOrigenPregunta } from "@/lib/questions/estados";
import {
  accionesDisponibles,
  CUERPO_REABRIR,
  cuerpoAprobar,
  cuerpoRechazar,
  MOTIVO_MAX,
  MOTIVO_MIN,
  motivoValido,
  NOTA_MAX,
  NOTA_MIN,
  notaObligatoria,
  notaParaAprobar,
} from "@/lib/serums-ia/revision/acciones";

/**
 * Zona de revisión del detalle (SIA-E3-09): solo las acciones válidas para el estado y el origen. Las validaciones
 * replican las del servidor, que sigue siendo la fuente de verdad. Va con `key={estado}` para limpiar los campos tras
 * cada transición.
 */
export function AccionesRevision({
  pregunta,
  ocupado,
  onCambiar,
}: {
  pregunta: Pregunta;
  ocupado: boolean;
  onCambiar: (payload: CambiarEstadoPayload) => void;
}) {
  const t = useTranslations("dashboard");
  const id = useId();
  const [verificado, setVerificado] = useState(false);
  const [nota, setNota] = useState("");
  const [motivo, setMotivo] = useState("");

  const acciones = accionesDisponibles(pregunta.estado, pregunta.origen);
  const origen = pregunta.origen;
  const puedeAprobar = acciones.aprobar || acciones.revalidar;
  const cuerpoAprobacion = origen ? cuerpoAprobar(origen, verificado, nota) : null;
  const cuerpoRechazo = cuerpoRechazar(motivo);
  const exigeNota = origen !== undefined && acciones.revalidar && notaObligatoria(origen);
  // Vacía o demasiado corta: «Revalidar» queda deshabilitado y el campo dice por qué.
  const notaInvalida = exigeNota && origen !== undefined && notaParaAprobar(origen, nota) === undefined;
  const motivoInvalido = motivo.trim() !== "" && !motivoValido(motivo);
  const ayudaNota = notaInvalida ? "error" : exigeNota ? undefined : "ayuda";
  const hayAlgunaAccion = puedeAprobar || acciones.rechazar || acciones.reabrir || acciones.seRevisaEnCola;
  const t_ = (clave: string, valores?: Record<string, string | number>) =>
    t(`serumsIa.revision.acciones.${clave}`, valores);

  return (
    <section aria-labelledby={`${id}-titulo`} className={`${TARJETA} grid gap-3 p-4`}>
      <h2 id={`${id}-titulo`} className="text-titulo text-foreground">
        {t_("titulo")}
      </h2>
      <div className="flex flex-wrap items-center gap-2">
        <PillEstadoPregunta estado={pregunta.estado} />
        {origen ? <Chip>{t(claveOrigenPregunta(origen))}</Chip> : null}
      </div>
      {pregunta.motivo_rechazo ? (
        <p className="text-control text-muted-foreground">
          <span className="font-medium">{t_("motivoRechazo")}: </span>
          {pregunta.motivo_rechazo}
        </p>
      ) : null}

      {acciones.seRevisaEnCola ? <p className="text-control text-foreground">{t_("seRevisaEnCola")}</p> : null}

      {puedeAprobar ? (
        <div className="grid gap-3">
          <Casilla
            id={`${id}-verificado`}
            etiqueta={t_("verificado")}
            checked={verificado}
            onCheckedChange={setVerificado}
            disabled={ocupado}
          />
          {origen ? (
            <Campo
              id={`${id}-nota`}
              etiqueta={t_("nota")}
              opcional={!exigeNota}
              ayuda={exigeNota ? undefined : t_("notaOpcional", { max: NOTA_MAX })}
              error={notaInvalida ? t_("notaObligatoria", { min: NOTA_MIN }) : undefined}
            >
              <AreaTexto
                id={`${id}-nota`}
                value={nota}
                maxLength={NOTA_MAX}
                disabled={ocupado}
                invalido={notaInvalida}
                required={exigeNota}
                onChange={(e) => setNota(e.target.value)}
                aria-describedby={ayudaNota && `${id}-nota-${ayudaNota}`}
              />
            </Campo>
          ) : null}
          <div>
            <BotonAsync
              pendiente={ocupado}
              textoPendiente={t_("procesando")}
              className={BOTON_PRIMARIO}
              disabled={cuerpoAprobacion === null}
              onClick={() => cuerpoAprobacion && onCambiar(cuerpoAprobacion)}
            >
              {t_(acciones.revalidar ? "revalidar" : "aprobar")}
            </BotonAsync>
          </div>
        </div>
      ) : null}

      {acciones.rechazar ? (
        <div className="grid gap-3">
          <Campo
            id={`${id}-motivo`}
            etiqueta={t_("motivo")}
            ayuda={t_("motivoAyuda", { min: MOTIVO_MIN, max: MOTIVO_MAX })}
            error={motivoInvalido ? t_("motivoMinimo", { min: MOTIVO_MIN }) : undefined}
          >
            <AreaTexto
              id={`${id}-motivo`}
              value={motivo}
              maxLength={MOTIVO_MAX}
              disabled={ocupado}
              invalido={motivoInvalido}
              onChange={(e) => setMotivo(e.target.value)}
              aria-describedby={`${id}-motivo-${motivoInvalido ? "error" : "ayuda"}`}
            />
          </Campo>
          <div>
            <BotonAsync
              pendiente={ocupado}
              textoPendiente={t_("procesando")}
              className={BOTON_DESTRUCTIVO}
              disabled={cuerpoRechazo === null}
              onClick={() => cuerpoRechazo && onCambiar(cuerpoRechazo)}
            >
              {t_("rechazar")}
            </BotonAsync>
          </div>
        </div>
      ) : null}

      {acciones.reabrir ? (
        <div className="grid gap-2">
          <p className="text-control text-muted-foreground">{t_("reabrirAyuda")}</p>
          <div>
            <BotonAsync
              pendiente={ocupado}
              textoPendiente={t_("procesando")}
              className={BOTON_SECUNDARIO}
              onClick={() => onCambiar(CUERPO_REABRIR)}
            >
              {t_("reabrir")}
            </BotonAsync>
          </div>
        </div>
      ) : null}

      {hayAlgunaAccion ? null : <p className="text-control text-muted-foreground">{t_("sinAcciones")}</p>}
    </section>
  );
}
