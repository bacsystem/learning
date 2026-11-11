"use client";

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";

import { DialogContent } from "@/components/app/dialogo";
import { TablaDatos } from "@/components/app/tabla-datos";
import { GraficoBarras } from "@/components/datos/grafico";
import { type Columna } from "@/components/datos/tabla-datos";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton, SkeletonMetricas } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { Campo, Entrada } from "@/components/formularios/campo";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { FilaMetricas, Metrica } from "@/components/patrones/metricas";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import {
  adminGetIngresos,
  adminGetIngresosConfig,
  adminPutIngresosConfig,
  type ConfigPago,
  formatSoles,
  type IngresosData,
} from "@/lib/admin/api";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { formatearFecha, formatearMonto } from "@/lib/formato";
import { cn } from "@/lib/utils";
import {
  CalendarIcon,
  ChartBarIcon,
  CoinsIcon,
  MoneyIcon,
  PencilSimpleIcon,
  TagIcon,
  UsersIcon,
  XIcon,
} from "@phosphor-icons/react";

const soles = (v: number) => formatearMonto("PEN", v);

// Mismo patrón que montoRe en backend/internal/admin/handler.go: DECIMAL(10,2), 0 … 99999999.99, sin signo ni exponente.
const MONTO_RE = /^(0|[1-9][0-9]{0,7})(\.[0-9]{1,2})?$/;

/** "2026-03" → "Mar 2026" (formatearFecha da "1 Mar 2026"; se quita el día). */
const etiquetaMes = (mes: string) => formatearFecha(`${mes}-01`).replace(/^\d+ /, "");

