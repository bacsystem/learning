"use client";

import { CeldaNombre, DialogoConfirmarAccion } from "../_catalogos/componentes";
import { FormularioTema } from "./formulario-tema";
import { ModoReordenar } from "./modo-reordenar";
import { useTemas } from "./use-temas";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";

import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { useToast } from "@/components/feedback/toast";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { Tabs } from "@/components/navegacion/tabs";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { useAuth } from "@/lib/auth/auth-context";
import { type Asignatura, catalogsApi, type Especialidad } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { type EstadoTema, type Tema, type TemaPayload, temasApi } from "@/lib/temas/api";
import { mensajeErrorTema } from "@/lib/temas/errores";
import {
  ArchiveIcon,
  ArrowCounterClockwiseIcon,
  ArrowsDownUpIcon,
  PencilSimpleIcon,
  PlusIcon,
  TreeStructureIcon,
} from "@phosphor-icons/react";

const TABS = ["todos", "activo", "archivado"] as const;
type TabTema = (typeof TABS)[number];

/** Formulario abierto: `tema` es `null` al crear. */
type DialogoTema = { tema: Tema | null } | null;

/** Tamaño de página inicial del listado (E1-02 regla 2). */
const POR_PAGINA_TEMAS = 20;

export default function TemasPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [asignaturas, setAsignaturas] = useState<Asignatura[]>([]);
  const [especialidades, setEspecialidades] = useState<Especialidad[]>([]);
  const [errorCatalogos, setErrorCatalogos] = useState(false);
  const [asignaturaId, setAsignaturaId] = useState(0);
  const [especialidadId, setEspecialidadId] = useState(0);
  const [tab, setTab] = useState<TabTema>("todos");
  const [reordenando, setReordenando] = useState(false);
  const [dialogo, setDialogo] = useState<DialogoTema>(null);
  // `tema` se conserva al cerrar para que el diálogo no cambie de textos durante su animación de salida.
  const [cambioEstado, setCambioEstado] = useState<{ open: boolean; tema: Tema | null }>({ open: false, tema: null });

  const cargarCatalogos = useCallback(async () => {
    if (!token) return;
    setErrorCatalogos(false);
    try {
      const [a, e] = await Promise.all([
        catalogsApi.listAsignaturas(token, { estado: "activo", limit: "all" }),
        catalogsApi.listEspecialidades(token, { estado: "activo", limit: "all" }),
      ]);
      setAsignaturas(a);
      setEspecialidades(e);
    } catch {
      setErrorCatalogos(true);
    }
  }, [token]);

  useEffect(() => {
    cargarCatalogos();
  }, [cargarCatalogos]);

  const filtro = useMemo(
    () => (asignaturaId ? { asignaturaId, especialidadId: especialidadId || undefined } : null),
    [asignaturaId, especialidadId],
  );
  const { temas, cargando, error, recargar } = useTemas(token, filtro);

  const filas = useMemo(() => (tab === "todos" ? temas : temas.filter((x) => x.estado === tab)), [temas, tab]);
  const activos = useMemo(() => temas.filter((x) => x.estado === "activo").length, [temas]);
  const asignatura = asignaturas.find((a) => a.id === asignaturaId);

  const opcionesAsignatura = useMemo(
    () => asignaturas.map((a) => ({ value: String(a.id), label: a.nombre })),
    [asignaturas],
  );
  const opcionesEspecialidad = useMemo(
    () => [
      { value: "0", label: t("serumsIa.temas.filtro.especialidadTodas") },
      ...especialidades.map((e) => ({ value: String(e.id), label: e.nombre })),
    ],
    [especialidades, t],
  );
  const especialidadesTema = useMemo(
    () => especialidades.map((e) => ({ id: e.id, nombre: e.nombre })),
    [especialidades],
  );

  const etiquetaEstado = useCallback((estado: EstadoTema) => t(`serumsIa.temas.estado.${estado}`), [t]);

  function elegirAsignatura(valor: string) {
    setAsignaturaId(Number(valor) || 0);
    setReordenando(false);
  }

  async function guardarTema(payload: TemaPayload) {
    if (!token || !dialogo) return;
    if (dialogo.tema) {
      await temasApi.updateTema(token, dialogo.tema.id, payload);
      toast.ok(t("serumsIa.temas.actualizado"));
    } else {
      await temasApi.createTema(token, { ...payload, asignatura_id: asignaturaId });
      toast.ok(t("serumsIa.temas.creado"));
    }
    setDialogo(null);
    recargar();
  }

  async function confirmarCambioEstado(): Promise<string | null> {
    const tema = cambioEstado.tema;
    if (!token || !tema) return null;
    const archivar = tema.estado === "activo";
    try {
      await temasApi.updateTemaEstado(token, tema.id, archivar ? "archivado" : "activo");
      toast.ok(archivar ? t("serumsIa.temas.archivado") : t("serumsIa.temas.reactivado"));
      recargar();
      return null;
    } catch (e) {
      return mensajeErrorTema(e, t, t("comun-error-estado"));
    }
  }

  function ordenGuardado() {
    toast.ok(t("serumsIa.temas.reordenar.guardado"));
    setReordenando(false);
    recargar();
  }

  const columnas = useMemo<ColumnaListado<Tema>[]>(
    () => [
      {
        id: "orden",
        titulo: t("serumsIa.temas.col.orden"),
        ancho: "64px",
        ordenable: true,
        texto: (r) => (r.estado === "activo" ? r.orden : ""),
        render: (r) =>
          r.estado === "activo" ? (
            <span className="font-mono tabular-nums">{r.orden}</span>
          ) : (
            <span className="text-texto-terciario">—</span>
          ),
      },
      {
        id: "nombre",
        titulo: t("serumsIa.temas.col.nombre"),
        ordenable: true,
        render: (r) => (
          <div className="grid gap-0.5">
            <CeldaNombre>{r.nombre}</CeldaNombre>
            {r.descripcion ? (
              <span className="text-etiqueta text-muted-foreground line-clamp-2">{r.descripcion}</span>
            ) : null}
          </div>
        ),
      },
      {
        id: "especialidades",
        titulo: t("serumsIa.temas.col.especialidades"),
        texto: (r) =>
          r.aplica_a_todas ? t("serumsIa.temas.aplicaATodas") : r.especialidades.map((e) => e.nombre).join(", "),
        render: (r) =>
          r.aplica_a_todas ? (
            <span className="text-muted-foreground">{t("serumsIa.temas.aplicaATodas")}</span>
          ) : (
            <div className="flex flex-wrap gap-1">
              {r.especialidades.map((e) => (
                <Chip key={e.id}>{e.nombre}</Chip>
              ))}
            </div>
          ),
      },
      {
        id: "preguntas_count",
        titulo: t("serumsIa.temas.col.preguntas"),
        ancho: "110px",
        alinear: "derecha",
        ordenable: true,
        render: (r) => <span className="font-mono tabular-nums">{r.preguntas_count}</span>,
      },
      {
        id: "estado",
        titulo: t("serumsIa.temas.col.estado"),
        ancho: "120px",
        ordenable: true,
        texto: (r) => etiquetaEstado(r.estado),
        render: (r) => (
          <PillEstado tono={r.estado === "activo" ? "ok" : "neutro"}>{etiquetaEstado(r.estado)}</PillEstado>
        ),
      },
      {
        id: "acciones",
        titulo: "",
        ancho: "56px",
        alinear: "derecha",
        exportable: false,
        ocultable: false,
        render: (r) => (
          <MenuAcciones
            etiqueta={t("serumsIa.temas.acciones", { nombre: r.nombre })}
            acciones={[
              { label: t("comun-editar"), icon: PencilSimpleIcon, onClick: () => setDialogo({ tema: r }) },
              r.estado === "activo"
                ? {
                    label: t("serumsIa.temas.archivar"),
                    icon: ArchiveIcon,
                    separador: true,
                    onClick: () => setCambioEstado({ open: true, tema: r }),
                  }
                : {
                    label: t("serumsIa.temas.reactivar"),
                    icon: ArrowCounterClockwiseIcon,
                    separador: true,
                    onClick: () => setCambioEstado({ open: true, tema: r }),
                  },
            ]}
          />
        ),
      },
    ],
    [etiquetaEstado, t],
  );

  const sinTemas = Boolean(asignaturaId) && !cargando && !error && temas.length === 0 && !especialidadId;
  const archivando = cambioEstado.tema?.estado === "activo";
  const botonCrear = (
    <button type="button" onClick={() => setDialogo({ tema: null })} className={ACCION_PRINCIPAL}>
      <PlusIcon weight="bold" className="size-4" />
      {t("serumsIa.temas.crear")}
    </button>
  );

  return (
    <div className={CONTENEDOR_PAGINA}>
      {errorCatalogos ? (
        <Alerta
          tono="error"
          accion={
            <button type="button" onClick={cargarCatalogos} className={ACCION_SECUNDARIA}>
              {t("comun-reintentar")}
            </button>
          }
        >
          {t("serumsIa.temas.errorCatalogos")}
        </Alerta>
      ) : null}

      <div className="flex flex-wrap items-end gap-2">
        <SelectCampo
          id="temas-asignatura"
          etiqueta={t("serumsIa.temas.filtro.asignatura")}
          className="w-full sm:w-auto sm:min-w-64"
          placeholder={t("serumsIa.temas.filtro.asignaturaElegir")}
          opciones={opcionesAsignatura}
          valor={asignaturaId ? String(asignaturaId) : ""}
          onCambio={elegirAsignatura}
          disabled={reordenando}
        />
        {asignaturaId && !reordenando ? (
          <SelectCampo
            id="temas-especialidad"
            etiqueta={t("serumsIa.temas.filtro.especialidad")}
            className="w-full sm:w-auto sm:min-w-56"
            opciones={opcionesEspecialidad}
            valor={String(especialidadId)}
            onCambio={(v) => setEspecialidadId(Number(v) || 0)}
          />
        ) : null}
      </div>

      {!asignaturaId ? (
        <EstadoVacio icon={TreeStructureIcon} titulo={t("serumsIa.temas.sinAsignatura.titulo")}>
          {t("serumsIa.temas.sinAsignatura.texto")}
        </EstadoVacio>
      ) : reordenando && token ? (
        <ModoReordenar
          token={token}
          asignaturaId={asignaturaId}
          onSalir={() => setReordenando(false)}
          onGuardado={ordenGuardado}
        />
      ) : (
        <>
          {error ? (
            <Alerta
              tono="error"
              accion={
                <button type="button" onClick={recargar} className={ACCION_SECUNDARIA}>
                  {t("comun-reintentar")}
                </button>
              }
            >
              {mensajeErrorTema(error, t, t("serumsIa.temas.errorCargar"))}
            </Alerta>
          ) : null}

          {sinTemas ? (
            <EstadoVacio icon={TreeStructureIcon} titulo={t("serumsIa.temas.vacio.titulo")} accion={botonCrear}>
              {t("serumsIa.temas.vacio.texto")}
            </EstadoVacio>
          ) : (
            <TablaListado
              columnas={columnas}
              filas={filas}
              clave={(r) => String(r.id)}
              titulo={t("serumsIa.temas.titulo")}
              placeholderBusqueda={t("serumsIa.temas.buscar")}
              unidad={t("serumsIa.temas.unidad")}
              cargando={cargando && temas.length === 0}
              vacio={t("serumsIa.temas.vacioFiltros")}
              ordenInicial={{ id: "orden", dir: "asc" }}
              porPaginaInicial={POR_PAGINA_TEMAS}
              reinicioPagina={`${asignaturaId}-${especialidadId}-${tab}`}
              filtros={
                <Tabs
                  items={TABS.map((id) => ({
                    id,
                    etiqueta: id === "todos" ? t("serumsIa.temas.filtro.todos") : etiquetaEstado(id),
                    contador: id === "todos" ? temas.length : temas.filter((x) => x.estado === id).length,
                  }))}
                  valor={tab}
                  onCambio={setTab}
                  estilo="segmentado"
                  className="[&_[role=tabpanel]]:hidden"
                />
              }
              acciones={
                <>
                  <button
                    type="button"
                    onClick={() => setReordenando(true)}
                    disabled={activos < 2}
                    title={activos < 2 ? t("serumsIa.temas.reordenar.minimo") : undefined}
                    className={ACCION_SECUNDARIA}
                  >
                    <ArrowsDownUpIcon className="size-4" />
                    {t("serumsIa.temas.reordenar.boton")}
                  </button>
                  {botonCrear}
                </>
              }
            />
          )}
        </>
      )}

      {dialogo && asignatura ? (
        <FormularioTema
          tema={dialogo.tema}
          asignatura={asignatura.nombre}
          especialidades={especialidadesTema}
          onCerrar={() => setDialogo(null)}
          onGuardar={guardarTema}
        />
      ) : null}

      <DialogoConfirmarAccion
        open={cambioEstado.open}
        onCerrar={() => setCambioEstado((c) => ({ ...c, open: false }))}
        icon={archivando ? ArchiveIcon : ArrowCounterClockwiseIcon}
        tono={archivando ? "aviso" : "ok"}
        titulo={archivando ? t("serumsIa.temas.archivarTitulo") : t("serumsIa.temas.reactivarTitulo")}
        descripcion={
          archivando
            ? t("serumsIa.temas.archivarConfirm", { n: cambioEstado.tema?.preguntas_count ?? 0 })
            : t("serumsIa.temas.reactivarConfirm")
        }
        registro={cambioEstado.tema?.nombre}
        detalle={asignatura?.nombre}
        textoConfirmar={archivando ? t("serumsIa.temas.archivar") : t("serumsIa.temas.reactivar")}
        textoPendiente={t("comun-procesando")}
        textoCancelar={t("comun-cancelar")}
        onConfirmar={confirmarCambioEstado}
      />
    </div>
  );
}
