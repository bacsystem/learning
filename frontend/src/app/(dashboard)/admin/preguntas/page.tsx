"use client";
import { PillEstadoPregunta } from "./_componentes/estado-pregunta";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { DialogContent } from "@/components/app/dialogo";
import { PieTabla } from "@/components/app/pie-tabla";
import { Alerta } from "@/components/feedback/alerta";
import { SkeletonTabla } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { Buscador } from "@/components/formularios/buscador";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { CONTENEDOR_TABLA, FilaVacia } from "@/components/patrones/pie-tabla";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Dialog } from "@/components/ui/dialog";
import { Menu, MenuContent, MenuGroup, MenuGroupLabel, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAuth } from "@/lib/auth/auth-context";
import { Asignatura, catalogsApi, Especialidad, Grado, Proceso } from "@/lib/catalogs/api";
import {
  ACCION_PRINCIPAL,
  ACCION_SECUNDARIA,
  BOTON_DESTRUCTIVO,
  BOTON_SECUNDARIO,
  CABECERA_TABLA,
  CONTENEDOR_PAGINA,
} from "@/lib/estilos";
import { type ColumnaExportable, descargarCsv, imprimirTabla } from "@/lib/exportar";
import { POR_PAGINA_DEFECTO } from "@/lib/paginacion";
import { type EstadoPregunta, type OrigenPregunta, Pregunta, questionsApi } from "@/lib/questions/api";
import {
  claveErrorEliminacion,
  claveEstadoPregunta,
  claveOrigenPregunta,
  ESTADOS_FILTRABLES,
  ORIGENES_PREGUNTA,
  puedeEliminar,
} from "@/lib/questions/estados";
import { cn } from "@/lib/utils";
import {
  ArrowClockwiseIcon,
  BookOpenTextIcon,
  CheckIcon,
  ColumnsIcon,
  DownloadSimpleIcon,
  FileCsvIcon,
  ListChecksIcon,
  PencilSimpleIcon,
  PlusIcon,
  PrinterIcon,
  TrashIcon,
  UploadSimpleIcon,
} from "@phosphor-icons/react";

/** Tono y clave i18n de cada dificultad. */
const DIFICULTAD: Record<string, { tono: "ok" | "aviso" | "error"; etiqueta: string }> = {
  facil: { tono: "ok", etiqueta: "admin-preguntas-dificultad-facil" },
  medio: { tono: "aviso", etiqueta: "admin-preguntas-dificultad-medio" },
  dificil: { tono: "error", etiqueta: "admin-preguntas-dificultad-dificil" },
};

/** Valor de una columna sin dato (backend anterior a SIA-E3-02). */
const SIN_DATO = "—";

/** Clave i18n de cada tipo. */
const TIPO: Record<string, string> = {
  unica: "admin-preguntas-import-tipo-unica-corto",
  multiple: "admin-preguntas-import-tipo-multiple-corto",
};

/** Espera tras la última tecla antes de buscar (H26). */
const DEBOUNCE_BUSQUEDA_MS = 300;

type Columna = {
  id: string;
  titulo: string;
  texto: (p: Pregunta) => string;
  render: (p: Pregunta) => React.ReactNode;
  className?: string;
  ancho?: string;
};

