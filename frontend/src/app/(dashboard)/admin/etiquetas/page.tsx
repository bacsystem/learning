"use client";

import {
  CeldaNombre,
  DialogoConfirmarAccion,
  etiquetaEstado,
  PillEstadoCatalogo,
  type TabEstado,
  TabsEstado,
} from "../_catalogos/componentes";
import { FormularioEtiqueta } from "./formulario-etiqueta";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { useToast } from "@/components/feedback/toast";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { useAuth } from "@/lib/auth/auth-context";
import { type Asignatura, catalogsApi } from "@/lib/catalogs/api";
import { claveErrorApi } from "@/lib/errores-api";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { type Etiqueta, questionsApi } from "@/lib/questions/api";
import { ERRORES_ETIQUETA, type ValoresEtiqueta } from "@/lib/questions/etiquetas";
import { CheckCircleIcon, EyeSlashIcon, PencilSimpleIcon, PlusIcon, TrashIcon } from "@phosphor-icons/react";

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

  /** `row` es `null` al crear; `null` en el estado, el diálogo cerrado. */
  const [dialog, setDialog] = useState<{ row: Etiqueta | null } | null>(null);

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

  const openCreate = () => setDialog({ row: null });
  const openEdit = useCallback((row: Etiqueta) => setDialog({ row }), []);
  const closeDialog = () => setDialog(null);

  // Rechaza si la API falla: el formulario lo traduce y lo muestra sin cerrarse.
  const submitDialog = async (valores: ValoresEtiqueta) => {
    if (!token || !dialog) return;
    if (dialog.row) {
      await questionsApi.updateEtiqueta(token, dialog.row.id, { nombre: valores.nombre });
      toast.ok(t("admin-etiquetas-actualizada"));
    } else {
      await questionsApi.createEtiqueta(token, valores);
      toast.ok(t("admin-etiquetas-creada"));
    }
    setDialog(null);
    load();
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
      return t(claveErrorApi(e, "estado", ERRORES_ETIQUETA.estado));
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
      return t(claveErrorApi(e, "eliminar", ERRORES_ETIQUETA.eliminar));
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

      {dialog ? (
        <FormularioEtiqueta
          etiqueta={dialog.row}
          opcionesAsignatura={opcionesAsignatura}
          onCerrar={closeDialog}
          onGuardar={submitDialog}
        />
      ) : null}

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
