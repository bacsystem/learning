"use client";

import {
  CeldaNombre,
  DialogoFormulario,
  etiquetaEstado,
  PillEstadoCatalogo,
  type TabEstado,
  TabsEstado,
} from "../_catalogos/componentes";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { Spinner } from "@/components/feedback/spinner";
import { useToast } from "@/components/feedback/toast";
import { AreaTexto, Campo, Entrada } from "@/components/formularios/campo";
import { Combobox } from "@/components/formularios/combobox";
import { SelectCampo } from "@/components/formularios/select-campo";
import { Chip } from "@/components/patrones/pill-estado";
import { Separator } from "@/components/ui/separator";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, type Proceso, type ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { type ConfiguracionExamen, simulacrosApi } from "@/lib/simulations/api";
import { cn } from "@/lib/utils";
import { ClipboardTextIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react";

type DialogMode = "create" | "edit";

interface AsignaturaEdit {
  asignatura_id: number;
  nombre: string;
  numero_preguntas: number;
  peso_porcentual: number;
  coeficiente: number;
}

interface DialogState {
  open: boolean;
  mode: DialogMode;
  editId: number | null;
  submitting: boolean;
  loadingAsignaturas: boolean;
  /** El paso 1 (configuración) ya se guardó y falló el 2: «Guardar» solo reintenta las asignaturas. */
  soloAsignaturas: boolean;
  error: string | null;
  form: {
    proceso_id: number | null;
    proceso_nombre: string;
    tiempo_total_seg: number;
    nota_minima: number;
    nota_maxima: number;
    descripcion: string;
    estado: string;
    asignaturas: AsignaturaEdit[];
  };
}

const DEFAULT_FORM = {
  proceso_id: null as number | null,
  proceso_nombre: "",
  tiempo_total_seg: 7200,
  nota_minima: 55.0,
  nota_maxima: 100.0,
  descripcion: "",
  estado: "activo",
  asignaturas: [] as AsignaturaEdit[],
};

const aEdicion = (items: ProcesoAsignaturaItem[]): AsignaturaEdit[] =>
  items.map((a) => ({
    asignatura_id: a.asignatura_id,
    nombre: a.nombre,
    numero_preguntas: a.numero_preguntas,
    peso_porcentual: a.peso_porcentual,
    coeficiente: a.coeficiente ?? 1,
  }));

// Límites del backend: CreateConfiguracionRequest / validateConfiguracion (simulations) y
// UpdateProcesoAsignaturaRequest (catalogs/dto.go: numero_preguntas 1..200, coeficiente 0.01..999.99 con dec2).
const MIN_TIEMPO_SEG = 60;
const ESCALA_MAXIMA = 100;
const MAX_PREGUNTAS_ASIGNATURA = 200;
const MIN_COEFICIENTE = 0.01;
const MAX_COEFICIENTE = 999.99;

const preguntasValidas = (n: number) => Number.isInteger(n) && n >= 1 && n <= MAX_PREGUNTAS_ASIGNATURA;
/** Misma tolerancia que `hasAtMost2Decimals` del backend. */
const conDosDecimales = (f: number) => Math.abs(f * 100 - Math.round(f * 100)) < 1e-6;
const coeficienteValido = (c: number) =>
  Number.isFinite(c) && c >= MIN_COEFICIENTE && c <= MAX_COEFICIENTE && conDosDecimales(c);

export default function ConfiguracionExamenPage() {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const toast = useToast();

  const [allRows, setAllRows] = useState<ConfiguracionExamen[]>([]);
  const [tab, setTab] = useState<TabEstado>("todos");
  const [loading, setLoading] = useState(true);
  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<DialogState>({
    open: false,
    mode: "create",
    editId: null,
    submitting: false,
    loadingAsignaturas: false,
    soloAsignaturas: false,
    error: null,
    form: { ...DEFAULT_FORM },
  });
  // Descarta respuestas de asignaturas que llegan tarde (otro proceso elegido, diálogo reabierto).
  const asignaturasReq = useRef(0);

  const loadData = useCallback(() => {
    if (!token) return;
    setLoading(true);
    setError(null);
    simulacrosApi
      .listConfiguraciones(token)
      .then((res) => setAllRows(res.data))
      .catch(() => setError(t("simulacros-config-error-cargar")))
      .finally(() => setLoading(false));
  }, [token, t]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  useEffect(() => {
    if (!token) return;
    catalogsApi
      .listProcesos(token)
      .then(setProcesos)
      .catch(() => {});
  }, [token]);

  // Carga las asignaturas del proceso elegido
  const loadAsignaturas = useCallback(
    async (procesoId: number) => {
      if (!token) return;
      const reqId = ++asignaturasReq.current;
      setDialog((d) => ({ ...d, loadingAsignaturas: true }));
      try {
        const items: ProcesoAsignaturaItem[] = await catalogsApi.listProcesoAsignaturas(token, procesoId);
        if (reqId !== asignaturasReq.current) return;
        setDialog((d) => ({ ...d, loadingAsignaturas: false, form: { ...d.form, asignaturas: aEdicion(items) } }));
      } catch {
        if (reqId !== asignaturasReq.current) return;
        setDialog((d) => ({
          ...d,
          loadingAsignaturas: false,
          error: t("admin-config-examen-error-cargar-asignaturas"),
        }));
      }
    },
    [token, t],
  );

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const totalPreguntas = useMemo(
    () => dialog.form.asignaturas.reduce((sum, a) => sum + (a.numero_preguntas || 0), 0),
    [dialog.form.asignaturas],
  );

  const itemsProceso = useMemo(() => procesos.map((p) => ({ value: String(p.id), label: p.nombre })), [procesos]);

  const setForm = (patch: Partial<DialogState["form"]>) => setDialog((d) => ({ ...d, form: { ...d.form, ...patch } }));

  const openCreate = () => {
    asignaturasReq.current++; // invalida cualquier carga pendiente del diálogo anterior
    setDialog({
      open: true,
      mode: "create",
      editId: null,
      submitting: false,
      loadingAsignaturas: false,
      soloAsignaturas: false,
      error: null,
      form: { ...DEFAULT_FORM },
    });
  };

  const openEdit = useCallback(
    (row: ConfiguracionExamen) => {
      asignaturasReq.current++;
      setDialog({
        open: true,
        mode: "edit",
        editId: row.id,
        submitting: false,
        loadingAsignaturas: false,
        soloAsignaturas: false,
        error: null,
        form: {
          proceso_id: row.proceso_id,
          proceso_nombre: row.proceso,
          tiempo_total_seg: row.tiempo_total_seg,
          nota_minima: row.nota_minima,
          nota_maxima: row.nota_maxima ?? 100,
          descripcion: row.descripcion ?? "",
          estado: row.estado,
          asignaturas: [],
        },
      });
      // Las asignaturas llegan después de abrir el diálogo
      if (row.proceso_id) loadAsignaturas(row.proceso_id);
    },
    [loadAsignaturas],
  );

  const handleProcesoChange = useCallback(
    (proceso: Proceso | null) => {
      asignaturasReq.current++;
      setDialog((d) => ({
        ...d,
        error: null,
        form: { ...d.form, proceso_id: proceso?.id ?? null, proceso_nombre: proceso?.nombre ?? "", asignaturas: [] },
      }));
      if (proceso) loadAsignaturas(proceso.id);
    },
    [loadAsignaturas],
  );

  const handleAsignaturaChange = (asignaturaId: number, field: "numero_preguntas" | "coeficiente", value: number) => {
    setDialog((d) => ({
      ...d,
      form: {
        ...d.form,
        asignaturas: d.form.asignaturas.map((a) => (a.asignatura_id === asignaturaId ? { ...a, [field]: value } : a)),
      },
    }));
  };

  // Valida todo antes del paso 1, con los mismos límites que el backend: así no queda la configuración guardada y
  // las asignaturas rechazadas.
  const errorValidacion = useMemo<string | null>(() => {
    const f = dialog.form;
    if (!Number.isInteger(f.tiempo_total_seg) || f.tiempo_total_seg < MIN_TIEMPO_SEG)
      return t("admin-config-examen-error-tiempo", { min: MIN_TIEMPO_SEG });
    if (!(f.nota_maxima > 0 && f.nota_maxima <= ESCALA_MAXIMA))
      return t("admin-config-examen-error-nota-maxima", { max: ESCALA_MAXIMA });
    if (!(f.nota_minima >= 0 && f.nota_minima <= f.nota_maxima)) return t("admin-config-examen-error-nota-minima");
    if (dialog.loadingAsignaturas || f.proceso_id === null) return null;
    if (f.asignaturas.length === 0) return t("admin-config-examen-error-sin-asignaturas");
    if (f.asignaturas.some((a) => !preguntasValidas(a.numero_preguntas)))
      return t("admin-config-examen-error-preguntas", { max: MAX_PREGUNTAS_ASIGNATURA });
    if (f.asignaturas.some((a) => !coeficienteValido(a.coeficiente)))
      return t("admin-config-examen-error-coeficiente", { min: MIN_COEFICIENTE, max: MAX_COEFICIENTE });
    return null;
  }, [dialog.form, dialog.loadingAsignaturas, t]);

  const puedeGuardar =
    dialog.form.proceso_id !== null &&
    !dialog.loadingAsignaturas &&
    !errorValidacion &&
    dialog.form.asignaturas.length > 0;

  const handleSubmit = async () => {
    if (!token || !puedeGuardar) return;
    const procesoId = dialog.form.proceso_id!;
    setDialog((d) => ({ ...d, submitting: true, error: null }));

    // 1. Guarda la configuración del examen (salvo que ya se guardara y solo falten las asignaturas)
    if (!dialog.soloAsignaturas) {
      try {
        if (dialog.mode === "create") {
          await simulacrosApi.createConfiguracion(token, {
            proceso_id: procesoId,
            tiempo_total_seg: dialog.form.tiempo_total_seg,
            total_preguntas: totalPreguntas,
            nota_minima: dialog.form.nota_minima,
            nota_maxima: dialog.form.nota_maxima,
            descripcion: dialog.form.descripcion,
          });
        } else {
          await simulacrosApi.updateConfiguracion(token, dialog.editId!, {
            tiempo_total_seg: dialog.form.tiempo_total_seg,
            total_preguntas: totalPreguntas,
            nota_minima: dialog.form.nota_minima,
            nota_maxima: dialog.form.nota_maxima,
            descripcion: dialog.form.descripcion,
            estado: dialog.form.estado,
          });
        }
      } catch (e: unknown) {
        const err = e as { message?: string };
        setDialog((d) => ({ ...d, submitting: false, error: err.message ?? t("simulacros-config-error-crear") }));
        return;
      }
      loadData();
    }

    // 2. Actualiza la cantidad de preguntas por asignatura
    try {
      await Promise.all(
        dialog.form.asignaturas.map((a) =>
          catalogsApi.updateProcesoAsignatura(token, procesoId, a.asignatura_id, {
            numero_preguntas: a.numero_preguntas,
            peso_porcentual: a.peso_porcentual,
            coeficiente: a.coeficiente,
          }),
        ),
      );
    } catch (e: unknown) {
      const err = e as { message?: string };
      setDialog((d) => ({
        ...d,
        submitting: false,
        soloAsignaturas: true,
        error: t("admin-config-examen-error-asignaturas-parcial", {
          error: err.message ?? t("comun-error-guardar"),
          accion: t("admin-config-examen-reintentar-asignaturas"),
        }),
      }));
      return;
    }

    setDialog((d) => ({ ...d, open: false, submitting: false, soloAsignaturas: false }));
    toast.ok(t("admin-config-examen-guardada"));
    loadData();
  };

  const columns = useMemo<ColumnaListado<ConfiguracionExamen>[]>(
    () => [
      {
        id: "proceso",
        titulo: t("simulacros-proceso"),
        ordenable: true,
        render: (r) => <CeldaNombre>{r.proceso}</CeldaNombre>,
      },
      {
        id: "total_preguntas",
        titulo: t("simulacros-total-preguntas"),
        ancho: "140px",
        alinear: "derecha",
        ordenable: true,
      },
      {
        id: "tiempo_total_seg",
        titulo: t("simulacros-tiempo"),
        ancho: "110px",
        alinear: "derecha",
        ordenable: true,
        texto: (r) => t("admin-config-examen-minutos", { n: Math.round(r.tiempo_total_seg / 60) }),
        render: (r) => t("admin-config-examen-minutos", { n: Math.round(r.tiempo_total_seg / 60) }),
      },
      {
        id: "nota_minima",
        titulo: t("simulacros-config-nota-min"),
        ancho: "110px",
        alinear: "derecha",
        ordenable: true,
        texto: (r) => r.nota_minima.toFixed(1),
        render: (r) => r.nota_minima.toFixed(1),
      },
      {
        id: "estado",
        titulo: t("simulacros-config-estado"),
        ancho: "120px",
        ordenable: true,
        texto: (r) => etiquetaEstado(r.estado, t),
        render: (r) => <PillEstadoCatalogo estado={r.estado} />,
      },
      {
        id: "descripcion",
        titulo: t("simulacros-config-descripcion"),
        className: "text-muted-foreground",
        texto: (r) => r.descripcion ?? "",
        render: (r) =>
          r.descripcion ? (
            <span title={r.descripcion} className="block max-w-72 truncate">
              {r.descripcion}
            </span>
          ) : (
            <span className="text-texto-terciario">—</span>
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
          <button
            type="button"
            onClick={() => openEdit(r)}
            aria-label={t("simulacros-config-editar")}
            title={t("simulacros-config-editar")}
            className="size-control text-muted-foreground/80 group-hover:text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:ring-ring/50 inline-flex items-center justify-center rounded-lg transition-colors outline-none focus-visible:ring-2"
          >
            <PencilSimpleIcon className="size-4" />
          </button>
        ),
      },
    ],
    [t, openEdit],
  );

  const isCreate = dialog.mode === "create";

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error ? (
        <Alerta
          tono="error"
          accion={
            <button type="button" onClick={loadData} className={ACCION_SECUNDARIA}>
              {t("admin-catalogos-reintentar")}
            </button>
          }
        >
          {error}
        </Alerta>
      ) : null}

      <TablaListado
        columnas={columns}
        filas={rows}
        clave={(r) => String(r.id)}
        titulo={t("simulacros-config-titulo")}
        placeholderBusqueda={t("admin-config-examen-buscar")}
        unidad={t("admin-config-examen-unidad")}
        cargando={loading && allRows.length === 0}
        nota={t("admin-config-examen-nota", { n: allRows.length, subtitulo: t("simulacros-config-subtitulo") })}
        vacio={
          allRows.length === 0
            ? t("admin-config-examen-vacio", { accion: t("simulacros-config-nueva") })
            : t("admin-config-examen-vacio-filtros")
        }
        filtros={<TabsEstado valor={tab} onCambio={setTab} filas={allRows} />}
        acciones={
          <button type="button" onClick={openCreate} className={ACCION_PRINCIPAL}>
            <PlusIcon weight="bold" className="size-4" />
            {t("simulacros-config-nueva")}
          </button>
        }
      />

      <DialogoFormulario
        open={dialog.open}
        onCerrar={() => setDialog((d) => ({ ...d, open: false }))}
        icon={ClipboardTextIcon}
        titulo={isCreate ? t("simulacros-config-dialog-titulo") : t("simulacros-config-dialog-editar")}
        error={dialog.error}
        pendiente={dialog.submitting}
        puedeGuardar={puedeGuardar}
        textoGuardar={
          dialog.soloAsignaturas ? t("admin-config-examen-reintentar-asignaturas") : t("simulacros-config-guardar")
        }
        textoGuardando={t("simulacros-config-guardando")}
        textoCancelar={t("simulacros-cancelar")}
        onGuardar={handleSubmit}
        ancho="sm:max-w-xl"
      >
        {/* Proceso */}
        <Campo id="config-proceso" etiqueta={t("simulacros-proceso")} className="md:col-span-2">
          {isCreate ? (
            <Combobox
              id="config-proceso"
              items={itemsProceso}
              valor={dialog.form.proceso_id !== null ? String(dialog.form.proceso_id) : null}
              onCambio={(v) => handleProcesoChange(procesos.find((p) => String(p.id) === v) ?? null)}
              placeholder={t("simulacros-seleccionar-proceso-placeholder")}
              disabled={dialog.soloAsignaturas}
            />
          ) : (
            <Entrada id="config-proceso" value={dialog.form.proceso_nombre} disabled readOnly />
          )}
        </Campo>

        {/* Tiempo + Estado */}
        <Campo
          id="config-tiempo"
          etiqueta={t("simulacros-config-tiempo-seg")}
          ayuda={t("admin-config-examen-tiempo-ayuda", { n: Math.round(dialog.form.tiempo_total_seg / 60) })}
          className={cn(isCreate && "md:col-span-2")}
        >
          <Entrada
            id="config-tiempo"
            type="number"
            inputMode="numeric"
            min={MIN_TIEMPO_SEG}
            required
            disabled={dialog.soloAsignaturas}
            className="tabular-nums"
            value={dialog.form.tiempo_total_seg}
            onChange={(e) => setForm({ tiempo_total_seg: parseInt(e.target.value) || 60 })}
          />
        </Campo>
        {!isCreate ? (
          <Campo id="config-estado" etiqueta={t("simulacros-config-estado")}>
            <SelectCampo
              id="config-estado"
              disabled={dialog.soloAsignaturas}
              opciones={[
                { value: "activo", label: t("simulacros-estado-activo") },
                { value: "inactivo", label: t("simulacros-estado-inactivo") },
              ]}
              valor={dialog.form.estado}
              onCambio={(v) => setForm({ estado: v })}
            />
          </Campo>
        ) : null}

        {/* Nota mínima | Nota máxima */}
        <Campo id="config-nota-minima" etiqueta={t("simulacros-nota-minima")}>
          <Entrada
            id="config-nota-minima"
            type="number"
            inputMode="decimal"
            min={0}
            max={dialog.form.nota_maxima}
            step={0.5}
            disabled={dialog.soloAsignaturas}
            className="tabular-nums"
            value={dialog.form.nota_minima}
            onChange={(e) => setForm({ nota_minima: parseFloat(e.target.value) || 0 })}
          />
        </Campo>
        <Campo id="config-nota-maxima" etiqueta={t("admin-config-examen-nota-maxima")}>
          <Entrada
            id="config-nota-maxima"
            type="number"
            inputMode="decimal"
            min={1}
            max={ESCALA_MAXIMA}
            step={1}
            disabled={dialog.soloAsignaturas}
            className="tabular-nums"
            value={dialog.form.nota_maxima}
            onChange={(e) => setForm({ nota_maxima: parseFloat(e.target.value) || 100 })}
          />
        </Campo>

        {/* Descripción */}
        <Campo id="config-descripcion" etiqueta={t("simulacros-config-descripcion-optional")} className="md:col-span-2">
          <AreaTexto
            id="config-descripcion"
            rows={2}
            disabled={dialog.soloAsignaturas}
            value={dialog.form.descripcion}
            onChange={(e) => setForm({ descripcion: e.target.value })}
          />
        </Campo>

        {/* Preguntas por asignatura */}
        {dialog.form.asignaturas.length > 0 || dialog.loadingAsignaturas ? (
          <div className="grid gap-3 md:col-span-2">
            <Separator className="bg-border/60" />
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-titulo text-foreground">{t("admin-config-examen-preguntas-por-asignatura")}</h3>
              <Chip className={cn(totalPreguntas > 0 && "bg-accent text-accent-foreground")}>
                {t("admin-config-examen-total", { n: totalPreguntas })}
              </Chip>
            </div>
            {dialog.loadingAsignaturas ? (
              <p className="text-control text-muted-foreground flex items-center gap-2">
                <Spinner tamano="sm" />
                {t("admin-config-examen-cargando-asignaturas")}
              </p>
            ) : (
              <div className="grid gap-1.5">
                <div className="text-etiqueta text-texto-terciario grid grid-cols-[minmax(0,1fr)_6rem_7rem] gap-2 px-1 font-medium">
                  <span>{t("admin-config-examen-col-asignatura")}</span>
                  <span className="text-center">{t("admin-config-examen-col-preguntas")}</span>
                  <span className="text-center">{t("admin-config-examen-col-coeficiente")}</span>
                </div>
                {dialog.form.asignaturas.map((a) => (
                  <div
                    key={a.asignatura_id}
                    className="bg-secondary grid grid-cols-[minmax(0,1fr)_6rem_7rem] items-center gap-2 rounded-lg py-1 pr-1 pl-3"
                  >
                    <span className="text-control text-foreground truncate" title={a.nombre}>
                      {a.nombre}
                    </span>
                    <Entrada
                      type="number"
                      inputMode="numeric"
                      min={1}
                      max={MAX_PREGUNTAS_ASIGNATURA}
                      step={1}
                      aria-label={t("admin-config-examen-preguntas-de", { nombre: a.nombre })}
                      className="text-center tabular-nums"
                      invalido={!preguntasValidas(a.numero_preguntas)}
                      // En el reintento del paso 2 la configuración ya se guardó con este total: no se edita.
                      disabled={dialog.soloAsignaturas}
                      value={a.numero_preguntas}
                      onChange={(e) =>
                        handleAsignaturaChange(a.asignatura_id, "numero_preguntas", parseInt(e.target.value) || 0)
                      }
                    />
                    <Entrada
                      type="number"
                      inputMode="decimal"
                      min={MIN_COEFICIENTE}
                      max={MAX_COEFICIENTE}
                      step={0.01}
                      aria-label={t("admin-config-examen-coeficiente-de", { nombre: a.nombre })}
                      className="text-center tabular-nums"
                      invalido={!coeficienteValido(a.coeficiente)}
                      // En el reintento del paso 2 la configuración ya se guardó con este total: no se edita.
                      disabled={dialog.soloAsignaturas}
                      value={a.coeficiente}
                      onChange={(e) =>
                        handleAsignaturaChange(a.asignatura_id, "coeficiente", parseFloat(e.target.value) || 0)
                      }
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : null}
        {errorValidacion ? (
          <p role="alert" className="text-control text-destructive md:col-span-2">
            {errorValidacion}
          </p>
        ) : null}
      </DialogoFormulario>
    </div>
  );
}
