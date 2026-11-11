"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";

import { PieTabla } from "@/components/app/pie-tabla";
import { Alerta } from "@/components/feedback/alerta";
import { SkeletonTabla } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { CABECERA_TABLA, CONTENEDOR_TABLA, FilaVacia } from "@/components/patrones/pie-tabla";
import { PillEstado } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { Menu, MenuContent, MenuGroup, MenuGroupLabel, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA, SEGMENTADO, SEGMENTO } from "@/lib/estilos";
import { type ColumnaExportable, descargarCsv, imprimirTabla } from "@/lib/exportar";
import { formatearFechaHora } from "@/lib/formato";
import { POR_PAGINA_DEFECTO } from "@/lib/paginacion";
import { HistorialSimulacroItem, simulacrosApi } from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import {
  CheckIcon,
  ColumnsIcon,
  DownloadSimpleIcon,
  FileCsvIcon,
  FileTextIcon,
  ListChecksIcon,
  PlusIcon,
  PrinterIcon,
  TimerIcon,
  XIcon,
} from "@phosphor-icons/react";

type FilterTab = "" | "aprobado" | "desaprobado";

const FILTROS: { id: FilterTab; clave: string }[] = [
  { id: "", clave: "simulacros-todos" },
  { id: "aprobado", clave: "simulacros-aprobado-si" },
  { id: "desaprobado", clave: "simulacros-aprobado-no" },
];

type Columna = {
  id: string;
  titulo: string;
  render: (row: HistorialSimulacroItem) => ReactNode;
  /** Valor plano para CSV e impresión; sin él la columna no se exporta. */
  texto?: (row: HistorialSimulacroItem) => string | number;
  derecha?: boolean;
  ancho?: string;
  /** `false`: siempre visible (no aparece en el selector de columnas). */
  ocultable?: boolean;
};

