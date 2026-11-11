"use client";

import { FormularioPesos } from "./formulario-pesos";
import { useCarga } from "./use-carga";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { SkeletonTabla } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { SelectCampo } from "@/components/formularios/select-campo";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { PillEstado } from "@/components/patrones/pill-estado";
import { useAuth } from "@/lib/auth/auth-context";
import { type Asignatura, catalogsApi, type Proceso } from "@/lib/catalogs/api";
import { ACCION_SECUNDARIA, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { type AreaPesoPayload, type EstadoPesos, pesosApi } from "@/lib/serums-ia/pesos/api";
import { mensajeErrorPesos } from "@/lib/serums-ia/pesos/errores";
import { cn } from "@/lib/utils";
import { ScalesIcon } from "@phosphor-icons/react";

export default function PesosPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [asignaturas, setAsignaturas] = useState<Asignatura[]>([]);
  const [errorCatalogos, setErrorCatalogos] = useState(false);
  const [procesoId, setProcesoId] = useState(0);
  const [especialidadId, setEspecialidadId] = useState(0);
  // Cambia tras cada guardado: vuelve a montar el formulario con la configuración que devolvió el servidor.
  const [version, setVersion] = useState(0);

  const cargarCatalogos = useCallback(async () => {
    if (!token) return;
    setErrorCatalogos(false);
    try {
      const [p, a] = await Promise.all([
        catalogsApi.listProcesos(token, { estado: "activo", limit: "all" }),
        catalogsApi.listAsignaturas(token, { estado: "activo", limit: "all" }),
      ]);
      setProcesos(p);
      setAsignaturas(a);
    } catch {
      setErrorCatalogos(true);
    }
  }, [token]);

  useEffect(() => {
    cargarCatalogos();
  }, [cargarCatalogos]);

  const resumen = useCarga(token && procesoId ? `p${procesoId}` : null, () =>
    pesosApi.getResumen(token ?? "", procesoId),
  );
  const clavePar = token && procesoId && especialidadId ? `p${procesoId}-e${especialidadId}` : null;
  const configuracion = useCarga(clavePar, () => pesosApi.getConfiguracion(token ?? "", procesoId, especialidadId));

  const etiquetaEstado = useCallback((estado: EstadoPesos) => t(`serumsIa.pesos.estado.${estado}`), [t]);
  const opcionesProceso = useMemo(
    () => procesos.map((p) => ({ value: String(p.id), label: `${p.nombre} (${p.anio})` })),
    [procesos],
  );
  const especialidades = resumen.datos?.especialidades ?? [];
  const opcionesEspecialidad = especialidades.map((e) => ({
    value: String(e.especialidad_id),
    label: `${e.nombre} · ${etiquetaEstado(e.estado)}`,
  }));
  const asignaturasActivas = useMemo(() => asignaturas.map((a) => ({ id: a.id, nombre: a.nombre })), [asignaturas]);

  function elegirProceso(valor: string) {
    setProcesoId(Number(valor) || 0);
    setEspecialidadId(0);
  }

  async function guardar(areas: AreaPesoPayload[]) {
    if (!token) return;
    const guardada = await pesosApi.guardar(token, procesoId, especialidadId, areas);
    configuracion.reemplazar(guardada);
    setVersion((v) => v + 1);
    toast.ok(t("serumsIa.pesos.guardado"));
    resumen.recargar();
  }

  const reintentar = (onClick: () => void) => (
    <button type="button" onClick={onClick} className={ACCION_SECUNDARIA}>
      {t("comun-reintentar")}
    </button>
  );
  const config = configuracion.datos;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {errorCatalogos ? (
        <Alerta tono="error" accion={reintentar(cargarCatalogos)}>
          {t("serumsIa.pesos.errorCatalogos")}
        </Alerta>
      ) : null}

      <div className="flex flex-wrap items-end gap-2">
        <SelectCampo
          id="pesos-proceso"
          etiqueta={t("serumsIa.pesos.filtro.proceso")}
          className="w-full sm:w-auto sm:min-w-72"
          placeholder={t("serumsIa.pesos.filtro.procesoElegir")}
          opciones={opcionesProceso}
          valor={procesoId ? String(procesoId) : ""}
          onCambio={elegirProceso}
        />
        {procesoId && especialidades.length > 0 ? (
          <SelectCampo
            id="pesos-especialidad"
            etiqueta={t("serumsIa.pesos.filtro.especialidad")}
            className="w-full sm:w-auto sm:min-w-64"
            placeholder={t("serumsIa.pesos.filtro.especialidadElegir")}
            opciones={opcionesEspecialidad}
            valor={especialidadId ? String(especialidadId) : ""}
            onCambio={(v) => setEspecialidadId(Number(v) || 0)}
          />
        ) : null}
      </div>

      {!procesoId ? (
        <EstadoVacio icon={ScalesIcon} titulo={t("serumsIa.pesos.sinProceso.titulo")}>
          {t("serumsIa.pesos.sinProceso.texto")}
        </EstadoVacio>
      ) : resumen.error ? (
        <Alerta tono="error" accion={reintentar(resumen.recargar)}>
          {mensajeErrorPesos(resumen.error, t, t("serumsIa.pesos.errorCargar"))}
        </Alerta>
      ) : resumen.cargando && !resumen.datos ? (
        <SkeletonTabla columnas={2} filas={2} />
      ) : especialidades.length === 0 ? (
        <EstadoVacio icon={ScalesIcon} titulo={t("serumsIa.pesos.sinEspecialidades.titulo")}>
          {t("serumsIa.pesos.sinEspecialidades.texto")}
        </EstadoVacio>
      ) : !especialidadId ? (
        <EstadoVacio icon={ScalesIcon} titulo={t("serumsIa.pesos.sinEspecialidad.titulo")}>
          {t("serumsIa.pesos.sinEspecialidad.texto")}
        </EstadoVacio>
      ) : configuracion.error ? (
        <Alerta tono="error" accion={reintentar(configuracion.recargar)}>
          {mensajeErrorPesos(configuracion.error, t, t("serumsIa.pesos.errorCargar"))}
        </Alerta>
      ) : !config ? (
        <SkeletonTabla columnas={4} />
      ) : (
        <section className={cn(TARJETA, "flex flex-col")} aria-labelledby="pesos-titulo">
          <CabeceraSeccion
            icon={ScalesIcon}
            titulo={<span id="pesos-titulo">{t("serumsIa.pesos.titulo", { especialidad: config.especialidad })}</span>}
            subtitulo={t("serumsIa.pesos.ayuda")}
            derecha={
              <PillEstado tono={config.estado === "completa" ? "ok" : "aviso"}>
                {etiquetaEstado(config.estado)}
              </PillEstado>
            }
          />
          <div className="flex flex-col gap-3 p-4">
            {config.estado === "incompleta" ? <Alerta tono="info">{t("serumsIa.pesos.estimados")}</Alerta> : null}
            <FormularioPesos
              key={`${clavePar}-${version}`}
              configuracion={config}
              asignaturas={asignaturasActivas}
              onGuardar={guardar}
            />
          </div>
        </section>
      )}
    </div>
  );
}
