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
import { excede, LIMITES } from "@/lib/admin/limites";
import { useAuth } from "@/lib/auth/auth-context";
import { type Asignatura, catalogsApi } from "@/lib/catalogs/api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { formatearFechaHora } from "@/lib/formato";
import { BookOpenIcon, CheckCircleIcon, EyeSlashIcon, PencilSimpleIcon, PlusIcon } from "@phosphor-icons/react";

type DialogMode = "create" | "edit";

export default function AsignaturasPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [tab, setTab] = useState<TabEstado>("todos");
  const [allRows, setAllRows] = useState<Asignatura[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [dialog, setDialog] = useState<{
    open: boolean;
    mode: DialogMode;
    row: Asignatura | null;
    nombre: string;
    codigo: string;
    submitting: boolean;
    error: string | null;
  }>({ open: false, mode: "create", row: null, nombre: "", codigo: "", submitting: false, error: null });

  const [confirmDialog, setConfirmDialog] = useState<{ open: boolean; row: Asignatura | null }>({
    open: false,
    row: null,
  });

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      setAllRows(await catalogsApi.listAsignaturas(token));
    } catch {
      setError(t("admin-error-cargar-asignaturas"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    load();
  }, [load]);

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const openCreate = () =>
    setDialog({ open: true, mode: "create", row: null, nombre: "", codigo: "", submitting: false, error: null });

  const openEdit = useCallback(
    (row: Asignatura) =>
      setDialog({
        open: true,
        mode: "edit",
        row,
        nombre: row.nombre,
        codigo: row.codigo,
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
        await catalogsApi.createAsignatura(token, { nombre: dialog.nombre, codigo: dialog.codigo });
        toast.ok(t("admin-asignaturas-creada"));
      } else if (dialog.row) {
        await catalogsApi.updateAsignatura(token, dialog.row.id, { nombre: dialog.nombre, codigo: dialog.codigo });
        toast.ok(t("admin-asignaturas-actualizada"));
      }
      setDialog((d) => ({ ...d, open: false, submitting: false }));
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      setDialog((d) => ({ ...d, submitting: false, error: err.message ?? t("comun-error-guardar") }));
    }
  };

  const openConfirm = useCallback((row: Asignatura) => setConfirmDialog({ open: true, row }), []);
  const closeConfirm = () => setConfirmDialog((d) => ({ ...d, open: false }));

  const submitEstado = async (): Promise<string | null> => {
    if (!token || !confirmDialog.row) return null;
    const nuevoEstado = confirmDialog.row.estado === "activo" ? "inactivo" : "activo";
    try {
      await catalogsApi.updateAsignaturaEstado(token, confirmDialog.row.id, nuevoEstado);
      toast.ok(nuevoEstado === "activo" ? t("admin-asignaturas-activada") : t("admin-asignaturas-desactivada"));
      load();
      return null;
    } catch (e: unknown) {
      const err = e as { message?: string };
      return err.message ?? t("comun-error-estado");
    }
  };

  const columns = useMemo<ColumnaListado<Asignatura>[]>(
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
        id: "estado",
        titulo: t("admin-catalogos-col-estado"),
        ancho: "120px",
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
        titulo={t("menu-admin-asignaturas")}
        placeholderBusqueda={t("admin-asignaturas-buscar")}
        unidad={t("admin-asignaturas-unidad")}
        cargando={loading && allRows.length === 0}
        nota={t("admin-asignaturas-nota", { n: allRows.length })}
        vacio={
          allRows.length === 0
            ? t("admin-asignaturas-vacio", { accion: t("admin-nueva-asignatura") })
            : t("admin-asignaturas-vacio-filtros")
        }
        filtros={<TabsEstado valor={tab} onCambio={setTab} filas={allRows} />}
        acciones={
          <button type="button" onClick={openCreate} className={ACCION_PRINCIPAL}>
            <PlusIcon weight="bold" className="size-4" />
            {t("admin-nueva-asignatura")}
          </button>
        }
      />

      <DialogoFormulario
        open={dialog.open}
        onCerrar={closeDialog}
        icon={BookOpenIcon}
        titulo={dialog.mode === "create" ? t("admin-nueva-asignatura") : t("admin-editar-asignatura")}
        error={dialog.error}
        pendiente={dialog.submitting}
        puedeGuardar={
          dialog.nombre.trim().length >= 3 &&
          dialog.codigo.trim().length >= 2 &&
          !excede(dialog.nombre, LIMITES.asignatura.nombre) &&
          !excede(dialog.codigo, LIMITES.asignatura.codigo)
        }
        textoGuardar={dialog.mode === "create" ? t("comun-crear") : t("comun-guardar-cambios")}
        textoGuardando={t("comun-guardando")}
        textoCancelar={t("comun-cancelar")}
        onGuardar={submitDialog}
        ancho="sm:max-w-md"
      >
        <Campo
          id="asignatura-nombre"
          etiqueta={t("admin-catalogos-col-nombre")}
          ayuda={t("admin-catalogos-minimo-caracteres", { n: 3 })}
          error={
            excede(dialog.nombre, LIMITES.asignatura.nombre)
              ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.asignatura.nombre })
              : undefined
          }
          className="md:col-span-2"
        >
          <Entrada
            id="asignatura-nombre"
            required
            maxLength={LIMITES.asignatura.nombre}
            invalido={excede(dialog.nombre, LIMITES.asignatura.nombre)}
            autoFocus
            value={dialog.nombre}
            onChange={(e) => setDialog((d) => ({ ...d, nombre: e.target.value }))}
            placeholder={t("admin-asignaturas-nombre-placeholder")}
          />
        </Campo>
        <Campo
          id="asignatura-codigo"
          etiqueta={t("admin-catalogos-col-codigo")}
          ayuda={t("admin-catalogos-minimo-caracteres", { n: 2 })}
          error={
            excede(dialog.codigo, LIMITES.asignatura.codigo)
              ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.asignatura.codigo })
              : undefined
          }
          className="md:col-span-2"
        >
          <Entrada
            id="asignatura-codigo"
            required
            maxLength={LIMITES.asignatura.codigo}
            invalido={excede(dialog.codigo, LIMITES.asignatura.codigo)}
            mono
            value={dialog.codigo}
            onChange={(e) => setDialog((d) => ({ ...d, codigo: e.target.value }))}
            placeholder={t("admin-asignaturas-codigo-placeholder")}
          />
        </Campo>
      </DialogoFormulario>

      <DialogoConfirmarAccion
        open={confirmDialog.open}
        onCerrar={closeConfirm}
        icon={desactivando ? EyeSlashIcon : CheckCircleIcon}
        tono={desactivando ? "aviso" : "ok"}
        titulo={desactivando ? t("admin-desactivar-asignatura") : t("admin-activar-asignatura")}
        registro={confirmDialog.row?.nombre}
        detalle={
          confirmDialog.row ? t("admin-catalogos-detalle-codigo", { codigo: confirmDialog.row.codigo }) : undefined
        }
        textoConfirmar={desactivando ? t("comun-confirmar-desactivacion") : t("comun-confirmar-activacion")}
        textoPendiente={t("comun-procesando")}
        textoCancelar={t("comun-cancelar")}
        onConfirmar={submitEstado}
      />
    </div>
  );
}