function formatTime(sec: number): string {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

export default function SimulacrosPage() {
  const t = useTranslations("dashboard");
  const toast = useToast();
  const { token } = useAuth();
  const router = useRouter();

  const [rows, setRows] = useState<HistorialSimulacroItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(POR_PAGINA_DEFECTO);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterTab, setFilterTab] = useState<FilterTab>("");
  const [ocultas, setOcultas] = useState<string[]>([]);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const aprobado = filterTab === "" ? undefined : filterTab === "aprobado";
      const res = await simulacrosApi.getHistorial(token, {
        page: page + 1,
        per_page: pageSize,
        aprobado,
      });
      setRows(res.data);
      setTotal(res.total);
    } catch {
      setError(t("simulacros-error-cargar"));
    } finally {
      setLoading(false);
    }
  }, [token, page, pageSize, filterTab, t]);

  useEffect(() => {
    load();
  }, [load]);

  const columns = useMemo<Columna[]>(
    () => [
      {
        id: "numero",
        titulo: "N°",
        ancho: "64px",
        ocultable: false,
        render: (r) => <span className="text-muted-foreground tabular-nums">{r.numero}</span>,
        texto: (r) => r.numero,
      },
      {
        id: "proceso",
        titulo: t("simulacros-proceso"),
        ocultable: false,
        render: (r) => <span className="text-foreground font-medium">{r.proceso}</span>,
        texto: (r) => r.proceso,
      },
      {
        id: "nota_centesimal",
        titulo: t("simulacros-nota"),
        derecha: true,
        // H36: nota_maxima is configurable and the DTO does not carry it, so no fixed «/ 100».
        render: (r) => <span className="text-foreground font-semibold">{r.nota_centesimal.toFixed(1)}</span>,
        texto: (r) => r.nota_centesimal.toFixed(1),
      },
      {
        id: "porcentaje",
        titulo: t("simulacros-porcentaje"),
        derecha: true,
        render: (r) => `${r.porcentaje.toFixed(1)}%`,
        texto: (r) => `${r.porcentaje.toFixed(1)}%`,
      },
      {
        id: "aprobado",
        titulo: t("simulacros-aprobado"),
        render: (r) => (
          <PillEstado tono={r.aprobado ? "ok" : "error"}>
            {r.aprobado ? t("simulacros-aprobado-si") : t("simulacros-aprobado-no")}
          </PillEstado>
        ),
        texto: (r) => (r.aprobado ? t("simulacros-aprobado-si") : t("simulacros-aprobado-no")),
      },
      {
        id: "tiempo_usado_seg",
        titulo: t("simulacros-tiempo"),
        derecha: true,
        render: (r) => formatTime(r.tiempo_usado_seg),
        texto: (r) => formatTime(r.tiempo_usado_seg),
      },
      {
        id: "fecha",
        titulo: t("simulacros-fecha"),
        render: (r) => <span className="text-muted-foreground tabular-nums">{formatearFechaHora(r.fecha)}</span>,
        texto: (r) => formatearFechaHora(r.fecha),
      },
      {
        id: "actions",
        titulo: t("simulacros-acciones"),
        derecha: true,
        ancho: "72px",
        ocultable: false,
        render: (r) => (
          <MenuAcciones
            etiqueta={t("simulacros-acciones")}
            className="ml-auto"
            acciones={[
              { label: t("simulacros-ver-resultado"), icon: FileTextIcon, href: `/simulacros/${r.id}/resultado` },
              { label: t("simulacros-revisar"), icon: ListChecksIcon, href: `/simulacros/${r.id}/revision` },
            ]}
          />
        ),
      },
    ],
    [t],
  );

  const visibles = columns.filter((c) => !ocultas.includes(c.id));
  const exportables: ColumnaExportable<HistorialSimulacroItem>[] = visibles
    .filter((c) => c.texto)
    .map((c) => ({ titulo: c.titulo, valor: c.texto! }));
  const ocultables = columns.filter((c) => c.ocultable !== false);

  const ultimaPagina = Math.max(1, Math.ceil(total / pageSize));
  const desde = rows.length === 0 ? 0 : page * pageSize + 1;
  const hasta = page * pageSize + rows.length;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error && (
        <Alerta
          tono="error"
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setError(null)}>
              <XIcon weight="bold" />
            </Button>
          }
        >
          {error}
        </Alerta>
      )}

      <section className={cn(CONTENEDOR_TABLA, loading && rows.length > 0 && "opacity-60")}>
        <CabeceraSeccion
          icon={TimerIcon}
          titulo={t("simulacros-historial")}
          subtitulo={t("simulacros-historial-subtitulo")}
          className="px-2"
          derecha={
            <div className="flex flex-wrap items-center gap-2" data-sin-imprimir>
              <Menu>
                <MenuTrigger className={ACCION_SECUNDARIA} aria-label={t("tabla-columnas")} title={t("tabla-columnas")}>
                  <ColumnsIcon className="size-4" />
                  <span className="max-sm:sr-only">{t("tabla-columnas")}</span>
                </MenuTrigger>
                <MenuContent align="end" className="min-w-48">
                  <MenuGroup>
                    <MenuGroupLabel>{t("tabla-columnas-visibles")}</MenuGroupLabel>
                    {ocultables.map((c) => {
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
                <MenuTrigger className={ACCION_SECUNDARIA} aria-label={t("tabla-exportar")} title={t("tabla-exportar")}>
                  <DownloadSimpleIcon className="size-4" />
                  <span className="max-sm:sr-only">{t("tabla-exportar")}</span>
                </MenuTrigger>
                <MenuContent align="end" className="min-w-44">
                  <MenuItem
                    onClick={() => {
                      if (!imprimirTabla(t("simulacros-historial"), exportables, rows))
                        toast.error(t("tabla-impresion-bloqueada"));
                    }}
                  >
                    <PrinterIcon className="size-4" />
                    {t("simulacros-imprimir")}
                  </MenuItem>
                  <MenuItem onClick={() => descargarCsv(t("simulacros-historial"), exportables, rows)}>
                    <FileCsvIcon className="size-4" />
                    {t("simulacros-exportar-csv")}
                  </MenuItem>
                </MenuContent>
              </Menu>
              <button
                id="tour-simulacros-btn-nuevo"
                type="button"
                className={ACCION_PRINCIPAL}
                onClick={() => router.push("/simulacros/nueva")}
              >
                <PlusIcon weight="bold" className="size-4" />
                {t("simulacros-nuevo")}
              </button>
            </div>
          }
        />

        {/* Filtro por estado */}
        <div className="px-2 pb-2">
          <div role="radiogroup" aria-label={t("simulacros-aprobado")} className={SEGMENTADO}>
            {FILTROS.map((f) => (
              <button
                key={f.id}
                type="button"
                role="radio"
                aria-checked={filterTab === f.id}
                data-active={filterTab === f.id ? "" : undefined}
                className={SEGMENTO}
                onClick={() => {
                  setFilterTab(f.id);
                  setPage(0);
                }}
              >
                {t(f.clave)}
              </button>
            ))}
          </div>
        </div>

        {loading && rows.length === 0 ? (
          <div className="pb-2">
            <SkeletonTabla columnas={visibles.length} />
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                {visibles.map((c, i) => (
                  <TableHead
                    key={c.id}
                    style={{ width: c.ancho }}
                    className={cn(
                      CABECERA_TABLA,
                      i === 0 && "pl-4",
                      i === visibles.length - 1 && "pr-4",
                      c.derecha && "text-right",
                    )}
                  >
                    {c.titulo}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody className="text-control">
              {rows.map((r) => (
                <TableRow key={r.id} className="group">
                  {visibles.map((c, i) => (
                    <TableCell
                      key={c.id}
                      className={cn(
                        "px-3 py-2",
                        i === 0 && "pl-4",
                        i === visibles.length - 1 && "pr-4",
                        c.derecha && "text-right tabular-nums",
                      )}
                    >
                      {c.render(r)}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
              {rows.length === 0 ? (
                <FilaVacia columnas={visibles.length} icon={<TimerIcon className="size-6" />}>
                  {t("simulacros-vacio", { accion: t("simulacros-nuevo") })}
                </FilaVacia>
              ) : null}
            </TableBody>
          </Table>
        )}

        <PieTabla
          desde={desde}
          hasta={hasta}
          total={total}
          unidad={t("menu-simulacros").toLocaleLowerCase()}
          porPagina={pageSize}
          onPorPagina={(n) => {
            setPageSize(n);
            setPage(0);
          }}
          pagina={page + 1}
          ultimaPagina={ultimaPagina}
          onPagina={(p) => setPage(p - 1)}
        />
      </section>
    </div>
  );
}
