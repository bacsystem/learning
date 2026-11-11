"use client";

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";

import { PieTabla } from "@/components/app/pie-tabla";
import { Alerta } from "@/components/feedback/alerta";
import { SkeletonTabla } from "@/components/feedback/skeleton";
import { Buscador } from "@/components/formularios/buscador";
import { EntradaFecha } from "@/components/formularios/entrada-fecha";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { CABECERA_TABLA, CONTENEDOR_TABLA, CONTROL_FILTRO, FilaVacia } from "@/components/patrones/pie-tabla";
import { Chip } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { adminGetAuditLog, type AuditLogItem } from "@/lib/admin/api";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { formatearFechaHora } from "@/lib/formato";
import { cn } from "@/lib/utils";
import { ArrowClockwiseIcon, ListMagnifyingGlassIcon, XIcon } from "@phosphor-icons/react";

const POR_PAGINA_INICIAL = 20;

const COLUMNAS: { id: string; clave: string; className?: string }[] = [
  { id: "id", clave: "admin-audit-col-id" },
  { id: "username", clave: "admin-audit-col-usuario" },
  { id: "accion", clave: "admin-audit-col-accion" },
  { id: "entidad", clave: "admin-audit-col-entidad" },
  { id: "entidad_id", clave: "admin-audit-col-entidad-id" },
  { id: "ip", clave: "admin-audit-col-ip" },
  { id: "created_at", clave: "admin-audit-col-fecha" },
  { id: "detalle", clave: "admin-audit-col-detalle", className: "min-w-52" },
];

