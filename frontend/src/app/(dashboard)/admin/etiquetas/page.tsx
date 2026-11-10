"use client";

import {
  CeldaNombre,
  DialogoConfirmarAccion,
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
import { useToast } from "@/components/feedback/toast";
import { Campo, Entrada } from "@/components/formularios/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { useAuth } from "@/lib/auth/auth-context";
import { type Asignatura, catalogsApi } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { type Etiqueta, questionsApi } from "@/lib/questions/api";
import { CheckCircleIcon, EyeSlashIcon, PencilSimpleIcon, PlusIcon, TagIcon, TrashIcon } from "@phosphor-icons/react";

type DialogMode = "create" | "edit";

const DEFAULT_FORM = { asignatura_id: 0, nombre: "" };

export default function EtiquetasPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [allRows, setAllRows] = useState<Etiqueta[]>([]);
  const [tab, setTab] = useState<TabEstado>("todos");
  const [asignaturas, setAsignaturas] = useState<Asignatura[]>([]);
  const [filterAsignaturaID, setFilterAsignaturaID] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [dialog, setDialog] = useState<{
    open: boolean;
    mode: DialogMode;
    row: Etiqueta | null;
    form: typeof DEFAULT_FORM;
    submitting: boolean;
    error: string | null;
  }>({ open: false, mode: "create", row: null, form: DEFAULT_FORM, submitting: false, error: null });

  const [confirmDialog, setConfirmDialog] = useState<{ open: boolean; row: Etiqueta | null }>({
    open: false,
    row: null,
  });
  const [estadoDialog, setEstadoDialog] = useState<{ open: boolean; row: Etiqueta | null }>({
    open: false,
    row: null,
  });

  // Solo se aplica la respuesta de la última carga: cambiar rápido el filtro por asignatura no deja las filas de otro.
  const loadReq = useRef(0);
  const load = useCallback(async () => {
    if (!token) return;
    const reqId = ++loadReq.current;
    setLoading(true);
    setError(null);
    try {
      const data = await questionsApi.listEtiquetas(token, {
        asignatura_id: filterAsignaturaID || undefined,
      });
      if (reqId !== loadReq.current) return;
      setAllRows(data);
    } catch {
      if (reqId !== loadReq.current) return;
      setError(t("admin-error-cargar-etiquetas"));
    } finally {
      if (reqId === loadReq.current) setLoading(false);
    }
  }, [token, filterAsignaturaID, t]);

  useEffect(() => {
    if (!token) return;
    catalogsApi
      .listAsignaturas(token, {})
      .then(setAsignaturas)
      .catch(() => {});
  }, [token]);

  useEffect(() => {
    load();
  }, [load]);

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const opcionesAsignatura = useMemo(
    () => asignaturas.map((a) => ({ value: String(a.id), label: a.nombre })),
    [asignaturas],
  );

  const openCreate = () =>
    setDialog({ open: true, mode: "create", row: null, form: DEFAULT_FORM, submitting: false, error: null });

  const openEdit = useCallback(
    (row: Etiqueta) =>
      setDialog({
        open: true,
        mode: "edit",
        row,
        form: { asignatura_id: row.asignatura_id, nombre: row.nombre },
        submitting: false,
        error: null,
      }),
    [],
  );

  const closeDialog = () => setDialog((d) => ({ ...d, open: false }));

  const submitDialog = async () => {
    if (!token) return;
    setDialog((d) => ({ ...d, submitting: true, error: null }));
    try {
      if (dialog.mode === "create") {
        await questionsApi.createEtiqueta(token, {
          asignatura_id: dialog.form.asignatura_id,
          nombre: dialog.form.nombre,
        });
        toast.ok(t("admin-etiquetas-creada"));
      } else if (dialog.row) {
        await questionsApi.updateEtiqueta(token, dialog.row.id, { nombre: dialog.form.nombre });
        toast.ok(t("admin-etiquetas-actualizada"));
      }
      setDialog((d) => ({ ...d, open: false, submitting: false }));
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      setDialog((d) => ({ ...d, submitting: false, error: err.message ?? t("comun-error-guardar") }));
    }
  };

  const openEstado = useCallback((row: Etiqueta) => setEstadoDialog({ open: true, row }), []);
  const closeEstado = () => setEstadoDialog((d) => ({ ...d, open: false }));

  // PUT /etiquetas/{id} exige `nombre` (UpdateEtiquetaRequest): se reenvía el actual junto al nuevo estado.
  const submitEstado = async (): Promise<string | null> => {
    if (!token || !estadoDialog.row) return null;
    const row = estadoDialog.row;
    const nuevoEstado = row.estado === "activo" ? "inactivo" : "activo";
    try {
      await questionsApi.updateEtiqueta(token, row.id, { nombre: row.nombre, estado: nuevoEstado });
      toast.ok(nuevoEstado === "activo" ? t("admin-etiquetas-activada") : t("admin-etiquetas-desactivada"));
      load();
      return null;
    } catch (e: unknown) {
      const err = e as { message?: string };
      return err.message ?? t("comun-error-estado");
    }
  };

  const openConfirm = useCallback((row: Etiqueta) => setConfirmDialog({ open: true, row }), []);
  const closeConfirm = () => setConfirmDialog((d) => ({ ...d, open: false }));

  const submitDelete = async (): Promise<string | null> => {
    if (!token || !confirmDialog.row) return null;
    try {
      await questionsApi.deleteEtiqueta(token, confirmDialog.row.id);
      toast.ok(t("admin-etiquetas-eliminada"));
      load();
      return null;
    } catch (e: unknown) {
      const err = e as { message?: string };
      return err.message ?? t("comun-error-eliminar");
    }
  };

  const columns = useMemo<ColumnaListado<Etiqueta>[]>(
    () => [
      {
        id: "nombre",
        titulo: t("admin-catalogos-col-nombre"),
        ordenable: true,
        render: (r) => <CeldaNombre>{r.nombre}</CeldaNombre>,
      },
      {
        id: "asignatura",
        titulo: t("admin-catalogos-col-asignatura"),
        ancho: "220px",
        ordenable: true,
        className: "text-muted-foreground",
      },
      {
        id: "estado",
        titulo: t("admin-catalogos-col-estado"),
        ancho: "120px",
        ordenable: true,
        texto: (r) => etiquetaEstado(r.estado, t),
        render: (r) => <PillEstadoCatalogo estado={r.estado} />,
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
            acciones={[
              { label: t("comun-editar"), icon: PencilSimpleIcon, onClick: () => openEdit(r) },
              r.estado === "activo"
                ? { label: t("comun-desactivar"), icon: EyeSlashIcon, onClick: () => openEstado(r) }
                : { label: t("comun-activar"), icon: CheckCircleIcon, onClick: () => openEstado(r) },
              {
                label: t("comun-eliminar"),
                icon: TrashIcon,
                destructiva: true,
                separador: true,
                onClick: () => openConfirm(r),
              },
            ]}
          />
        ),
      },
    ],
    [openEdit, openConfirm, openEstado, t],
  );

  const desactivando = estadoDialog.row?.estado === "activo";

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error ? (
        <Alerta
          tono="error"
          accion={
            <button type="button" onClick={load} className={ACCION_SECUNDARIA}>
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
        titulo={t("menu-admin-etiquetas")}
        placeholderBusqueda={t("admin-etiquetas-buscar")}
        unidad={t("admin-etiquetas-unidad")}
        cargando={loading && allRows.length === 0}
        nota={t("admin-etiquetas-nota", { n: allRows.length })}
        vacio={
          allRows.length === 0 && !filterAsignaturaID
            ? t("admin-etiquetas-vacio", { accion: t("admin-nueva-etiqueta") })
            : t("admin-etiquetas-vacio-filtros")
        }
        filtros={
          <>
            <TabsEstado valor={tab} onCambio={setTab} filas={allRows} />
            <SelectCampo
              id="filtro-asignatura"
              etiqueta={t("admin-catalogos-col-asignatura")}
              className="w-auto min-w-56"
              opciones={[{ value: "0", label: t("admin-etiquetas-todas-asignaturas") }, ...opcionesAsignatura]}
              valor={String(filterAsignaturaID)}
              onCambio={(v) => setFilterAsignaturaID(Number(v) || 0)}
            />
          </>
        }
        acciones={
          <button type="button" onClick={openCreate} className={ACCION_PRINCIPAL}>
            <PlusIcon weight="bold" className="size-4" />
            {t("admin-nueva-etiqueta")}
          </button>
        }
      />

      <DialogoFormulario
        open={dialog.open}
        onCerrar={closeDialog}
        icon={TagIcon}
        titulo={dialog.mode === "create" ? t("admin-nueva-etiqueta") : t("admin-editar-etiqueta")}
        descripcion={dialog.mode === "edit" && dialog.row ? dialog.row.asignatura : undefined}
        error={dialog.error}
        pendiente={dialog.submitting}
        textoGuardar={t("comun-guardar")}
        textoGuardando={t("comun-guardando")}
        textoCancelar={t("comun-cancelar")}
        onGuardar={submitDialog}
        ancho="sm:max-w-md"
      >
        {dialog.mode === "create" ? (
          <Campo id="etiqueta-asignatura" etiqueta={t("admin-catalogos-col-asignatura")} className="md:col-span-2">
            <SelectCampo
              id="etiqueta-asignatura"
              required
              placeholder={t("admin-seleccionar-asignatura")}
              opciones={opcionesAsignatura}
              valor={dialog.form.asignatura_id ? String(dialog.form.asignatura_id) : ""}
              onCambio={(v) => setDialog((d) => ({ ...d, form: { ...d.form, asignatura_id: Number(v) || 0 } }))}
            />
          </Campo>
        ) : null}
        <Campo id="etiqueta-nombre" etiqueta={t("admin-catalogos-col-nombre")} className="md:col-span-2">
          <Entrada
            id="etiqueta-nombre"
            required
            maxLength={100}
            value={dialog.form.nombre}
            onChange={(e) => setDialog((d) => ({ ...d, form: { ...d.form, nombre: e.target.value } }))}
          />
        </Campo>
      </DialogoFormulario>

      <DialogoConfirmarAccion
        open={confirmDialog.open}
        onCerrar={closeConfirm}
        icon={TrashIcon}
        tono="peligro"
        destructiva
        titulo={t("admin-eliminar-etiqueta-titulo")}
        descripcion={t("admin-eliminar-etiqueta-confirm", { nombre: confirmDialog.row?.nombre ?? "" })}
        textoConfirmar={t("comun-eliminar")}
        textoPendiente={t("comun-eliminando")}
        textoCancelar={t("comun-cancelar")}
        onConfirmar={submitDelete}
      />

      <DialogoConfirmarAccion
        open={estadoDialog.open}
        onCerrar={closeEstado}
        icon={desactivando ? EyeSlashIcon : CheckCircleIcon}
        tono={desactivando ? "aviso" : "ok"}
        titulo={desactivando ? t("admin-etiquetas-desactivar-titulo") : t("admin-etiquetas-activar-titulo")}
        registro={estadoDialog.row?.nombre}
        detalle={estadoDialog.row?.asignatura}
        textoConfirmar={desactivando ? t("comun-confirmar-desactivacion") : t("comun-confirmar-activacion")}
        textoPendiente={t("comun-procesando")}
        textoCancelar={t("comun-cancelar")}
        onConfirmar={submitEstado}
      />
    </div>
  );
}
