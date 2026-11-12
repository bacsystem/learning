"use client";

import { AccionesRevision } from "../_componentes/acciones-revision";
import { BuscadorPasajes } from "../_componentes/buscador-pasajes";
import { CitasPregunta } from "../_componentes/citas-pregunta";
import { PillEstadoPregunta } from "../_componentes/estado-pregunta";
import { useRevisionPregunta } from "../_componentes/use-revision-pregunta";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMemo } from "react";

import { BotonReintentar } from "@/components/app/boton-reintentar";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { claveErrorPregunta } from "@/lib/serums-ia/revision/errores";
import { idsDeCitas, MAX_CITAS } from "@/lib/serums-ia/revision/reglas";

/** Detalle de una pregunta para revisarla: citas actuales y buscador de pasajes (SIA-E3-08). */
export default function DetallePreguntaPage() {
  const t = useTranslations("dashboard");
  const params = useParams<{ id: string }>();
  const preguntaId = Number(params.id);
  const { pregunta, carga, guardando, error, cargar, anadir, quitar, cambiarEstado } = useRevisionPregunta(preguntaId);
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

      <AccionesRevision key={pregunta.estado} pregunta={pregunta} ocupado={guardando} onCambiar={cambiarEstado} />
    </div>
  );
}
