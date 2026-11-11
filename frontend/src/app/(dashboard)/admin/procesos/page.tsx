"use client";
import { useTranslations } from "next-intl";
import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Campo } from "@/components/app/campo";
import { DialogContent } from "@/components/app/dialogo";
import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { ProgresoLineal } from "@/components/feedback/progreso";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { AreaTexto, Entrada } from "@/components/formularios/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { Tabs } from "@/components/navegacion/tabs";
import { BotonAsync } from "@/components/patrones/boton-async";
import { Aviso, CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { FilaVacia } from "@/components/patrones/pie-tabla";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Dialog } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { excede, LIMITES } from "@/lib/admin/limites";
import { useAuth } from "@/lib/auth/auth-context";
import { Asignatura, catalogsApi, Grado, Institucion, Proceso, ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import {
  ACCION_PRINCIPAL,
  ACCION_SECUNDARIA,
  BOTON_DESTRUCTIVO,
  BOTON_PRIMARIO,
  BOTON_SECUNDARIO,
  CABECERA_TABLA,
  CONTENEDOR_PAGINA,
} from "@/lib/estilos";
import { cn } from "@/lib/utils";
import {
  ArrowClockwiseIcon,
  CheckSquareIcon,
  EyeSlashIcon,
  ListChecksIcon,
  PencilSimpleIcon,
  PlusIcon,
  SignpostIcon,
  TrashIcon,
  WarningIcon,
} from "@phosphor-icons/react";

type DialogMode = "create" | "edit";

const TABS = ["todos", "activo", "inactivo"] as const;
type Tab = (typeof TABS)[number];

type ProcesoTipo = "ascenso" | "pase_efectividad" | "admision";

const TIPOS: ProcesoTipo[] = ["ascenso", "pase_efectividad", "admision"];

/** Color de categoría de cada tipo (no es un estado: acentos y tonos suaves). */
const TIPO_CLASES: Record<ProcesoTipo, string> = {
  ascenso: "bg-acento-2-suave text-acento-2-texto dark:text-acento-2",
  pase_efectividad: "bg-warning text-warning-foreground",
  admision: "bg-acento-suave text-acento-texto dark:text-acento",
};

/** Clave i18n de cada tipo de proceso. */
const TIPO_CLAVE: Record<ProcesoTipo, string> = {
  ascenso: "procesos-tipo-ascenso",
  pase_efectividad: "procesos-tipo-pase-efectividad",
  admision: "admin-procesos-tipo-admision",
};

/** Límites de `AsignarAsignaturaRequest` (backend/internal/catalogs/dto.go). */
const PESO_MIN = 0.01;
const PESO_MAX = 100;
const PREGUNTAS_MIN = 1;
const PREGUNTAS_MAX = 200;
/** `dec2`: como mucho dos decimales (NUMERIC(5,2)). */
const tieneDosDecimales = (n: number) => Math.abs(Math.round(n * 100) - n * 100) < 1e-6;

const DEFAULT_FORM = {
  institucion_id: 0,
  nombre: "",
  tipo: "ascenso" as ProcesoTipo,
  grado_id: null as number | null,
  descripcion: "",
  anio: new Date().getFullYear(),
};

const DEFAULT_ADD_FORM = { asignatura_id: 0, numero_preguntas: 1, peso_porcentual: 0, coeficiente: 1 };

export default function ProcesosPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  /** Etiquetas traducidas; un valor desconocido del backend se muestra tal cual. */
  const etiquetaTipo = useCallback(
    (tipo: string) => (tipo in TIPO_CLAVE ? t(TIPO_CLAVE[tipo as ProcesoTipo]) : tipo),
    [t],
  );
  const etiquetaEstado = useCallback(
    (estado: string) => (TABS.includes(estado as Tab) ? t(`admin-procesos-estado-${estado}`) : estado),
    [t],
  );
  const [allRows, setAllRows] = useState<Proceso[]>([]);
  const [tab, setTab] = useState<Tab>("todos");
  const [instituciones, setInstituciones] = useState<Institucion[]>([]);
  const [grados, setGrados] = useState<Grado[]>([]);
  const [asignaturas, setAsignaturas] = useState<Asignatura[]>([]);
  const [filterInstId, setFilterInstId] = useState<number>(0);
  const [filterTipo, setFilterTipo] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [dialog, setDialog] = useState<{
    open: boolean;
    mode: DialogMode;
    row: Proceso | null;
    form: typeof DEFAULT_FORM;
    submitting: boolean;
  }>({ open: false, mode: "create", row: null, form: DEFAULT_FORM, submitting: false });

  const [confirmDialog, setConfirmDialog] = useState<{
    open: boolean;
    row: Proceso | null;
    submitting: boolean;
  }>({ open: false, row: null, submitting: false });

  const [asigDialog, setAsigDialog] = useState<{
    open: boolean;
    proceso: Proceso | null;
    items: ProcesoAsignaturaItem[];
    loadingAsig: boolean;
    addForm: { asignatura_id: number; numero_preguntas: number; peso_porcentual: number; coeficiente: number };
    addSubmitting: boolean;
  }>({
    open: false,
    proceso: null,
    items: [],
    loadingAsig: false,
    addForm: DEFAULT_ADD_FORM,
    addSubmitting: false,
  });

  // H29: removing a subject asks for confirmation.
  const [quitarDialog, setQuitarDialog] = useState<{
    open: boolean;
    item: ProcesoAsignaturaItem | null;
    submitting: boolean;
  }>({ open: false, item: null, submitting: false });

  // H26: only the response for the last opened process is applied.
  const asigReqRef = useRef(0);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const q: Record<string, string | number> = {};
      if (filterInstId) q.institucion_id = filterInstId;
      if (filterTipo) q.tipo = filterTipo;

      const [data, insts, asigs] = await Promise.all([
        catalogsApi.listProcesos(token, q),
        catalogsApi.listInstituciones(token, { limit: "all", estado: "activo" }),
        catalogsApi.listAsignaturas(token, { limit: "all", estado: "activo" }),
      ]);
      setAllRows(data);
      setInstituciones(insts);
      setAsignaturas(asigs);
    } catch {
      setError(t("admin-error-cargar-procesos"));
    } finally {
      setLoading(false);
    }
  }, [token, filterInstId, filterTipo, t]);

  useEffect(() => {
    load();
  }, [load]);

  // Solo se aplica la respuesta de la última petición de grados (cambiar rápido de institución no deja los de otra).
  const gradosReq = useRef(0);
  useEffect(() => {
    const reqId = ++gradosReq.current;
    if (!token || !dialog.open || !dialog.form.institucion_id) return;
    catalogsApi
      .listGrados(token, { institucion_id: dialog.form.institucion_id, limit: "all", estado: "activo" })
      .then((data) => {
        if (reqId === gradosReq.current) setGrados(data);
      })
      .catch(() => {
        if (reqId === gradosReq.current) setGrados([]);
      });
  }, [token, dialog.open, dialog.form.institucion_id]);

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const openCreate = () => setDialog({ open: true, mode: "create", row: null, form: DEFAULT_FORM, submitting: false });

  const openEdit = useCallback(
    (row: Proceso) =>
      setDialog({
        open: true,
        mode: "edit",
        row,
        form: {
          institucion_id: row.institucion_id,
          nombre: row.nombre,
          tipo: row.tipo,
          grado_id: row.grado_id ?? null,
          descripcion: row.descripcion ?? "",
          anio: row.anio,
        },
        submitting: false,
      }),
    [],
  );

  const closeDialog = () => setDialog((d) => ({ ...d, open: false }));
  const setForm = (patch: Partial<typeof DEFAULT_FORM>) => setDialog((d) => ({ ...d, form: { ...d.form, ...patch } }));

  const submitDialog = async () => {
    if (!token) return;
    setDialog((d) => ({ ...d, submitting: true }));
    try {
      const body = {
        ...dialog.form,
        grado_id: dialog.form.tipo === "pase_efectividad" ? dialog.form.grado_id : null,
      };
      if (dialog.mode === "create") {
        await catalogsApi.createProceso(token, body);
        toast.ok(t("admin-procesos-creado"));
      } else if (dialog.row) {
        await catalogsApi.updateProceso(token, dialog.row.id, body);
        toast.ok(t("admin-procesos-actualizado"));
      }
      closeDialog();
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      toast.error(err.message ?? t("comun-error-guardar"));
      setDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const openConfirm = useCallback((row: Proceso) => setConfirmDialog({ open: true, row, submitting: false }), []);
  const closeConfirm = () => setConfirmDialog((d) => ({ ...d, open: false }));

  const submitEstado = async () => {
    if (!token || !confirmDialog.row) return;
    const nuevoEstado = confirmDialog.row.estado === "activo" ? "inactivo" : "activo";
    setConfirmDialog((d) => ({ ...d, submitting: true }));
    try {
      await catalogsApi.updateProcesoEstado(token, confirmDialog.row!.id, nuevoEstado);
      toast.ok(nuevoEstado === "activo" ? t("admin-procesos-activado") : t("admin-procesos-desactivado"));
      closeConfirm();
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      toast.error(err.message ?? t("comun-error-estado"));
      setConfirmDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const openAsigDialog = useCallback(
    async (proceso: Proceso) => {
      if (!token) return;
      const reqId = ++asigReqRef.current;
      setAsigDialog((d) => ({
        ...d,
        open: true,
        proceso,
        items: [],
        loadingAsig: true,
        addForm: DEFAULT_ADD_FORM,
      }));
      try {
        const items = await catalogsApi.listProcesoAsignaturas(token, proceso.id);
        if (reqId !== asigReqRef.current) return;
        setAsigDialog((d) => ({ ...d, items, loadingAsig: false }));
      } catch {
        if (reqId !== asigReqRef.current) return;
        setAsigDialog((d) => ({ ...d, loadingAsig: false }));
      }
    },
    [token],
  );

  const closeAsigDialog = () => {
    // Invalidates any in-flight load for the process being closed.
    asigReqRef.current++;
    setAsigDialog((d) => ({ ...d, open: false, proceso: null }));
  };
  /** Applies a refreshed list only if the dialog still shows that process. */
  const setItemsDe = (procesoId: number, items: ProcesoAsignaturaItem[], extra: Partial<typeof asigDialog> = {}) =>
    setAsigDialog((d) => (d.proceso?.id === procesoId ? { ...d, ...extra, items } : d));
  const setAddForm = (patch: Partial<typeof DEFAULT_ADD_FORM>) =>
    setAsigDialog((d) => ({ ...d, addForm: { ...d.addForm, ...patch } }));

  /**
   * Recarga las asignaturas del proceso tras asignar o quitar una. Va aparte del POST/DELETE: si la operación salió bien
   * y solo falla la recarga, no se informa como error de la operación (el reintento daría 409), sino con un aviso.
   */
  const recargarAsignaturas = async (procesoId: number) => {
    if (!token) return;
    try {
      const items = await catalogsApi.listProcesoAsignaturas(token, procesoId);
      setItemsDe(procesoId, items);
    } catch {
      toast.aviso(t("admin-procesos-aviso-recargar-asignaturas"));
    }
  };

  const addAsignatura = async () => {
    if (!token || !asigDialog.proceso) return;
    const procesoId = asigDialog.proceso.id;
    setAsigDialog((d) => ({ ...d, addSubmitting: true }));
    try {
      await catalogsApi.asignarAsignatura(token, procesoId, asigDialog.addForm);
    } catch (e: unknown) {
      const err = e as { message?: string };
      toast.error(err.message ?? t("admin-error-asignar-asignatura"));
      setAsigDialog((d) => ({ ...d, addSubmitting: false }));
      return;
    }
    toast.ok(t("admin-procesos-asignatura-asignada"));
    setAsigDialog((d) => (d.proceso?.id === procesoId ? { ...d, addForm: DEFAULT_ADD_FORM } : d));
    await recargarAsignaturas(procesoId);
    setAsigDialog((d) => ({ ...d, addSubmitting: false }));
  };

  const removeAsignatura = async () => {
    if (!token || !asigDialog.proceso || !quitarDialog.item) return;
    const procesoId = asigDialog.proceso.id;
    setQuitarDialog((d) => ({ ...d, submitting: true }));
    try {
      await catalogsApi.deleteProcesoAsignatura(token, procesoId, quitarDialog.item.asignatura_id);
    } catch (e: unknown) {
      const err = e as { message?: string };
      toast.error(err.message ?? t("admin-error-eliminar-asignatura"));
      setQuitarDialog((d) => ({ ...d, submitting: false }));
      return;
    }
    setQuitarDialog({ open: false, item: null, submitting: false });
    toast.ok(t("admin-procesos-asignatura-quitada"));
    await recargarAsignaturas(procesoId);
  };

  const pesoTotal = asigDialog.items.reduce((acc, i) => acc + i.peso_porcentual, 0);
  const pesoOk = Math.abs(pesoTotal - 100) < 0.01;

  const columns = useMemo<ColumnaListado<Proceso>[]>(
    () => [
      {
        id: "nombre",
        titulo: t("admin-procesos-col-nombre"),
        ordenable: true,
        ocultable: false,
        render: (r) => <span className="text-foreground font-medium">{r.nombre}</span>,
      },
      {
        id: "tipo",
        titulo: t("admin-procesos-tipo"),
        ordenable: true,
        texto: (r) => etiquetaTipo(r.tipo),
        render: (r) => (
          <span
            className={cn(
              "text-etiqueta inline-flex items-center rounded-full px-2.5 py-0.5 font-medium whitespace-nowrap",
              TIPO_CLASES[r.tipo as ProcesoTipo] ?? "bg-secondary text-muted-foreground",
            )}
          >
            {etiquetaTipo(r.tipo)}
          </span>
        ),
      },
      { id: "anio", titulo: t("admin-procesos-anio"), ordenable: true, alinear: "derecha", valor: (r) => r.anio },
      { id: "institucion", titulo: t("perfil-institucion"), ordenable: true },
      {
        id: "grado",
        titulo: t("perfil-grado"),
        texto: (r) => r.grado ?? "",
        render: (r) => (r.grado ? r.grado : <span className="text-texto-terciario">—</span>),
      },
      {
        id: "estado",
        titulo: t("perfil-estado"),
        ordenable: true,
        render: (r) => (
          <PillEstado tono={r.estado === "activo" ? "ok" : "neutro"}>{etiquetaEstado(r.estado)}</PillEstado>
        ),
        texto: (r) => etiquetaEstado(r.estado),
      },
      {
        id: "acciones",
        titulo: <span className="sr-only">{t("admin-procesos-acciones")}</span>,
        exportable: false,
        ocultable: false,
        alinear: "derecha",
        ancho: "3rem",
        render: (r) => (
          <MenuAcciones
            etiqueta={t("admin-procesos-acciones-de", { nombre: r.nombre })}
            acciones={[
              {
                label: t("admin-procesos-gestionar-asignaturas"),
                icon: ListChecksIcon,
                onClick: () => openAsigDialog(r),
              },
              { label: t("comun-editar"), icon: PencilSimpleIcon, onClick: () => openEdit(r) },
              {
                label: r.estado === "activo" ? t("comun-desactivar") : t("comun-activar"),
                icon: r.estado === "activo" ? EyeSlashIcon : CheckSquareIcon,
                separador: true,
                onClick: () => openConfirm(r),
              },
            ]}
          />
        ),
      },
    ],
    [openEdit, openConfirm, openAsigDialog, t, etiquetaTipo, etiquetaEstado],
  );

  const formValid =
    dialog.form.institucion_id > 0 &&
    dialog.form.nombre.trim().length >= 3 &&
    !excede(dialog.form.nombre, LIMITES.proceso.nombre) &&
    !excede(dialog.form.descripcion, LIMITES.proceso.descripcion) &&
    dialog.form.anio >= 2024 &&
    dialog.form.anio <= 2099;

  const contar = (estado: Tab) =>
    estado === "todos" ? allRows.length : allRows.filter((r) => r.estado === estado).length;
  const asignaturasDisponibles = asignaturas.filter((a) => !asigDialog.items.find((i) => i.asignatura_id === a.id));
  const { peso_porcentual: pesoNuevo, numero_preguntas: preguntasNuevas } = asigDialog.addForm;
  // H26: same rules as the backend DTO, checked before sending.
  const avisoAgregar = !asigDialog.addForm.asignatura_id
    ? t("admin-procesos-aviso-selecciona-asignatura")
    : preguntasNuevas < PREGUNTAS_MIN || preguntasNuevas > PREGUNTAS_MAX
      ? t("admin-procesos-aviso-preguntas-rango", { min: PREGUNTAS_MIN, max: PREGUNTAS_MAX })
      : pesoNuevo < PESO_MIN || pesoNuevo > PESO_MAX || !tieneDosDecimales(pesoNuevo)
        ? t("admin-procesos-aviso-peso-rango", { min: PESO_MIN, max: PESO_MAX })
        : pesoTotal + pesoNuevo > 100.01
          ? t("admin-procesos-aviso-peso-excede")
          : "";

  const desactivando = confirmDialog.row?.estado === "activo";

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error ? (
        <Alerta
          tono="error"
          accion={
            <button type="button" onClick={load} className={ACCION_SECUNDARIA}>
              <ArrowClockwiseIcon weight="bold" className="size-4" />
              {t("admin-procesos-reintentar")}
            </button>
          }
        >
          {error}
        </Alerta>
      ) : null}

      <TablaListado
        titulo={t("admin-procesos-titulo")}
        columnas={columns}
        filas={rows}
        clave={(r) => String(r.id)}
        cargando={loading && allRows.length === 0}
        unidad={t("admin-procesos-unidad")}
        placeholderBusqueda={t("admin-procesos-buscar")}
        porPaginaInicial={10}
        vacio={allRows.length === 0 ? t("admin-procesos-vacio") : t("admin-procesos-sin-resultados-filtros")}
        acciones={
          <button type="button" className={ACCION_PRINCIPAL} onClick={openCreate}>
            <PlusIcon weight="bold" className="size-4" />
            {t("admin-nuevo-proceso")}
          </button>
        }
        filtros={
          <>
            <Tabs
              estilo="segmentado"
              valor={tab}
              onCambio={setTab}
              className="[&>[role=tabpanel]]:hidden"
              items={TABS.map((v) => ({
                id: v,
                etiqueta: etiquetaEstado(v),
                contador: contar(v),
              }))}
            />
            <div className="w-full sm:w-56">
              <SelectCampo
                id="filtro-institucion"
                etiqueta={t("perfil-institucion")}
                valor={filterInstId ? String(filterInstId) : ""}
                onCambio={(v) => setFilterInstId(Number(v) || 0)}
                opciones={[
                  { value: "", label: t("admin-procesos-filtro-institucion-todas") },
                  ...instituciones.map((i) => ({ value: String(i.id), label: i.nombre })),
                ]}
              />
            </div>
            <div className="w-full sm:w-48">
              <SelectCampo
                id="filtro-tipo"
                etiqueta={t("admin-procesos-tipo")}
                valor={filterTipo}
                onCambio={setFilterTipo}
                opciones={[
                  { value: "", label: t("admin-procesos-filtro-tipo-todos") },
                  ...TIPOS.map((tipo) => ({ value: tipo, label: etiquetaTipo(tipo) })),
                ]}
              />
            </div>
          </>
        }
      />

      {/* Create / Edit Dialog */}
      <Dialog
        open={dialog.open}
        onOpenChange={(open) => {
          if (!open && !dialog.submitting) closeDialog();
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-lg">
          <CabeceraDialogo
            icon={SignpostIcon}
            titulo={dialog.mode === "create" ? t("admin-nuevo-proceso") : t("admin-editar-proceso")}
          />
          {/* noValidate: las reglas son las de `formValid`, igual que antes; la validación nativa del navegador no aplica. */}
          <form
            id="form-proceso"
            noValidate
            className="grid gap-4 px-5 pb-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (formValid && !dialog.submitting) submitDialog();
            }}
          >
            <Campo id="proceso-institucion" etiqueta={t("perfil-institucion")}>
              <SelectCampo
                id="proceso-institucion"
                placeholder={t("admin-procesos-seleccionar-institucion")}
                valor={dialog.form.institucion_id ? String(dialog.form.institucion_id) : ""}
                onCambio={(v) => setForm({ institucion_id: Number(v) || 0, grado_id: null })}
                opciones={instituciones.map((i) => ({ value: String(i.id), label: i.nombre }))}
                required
              />
            </Campo>

            <Campo
              id="proceso-nombre"
              etiqueta={t("admin-procesos-col-nombre")}
              ayuda={t("admin-procesos-nombre-ayuda")}
              error={
                excede(dialog.form.nombre, LIMITES.proceso.nombre)
                  ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.proceso.nombre })
                  : undefined
              }
            >
              <Entrada
                id="proceso-nombre"
                required
                maxLength={LIMITES.proceso.nombre}
                invalido={excede(dialog.form.nombre, LIMITES.proceso.nombre)}
                value={dialog.form.nombre}
                onChange={(e) => setForm({ nombre: e.target.value })}
                placeholder={t("admin-procesos-nombre-placeholder")}
              />
            </Campo>

            <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_140px]">
              <Campo id="proceso-tipo" etiqueta={t("admin-procesos-tipo")}>
                <SelectCampo
                  id="proceso-tipo"
                  valor={dialog.form.tipo}
                  onCambio={(v) => setForm({ tipo: v as ProcesoTipo, grado_id: null })}
                  opciones={TIPOS.map((tipo) => ({ value: tipo, label: etiquetaTipo(tipo) }))}
                  required
                />
              </Campo>
              <Campo id="proceso-anio" etiqueta={t("admin-procesos-anio")}>
                <Entrada
                  id="proceso-anio"
                  type="number"
                  inputMode="numeric"
                  required
                  min={2024}
                  max={2099}
                  className="tabular-nums"
                  value={dialog.form.anio}
                  onChange={(e) => setForm({ anio: parseInt(e.target.value) || 2024 })}
                />
              </Campo>
            </div>

            {dialog.form.tipo === "pase_efectividad" && (
              <Campo id="proceso-grado" etiqueta={t("admin-procesos-grado-requerido")}>
                <SelectCampo
                  id="proceso-grado"
                  placeholder={
                    dialog.form.institucion_id
                      ? t("admin-procesos-seleccionar-grado")
                      : t("admin-procesos-elige-institucion")
                  }
                  valor={dialog.form.grado_id ? String(dialog.form.grado_id) : ""}
                  onCambio={(v) => setForm({ grado_id: Number(v) || null })}
                  opciones={grados.map((g) => ({ value: String(g.id), label: `${g.nombre} (${g.abreviatura})` }))}
                  disabled={!dialog.form.institucion_id}
                  required
                />
              </Campo>
            )}

            <Campo
              id="proceso-descripcion"
              etiqueta={t("admin-procesos-descripcion")}
              opcional
              error={
                excede(dialog.form.descripcion, LIMITES.proceso.descripcion)
                  ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.proceso.descripcion })
                  : undefined
              }
            >
              <AreaTexto
                id="proceso-descripcion"
                rows={2}
                maxLength={LIMITES.proceso.descripcion}
                invalido={excede(dialog.form.descripcion, LIMITES.proceso.descripcion)}
                value={dialog.form.descripcion}
                onChange={(e) => setForm({ descripcion: e.target.value })}
                placeholder={t("admin-procesos-descripcion-placeholder")}
              />
            </Campo>
          </form>
          <PieDialogo>
            <button type="button" onClick={closeDialog} disabled={dialog.submitting} className={BOTON_SECUNDARIO}>
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              type="submit"
              form="form-proceso"
              pendiente={dialog.submitting}
              textoPendiente={t("comun-guardando")}
              disabled={!formValid}
              className={BOTON_PRIMARIO}
            >
              {dialog.mode === "create" ? t("comun-crear") : t("comun-guardar-cambios")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>

      {/* Confirm Estado Dialog */}
      <Dialog
        open={confirmDialog.open}
        onOpenChange={(open) => {
          if (!open && !confirmDialog.submitting) closeConfirm();
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={desactivando ? EyeSlashIcon : CheckSquareIcon}
            tono={desactivando ? "aviso" : "ok"}
            titulo={desactivando ? t("admin-procesos-desactivar") : t("admin-procesos-activar")}
            descripcion={desactivando ? undefined : t("admin-procesos-activar-desc")}
          />
          {confirmDialog.row && (
            <div className="px-5 pb-2">
              <div className="bg-secondary rounded-lg px-3 py-2.5">
                <p className="text-control text-foreground font-semibold">{confirmDialog.row.nombre}</p>
                <p className="text-etiqueta text-muted-foreground">
                  {etiquetaTipo(confirmDialog.row.tipo)} ·{" "}
                  <span className="tabular-nums">{confirmDialog.row.anio}</span>
                </p>
              </div>
            </div>
          )}
          <PieDialogo>
            <button
              type="button"
              onClick={closeConfirm}
              disabled={confirmDialog.submitting}
              className={BOTON_SECUNDARIO}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={confirmDialog.submitting}
              textoPendiente={t("comun-procesando")}
              onClick={submitEstado}
              className={BOTON_PRIMARIO}
            >
              {desactivando ? t("comun-confirmar-desactivacion") : t("comun-confirmar-activacion")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>

      {/* Asignaturas sub-panel Dialog */}
      <Dialog open={asigDialog.open} onOpenChange={(open) => !open && closeAsigDialog()}>
        <DialogContent className="gap-0 p-0 sm:max-w-3xl">
          <CabeceraDialogo
            icon={ListChecksIcon}
            titulo={t("admin-procesos-asignaturas-titulo", { nombre: asigDialog.proceso?.nombre ?? "" })}
            descripcion={t("admin-procesos-asignaturas-desc")}
          />
          <div className="grid gap-4 px-5 pb-2">
            <ProgresoLineal
              etiqueta={
                <span className={cn("tabular-nums", pesoOk ? "text-success-foreground" : "text-destructive-texto")}>
                  {t("admin-procesos-peso-total", { peso: pesoTotal.toFixed(2) })}
                </span>
              }
              mostrarValor={false}
              valor={Math.min(pesoTotal, 100)}
              tono={pesoOk ? "success" : pesoTotal > 100 ? "destructive" : "warning"}
            />

            {asigDialog.loadingAsig ? (
              <div className="grid gap-1.5" aria-busy="true" aria-label={t("admin-procesos-cargando-asignaturas")}>
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full rounded-xl" />
                ))}
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className={cn(CABECERA_TABLA, "pl-4")}>{t("admin-procesos-asignatura")}</TableHead>
                    <TableHead className={CABECERA_TABLA}>{t("admin-procesos-codigo")}</TableHead>
                    <TableHead className={cn(CABECERA_TABLA, "text-right")}>
                      {t("admin-procesos-num-preguntas")}
                    </TableHead>
                    <TableHead className={cn(CABECERA_TABLA, "text-right")}>{t("admin-procesos-peso")}</TableHead>
                    <TableHead className={cn(CABECERA_TABLA, "pr-4 text-right")}>
                      <span className="sr-only">{t("admin-procesos-accion")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody className="text-control">
                  {asigDialog.items.map((item) => (
                    <TableRow key={item.asignatura_id}>
                      <TableCell className="text-foreground pl-4 font-medium">{item.nombre}</TableCell>
                      <TableCell>
                        <Chip className="font-mono">{item.codigo}</Chip>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{item.numero_preguntas}</TableCell>
                      <TableCell className="text-right tabular-nums">{item.peso_porcentual}%</TableCell>
                      <TableCell className="pr-2 text-right">
                        <button
                          type="button"
                          onClick={() => setQuitarDialog({ open: true, item, submitting: false })}
                          className="h-control text-control text-destructive-texto hover:bg-destructive-suave inline-flex items-center gap-1.5 rounded-lg px-2.5 font-medium transition-colors"
                        >
                          <TrashIcon className="size-3.5" />
                          {t("admin-procesos-quitar")}
                        </button>
                      </TableCell>
                    </TableRow>
                  ))}
                  {asigDialog.items.length === 0 ? (
                    <FilaVacia columnas={5} icon={<ListChecksIcon className="size-6" />}>
                      {t("admin-procesos-sin-asignaturas")}
                    </FilaVacia>
                  ) : null}
                </TableBody>
              </Table>
            )}

            <form
              noValidate
              className="bg-secondary grid gap-3 rounded-xl p-4"
              onSubmit={(e) => {
                e.preventDefault();
                if (!asigDialog.addSubmitting && !avisoAgregar) {
                  addAsignatura();
                }
              }}
            >
              <p className="text-titulo text-foreground">{t("admin-procesos-agregar-asignatura")}</p>
              <div className="grid items-end gap-3 sm:grid-cols-[minmax(0,1fr)_120px_120px_auto]">
                <Campo id="asig-asignatura" etiqueta={t("admin-procesos-asignatura")}>
                  <SelectCampo
                    id="asig-asignatura"
                    placeholder={
                      asignaturasDisponibles.length
                        ? t("admin-procesos-seleccionar")
                        : t("admin-procesos-sin-asignaturas-disponibles")
                    }
                    valor={asigDialog.addForm.asignatura_id ? String(asigDialog.addForm.asignatura_id) : ""}
                    onCambio={(v) => setAddForm({ asignatura_id: Number(v) || 0 })}
                    opciones={asignaturasDisponibles.map((a) => ({
                      value: String(a.id),
                      label: `${a.nombre} (${a.codigo})`,
                    }))}
                    required
                  />
                </Campo>
                <Campo id="asig-preguntas" etiqueta={t("admin-procesos-num-preguntas")}>
                  <Entrada
                    id="asig-preguntas"
                    type="number"
                    inputMode="numeric"
                    min={PREGUNTAS_MIN}
                    max={PREGUNTAS_MAX}
                    className="tabular-nums"
                    value={asigDialog.addForm.numero_preguntas}
                    onChange={(e) => setAddForm({ numero_preguntas: parseInt(e.target.value) || 1 })}
                  />
                </Campo>
                <Campo id="asig-peso" etiqueta={t("admin-procesos-peso")}>
                  <Entrada
                    id="asig-peso"
                    type="number"
                    inputMode="decimal"
                    min={PESO_MIN}
                    max={PESO_MAX}
                    step={0.01}
                    className="tabular-nums"
                    value={asigDialog.addForm.peso_porcentual}
                    onChange={(e) => setAddForm({ peso_porcentual: parseFloat(e.target.value) || 0 })}
                  />
                </Campo>
                <BotonAsync
                  type="submit"
                  pendiente={asigDialog.addSubmitting}
                  textoPendiente="..."
                  icon={<PlusIcon weight="bold" className="size-4" />}
                  disabled={Boolean(avisoAgregar)}
                  title={avisoAgregar || undefined}
                  className={BOTON_PRIMARIO}
                >
                  {t("comun-agregar")}
                </BotonAsync>
              </div>
              {avisoAgregar && asigDialog.addForm.asignatura_id ? (
                <Aviso tono="aviso" icon={WarningIcon}>
                  {avisoAgregar}
                </Aviso>
              ) : null}
            </form>
          </div>
          <PieDialogo>
            <button type="button" onClick={closeAsigDialog} className={BOTON_SECUNDARIO}>
              {t("comun-cerrar")}
            </button>
          </PieDialogo>
        </DialogContent>
      </Dialog>

      {/* Confirmar quitar asignatura (mismo patrón que el cambio de estado de usuarios) */}
      <Dialog
        open={quitarDialog.open}
        onOpenChange={(open) => {
          if (!open && !quitarDialog.submitting) setQuitarDialog((d) => ({ ...d, open: false }));
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={TrashIcon}
            tono="peligro"
            titulo={t("admin-procesos-quitar-titulo")}
            descripcion={t("admin-procesos-quitar-desc", { proceso: asigDialog.proceso?.nombre ?? "" })}
          />
          {quitarDialog.item ? (
            <div className="px-5 pb-2">
              <div className="bg-secondary rounded-lg px-3 py-2.5">
                <p className="text-control text-foreground font-semibold">{quitarDialog.item.nombre}</p>
                <p className="text-etiqueta text-muted-foreground font-mono">{quitarDialog.item.codigo}</p>
              </div>
            </div>
          ) : null}
          <PieDialogo>
            <button
              type="button"
              onClick={() => setQuitarDialog((d) => ({ ...d, open: false }))}
              disabled={quitarDialog.submitting}
              className={BOTON_SECUNDARIO}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={quitarDialog.submitting}
              textoPendiente={t("comun-eliminando")}
              onClick={removeAsignatura}
              className={BOTON_DESTRUCTIVO}
            >
              {t("admin-procesos-quitar")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