export default function AuditLogPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");

  const [rows, setRows] = useState<AuditLogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [pagina, setPagina] = useState(1);
  const [porPagina, setPorPagina] = useState(POR_PAGINA_INICIAL);

  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [q, setQ] = useState("");
  const [debouncedQ, setDebouncedQ] = useState("");

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQ(q), 300);
    return () => clearTimeout(timer);
  }, [q]);

  // Al cambiar filtros o página rápido, solo cuenta la última petición.
  const loadReq = useRef(0);

  const load = useCallback(async () => {
    if (!token) return;
    const reqId = ++loadReq.current;
    setLoading(true);
    setError(null);
    try {
      const resp = await adminGetAuditLog(token, {
        page: pagina,
        per_page: porPagina,
        from: from || undefined,
        to: to || undefined,
        q: debouncedQ || undefined,
      });
      if (reqId !== loadReq.current) return;
      setRows(resp.data);
      setTotal(resp.total);
    } catch (e: unknown) {
      if (reqId !== loadReq.current) return;
      const err = e as { message?: string };
      setError(err.message ?? t("admin-audit-error-cargar"));
    } finally {
      if (reqId === loadReq.current) setLoading(false);
    }
  }, [token, pagina, porPagina, from, to, debouncedQ, t]);

  useEffect(() => {
    load();
  }, [load]);

  const hayFiltros = from !== "" || to !== "" || q !== "";
  const ultimaPagina = Math.max(1, Math.ceil(total / porPagina));
  const primeraCarga = loading && rows.length === 0;

  function limpiar() {
    setFrom("");
    setTo("");
    setQ("");
    setPagina(1);
    setPorPagina(POR_PAGINA_INICIAL);
  }

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error ? (
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
      ) : null}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <CabeceraSeccion
          icon={ListMagnifyingGlassIcon}
          titulo={t("admin-audit-titulo")}
          subtitulo={loading ? t("admin-audit-cargando") : t("admin-audit-total", { n: total })}
          conBorde={false}
          className="min-w-0 flex-nowrap px-0 py-0"
        />
        <button
          type="button"
          title={t("admin-audit-refrescar")}
          aria-label={t("admin-audit-refrescar")}
          onClick={load}
          disabled={loading}
          className={cn(
            CONTROL_FILTRO,
            "size-control text-muted-foreground hover:text-foreground inline-flex items-center justify-center disabled:opacity-60",
          )}
        >
          <ArrowClockwiseIcon weight="bold" className={cn("size-4", loading && "motion-safe:animate-spin")} />
        </button>
      </div>

      {/* Filtros */}
      <div className="flex flex-wrap items-center gap-2">
        <label htmlFor="audit-desde" className="sr-only">
          {t("admin-audit-desde")}
        </label>
        <EntradaFecha
          id="audit-desde"
          enBarra
          limpiable
          placeholder={t("admin-audit-desde")}
          valor={from}
          max={to || undefined}
          onCambio={(v) => {
            setFrom(v);
            setPagina(1);
          }}
          className="w-44"
        />
        <label htmlFor="audit-hasta" className="sr-only">
          {t("admin-audit-hasta")}
        </label>
        <EntradaFecha
          id="audit-hasta"
          enBarra
          limpiable
          placeholder={t("admin-audit-hasta")}
          valor={to}
          min={from || undefined}
          onCambio={(v) => {
            setTo(v);
            setPagina(1);
          }}
          className="w-44"
        />
        <div className="min-w-56 flex-1">
          <Buscador
            etiqueta={t("admin-audit-buscar-etiqueta")}
            placeholder={t("admin-audit-buscar")}
            valor={q}
            onCambio={(v) => {
              setQ(v);
              setPagina(1);
            }}
          />
        </div>
        <button type="button" onClick={limpiar} disabled={!hayFiltros} className={ACCION_SECUNDARIA}>
          {t("admin-audit-limpiar")}
        </button>
      </div>

      {primeraCarga ? (
        <SkeletonTabla columnas={COLUMNAS.length} filas={8} />
      ) : (
        <section
          aria-label={t("menu-admin-audit-log")}
          aria-busy={loading}
          className={cn(CONTENEDOR_TABLA, loading && "opacity-60")}
        >
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                {COLUMNAS.map((c, i) => (
                  <TableHead
                    key={c.id}
                    className={cn(CABECERA_TABLA, i === 0 && "pl-4", i === COLUMNAS.length - 1 && "pr-4", c.className)}
                  >
                    {t(c.clave)}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody className="text-control">
              {rows.length === 0 ? (
                <FilaVacia columnas={COLUMNAS.length} icon={<ListMagnifyingGlassIcon className="size-6" />}>
                  {hayFiltros ? t("admin-audit-vacio-filtros") : t("admin-audit-vacio")}
                </FilaVacia>
              ) : (
                rows.map((r) => (
                  <TableRow key={r.id} className="group align-top">
                    <TableCell className="text-etiqueta text-muted-foreground py-2 pr-3 pl-4 font-mono tabular-nums">
                      {r.id}
                    </TableCell>
                    <TableCell className="px-3 py-2 whitespace-nowrap">{r.username ? `@${r.username}` : "—"}</TableCell>
                    <TableCell className="px-3 py-2">
                      <Chip className="font-mono">{r.accion}</Chip>
                    </TableCell>
                    <TableCell className="px-3 py-2 whitespace-nowrap">{r.entidad}</TableCell>
                    <TableCell className="px-3 py-2 font-mono tabular-nums">{r.entidad_id ?? "—"}</TableCell>
                    <TableCell className="text-muted-foreground px-3 py-2 font-mono whitespace-nowrap">
                      {r.ip || "—"}
                    </TableCell>
                    <TableCell className="px-3 py-2 whitespace-nowrap tabular-nums">
                      {r.created_at ? formatearFechaHora(r.created_at) : "—"}
                    </TableCell>
                    <TableCell className="py-2 pr-4 pl-3">
                      {r.detalle ? (
                        <pre className="text-etiqueta text-muted-foreground m-0 max-h-16 overflow-auto font-mono break-words whitespace-pre-wrap">
                          {JSON.stringify(r.detalle, null, 2)}
                        </pre>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
          <PieTabla
            desde={rows.length === 0 ? 0 : (pagina - 1) * porPagina + 1}
            hasta={(pagina - 1) * porPagina + rows.length}
            total={total}
            unidad={t("admin-audit-unidad")}
            porPagina={porPagina}
            onPorPagina={(n) => {
              setPorPagina(n);
              setPagina(1);
            }}
            pagina={Math.min(pagina, ultimaPagina)}
            ultimaPagina={ultimaPagina}
            onPagina={setPagina}
          />
        </section>
      )}
    </div>
  );
}
