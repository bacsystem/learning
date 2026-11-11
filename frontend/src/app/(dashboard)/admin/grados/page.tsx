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
import { excede, LIMITES } from "@/lib/admin/limites";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, type Grado, type Institucion } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { CheckCircleIcon, EyeSlashIcon, MedalIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react";

type DialogMode = "create" | "edit";

const DEFAULT_FORM = {
  institucion_id: 0,
  nombre: "",
  abreviatura: "",
  nivel: "suboficial" as "suboficial" | "oficial",
  orden: 1,
};

export default function GradosPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [tab, setTab] = useState<TabEstado>("todos");
  const [allRows, setAllRows] = useState<Grado[]>([]);
  const [instituciones, setInstituciones] = useState<Institucion[]>([]);
  const [filterInstId, setFilterInstId] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [dialog, setDialog] = useState<{
    open: boolean;
    mode: DialogMode;
    row: Grado | null;
    form: typeof DEFAULT_FORM;
    submitting: boolean;
    error: string | null;
  }>({ open: false, mode: "create", row: null, form: DEFAULT_FORM, submitting: false, error: null });

  const [confirmDialog, setConfirmDialog] = useState<{ open: boolean; row: Grado | null }>({ open: false, row: null });
  // Al cambiar de filtro rápido, solo cuenta la última petición.
  const loadReq = useRef(0);

  const load = useCallback(async () => {
    if (!token) return;
    const reqId = ++loadReq.current;
    setLoading(true);
    setError(null);
    try {
      const [data, insts] = await Promise.all([
        catalogsApi.listGrados(token, filterInstId ? { institucion_id: filterInstId } : {}),
        catalogsApi.listInstituciones(token, { limit: "all", estado: "activo" }),
      ]);
      if (reqId !== loadReq.current) return;
      setAllRows(data);
      setInstituciones(insts);
    } catch {
      if (reqId !== loadReq.current) return;
      setError(t("admin-error-cargar-grados"));
    } finally {
      if (reqId === loadReq.current) setLoading(false);
    }
  }, [token, filterInstId, t]);

  const niveles = useMemo(
    () => [
      { value: "suboficial", label: t("admin-catalogos-nivel-suboficial") },
      { value: "oficial", label: t("admin-catalogos-nivel-oficial") },
    ],
    [t],
  );

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
    (row: Grado) =>
      setDialog({
        open: true,
        mode: "edit",
        row,
        form: {
          institucion_id: row.institucion_id,
          nombre: row.nombre,
          abreviatura: row.abreviatura,
          nivel: row.nivel,
          orden: row.orden,
        },
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
        await catalogsApi.createGrado(token, dialog.form);
        toast.ok(t("admin-grados-creado"));
      } else if (dialog.row) {
        await catalogsApi.updateGrado(token, dialog.row.id, dialog.form);
        toast.ok(t("admin-grados-actualizado"));
      }
      setDialog((d) => ({ ...d, open: false, submitting: false }));
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      setDialog((d) => ({ ...d, submitting: false, error: err.message ?? t("comun-error-guardar") }));
    }
  };

  const openConfirm = useCallback((row: Grado) => setConfirmDialog({ open: true, row }), []);
  const closeConfirm = () => setConfirmDialog((d) => ({ ...d, open: false }));

  const submitEstado = async (): Promise<string | null> => {
    if (!token || !confirmDialog.row) return null;
    const nuevoEstado = confirmDialog.row.estado === "activo" ? "inactivo" : "activo";
    try {
      await catalogsApi.updateGradoEstado(token, confirmDialog.row.id, nuevoEstado);
      toast.ok(nuevoEstado === "activo" ? t("admin-grados-activado") : t("admin-grados-desactivado"));
      load();
      return null;
    } catch (e: unknown) {
      const err = e as { message?: string };
      return err.message ?? t("comun-error-estado");
    }
  };

  const formValid =
    dialog.form.institucion_id > 0 &&
    dialog.form.nombre.trim().length >= 2 &&
    dialog.form.abreviatura.trim().length >= 2 &&
    !excede(dialog.form.nombre, LIMITES.grado.nombre) &&
    !excede(dialog.form.abreviatura, LIMITES.grado.abreviatura) &&
    dialog.form.orden >= 1;

  const columns = useMemo<ColumnaListado<Grado>[]>(
    () => [
      {
        id: "nombre",
        titulo: t("admin-catalogos-col-nombre"),
        ordenable: true,
        render: (r) => <CeldaNombre>{r.nombre}</CeldaNombre>,
      },
      {
        id: "abreviatura",
        titulo: t("admin-grados-col-abreviatura"),
        ancho: "130px",
        ordenable: true,
        className: "whitespace-nowrap",
      },
      {
        id: "nivel",
        titulo: t("admin-catalogos-col-nivel"),
        ancho: "130px",
        ordenable: true,
        texto: (r) => etiquetaEstado(r.nivel, t),
        render: (r) => <ChipNivel nivel={r.nivel} />,
      },
      { id: "orden", titulo: t("admin-grados-col-orden"), ancho: "90px", alinear: "derecha", ordenable: true },
      {
        id: "institucion",
        titulo: t("admin-catalogos-col-institucion"),
        ancho: "200px",
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
                ? { label: t("comun-desactivar"), icon: EyeSlashIcon, onClick: () => openConfirm(r) }
                : { label: t("comun-activar"), icon: CheckCircleIcon, onClick: () => openConfirm(r) },
            ]}
          />
        ),
      },
    ],
    [openEdit, openConfirm, t],
  );

  const desactivando = confirmDialog.row?.estado === "activo";

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
        titulo={t("menu-admin-grados")}
        placeholderBusqueda={t("admin-grados-buscar")}
        unidad={t("admin-grados-unidad")}
        cargando={loading && allRows.length === 0}
        nota={t("admin-grados-nota", { n: allRows.length })}
        vacio={
          allRows.length === 0 && !filterInstId
            ? t("admin-grados-vacio", { accion: t("admin-nuevo-grado") })
            : t("admin-grados-vacio-filtros")
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
          </>
        }
        acciones={
          <button type="button" onClick={openCreate} className={ACCION_PRINCIPAL}>
            <PlusIcon weight="bold" className="size-4" />
            {t("admin-nuevo-grado")}
          </button>
        }
      />

      <DialogoFormulario
        open={dialog.open}
        onCerrar={closeDialog}
        icon={MedalIcon}
        titulo={dialog.mode === "create" ? t("admin-nuevo-grado") : t("admin-editar-grado")}
        error={dialog.error}
        pendiente={dialog.submitting}
        puedeGuardar={formValid}
        textoGuardar={dialog.mode === "create" ? t("comun-crear") : t("comun-guardar-cambios")}
        textoGuardando={t("comun-guardando")}
        textoCancelar={t("comun-cancelar")}
        onGuardar={submitDialog}
      >
        <Campo id="grado-institucion" etiqueta={t("admin-catalogos-col-institucion")} className="md:col-span-2">
          <SelectCampo
            id="grado-institucion"
            required
            placeholder={t("admin-catalogos-seleccionar-institucion")}
            opciones={opcionesInstitucion}
            valor={dialog.form.institucion_id ? String(dialog.form.institucion_id) : ""}
            onCambio={(v) => setForm({ institucion_id: Number(v) || 0 })}
          />
        </Campo>
        <Campo
          id="grado-nombre"
          etiqueta={t("admin-catalogos-col-nombre")}
          error={
            excede(dialog.form.nombre, LIMITES.grado.nombre)
              ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.grado.nombre })
              : undefined
          }
          className="md:col-span-2"
        >
          <Entrada
            id="grado-nombre"
            required
            maxLength={LIMITES.grado.nombre}
            invalido={excede(dialog.form.nombre, LIMITES.grado.nombre)}
            value={dialog.form.nombre}
            onChange={(e) => setForm({ nombre: e.target.value })}
            placeholder={t("admin-grados-nombre-placeholder")}
          />
        </Campo>
        <Campo
          id="grado-abreviatura"
          etiqueta={t("admin-grados-col-abreviatura")}
          error={
            excede(dialog.form.abreviatura, LIMITES.grado.abreviatura)
              ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.grado.abreviatura })
              : undefined
          }
        >
          <Entrada
            id="grado-abreviatura"
            required
            maxLength={LIMITES.grado.abreviatura}
            invalido={excede(dialog.form.abreviatura, LIMITES.grado.abreviatura)}
            value={dialog.form.abreviatura}
            onChange={(e) => setForm({ abreviatura: e.target.value })}
            placeholder={t("admin-grados-abreviatura-placeholder")}
          />
        </Campo>
        <Campo id="grado-nivel" etiqueta={t("admin-catalogos-col-nivel")}>
          <SelectCampo
            id="grado-nivel"
            required
            opciones={niveles}
            valor={dialog.form.nivel}
            onCambio={(v) => setForm({ nivel: v as "suboficial" | "oficial" })}
          />
        </Campo>
        <Campo id="grado-orden" etiqueta={t("admin-grados-col-orden")} ayuda={t("admin-grados-orden-ayuda")}>
          <Entrada
            id="grado-orden"
            type="number"
            inputMode="numeric"
            min={1}
            required
            className="tabular-nums"
            value={dialog.form.orden}
            onChange={(e) => setForm({ orden: parseInt(e.target.value) || 1 })}
          />
        </Campo>
      </DialogoFormulario>

      <DialogoConfirmarAccion
        open={confirmDialog.open}
        onCerrar={closeConfirm}
        icon={desactivando ? EyeSlashIcon : CheckCircleIcon}
        tono={desactivando ? "aviso" : "ok"}
        titulo={desactivando ? t("admin-grados-desactivar-titulo") : t("admin-grados-activar-titulo")}
        descripcion={desactivando ? t("admin-grados-desactivar-desc") : t("admin-grados-activar-desc")}
        registro={confirmDialog.row?.nombre}
        detalle={confirmDialog.row ? `${confirmDialog.row.abreviatura} · ${confirmDialog.row.institucion}` : undefined}
        textoConfirmar={desactivando ? t("comun-confirmar-desactivacion") : t("comun-confirmar-activacion")}
        textoPendiente={t("comun-procesando")}
        textoCancelar={t("comun-cancelar")}
        onConfirmar={submitEstado}
      />
    </div>
  );
}
