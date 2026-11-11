"use client";

import { BuscadorPasajes } from "../_componentes/buscador-pasajes";
import { CitasPregunta } from "../_componentes/citas-pregunta";
import { PillEstadoPregunta } from "../_componentes/estado-pregunta";
import { useCitasPregunta } from "../_componentes/use-citas-pregunta";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMemo } from "react";

import { BotonReintentar } from "@/components/app/boton-reintentar";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { Chip } from "@/components/patrones/pill-estado";
import { CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { claveOrigenPregunta } from "@/lib/questions/estados";
import { claveErrorPregunta } from "@/lib/serums-ia/revision/errores";
import { idsDeCitas, MAX_CITAS } from "@/lib/serums-ia/revision/reglas";

/** Detalle de una pregunta para revisarla: citas actuales y buscador de pasajes (SIA-E3-08). */
export default function DetallePreguntaPage() {
  const t = useTranslations("dashboard");
  const params = useParams<{ id: string }>();
  const preguntaId = Number(params.id);
  const { pregunta, carga, guardando, error, cargar, anadir, quitar } = useCitasPregunta(preguntaId);
  const citas = useMemo(() => pregunta?.citas ?? [], [pregunta]);
  const citados = useMemo(() => new Set(idsDeCitas(citas)), [citas]);

  const volver = <EnlaceVolver href="/admin/preguntas">{t("serumsIa.revision.citas.volver")}</EnlaceVolver>;
  const titulo = t("serumsIa.revision.citas.tituloPagina", { id: preguntaId });

  if (carga === "cargando") {
    return (
      <div className={CONTENEDOR_PAGINA} aria-busy="true" aria-label={t("serumsIa.revision.citas.cargando")}>
        <CabeceraPagina volver={volver} titulo={titulo} />
        <div className={`${TARJETA} grid gap-3 p-4`}>
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-20 w-full rounded-lg" />
        </div>
      </div>
    );
  }

  if (!pregunta) {
    const noExiste = carga === "no_encontrada";
    return (
      <div className={CONTENEDOR_PAGINA}>
        <CabeceraPagina volver={volver} titulo={titulo} />
        <Alerta tono="error" accion={noExiste ? undefined : <BotonReintentar onReintentar={cargar} />}>
          {t(noExiste ? "serumsIa.revision.citas.noEncontrada" : "serumsIa.revision.citas.errorCarga")}
        </Alerta>
      </div>
    );
  }

  return (
    <div className={CONTENEDOR_PAGINA}>
      <CabeceraPagina volver={volver} titulo={titulo} acciones={<PillEstadoPregunta estado={pregunta.estado} />} />

      <section className={`${TARJETA} grid gap-2 p-4`} aria-labelledby="enunciado-titulo">
        <h2 id="enunciado-titulo" className="text-titulo text-foreground">
          {t("serumsIa.revision.citas.enunciado")}
        </h2>
        <p className="text-cuerpo text-foreground whitespace-pre-wrap">{pregunta.enunciado}</p>
      </section>

      {error ? <Alerta tono="error">{t(claveErrorPregunta(error, t.has), { maximo: MAX_CITAS })}</Alerta> : null}

      <CitasPregunta citas={citas} ocupado={guardando} onQuitar={quitar} />
      <BuscadorPasajes citados={citados} llena={citas.length >= MAX_CITAS} ocupado={guardando} onAnadir={anadir} />

      {/* Zona de revisión: SIA-E3-09 añade aquí las acciones (aprobar, revalidar, rechazar, reabrir). */}
      <section aria-labelledby="revision-titulo" className={`${TARJETA} grid gap-2 p-4`}>
        <h2 id="revision-titulo" className="text-titulo text-foreground">
          {t("serumsIa.revision.citas.acciones.titulo")}
        </h2>
        <div className="flex flex-wrap items-center gap-2">
          <PillEstadoPregunta estado={pregunta.estado} />
          {pregunta.origen ? <Chip>{t(claveOrigenPregunta(pregunta.origen))}</Chip> : null}
        </div>
        {pregunta.motivo_rechazo ? (
          <p className="text-control text-muted-foreground">
            <span className="font-medium">{t("serumsIa.revision.citas.acciones.motivoRechazo")}: </span>
            {pregunta.motivo_rechazo}
          </p>
        ) : null}
      </section>
    </div>
  );
}