export default function IngresosPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [data, setData] = useState<IngresosData | null>(null);
  const [config, setConfig] = useState<ConfigPago[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingConfig, setLoadingConfig] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [editDialog, setEditDialog] = useState<{
    open: boolean;
    anio: number;
    monto: string;
    submitting: boolean;
    error: string | null;
  }>({ open: false, anio: 0, monto: "", submitting: false, error: null });

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const resp = await adminGetIngresos(token);
      setData(resp);
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message ?? t("admin-ingresos-error-cargar"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  const loadConfig = useCallback(async () => {
    if (!token) return;
    setLoadingConfig(true);
    try {
      const resp = await adminGetIngresosConfig(token);
      setConfig(resp);
    } catch {
      // non-critical
    } finally {
      setLoadingConfig(false);
    }
  }, [token]);

  useEffect(() => {
    load();
    loadConfig();
  }, [load, loadConfig]);

  const montoInvalido = !MONTO_RE.test(editDialog.monto.trim());

  const handleSaveConfig = async () => {
    if (!token || montoInvalido) return;
    // El literal ya pasó MONTO_RE (≤ 8 enteros, ≤ 2 decimales): Number() lo representa y serializa sin pérdida.
    const monto = Number(editDialog.monto.trim());
    setEditDialog((d) => ({ ...d, submitting: true, error: null }));
    try {
      const updated = await adminPutIngresosConfig(token, editDialog.anio, monto);
      setConfig((prev) => {
        const idx = prev.findIndex((c) => c.anio === updated.anio);
        if (idx >= 0) {
          const next = [...prev];
          next[idx] = updated;
          return next;
        }
        return [...prev, updated];
      });
      setEditDialog({ open: false, anio: 0, monto: "", submitting: false, error: null });
      toast.ok(t("admin-ingresos-tarifa-actualizada", { anio: updated.anio }));
    } catch (e: unknown) {
      // El error va dentro del diálogo: la alerta de página queda tapada por el modal.
      const err = e as { message?: string };
      setEditDialog((d) => ({ ...d, submitting: false, error: err.message ?? t("admin-ingresos-error-tarifa") }));
    }
  };

  const monthLabels = (data?.serie_mensual ?? []).map((m) => etiquetaMes(m.mes));
  // Charting needs numbers; the displayed totals stay exact decimal strings.
  const monthAmounts = (data?.serie_mensual ?? []).map((m) => Number(m.total));
  // The series is zero-filled (H142): twelve empty months are still "no data".
  const sinIngresos = monthAmounts.every((v) => v === 0);
  const mesPico = monthAmounts.length > 0 ? monthAmounts.indexOf(Math.max(...monthAmounts)) : undefined;

  const columnasTarifas: Columna<ConfigPago>[] = [
    {
      id: "anio",
      titulo: t("admin-ingresos-anio"),
      ordenable: true,
      render: (c) => (
        <span className="inline-flex items-center gap-2 font-medium tabular-nums">
          <CalendarIcon aria-hidden className="text-muted-foreground size-4" />
          {c.anio}
        </span>
      ),
    },
    {
      id: "monto",
      titulo: t("admin-ingresos-monto"),
      alinear: "derecha",
      ordenable: true,
      render: (c) => <span className="font-semibold">{soles(c.monto)}</span>,
    },
    {
      id: "editar",
      titulo: t("admin-ingresos-editar"),
      alinear: "derecha",
      ancho: "6rem",
      render: (c) => (
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`${t("admin-ingresos-editar")} ${c.anio}`}
          title={`${t("admin-ingresos-editar")} ${c.anio}`}
          onClick={() =>
            setEditDialog({ open: true, anio: c.anio, monto: String(c.monto), submitting: false, error: null })
          }
          className="text-muted-foreground"
        >
          <PencilSimpleIcon className="size-4" />
        </Button>
      ),
    },
  ];

  return (
    <div className={CONTENEDOR_PAGINA}>
      <h2 className="sr-only">{t("admin-ingresos-titulo")}</h2>
      <p className="text-cuerpo text-muted-foreground">{t("admin-ingresos-subtitulo")}</p>

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

      {/* Métricas */}
      {loading ? (
        <SkeletonMetricas columnas={3} />
      ) : (
        <FilaMetricas columnas={3}>
          <Metrica
            etiqueta={t("admin-ingresos-total-historico")}
            icono={MoneyIcon}
            tono="acento"
            destacada
            ayuda={t("admin-ingresos-inscripciones-aprobadas-ayuda")}
          >
            {formatSoles(data?.total_recaudado)}
          </Metrica>
          <Metrica etiqueta={t("admin-ingresos-este-anio")} icono={CoinsIcon} ayuda={String(new Date().getFullYear())}>
            {formatSoles(data?.total_este_anio)}
          </Metrica>
          <Metrica
            etiqueta={t("admin-ingresos-inscripciones-aprobadas")}
            icono={UsersIcon}
            tono="ok"
            ayuda={t("admin-ingresos-total-historico-ayuda")}
          >
            {data?.total_inscripciones ?? 0}
          </Metrica>
        </FilaMetricas>
      )}

      {/* Ingresos por mes */}
      {loading ? (
        <div className={cn(TARJETA, "flex flex-col gap-3 px-4 py-3.5")}>
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-[280px] w-full rounded-xl" />
        </div>
      ) : monthLabels.length === 0 || sinIngresos ? (
        <EstadoVacio icon={ChartBarIcon} titulo={t("admin-ingresos-por-mes")}>
          {t("admin-ingresos-sin-datos")}
        </EstadoVacio>
      ) : (
        <GraficoBarras
          titulo={t("admin-ingresos-por-mes")}
          categorias={monthLabels}
          series={[{ nombre: t("admin-ingresos-serie"), valores: monthAmounts }]}
          formato={soles}
          destacado={mesPico}
          alto={280}
          descripcion={t("admin-ingresos-grafico-desc", {
            n: monthLabels.length,
            total: soles(monthAmounts.reduce((a, b) => a + b, 0)),
          })}
        />
      )}

      {/* Tarifas */}
      <section className="flex flex-col gap-1">
        <CabeceraSeccion
          icon={TagIcon}
          titulo={t("admin-ingresos-tarifas")}
          subtitulo={t("admin-ingresos-tarifas-subtitulo")}
          conBorde={false}
          className="px-0"
        />
        <TablaDatos
          columnas={columnasTarifas}
          filas={config}
          clave={(c) => String(c.anio)}
          cargando={loadingConfig}
          unidad={t("admin-ingresos-tarifas-unidad")}
          vacio={t("admin-ingresos-tarifas-vacio")}
          ordenInicial={{ id: "anio", dir: "desc" }}
        />
      </section>

      {/* Dialog: Edit tariff */}
      <Dialog
        open={editDialog.open}
        onOpenChange={(open) => {
          if (!open && !editDialog.submitting) setEditDialog((d) => ({ ...d, open: false }));
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={CoinsIcon}
            titulo={t("admin-ingresos-editar-tarifa", { anio: editDialog.anio })}
            descripcion={t("admin-ingresos-editar-tarifa-desc")}
          />
          <form
            className="grid gap-4 px-5 pb-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (!montoInvalido && !editDialog.submitting) handleSaveConfig();
            }}
          >
            <Campo
              id="tarifa-monto"
              etiqueta={t("admin-ingresos-monto")}
              ayuda={t("admin-ingresos-monto-ayuda")}
              error={
                editDialog.monto !== "" && montoInvalido
                  ? t("admin-ingresos-monto-invalido")
                  : (editDialog.error ?? undefined)
              }
            >
              <Entrada
                id="tarifa-monto"
                type="text"
                inputMode="decimal"
                required
                autoFocus
                className="tabular-nums"
                value={editDialog.monto}
                invalido={(editDialog.monto !== "" && montoInvalido) || !!editDialog.error}
                onChange={(e) => setEditDialog((d) => ({ ...d, monto: e.target.value, error: null }))}
              />
            </Campo>
          </form>
          <PieDialogo>
            <button
              type="button"
              className={BOTON_SECUNDARIO}
              disabled={editDialog.submitting}
              onClick={() => setEditDialog((d) => ({ ...d, open: false }))}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={editDialog.submitting}
              textoPendiente={t("comun-guardando")}
              disabled={montoInvalido}
              className={BOTON_PRIMARIO}
              onClick={handleSaveConfig}
            >
              {t("comun-guardar")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