export default function PreguntasPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const router = useRouter();
  const toast = useToast();

  const [rows, setRows] = useState<Pregunta[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(POR_PAGINA_DEFECTO);
  const [loading, setLoading] = useState(true);
  const [cargado, setCargado] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [asignaturas, setAsignaturas] = useState<Asignatura[]>([]);
  const [grados, setGrados] = useState<Grado[]>([]);
  const [especialidades, setEspecialidades] = useState<Especialidad[]>([]);
  const [filterProcesoID, setFilterProcesoID] = useState<number>(0);
  const [filterAsignaturaID, setFilterAsignaturaID] = useState<number>(0);
  const [filterGradoID, setFilterGradoID] = useState<number>(0);
  const [filterEspecialidadID, setFilterEspecialidadID] = useState<number>(0);
  const [filterDificultad, setFilterDificultad] = useState<string>("");
  const [filterEstado, setFilterEstado] = useState<Exclude<EstadoPregunta, "eliminada"> | "">("");
  const [filterOrigen, setFilterOrigen] = useState<OrigenPregunta | "">("");
  const [filterSearch, setFilterSearch] = useState<string>("");
  /** Texto de búsqueda ya estabilizado: es el que dispara la petición. */
  const [busqueda, setBusqueda] = useState<string>("");
  const [ocultas, setOcultas] = useState<string[]>([]);

  const [confirmDelete, setConfirmDelete] = useState<{
    open: boolean;
    row: Pregunta | null;
    submitting: boolean;
  }>({ open: false, row: null, submitting: false });

  // Contador de peticiones: solo se aplica la respuesta de la última (H26).
  const reqRef = useRef(0);

  useEffect(() => {
    const v = filterSearch.trim();
    if (v === busqueda) return;
    const id = setTimeout(() => {
      setBusqueda(v);
      setPage(0);
    }, DEBOUNCE_BUSQUEDA_MS);
    return () => clearTimeout(id);
  }, [filterSearch, busqueda]);

  const load = useCallback(async () => {
    if (!token) return;
    const reqId = ++reqRef.current;
    setLoading(true);
    setError(null);
    try {
      const result = await questionsApi.listPreguntas(token, {
        proceso_id: filterProcesoID || undefined,
        asignatura_id: filterAsignaturaID || undefined,
        grado_id: filterGradoID || undefined,
        especialidad_id: filterEspecialidadID || undefined,
        dificultad: filterDificultad || undefined,
        estado: filterEstado || undefined,
        origen: filterOrigen || undefined,
        search: busqueda || undefined,
        page: page + 1,
        per_page: pageSize,
      });
      if (reqId !== reqRef.current) return;
      setRows(result.data);
      setTotal(result.total);
    } catch {
      if (reqId !== reqRef.current) return;
      setError(t("admin-error-cargar-preguntas"));
    } finally {
      if (reqId === reqRef.current) {
        setLoading(false);
        setCargado(true);
      }
    }
  }, [
    token,
    filterProcesoID,
    filterAsignaturaID,
    filterGradoID,
    filterEspecialidadID,
    filterDificultad,
    filterEstado,
    filterOrigen,
    busqueda,
    page,
    pageSize,
    t,
  ]);

  useEffect(() => {
    if (!token) return;
    catalogsApi
      .listProcesos(token, {})
      .then(setProcesos)
      .catch(() => {});
    catalogsApi
      .listGrados(token, {})
      .then(setGrados)
      .catch(() => {});
    catalogsApi
      .listEspecialidades(token, {})
      .then(setEspecialidades)
      .catch(() => {});
  }, [token]);

  useEffect(() => {
    if (!token || !filterProcesoID) {
      setAsignaturas([]);
      setFilterAsignaturaID(0);
      return;
    }
    catalogsApi
      .listAsignaturas(token, {})
      .then(setAsignaturas)
      .catch(() => {});
  }, [token, filterProcesoID]);

  useEffect(() => {
    load();
  }, [load]);

  const handleDelete = async () => {
    if (!token || !confirmDelete.row) return;
    setConfirmDelete((d) => ({ ...d, submitting: true }));
    try {
      await questionsApi.deletePregunta(token, confirmDelete.row.id);
      toast.ok(t("admin-preguntas-eliminada"));
      setConfirmDelete({ open: false, row: null, submitting: false });
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      const clave = claveErrorEliminacion(e);
      toast.error(clave ? t(clave) : (err.message ?? t("comun-error-eliminar")));
      if (clave) {
        // El estado cambió mientras tanto: se cierra el diálogo y se muestra el actual.
        setConfirmDelete({ open: false, row: null, submitting: false });
        load();
        return;
      }
      setConfirmDelete((d) => ({ ...d, submitting: false }));
    }
  };

  const columns = useMemo<Columna[]>(
    () => [
      {
        id: "enunciado",
        titulo: t("admin-preguntas-enunciado"),
        texto: (p) => p.enunciado,
        className: "max-w-0 w-full",
        render: (p) => (
          <span className="text-foreground block truncate font-medium" title={p.enunciado}>
            {p.enunciado}
          </span>
        ),
      },
      { id: "proceso", titulo: t("admin-preguntas-proceso"), texto: (p) => p.proceso, render: (p) => p.proceso },
      {
        id: "asignatura",
        titulo: t("admin-preguntas-asignatura"),
        texto: (p) => p.asignatura,
        render: (p) => p.asignatura,
      },
      {
        id: "dificultad",
        titulo: t("admin-preguntas-dificultad"),
        texto: (p) => (DIFICULTAD[p.dificultad] ? t(DIFICULTAD[p.dificultad].etiqueta) : p.dificultad),
        render: (p) => {
          const d = DIFICULTAD[p.dificultad];
          return d ? <PillEstado tono={d.tono}>{t(d.etiqueta)}</PillEstado> : <Chip>{p.dificultad}</Chip>;
        },
      },
      {
        id: "tipo",
        titulo: t("admin-preguntas-tipo"),
        texto: (p) => (TIPO[p.tipo] ? t(TIPO[p.tipo]) : p.tipo),
        render: (p) => <Chip>{TIPO[p.tipo] ? t(TIPO[p.tipo]) : p.tipo}</Chip>,
      },
      {
        id: "estado",
        titulo: t("admin-preguntas-estado"),
        texto: (p) => t(claveEstadoPregunta(p.estado)),
        render: (p) => <PillEstadoPregunta estado={p.estado} />,
      },
      {
        id: "origen",
        titulo: t("serumsIa.revision.listado.columnaOrigen"),
        texto: (p) => (p.origen ? t(claveOrigenPregunta(p.origen)) : SIN_DATO),
        render: (p) => (p.origen ? <Chip>{t(claveOrigenPregunta(p.origen))}</Chip> : SIN_DATO),
      },
      {
        id: "citas",
        titulo: t("serumsIa.revision.listado.columnaCitas"),
        texto: (p) => (p.citas_count === undefined ? SIN_DATO : String(p.citas_count)),
        className: "text-right font-mono tabular-nums",
        render: (p) => (p.citas_count === undefined ? SIN_DATO : p.citas_count),
      },
    ],
    [t],
  );

  const visibles = columns.filter((c) => !ocultas.includes(c.id));
  const exportables: ColumnaExportable<Pregunta>[] = visibles.map((c) => ({ titulo: c.titulo, valor: c.texto }));

  const ultimaPagina = Math.max(1, Math.ceil(total / pageSize));
  const hayFiltros = Boolean(
    filterProcesoID ||
      filterAsignaturaID ||
      filterGradoID ||
      filterEspecialidadID ||
      filterDificultad ||
      filterEstado ||
      filterOrigen ||
      filterSearch,
  );

  const limpiarFiltros = () => {
    setFilterProcesoID(0);
    setFilterAsignaturaID(0);
    setFilterGradoID(0);
    setFilterEspecialidadID(0);
    setFilterDificultad("");
    setFilterEstado("");
    setFilterOrigen("");
    setFilterSearch("");
    setBusqueda("");
    setPage(0);
  };

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error ? (
        <Alerta
          tono="error"
          accion={
            <button type="button" onClick={load} className={ACCION_SECUNDARIA}>
              <ArrowClockwiseIcon weight="bold" className="size-4" />
              {t("admin-preguntas-reintentar")}
            </button>
          }
        >
          {error}
        </Alerta>
      ) : null}

      {/* Barra principal: búsqueda, columnas, exportar y acciones de la página */}
      <div className="flex flex-wrap items-center gap-2" data-sin-imprimir>
        <div className="min-w-56 flex-1">
          <Buscador
            etiqueta={t("admin-buscar-enunciado")}
            placeholder={t("admin-buscar-enunciado")}
            valor={filterSearch}
            onCambio={setFilterSearch}
          />
        </div>
        <Menu>
          <MenuTrigger className={ACCION_SECUNDARIA} aria-label={t("tabla-columnas")} title={t("tabla-columnas")}>
            <ColumnsIcon className="size-4" />
            <span className="max-sm:sr-only">{t("tabla-columnas")}</span>
          </MenuTrigger>
          <MenuContent align="end" className="min-w-48">
            <MenuGroup>
              <MenuGroupLabel>{t("tabla-columnas-visibles")}</MenuGroupLabel>
              {columns.map((c) => {
                const visible = !ocultas.includes(c.id);
                return (
                  <MenuItem
                    key={c.id}
                    closeOnClick={false}
                    onClick={() => setOcultas((o) => (visible ? [...o, c.id] : o.filter((x) => x !== c.id)))}
                  >
                    <CheckIcon weight="bold" className={cn("size-3.5", !visible && "invisible")} />
                    {c.titulo}
                  </MenuItem>
                );
              })}
            </MenuGroup>
          </MenuContent>
        </Menu>
        <Menu>
          <MenuTrigger className={ACCION_SECUNDARIA} aria-label={t("comun-exportar")} title={t("comun-exportar")}>
            <DownloadSimpleIcon className="size-4" />
            <span className="max-sm:sr-only">{t("comun-exportar")}</span>
          </MenuTrigger>
          <MenuContent align="end" className="min-w-44">
            <MenuItem
              onClick={() => {
                // H29: false = el navegador bloqueó la ventana de impresión.
                if (!imprimirTabla(t("menu-admin-preguntas"), exportables, rows)) {
                  toast.error(t("tabla-impresion-bloqueada"));
                }
              }}
            >
              <PrinterIcon className="size-4" />
              {t("tabla-imprimir")}
            </MenuItem>
            <MenuItem onClick={() => descargarCsv(t("menu-admin-preguntas"), exportables, rows)}>
              <FileCsvIcon className="size-4" />
              {t("tabla-exportar-csv")}
            </MenuItem>
          </MenuContent>
        </Menu>
        <button type="button" className={ACCION_SECUNDARIA} onClick={() => router.push("/admin/preguntas/import")}>
          <UploadSimpleIcon className="size-4" />
          {t("admin-preguntas-importar-csv")}
        </button>
        <button type="button" className={ACCION_PRINCIPAL} onClick={() => router.push("/admin/preguntas/new")}>
          <PlusIcon weight="bold" className="size-4" />
          {t("admin-preguntas-nueva")}
        </button>
      </div>

      {/* Filtros del banco */}
      <div className="flex flex-wrap items-center gap-2" data-sin-imprimir>
        <div className="w-full sm:w-48">
          <SelectCampo
            id="filtro-proceso"
            etiqueta={t("admin-preguntas-proceso")}
            valor={filterProcesoID ? String(filterProcesoID) : ""}
            onCambio={(v) => {
              setFilterProcesoID(Number(v) || 0);
              setFilterAsignaturaID(0);
              setPage(0);
            }}
            opciones={[
              { value: "", label: t("admin-preguntas-filtro-proceso-todos") },
              ...procesos.map((p) => ({ value: String(p.id), label: p.nombre })),
            ]}
          />
        </div>
        <div className="w-full sm:w-44">
          <SelectCampo
            id="filtro-asignatura"
            etiqueta={t("admin-preguntas-asignatura")}
            valor={filterAsignaturaID ? String(filterAsignaturaID) : ""}
            onCambio={(v) => {
              setFilterAsignaturaID(Number(v) || 0);
              setPage(0);
            }}
            disabled={!filterProcesoID}
            opciones={[
              { value: "", label: t("admin-preguntas-filtro-asignatura-todas") },
              ...asignaturas.map((a) => ({ value: String(a.id), label: a.nombre })),
            ]}
          />
        </div>
        <div className="w-[calc(50%-4px)] sm:w-36">
          <SelectCampo
            id="filtro-grado"
            etiqueta={t("admin-preguntas-grado")}
            valor={filterGradoID ? String(filterGradoID) : ""}
            onCambio={(v) => {
              setFilterGradoID(Number(v) || 0);
              setPage(0);
            }}
            opciones={[
              { value: "", label: t("admin-preguntas-filtro-grado-todos") },
              ...grados.map((g) => ({ value: String(g.id), label: g.abreviatura })),
            ]}
          />
        </div>
        <div className="w-[calc(50%-4px)] sm:w-44">
          <SelectCampo
            id="filtro-especialidad"
            etiqueta={t("admin-preguntas-especialidad")}
            valor={filterEspecialidadID ? String(filterEspecialidadID) : ""}
            onCambio={(v) => {
              setFilterEspecialidadID(Number(v) || 0);
              setPage(0);
            }}
            opciones={[
              { value: "", label: t("admin-preguntas-filtro-especialidad-todas") },
              ...especialidades.map((e) => ({ value: String(e.id), label: e.nombre })),
            ]}
          />
        </div>
        <div className="w-[calc(50%-4px)] sm:w-40">
          <SelectCampo
            id="filtro-dificultad"
            etiqueta={t("admin-preguntas-dificultad")}
            valor={filterDificultad}
            onCambio={(v) => {
              setFilterDificultad(v);
              setPage(0);
            }}
            opciones={[
              { value: "", label: t("admin-preguntas-filtro-dificultad-todas") },
              { value: "facil", label: t("admin-preguntas-dificultad-facil") },
              { value: "medio", label: t("admin-preguntas-dificultad-medio") },
              { value: "dificil", label: t("admin-preguntas-dificultad-dificil") },
            ]}
          />
        </div>
        <div className="w-[calc(50%-4px)] sm:w-36">
          <SelectCampo
            id="filtro-estado"
            etiqueta={t("admin-preguntas-estado")}
            valor={filterEstado}
            onCambio={(v) => {
              setFilterEstado(ESTADOS_FILTRABLES.find((e) => e === v) ?? "");
              setPage(0);
            }}
            opciones={[
              { value: "", label: t("admin-preguntas-filtro-estado-todos") },
              ...ESTADOS_FILTRABLES.map((e) => ({ value: e, label: t(claveEstadoPregunta(e)) })),
            ]}
          />
        </div>
        <div className="w-[calc(50%-4px)] sm:w-36">
          <SelectCampo
            id="filtro-origen"
            etiqueta={t("serumsIa.revision.listado.columnaOrigen")}
            valor={filterOrigen}
            onCambio={(v) => {
              setFilterOrigen(ORIGENES_PREGUNTA.find((o) => o === v) ?? "");
              setPage(0);
            }}
            opciones={[
              { value: "", label: t("serumsIa.revision.listado.filtroOrigenTodos") },
              ...ORIGENES_PREGUNTA.map((o) => ({ value: o, label: t(claveOrigenPregunta(o)) })),
            ]}
          />
        </div>
        {hayFiltros ? (
          <button
            type="button"
            onClick={limpiarFiltros}
            className="h-control text-control text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:ring-ring/50 rounded-full px-3 font-medium transition-colors outline-none focus-visible:ring-2"
          >
            {t("admin-preguntas-limpiar-filtros")}
          </button>
        ) : null}
      </div>

      {loading && !cargado ? (
        <SkeletonTabla columnas={visibles.length + 1} />
      ) : (
        <div className={cn(CONTENEDOR_TABLA, loading && "opacity-60")} aria-busy={loading}>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                {visibles.map((c, i) => (
                  <TableHead key={c.id} className={cn(CABECERA_TABLA, i === 0 && "pl-4")}>
                    {c.titulo}
                  </TableHead>
                ))}
                <TableHead className={cn(CABECERA_TABLA, "w-12 pr-4 text-right")}>
                  <span className="sr-only">{t("admin-preguntas-acciones")}</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="text-control">
              {rows.map((p) => (
                <TableRow key={p.id} className="group">
                  {visibles.map((c, i) => (
                    <TableCell key={c.id} className={cn("px-3 py-2", i === 0 && "pl-4", c.className)}>
                      {c.render(p)}
                    </TableCell>
                  ))}
                  <TableCell className="w-12 pr-2 text-right">
                    <MenuAcciones
                      etiqueta={t("admin-preguntas-acciones-de", { id: p.id })}
                      acciones={[
                        {
                          label: t("comun-editar"),
                          icon: PencilSimpleIcon,
                          onClick: () => router.push(`/admin/preguntas/${p.id}/edit`),
                        },
                        // P2 (SIA-E3-02): un borrador o una en revisión se rechazan antes de eliminarse.
                        {
                          label: t("serumsIa.revision.listado.verDetalle"),
                          icon: BookOpenTextIcon,
                          onClick: () => router.push(`/admin/preguntas/${p.id}`),
                        },
                        {
                          label: t("comun-eliminar"),
                          icon: TrashIcon,
                          destructiva: true,
                          separador: true,
                          disabled: !puedeEliminar(p.estado),
                          ayuda: puedeEliminar(p.estado) ? undefined : t("serumsIa.avisosPregunta.eliminarAyuda"),
                          onClick: () => setConfirmDelete({ open: true, row: p, submitting: false }),
                        },
                      ]}
                    />
                  </TableCell>
                </TableRow>
              ))}
              {rows.length === 0 ? (
                <FilaVacia columnas={visibles.length + 1} icon={<ListChecksIcon className="size-6" />}>
                  {hayFiltros ? (
                    <>
                      {t("serumsIa.revision.listado.vacio")}{" "}
                      <button
                        type="button"
                        onClick={limpiarFiltros}
                        className="text-foreground font-medium underline underline-offset-2"
                      >
                        {t("admin-preguntas-limpiar-filtros")}
                      </button>
                    </>
                  ) : (
                    t("admin-preguntas-vacio")
                  )}
                </FilaVacia>
              ) : null}
            </TableBody>
          </Table>
          <PieTabla
            desde={rows.length === 0 ? 0 : page * pageSize + 1}
            hasta={page * pageSize + rows.length}
            total={total}
            unidad={t("admin-preguntas-unidad")}
            porPagina={pageSize}
            onPorPagina={(n) => {
              setPageSize(n);
              setPage(0);
            }}
            pagina={page + 1}
            ultimaPagina={ultimaPagina}
            onPagina={(p) => setPage(p - 1)}
          />
        </div>
      )}

      {/* Delete confirmation dialog */}
      <Dialog
        open={confirmDelete.open}
        onOpenChange={(open) => {
          if (!open && !confirmDelete.submitting) setConfirmDelete((d) => ({ ...d, open: false }));
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={TrashIcon}
            tono="peligro"
            titulo={t("admin-preguntas-eliminar-titulo")}
            descripcion={t("admin-preguntas-eliminar-confirm")}
          />
          {confirmDelete.row ? (
            <div className="px-5 pb-2">
              <p className="bg-secondary text-control text-foreground line-clamp-3 rounded-lg px-3 py-2.5">
                {confirmDelete.row.enunciado}
              </p>
            </div>
          ) : null}
          <PieDialogo>
            <button
              type="button"
              className={BOTON_SECUNDARIO}
              disabled={confirmDelete.submitting}
              onClick={() => setConfirmDelete((d) => ({ ...d, open: false }))}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={confirmDelete.submitting}
              textoPendiente={t("comun-eliminando")}
              className={BOTON_DESTRUCTIVO}
              onClick={handleDelete}
            >
              {t("comun-eliminar")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
