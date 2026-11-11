"use client";

import { FormularioProfesion } from "./formulario-profesion";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { BotonReintentar } from "@/components/app/boton-reintentar";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Chip } from "@/components/patrones/pill-estado";
import { useCarga } from "@/hooks/use-carga";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { apiErrorCode } from "@/lib/http/api-error";
import { profesionApi } from "@/lib/serums-ia/profesion/api";
import { mensajeErrorProfesion } from "@/lib/serums-ia/profesion/errores";
import { cn } from "@/lib/utils";
import { GraduationCapIcon, StethoscopeIcon } from "@phosphor-icons/react";

/** Pantalla de la profesión del postulante (SIA-E1-04): la elegida con sus áreas, y el selector para elegir o cambiar. */
export default function ProfesionPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const opciones = useCarga(token ? "opciones" : null, () => profesionApi.getOpciones(token ?? ""));
  const perfil = useCarga(token ? "perfil" : null, () => profesionApi.getProfesion(token ?? ""));

  // F2: sin profesión elegida (404) no es un error; con la inscripción revocada (403) el perfil existe pero no se usa.
  const codigoPerfil = apiErrorCode(perfil.error);
  const sinProfesion = codigoPerfil === "profesion_no_definida";
  const inscripcionRevocada = codigoPerfil === "inscripcion_requerida";
  const errorPerfil = perfil.error && !sinProfesion && !inscripcionRevocada ? perfil.error : null;
  const activa = perfil.datos;

  async function guardar(seleccion: { procesoId: number; especialidadId: number }, confirmarReinicio: boolean) {
    if (!token) return;
    const guardada = await profesionApi.guardar(token, {
      proceso_id: seleccion.procesoId,
      especialidad_id: seleccion.especialidadId,
      confirmar_reinicio: confirmarReinicio,
    });
    perfil.reemplazar(guardada);
    opciones.recargar();
    toast.ok(t("serumsIa.profesion.guardada", { especialidad: guardada.especialidad.nombre }));
  }

  if (opciones.error || errorPerfil) {
    const error = opciones.error ?? errorPerfil;
    return (
      <div className={CONTENEDOR_PAGINA}>
        <Alerta
          tono="error"
          accion={<BotonReintentar onReintentar={() => Promise.all([opciones.recargar(), perfil.recargar()])} />}
        >
          {mensajeErrorProfesion(error, t, t("serumsIa.profesion.errorCargar"))}
        </Alerta>
      </div>
    );
  }

  // Solo en la primera carga: al recargar tras guardar se siguen viendo los datos (sin parpadeo).
  if (!opciones.datos || (!perfil.datos && !perfil.error)) {
    return (
      <div className={CONTENEDOR_PAGINA} aria-busy="true">
        <Skeleton className="h-28 w-full rounded-xl" />
        <Skeleton className="h-40 w-full rounded-xl" />
      </div>
    );
  }

  if (opciones.datos.procesos.length === 0) {
    return (
      <div className={CONTENEDOR_PAGINA}>
        <EstadoVacio
          icon={GraduationCapIcon}
          titulo={t("serumsIa.profesion.sinInscripcion.titulo")}
          accion={
            <Link href="/procesos" className={ACCION_PRINCIPAL}>
              {t("serumsIa.profesion.sinInscripcion.accion")}
            </Link>
          }
        >
          {t("serumsIa.profesion.sinInscripcion.texto")}
        </EstadoVacio>
      </div>
    );
  }

  return (
    <div className={CONTENEDOR_PAGINA}>
      {inscripcionRevocada ? <Alerta tono="aviso">{t("serumsIa.profesion.inscripcionRevocada")}</Alerta> : null}

      {activa ? (
        <section className={cn(TARJETA, "flex flex-col")} aria-labelledby="profesion-actual">
          <CabeceraSeccion
            icon={StethoscopeIcon}
            titulo={<span id="profesion-actual">{t("serumsIa.profesion.actual.titulo")}</span>}
            subtitulo={`${activa.especialidad.nombre} · ${activa.proceso.nombre}`}
          />
          <div className="flex flex-col gap-3 p-4">
            {activa.pesos.pesos_estimados ? (
              <Alerta tono="info">{t("serumsIa.profesion.actual.estimados")}</Alerta>
            ) : null}
            {activa.pesos.areas.length > 0 ? (
              <ul className="flex flex-wrap gap-2" aria-label={t("serumsIa.profesion.actual.areas")}>
                {activa.pesos.areas.map((a) => (
                  <li key={a.asignatura_id}>
                    <Chip>{t("serumsIa.profesion.actual.area", { nombre: a.nombre, peso: a.peso_porcentual })}</Chip>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-control text-muted-foreground">{t("serumsIa.profesion.actual.sinAreas")}</p>
            )}
          </div>
        </section>
      ) : null}

      <section className={cn(TARJETA, "flex flex-col")} aria-labelledby="profesion-elegir">
        <CabeceraSeccion
          icon={GraduationCapIcon}
          titulo={
            <span id="profesion-elegir">
              {activa ? t("serumsIa.profesion.elegir.tituloCambiar") : t("serumsIa.profesion.elegir.titulo")}
            </span>
          }
        />
        <div className="flex flex-col gap-3 p-4">
          {/* En el cuerpo y no como subtítulo: `CabeceraSeccion` lo trunca a una línea. */}
          <p className="text-control text-muted-foreground">{t("serumsIa.profesion.elegir.ayuda")}</p>
          <FormularioProfesion opciones={opciones.datos} activa={activa} onGuardar={guardar} />
        </div>
      </section>
    </div>
  );
}
