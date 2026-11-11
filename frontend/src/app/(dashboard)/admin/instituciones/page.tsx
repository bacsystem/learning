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
import { useCallback, useEffect, useMemo, useState } from "react";

import { Campo } from "@/components/app/campo";
import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { useToast } from "@/components/feedback/toast";
import { Entrada } from "@/components/formularios/campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { ChipCodigo } from "@/components/patrones/pill-estado";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, type Institucion } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { formatearFechaHora } from "@/lib/formato";
import { BankIcon, CheckCircleIcon, EyeSlashIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react";

type DialogMode = "create" | "edit";

// CreateInstitucionRequest (backend/internal/catalogs/dto.go): nombre min=3,max=150 · siglas min=2,max=20 (en runas).
const NOMBRE_MIN = 3;
const NOMBRE_MAX = 150;
const SIGLAS_MIN = 2;
const SIGLAS_MAX = 20;
const largo = (s: string) => Array.from(s.trim()).length;

export default function InstitucionesPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [tab, setTab] = useState<TabEstado>("todos");
  const [allRows, setAllRows] = useState<Institucion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [dialog, setDialog] = useState<{
    open: boolean;
    mode: DialogMode;
    row: Institucion | null;
    nombre: string;
    siglas: string;
    submitting: boolean;
    error: string | null;
  }>({ open: false, mode: "create", row: null, nombre: "", siglas: "", submitting: false, error: null });

  const [confirmDialog, setConfirmDialog] = useState<{ open: boolean; row: Institucion | null }>({
    open: false,
    row: null,
  });

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      setAllRows(await catalogsApi.listInstituciones(token));
    } catch {
      setError(t("admin-error-cargar-instituciones"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    load();
  }, [load]);

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const openCreate = () =>
    setDialog({ open: true, mode: "create", row: null, nombre: "", siglas: "", submitting: false, error: null });

  const openEdit = useCallback(
    (row: Institucion) =>
      setDialog({
        open: true,
        mode: "edit",
        row,
        nombre: row.nombre,
        siglas: row.siglas,
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
        await catalogsApi.createInstitucion(token, { nombre: dialog.nombre, siglas: dialog.siglas });
        toast.ok(t("admin-instituciones-creada"));
      } else if (dialog.row) {
        await catalogsApi.updateInstitucion(token, dialog.row.id, { nombre: dialog.nombre, siglas: dialog.siglas });
        toast.ok(t("admin-instituciones-actualizada"));
      }
      setDialog((d) => ({ ...d, open: false, submitting: false }));
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      setDialog((d) => ({ ...d, submitting: false, error: err.message ?? t("comun-error-guardar") }));
    }
  };

  const openConfirm = useCallback((row: Institucion) => setConfirmDialog({ open: true, row }), []);
  const closeConfirm = () => setConfirmDialog((d) => ({ ...d, open: false }));

  const submitEstado = async (): Promise<string | null> => {
    if (!token || !confirmDialog.row) return null;
    const nuevoEstado = confirmDialog.row.estado === "activo" ? "inactivo" : "activo";
    try {
      await catalogsApi.updateInstitucionEstado(token, confirmDialog.row.id, nuevoEstado);
      toast.ok(nuevoEstado === "activo" ? t("admin-instituciones-activada") : t("admin-instituciones-desactivada"));
      load();
      return null;
    } catch (e: unknown) {
      const err = e as { message?: string };
      return err.message ?? t("comun-error-estado");
    }
  };

  const columns = useMemo<ColumnaListado<Institucion>[]>(
    () => [
      {
        id: "nombre",
        titulo: t("admin-catalogos-col-nombre"),
        ordenable: true,
        render: (r) => <CeldaNombre>{r.nombre}</CeldaNombre>,
      },
      {
        id: "siglas",
        titulo: t("admin-instituciones-col-siglas"),
        ancho: "140px",
        ordenable: true,
        render: (r) => <ChipCodigo inactivo={r.estado !== "activo"}>{r.siglas}</ChipCodigo>,
      },
      {
        id: "estado",
        titulo: t("admin-catalogos-col-estado"),
        ancho: "130px",
        ordenable: true,
        texto: (r) => etiquetaEstado(r.estado, t),
        render: (r) => <PillEstadoCatalogo estado={r.estado} />,
      },
      {
        id: "created_at",
        titulo: t("admin-catalogos-col-creado"),
        ancho: "180px",
        ordenable: true,
        className: "whitespace-nowrap tabular-nums text-muted-foreground",
        texto: (r) => (r.created_at ? formatearFechaHora(r.created_at) : ""),
        render: (r) => (r.created_at ? formatearFechaHora(r.created_at) : "—"),
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

  const nombreValido = largo(dialog.nombre) >= NOMBRE_MIN && largo(dialog.nombre) <= NOMBRE_MAX;
  const siglasValidas = largo(dialog.siglas) >= SIGLAS_MIN && largo(dialog.siglas) <= SIGLAS_MAX;

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
        titulo={t("menu-admin-instituciones")}
        placeholderBusqueda={t("admin-instituciones-buscar")}
        unidad={t("admin-instituciones-unidad")}
        cargando={loading && allRows.length === 0}
        nota={t("admin-instituciones-nota", { n: allRows.length })}
        vacio={
          allRows.length === 0
            ? t("admin-instituciones-vacio", { accion: t("admin-nueva-institucion") })
            : t("admin-instituciones-vacio-filtros")
        }
        filtros={<TabsEstado valor={tab} onCambio={setTab} filas={allRows} />}
        acciones={
          <button type="button" onClick={openCreate} className={ACCION_PRINCIPAL}>
            <PlusIcon weight="bold" className="size-4" />
            {t("admin-nueva-institucion")}
          </button>
        }
      />

      <DialogoFormulario
        open={dialog.open}
        onCerrar={closeDialog}
        icon={BankIcon}
        titulo={dialog.mode === "create" ? t("admin-nueva-institucion") : t("admin-editar-institucion")}
        error={dialog.error}
        pendiente={dialog.submitting}
        puedeGuardar={nombreValido && siglasValidas}
        textoGuardar={dialog.mode === "create" ? t("comun-crear") : t("comun-guardar-cambios")}
        textoGuardando={t("comun-guardando")}
        textoCancelar={t("comun-cancelar")}
        onGuardar={submitDialog}
        ancho="sm:max-w-md"
      >
        <Campo
          id="institucion-nombre"
          etiqueta={t("admin-catalogos-col-nombre")}
          ayuda={t("admin-catalogos-minimo-caracteres", { n: NOMBRE_MIN })}
          error={dialog.nombre && !nombreValido ? t("admin-catalogos-minimo-caracteres", { n: NOMBRE_MIN }) : undefined}
          className="md:col-span-2"
        >
          <Entrada
            id="institucion-nombre"
            required
            autoFocus
            maxLength={NOMBRE_MAX}
            invalido={!!dialog.nombre && !nombreValido}
            value={dialog.nombre}
            onChange={(e) => setDialog((d) => ({ ...d, nombre: e.target.value }))}
            placeholder={t("admin-instituciones-nombre-placeholder")}
          />
        </Campo>
        <Campo
          id="institucion-siglas"
          etiqueta={t("admin-instituciones-col-siglas")}
          ayuda={t("admin-catalogos-minimo-caracteres", { n: SIGLAS_MIN })}
          error={
            dialog.siglas && !siglasValidas ? t("admin-catalogos-minimo-caracteres", { n: SIGLAS_MIN }) : undefined
          }
          className="md:col-span-2"
        >
          <Entrada
            id="institucion-siglas"
            required
            mono
            maxLength={SIGLAS_MAX}
            invalido={!!dialog.siglas && !siglasValidas}
            value={dialog.siglas}
            onChange={(e) => setDialog((d) => ({ ...d, siglas: e.target.value }))}
            placeholder={t("admin-instituciones-siglas-placeholder")}
          />
        </Campo>
      </DialogoFormulario>

      <DialogoConfirmarAccion
        open={confirmDialog.open}
        onCerrar={closeConfirm}
        icon={desactivando ? EyeSlashIcon : CheckCircleIcon}
        tono={desactivando ? "aviso" : "ok"}
        titulo={desactivando ? t("admin-instituciones-desactivar-titulo") : t("admin-instituciones-activar-titulo")}
        descripcion={desactivando ? t("admin-instituciones-desactivar-desc") : t("admin-instituciones-activar-desc")}
        registro={confirmDialog.row?.nombre}
        detalle={confirmDialog.row?.siglas}
        textoConfirmar={desactivando ? t("comun-confirmar-desactivacion") : t("comun-confirmar-activacion")}
        textoPendiente={t("comun-procesando")}
        textoCancelar={t("comun-cancelar")}
        onConfirmar={submitEstado}
      />
    </div>
  );
}
