"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";

import { PieTabla } from "@/components/app/pie-tabla";
import { Alerta } from "@/components/feedback/alerta";
import { SkeletonTabla } from "@/components/feedback/skeleton";
import { Buscador } from "@/components/formularios/buscador";
import { DialogoConfirmacion } from "@/components/navegacion/dialogo-confirmacion";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { Tabs } from "@/components/navegacion/tabs";
import { CONTENEDOR_TABLA, FilaVacia } from "@/components/patrones/pie-tabla";
import { PillEstado } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { Menu, MenuContent, MenuGroup, MenuGroupLabel, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CABECERA_TABLA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { type ColumnaExportable, descargarCsv, imprimirTabla } from "@/lib/exportar";
import { formatearFechaHora } from "@/lib/formato";
import { POR_PAGINA_DEFECTO } from "@/lib/paginacion";
import { HistorialItem, MAX_ELIMINAR_PRACTICAS, practicasApi } from "@/lib/practices/api";
import { cn } from "@/lib/utils";
import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";
import {
  ArrowDownIcon,
  ArrowsDownUpIcon,
  ArrowUpIcon,
  CheckIcon,
  ColumnsIcon,
  DownloadSimpleIcon,
  FileCsvIcon,
  ListChecksIcon,
  MinusIcon,
  PlusIcon,
  PrinterIcon,
  TrashIcon,
  TrayIcon,
  XIcon,
} from "@phosphor-icons/react";

type EstadoFilter = "" | "completada" | "abandonada";
/** `Tabs` necesita ids no vacíos: "todas" equivale al filtro vacío de la API. */
type TabEstado = "todas" | "completada" | "abandonada";

function formatTime(sec: number): string {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

const CASILLA =
  "flex size-[18px] shrink-0 items-center justify-center rounded-[6px] border-[1.5px] border-input bg-card transition-colors duration-(--duracion-rapida) ease-salida outline-none focus-visible:ring-3 focus-visible:ring-acento/55 data-checked:border-primary data-checked:bg-primary data-indeterminate:border-primary data-indeterminate:bg-primary";

/** Checkbox sin texto para la columna de selección. */
function CasillaFila({
  etiqueta,
  checked,
  indeterminate,
  onCheckedChange,
}: {
  etiqueta: string;
  checked: boolean;
  indeterminate?: boolean;
  onCheckedChange: () => void;
}) {
  return (
    <CheckboxPrimitive.Root
      aria-label={etiqueta}
      checked={checked}
      indeterminate={indeterminate}
      onCheckedChange={onCheckedChange}
      className={CASILLA}
    >
      <CheckboxPrimitive.Indicator className="text-primary-foreground">
        {indeterminate ? (
          <MinusIcon weight="bold" className="size-3" />
        ) : (
          <CheckIcon weight="bold" className="size-3" />
        )}
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

type Columna = {
  id: string;
  titulo: string;
  render: (f: HistorialItem) => ReactNode;
  texto: (f: HistorialItem) => string | number;
  /** Valor para ordenar la página visible; sin él la columna no se ordena. */
  orden?: (f: HistorialItem) => string | number;
  alinear?: "derecha";
  ancho?: string;
};

export default function PracticasPage() {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const router = useRouter();

  const [rows, setRows] = useState<HistorialItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(POR_PAGINA_DEFECTO);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterEstado, setFilterEstado] = useState<EstadoFilter>("");
  const [selection, setSelection] = useState<number[]>([]);
  const [confirmBatch, setConfirmBatch] = useState(false);
  const [confirmSingle, setConfirmSingle] = useState<HistorialItem | null>(null);
  const [busqueda, setBusqueda] = useState("");
  const [ocultas, setOcultas] = useState<string[]>([]);
  const [orden, setOrden] = useState<{ id: string; dir: "asc" | "desc" } | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await practicasApi.listHistorial(token, {
        page: page + 1,
        per_page: pageSize,
        estado: filterEstado || undefined,
      });
      setRows(res.data);
      setTotal(res.total);
    } catch {
      setError(t("practicas-error-cargar"));
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, page, pageSize, filterEstado]);

  useEffect(() => {
    load();
  }, [load]);

  // H33: the server message is Spanish-only and a network error says «Failed to
  // fetch», so the error is mapped by status to a translated key.
  const mensajeErrorEliminar = (e: unknown) => {
    const status = (e as { status?: number }).status;
    if (status === 404 || status === 403) return t("practicas-error-eliminar-no-encontrada");
    if (status === 422) return t("practicas-error-eliminar-limite", { max: MAX_ELIMINAR_PRACTICAS });
    return t("practicas-error-eliminar");
  };

  const handleBatchDelete = async () => {
    if (!token) return;
    try {
      await practicasApi.batchDeletePracticas(token, selection);
      setSelection([]);
      setConfirmBatch(false);
      load();
    } catch (e: unknown) {
      const msg = mensajeErrorEliminar(e);
      setError(msg);
      return msg;
    }
  };

  const handleDelete = async () => {
    if (!token || !confirmSingle) return;
    try {
      await practicasApi.deletePractica(token, confirmSingle.id);
      setConfirmSingle(null);
      load();
    } catch (e: unknown) {
      const msg = mensajeErrorEliminar(e);
      setError(msg);
      return msg;
    }
  };

  const estadoTono = (estado: string) =>
    estado === "completada" ? "ok" : estado === "abandonada" ? "aviso" : "neutro";

  const columnas = useMemo<Columna[]>(
    () => [
      {
        id: "numero",
        titulo: "N°",
        ancho: "4rem",
        render: (f) => <span className="text-muted-foreground tabular-nums">{f.numero}</span>,
        texto: (f) => f.numero,
        orden: (f) => f.numero,
      },
      {
        id: "asignaturas",
        titulo: t("practicas-asignaturas"),
        render: (f) => <span className="text-foreground font-medium">{f.asignaturas}</span>,
        texto: (f) => f.asignaturas,
        orden: (f) => f.asignaturas,
      },
      {
        id: "total_preguntas",
        titulo: t("practicas-total"),
        alinear: "derecha",
        render: (f) => f.total_preguntas,
        texto: (f) => f.total_preguntas,
        orden: (f) => f.total_preguntas,
      },
      {
        id: "correctas",
        titulo: t("practicas-correctas"),
        alinear: "derecha",
        render: (f) => f.correctas,
        texto: (f) => f.correctas,
        orden: (f) => f.correctas,
      },
      {
        id: "porcentaje",
        titulo: t("practicas-porcentaje"),
        alinear: "derecha",
        render: (f) => (
          <span
            className={cn("font-semibold", f.porcentaje >= 60 ? "text-success-foreground" : "text-destructive-texto")}
          >
            {f.porcentaje.toFixed(1)}%
          </span>
        ),
        texto: (f) => `${f.porcentaje.toFixed(1)}%`,
        orden: (f) => f.porcentaje,
      },
      {
        id: "tiempo_usado_seg",
        titulo: t("practicas-tiempo"),
        alinear: "derecha",
        render: (f) => formatTime(f.tiempo_usado_seg),
        texto: (f) => formatTime(f.tiempo_usado_seg),
        orden: (f) => f.tiempo_usado_seg,
      },
      {
        id: "fecha",
        titulo: t("practicas-fecha"),
        render: (f) => <span className="whitespace-nowrap tabular-nums">{formatearFechaHora(f.fecha)}</span>,
        texto: (f) => formatearFechaHora(f.fecha),
        orden: (f) => f.fecha,
      },
      {
        id: "estado",
        titulo: t("practicas-estado"),
        render: (f) => (
          <PillEstado tono={estadoTono(f.estado)}>{t(`practicas-estado-${f.estado}`) || f.estado}</PillEstado>
        ),
        texto: (f) => t(`practicas-estado-${f.estado}`) || f.estado,
        orden: (f) => f.estado,
      },
    ],
    [t],
  );

  const visibles = columnas.filter((c) => !ocultas.includes(c.id));

  const filas = useMemo(() => {
    const q = busqueda.trim().toLocaleLowerCase();
    let r = q ? rows.filter((f) => columnas.some((c) => String(c.texto(f)).toLocaleLowerCase().includes(q))) : rows;
    const col = orden && columnas.find((c) => c.id === orden.id);
    if (orden && col?.orden) {
      const v = col.orden;
      r = [...r].sort((a, b) => {
        const x = v(a);
        const y = v(b);
        const d = typeof x === "number" && typeof y === "number" ? x - y : String(x).localeCompare(String(y));
        return orden.dir === "asc" ? d : -d;
      });
    }
    return r;
  }, [rows, columnas, busqueda, orden]);

  const exportables: ColumnaExportable<HistorialItem>[] = visibles.map((c) => ({ titulo: c.titulo, valor: c.texto }));
  const tituloExport = t("practicas-historial");

  const idsVisibles = filas.map((f) => f.id);
  const todos = idsVisibles.length > 0 && idsVisibles.every((id) => selection.includes(id));
  const alguno = !todos && idsVisibles.some((id) => selection.includes(id));

  function alternarOrden(id: string) {
    setOrden((o) => (o?.id === id ? (o.dir === "asc" ? { id, dir: "desc" } : null) : { id, dir: "asc" }));
  }

  const ultimaPagina = Math.max(1, Math.ceil(total / pageSize));
  const desde = rows.length === 0 ? 0 : page * pageSize + 1;
  const hasta = page * pageSize + rows.length;
  const tabActual: TabEstado = filterEstado === "" ? "todas" : filterEstado;
  const columnasTotales = visibles.length + 2;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error && (
        <Alerta
          tono="error"
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setError(null)}>
              <XIcon weight="bold" className="size-4" />
            </Button>
          }
        >
          {error}
        </Alerta>
      )}

      {/* Contexto + acciones */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-cuerpo text-muted-foreground">{t("practicas-historial-subtitulo")}</p>
        <div className="flex flex-wrap items-center gap-2">
          {selection.length > 0 && (
            <Button variant="destructive" onClick={() => setConfirmBatch(true)}>
              <TrashIcon weight="duotone" className="size-4" />
              {t("practicas-eliminar-seleccionadas")} ({selection.length})
            </Button>
          )}
          <button
            id="tour-practicas-btn-nueva"
            type="button"
            className={ACCION_PRINCIPAL}
            onClick={() => router.push("/practicas/nueva")}
          >
            <PlusIcon weight="bold" className="size-3.5" />
            {t("practicas-nueva")}
          </button>
        </div>
      </div>

      {/* Filtros */}
      <div className="flex flex-wrap items-center gap-2">
        <Tabs<TabEstado>
          estilo="segmentado"
          valor={tabActual}
          onCambio={(v) => {
            setFilterEstado(v === "todas" ? "" : v);
            setPage(0);
            setSelection([]);
          }}
          items={[
            { id: "todas", etiqueta: t("practicas-todas") },
            { id: "completada", etiqueta: t("practicas-estado-completada") },
            { id: "abandonada", etiqueta: t("practicas-estado-abandonada") },
          ]}
          className="[&_[role=tabpanel]]:hidden"
        />
        <div className="min-w-48 flex-1">
          <Buscador
            etiqueta={t("practicas-filtros")}
            placeholder={`${t("practicas-filtros")}…`}
            valor={busqueda}
            onCambio={setBusqueda}
          />
        </div>
        <Menu>
          <MenuTrigger
            className={ACCION_SECUNDARIA}
            aria-label={t("practicas-columnas")}
            title={t("practicas-columnas")}
          >
            <ColumnsIcon weight="duotone" className="size-4" />
            <span className="max-sm:sr-only">{t("practicas-columnas")}</span>
          </MenuTrigger>
          <MenuContent align="end" className="min-w-48">
            <MenuGroup>
              <MenuGroupLabel>{t("practicas-columnas")}</MenuGroupLabel>
              {columnas.map((c) => {
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
          <MenuTrigger
            className={ACCION_SECUNDARIA}
            aria-label={t("practicas-exportar")}
            title={t("practicas-exportar")}
          >
            <DownloadSimpleIcon weight="duotone" className="size-4" />
            <span className="max-sm:sr-only">{t("practicas-exportar")}</span>
          </MenuTrigger>
          <MenuContent align="end" className="min-w-44">
            <MenuItem onClick={() => imprimirTabla(tituloExport, exportables, filas)}>
              <PrinterIcon className="size-4" />
              {t("practicas-imprimir")}
            </MenuItem>
            <MenuItem onClick={() => descargarCsv(tituloExport, exportables, filas)}>
              <FileCsvIcon className="size-4" />
              {t("practicas-exportar-csv")}
            </MenuItem>
          </MenuContent>
        </Menu>
      </div>

      {/* Tabla */}
      {loading && rows.length === 0 ? (
        <SkeletonTabla columnas={6} filas={Math.min(pageSize, 6)} />
      ) : (
        <div className={cn(CONTENEDOR_TABLA, loading && "opacity-60")} aria-busy={loading}>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-10 pl-4">
                  <CasillaFila
                    etiqueta={t("practicas-todas")}
                    checked={todos}
                    indeterminate={alguno}
                    onCheckedChange={() =>
                      setSelection((s) =>
                        todos ? s.filter((id) => !idsVisibles.includes(id)) : [...new Set([...s, ...idsVisibles])],
                      )
                    }
                  />
                </TableHead>
                {visibles.map((c) => (
                  <TableHead
                    key={c.id}
                    style={{ width: c.ancho }}
                    className={cn(CABECERA_TABLA, c.alinear === "derecha" && "text-right")}
                    aria-sort={orden?.id === c.id ? (orden.dir === "asc" ? "ascending" : "descending") : undefined}
                  >
                    {c.orden ? (
                      <button
                        type="button"
                        onClick={() => alternarOrden(c.id)}
                        className={cn(
                          "hover:text-foreground inline-flex items-center gap-1",
                          orden?.id === c.id && "text-foreground",
                        )}
                      >
                        {c.titulo}
                        {orden?.id === c.id ? (
                          orden.dir === "asc" ? (
                            <ArrowUpIcon weight="bold" className="size-3" />
                          ) : (
                            <ArrowDownIcon weight="bold" className="size-3" />
                          )
                        ) : (
                          <ArrowsDownUpIcon weight="bold" className="size-3 opacity-50" />
                        )}
                      </button>
                    ) : (
                      c.titulo
                    )}
                  </TableHead>
                ))}
                <TableHead className={cn(CABECERA_TABLA, "w-14 pr-4 text-right")}>
                  <span className="sr-only">{t("practicas-acciones")}</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="text-control">
              {filas.map((f) => {
                const seleccionada = selection.includes(f.id);
                return (
                  <TableRow key={f.id} data-state={seleccionada ? "selected" : undefined} className="group">
                    <TableCell className="w-10 pl-4">
                      <CasillaFila
                        etiqueta={`#${f.numero}`}
                        checked={seleccionada}
                        onCheckedChange={() =>
                          setSelection((s) => (seleccionada ? s.filter((x) => x !== f.id) : [...s, f.id]))
                        }
                      />
                    </TableCell>
                    {visibles.map((c) => (
                      <TableCell
                        key={c.id}
                        className={cn("px-3 py-2.5", c.alinear === "derecha" && "text-right tabular-nums")}
                      >
                        {c.render(f)}
                      </TableCell>
                    ))}
                    <TableCell className="pr-4 text-right">
                      <MenuAcciones
                        etiqueta={t("practicas-acciones")}
                        className="ml-auto"
                        acciones={[
                          {
                            label: t("practicas-revisar"),
                            icon: ListChecksIcon,
                            onClick: () => router.push(`/practicas/${f.id}/revision`),
                          },
                          {
                            label: t("practicas-eliminar"),
                            icon: TrashIcon,
                            destructiva: true,
                            separador: true,
                            onClick: () => setConfirmSingle(f),
                          },
                        ]}
                      />
                    </TableCell>
                  </TableRow>
                );
              })}
              {filas.length === 0 ? (
                <FilaVacia columnas={columnasTotales} icon={<TrayIcon className="size-6" />}>
                  {t("practicas-vacio")}
                </FilaVacia>
              ) : null}
            </TableBody>
          </Table>
          <PieTabla
            desde={desde}
            hasta={hasta}
            total={total}
            unidad={t("practicas-unidad")}
            porPagina={pageSize}
            onPorPagina={(n) => {
              setPageSize(n);
              setPage(0);
              setSelection([]);
            }}
            pagina={page + 1}
            ultimaPagina={ultimaPagina}
            onPagina={(p) => {
              setPage(p - 1);
              setSelection([]);
            }}
          />
        </div>
      )}

      {/* Eliminar varias */}
      <DialogoConfirmacion
        open={confirmBatch}
        onOpenChange={setConfirmBatch}
        icon={TrashIcon}
        titulo={t("practicas-eliminar-titulo-batch")}
        descripcion={t("practicas-eliminar-batch-confirm", { count: selection.length })}
        textoConfirmar={t("practicas-eliminar")}
        textoCancelar={t("practicas-cancelar")}
        textoPendiente={t("comun-procesando")}
        etiquetaCerrar={t("comun-cerrar")}
        onConfirmar={handleBatchDelete}
      />

      {/* Eliminar una */}
      <DialogoConfirmacion
        open={!!confirmSingle}
        onOpenChange={(open) => {
          if (!open) setConfirmSingle(null);
        }}
        icon={TrashIcon}
        titulo={t("practicas-eliminar-titulo-single")}
        descripcion={t("practicas-eliminar-single-confirm", { id: confirmSingle?.id ?? "" })}
        textoConfirmar={t("practicas-eliminar")}
        textoCancelar={t("practicas-cancelar")}
        textoPendiente={t("comun-procesando")}
        etiquetaCerrar={t("comun-cerrar")}
        onConfirmar={handleDelete}
      />
    </div>
  );
}
