"use client";

import {
  CeldaNombre,
  ChipNivel,
  DialogoConfirmarAccion,
  DialogoFormulario,
  etiquetaEstado,
  PillEstadoCatalogo,
  type TabEstado,
  TabsEstado,
} from "../_catalogos/componentes";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Campo } from "@/components/app/campo";
import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { useToast } from "@/components/feedback/toast";
import { Entrada } from "@/components/formularios/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { ChipCodigo } from "@/components/patrones/pill-estado";
import { excede, LIMITES } from "@/lib/admin/limites";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, type Especialidad, type Institucion } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { CheckCircleIcon, EyeSlashIcon, GraduationCapIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react";

type DialogMode = "create" | "edit";

const DEFAULT_FORM = { institucion_id: 0, nombre: "", codigo: "", nivel: "" };

export default function EspecialidadesPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [allRows, setAllRows] = useState<Especialidad[]>([]);
  const [tab, setTab] = useState<TabEstado>("todos");
  const [instituciones, setInstituciones] = useState<Institucion[]>([]);
  const [filterInstId, setFilterInstId] = useState<number>(0);
  const [filterNivel, setFilterNivel] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [dialog, setDialog] = useState<{
    open: boolean;
    mode: DialogMode;
    row: Especialidad | null;
    form: typeof DEFAULT_FORM;
    submitting: boolean;
    error: string | null;
  }>({ open: false, mode: "create", row: null, form: DEFAULT_FORM, submitting: false, error: null });

  const [confirmDialog, setConfirmDialog] = useState<{ open: boolean; row: Especialidad | null }>({
    open: false,
    row: null,
  });

  // Al cambiar de filtro rápido, solo cuenta la última petición.
  const loadReq = useRef(0);

  const load = useCallback(async () => {
    if (!token) return;
    const reqId = ++loadReq.current;
    setLoading(true);
    setError(null);
    try {
      const [data, insts] = await Promise.all([
        catalogsApi.listEspecialidades(token, {
          ...(filterInstId ? { institucion_id: filterInstId } : {}),
          ...(filterNivel ? { nivel: filterNivel } : {}),
        }),
        catalogsApi.listInstituciones(token, { limit: "all", estado: "activo" }),
      ]);
      if (reqId !== loadReq.current) return;
      setAllRows(data);
      setInstituciones(insts);
    } catch {
      if (reqId !== loadReq.current) return;
      setError(t("admin-especialidades-error-cargar"));
    } finally {
      if (reqId === loadReq.current) setLoading(false);
    }
  }, [token, filterInstId, filterNivel, t]);

  useEffect(() => {
    load();
  }, [load]);

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const opcionesInstitucion = useMemo(
    () => instituciones.map((i) => ({ value: String(i.id), label: i.nombre })),
    [instituciones],
  );

  const setForm = (patch: Partial<typeof DEFAULT_FORM>) => setDialog((d) => ({ ...d, form: { ...d.form, ...patch } }));

  const openCreate = () =>
    setDialog({ open: true, mode: "create", row: null, form: DEFAULT_FORM, submitting: false, error: null });

  const openEdit = useCallback(
    (row: Especialidad) =>
      setDialog({
        open: true,
        mode: "edit",
        row,
        form: { institucion_id: row.institucion_id, nombre: row.nombre, codigo: row.codigo, nivel: row.nivel ?? "" },
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
        await catalogsApi.createEspecialidad(token, dialog.form);
        toast.ok(t("admin-especialidades-creada"));
      } else if (dialog.row) {
        await catalogsApi.updateEspecialidad(token, dialog.row.id, dialog.form);
        toast.ok(t("admin-especialidades-actualizada"));
      }
      setDialog((d) => ({ ...d, open: false, submitting: false }));
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      setDialog((d) => ({ ...d, submitting: false, error: err.message ?? t("comun-error-guardar") }));
    }
  };

  const openConfirm = useCallback((row: Especialidad) => setConfirmDialog({ open: true, row }), []);
  const closeConfirm = () => setConfirmDialog((d) => ({ ...d, open: false }));

  const submitEstado = async (): Promise<string | null> => {
    if (!token || !confirmDialog.row) return null;
    const nuevoEstado = confirmDialog.row.estado === "activo" ? "inactivo" : "activo";
    try {
      await catalogsApi.updateEspecialidadEstado(token, confirmDialog.row.id, nuevoEstado);
      toast.ok(nuevoEstado === "activo" ? t("admin-especialidades-activada") : t("admin-especialidades-desactivada"));
      load();
      return null;
    } catch (e: unknown) {
      const err = e as { message?: string };
      return err.message ?? t("comun-error-estado");
    }
  };

  const columns = useMemo<ColumnaListado<Especialidad>[]>(
    () => [
      {
        id: "nombre",
        titulo: t("admin-catalogos-col-nombre"),
        ordenable: true,
        render: (r) => <CeldaNombre>{r.nombre}</CeldaNombre>,
      },
      {
        id: "codigo",
        titulo: t("admin-catalogos-col-codigo"),
        ancho: "140px",
        ordenable: true,
        render: (r) => <ChipCodigo inactivo={r.estado !== "activo"}>{r.codigo}</ChipCodigo>,
      },
      {
        id: "institucion",
        titulo: t("admin-catalogos-col-institucion"),
        ancho: "200px",
        ordenable: true,
        className: "text-muted-foreground",
      },
      {
        id: "nivel",
        titulo: t("admin-catalogos-col-nivel"),
        ancho: "130px",
        ordenable: true,
        texto: (r) => etiquetaEstado(r.nivel, t),
        render: (r) => <ChipNivel nivel={r.nivel} />,
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
                ? { label: t("comun-desactivar"), icon: EyeSlashIcon, onClick: () => openConfirm(r) }
                : { label: t("comun-activar"), icon: CheckCircleIcon, onClick: () => openConfirm(r) },
            ]}
          />
        ),
      },
    ],
    [openEdit, openConfirm, t],
  );

  const formValid =
    dialog.form.institucion_id > 0 &&
    dialog.form.nombre.trim().length >= 3 &&
    dialog.form.codigo.trim().length >= 2 &&
    !excede(dialog.form.nombre, LIMITES.especialidad.nombre) &&
    !excede(dialog.form.codigo, LIMITES.especialidad.codigo);

  const desactivando = confirmDialog.row?.estado === "activo";
  const conFiltros = !!filterInstId || !!filterNivel;

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
        titulo={t("menu-admin-especialidades")}
        placeholderBusqueda={t("admin-especialidades-buscar")}
        unidad={t("admin-especialidades-unidad")}
        cargando={loading && allRows.length === 0}
        nota={t("admin-especialidades-nota", { n: allRows.length })}
        vacio={
          allRows.length === 0 && !conFiltros
            ? t("admin-especialidades-vacio", { accion: t("admin-nueva-especialidad") })
            : t("admin-especialidades-vacio-filtros")
        }
        filtros={
          <>
            <TabsEstado valor={tab} onCambio={setTab} filas={allRows} />
            <SelectCampo
              id="filtro-institucion"
              etiqueta={t("admin-catalogos-col-institucion")}
              className="w-auto min-w-56"
              opciones={[{ value: "0", label: t("admin-catalogos-todas-instituciones") }, ...opcionesInstitucion]}
              valor={String(filterInstId)}
              onCambio={(v) => setFilterInstId(Number(v) || 0)}
            />
            <SelectCampo
              id="filtro-nivel"
              etiqueta={t("admin-catalogos-col-nivel")}
              className="w-auto min-w-44"
              opciones={[
                { value: "", label: t("admin-especialidades-todos-niveles") },
                { value: "suboficial", label: t("admin-catalogos-nivel-suboficial") },
                { value: "oficial", label: t("admin-catalogos-nivel-oficial") },
              ]}
              valor={filterNivel}
              onCambio={setFilterNivel}
            />
          </>
        }
        acciones={
          <button type="button" onClick={openCreate} className={ACCION_PRINCIPAL}>
            <PlusIcon weight="bold" className="size-4" />
            {t("admin-nueva-especialidad")}
          </button>
        }
      />

      <DialogoFormulario
        open={dialog.open}
        onCerrar={closeDialog}
        icon={GraduationCapIcon}
        titulo={dialog.mode === "create" ? t("admin-nueva-especialidad") : t("admin-editar-especialidad")}
        error={dialog.error}
        pendiente={dialog.submitting}
        puedeGuardar={formValid}
        textoGuardar={dialog.mode === "create" ? t("comun-crear") : t("comun-guardar-cambios")}
        textoGuardando={t("comun-guardando")}
        textoCancelar={t("comun-cancelar")}
        onGuardar={submitDialog}
      >
        <Campo id="especialidad-institucion" etiqueta={t("admin-catalogos-col-institucion")} className="md:col-span-2">
          <SelectCampo
            id="especialidad-institucion"
            required
            placeholder={t("admin-catalogos-seleccionar-institucion")}
            opciones={opcionesInstitucion}
            valor={dialog.form.institucion_id ? String(dialog.form.institucion_id) : ""}
            onCambio={(v) => setForm({ institucion_id: Number(v) || 0 })}
          />
        </Campo>
        <Campo
          id="especialidad-nombre"
          etiqueta={t("admin-catalogos-col-nombre")}
          error={
            excede(dialog.form.nombre, LIMITES.especialidad.nombre)
              ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.especialidad.nombre })
              : undefined
          }
          className="md:col-span-2"
        >
          <Entrada
            id="especialidad-nombre"
            required
            maxLength={LIMITES.especialidad.nombre}
            invalido={excede(dialog.form.nombre, LIMITES.especialidad.nombre)}
            value={dialog.form.nombre}
            onChange={(e) => setForm({ nombre: e.target.value })}
            placeholder={t("admin-especialidades-nombre-placeholder")}
          />
        </Campo>
        <Campo
          id="especialidad-codigo"
          etiqueta={t("admin-catalogos-col-codigo")}
          error={
            excede(dialog.form.codigo, LIMITES.especialidad.codigo)
              ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.especialidad.codigo })
              : undefined
          }
        >
          <Entrada
            id="especialidad-codigo"
            required
            maxLength={LIMITES.especialidad.codigo}
            invalido={excede(dialog.form.codigo, LIMITES.especialidad.codigo)}
            mono
            value={dialog.form.codigo}
            onChange={(e) => setForm({ codigo: e.target.value })}
            placeholder={t("admin-especialidades-codigo-placeholder")}
          />
        </Campo>
        <Campo id="especialidad-nivel" etiqueta={t("admin-catalogos-col-nivel")} opcional>
          <SelectCampo
            id="especialidad-nivel"
            opciones={[
              { value: "", label: t("admin-especialidades-sin-nivel") },
              { value: "suboficial", label: t("admin-catalogos-nivel-suboficial") },
              { value: "oficial", label: t("admin-catalogos-nivel-oficial") },
            ]}
            valor={dialog.form.nivel}
            onCambio={(v) => setForm({ nivel: v })}
          />
        </Campo>
      </DialogoFormulario>

      <DialogoConfirmarAccion
        open={confirmDialog.open}
        onCerrar={closeConfirm}
        icon={desactivando ? EyeSlashIcon : CheckCircleIcon}
        tono={desactivando ? "aviso" : "ok"}
        titulo={desactivando ? t("admin-desactivar-especialidad") : t("admin-activar-especialidad")}
        descripcion={desactivando ? t("admin-especialidades-desactivar-desc") : t("admin-especialidades-activar-desc")}
        registro={confirmDialog.row?.nombre}
        detalle={confirmDialog.row ? `${confirmDialog.row.codigo} · ${confirmDialog.row.institucion}` : undefined}
        textoConfirmar={desactivando ? t("comun-confirmar-desactivacion") : t("comun-confirmar-activacion")}
        textoPendiente={t("comun-procesando")}
        textoCancelar={t("comun-cancelar")}
        onConfirmar={submitEstado}
      />
    </div>
  );
}
