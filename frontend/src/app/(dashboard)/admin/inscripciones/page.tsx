"use client";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";

import { Campo } from "@/components/app/campo";
import { DialogContent } from "@/components/app/dialogo";
import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { useToast } from "@/components/feedback/toast";
import { AreaTexto } from "@/components/formularios/campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import type { Tono } from "@/components/patrones/metricas";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { GrupoBotones } from "@/components/ui/grupo-botones";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_DESTRUCTIVO, BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { InscripcionAdmin, inscripcionesApi } from "@/lib/inscripciones/api";
import { CheckCircleIcon, XCircleIcon, XIcon } from "@phosphor-icons/react";

const TABS = ["todos", "pendiente", "aprobado", "rechazado"] as const;
type TabVal = (typeof TABS)[number];

const ESTADO_TONO: Record<string, Tono> = {
  pendiente: "aviso",
  aprobado: "ok",
  rechazado: "error",
};

const TIPO_CLAVE: Record<string, string> = {
  ascenso: "procesos-tipo-ascenso",
  pase_efectividad: "procesos-tipo-pase-efectividad",
  admision: "admin-inscripciones-tipo-admision",
};

const ESTADO_CLAVE: Record<string, string> = {
  pendiente: "admin-inscripciones-estado-pendiente",
  aprobado: "admin-inscripciones-estado-aprobado",
  rechazado: "admin-inscripciones-estado-rechazado",
};

/** "dd/mm/yyyy hh:mm" (formato del backend) → "yyyy-mm-dd hh:mm", que sí ordena como fecha. */
const fechaOrdenable = (s: string) => {
  const m = /^(\d{2})\/(\d{2})\/(\d{4})(?: (\d{2}):(\d{2}))?$/.exec(s ?? "");
  return m ? `${m[3]}-${m[2]}-${m[1]} ${m[4] ?? "00"}:${m[5] ?? "00"}` : (s ?? "");
};

export default function Page() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const tipoLabel = useCallback((tipo: string) => (TIPO_CLAVE[tipo] ? t(TIPO_CLAVE[tipo]) : tipo), [t]);
  const estadoLabel = useCallback((estado: string) => (ESTADO_CLAVE[estado] ? t(ESTADO_CLAVE[estado]) : estado), [t]);
  const [tab, setTab] = useState<TabVal>("pendiente");
  const [allRows, setAllRows] = useState<InscripcionAdmin[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [dialog, setDialog] = useState<{
    open: boolean;
    ins: InscripcionAdmin | null;
    accion: "aprobado" | "rechazado" | null;
    observacion: string;
    submitting: boolean;
  }>({ open: false, ins: null, accion: null, observacion: "", submitting: false });

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      // The server pages GET /admin/inscripciones; fetch every page so tabs,
      // counts, search and export keep working over the full list.
      const data = await inscripcionesApi.listAllInscripciones(token);
      setAllRows(data);
    } catch {
      setError(t("admin-error-cargar-inscripciones"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    load();
  }, [load]);

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const openDialog = useCallback(
    (ins: InscripcionAdmin, accion: "aprobado" | "rechazado") =>
      setDialog({ open: true, ins, accion, observacion: "", submitting: false }),
    [],
  );

  const closeDialog = () => setDialog((d) => ({ ...d, open: false, submitting: false }));

  const confirm = async () => {
    if (!token || !dialog.ins || !dialog.accion) return;
    if (dialog.accion === "rechazado" && !dialog.observacion.trim()) return;
    setDialog((d) => ({ ...d, submitting: true }));
    try {
      await inscripcionesApi.updateInscripcionEstado(token, dialog.ins.id, dialog.accion, dialog.observacion);
      toast.ok(
        dialog.accion === "aprobado" ? t("admin-inscripciones-aprobada") : t("admin-inscripciones-rechazada"),
        dialog.ins.nombre_completo,
      );
      closeDialog();
      load();
    } catch (e: unknown) {
      const err = e as { message?: string; status?: number };
      if (err.status === 409 || err.status === 404) {
        // Someone else decided (409) or removed (404) it meanwhile: say so and
        // reload so the row shows its real state instead of stale actions.
        closeDialog();
        setNotice(
          t(err.status === 409 ? "admin-inscripciones-ya-revisada" : "admin-inscripciones-no-encontrada", {
            nombre: dialog.ins.nombre_completo,
          }),
        );
        load();
        return;
      }
      // El diálogo sigue abierto: el aviso va en un toast, no en la alerta de página que queda debajo del modal.
      toast.error(err.message ?? t("admin-error-actualizar"));
      setDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const columns = useMemo<ColumnaListado<InscripcionAdmin>[]>(
    () => [
      {
        id: "nombre_completo",
        titulo: t("admin-inscripciones-col-nombre"),
        ordenable: true,
        ocultable: false,
        render: (r) => (
          <div className="flex min-w-48 items-center gap-2.5">
            <Avatar nombre={r.nombre_completo} tono="accent" />
            <div className="min-w-0">
              <p className="text-foreground truncate font-medium">{r.nombre_completo}</p>
              <p className="text-etiqueta text-texto-terciario truncate font-mono">
                {t("admin-inscripciones-dni", { dni: r.dni })}
              </p>
            </div>
          </div>
        ),
        texto: (r) => `${r.nombre_completo} ${r.dni}`,
      },
      {
        id: "username",
        titulo: t("perfil-usuario"),
        ordenable: true,
        ocultaInicial: true,
        render: (r) => <span className="text-muted-foreground">@{r.username}</span>,
      },
      { id: "grado", titulo: t("perfil-grado"), ordenable: true },
      { id: "especialidad", titulo: t("perfil-especialidad"), ordenable: true },
      {
        id: "proceso_nombre",
        titulo: t("simulacros-proceso"),
        ordenable: true,
        render: (r) => (
          <div className="min-w-56">
            <p className="text-foreground">{r.proceso_nombre}</p>
            <p className="text-etiqueta text-texto-terciario">{tipoLabel(r.proceso_tipo)}</p>
          </div>
        ),
      },
      {
        id: "proceso_anio",
        titulo: t("admin-ingresos-anio"),
        ordenable: true,
        ocultaInicial: true,
        className: "tabular-nums",
      },
      {
        id: "fecha_solicitud",
        titulo: t("admin-inscripciones-col-fecha"),
        ordenable: true,
        valor: (r) => fechaOrdenable(r.fecha_solicitud),
        // Buscar, exportar e imprimir con la fecha tal como se ve (la celda pinta `fecha_solicitud`); `valor` solo ordena.
        texto: (r) => r.fecha_solicitud ?? "",
        className: "whitespace-nowrap tabular-nums",
      },
      {
        id: "estado",
        titulo: t("perfil-estado"),
        ordenable: true,
        render: (r) => <PillEstado tono={ESTADO_TONO[r.estado] ?? "neutro"}>{estadoLabel(r.estado)}</PillEstado>,
        texto: (r) => estadoLabel(r.estado),
      },
      {
        id: "observacion",
        titulo: t("admin-inscripciones-col-observacion"),
        ocultaInicial: true,
        render: (r) => <span className="text-muted-foreground">{r.observacion ?? "—"}</span>,
        texto: (r) => r.observacion ?? "",
      },
      {
        // Only pendientes can be decided (the server answers 409 otherwise);
        // decided rows show who reviewed them and when instead of actions.
        id: "actions",
        titulo: "",
        alinear: "derecha",
        exportable: false,
        ocultable: false,
        render: (r) =>
          r.estado === "pendiente" ? (
            <MenuAcciones
              etiqueta={t("admin-inscripciones-acciones-para", { nombre: r.nombre_completo })}
              acciones={[
                {
                  label: t("admin-inscripciones-aprobar"),
                  icon: CheckCircleIcon,
                  onClick: () => openDialog(r, "aprobado"),
                },
                {
                  label: t("admin-inscripciones-rechazar"),
                  icon: XCircleIcon,
                  destructiva: true,
                  onClick: () => openDialog(r, "rechazado"),
                },
              ]}
            />
          ) : (
            <span className="text-etiqueta text-texto-terciario whitespace-nowrap">
              {r.revisado_por
                ? t("admin-inscripciones-revisado-por", { usuario: r.revisado_por, fecha: r.revisado_at ?? "—" })
                : t("admin-inscripciones-revisada")}
            </span>
          ),
      },
    ],
    [openDialog, t, estadoLabel, tipoLabel],
  );

  const contar = (v: TabVal) => (v === "todos" ? allRows.length : allRows.filter((r) => r.estado === v).length);

  const aprobando = dialog.accion === "aprobado";

  return (
    <div className={CONTENEDOR_PAGINA}>
      <p className="text-cuerpo text-muted-foreground">
        {t.rich("admin-inscripciones-resumen", {
          n: allRows.length,
          num: (chunks) => <span className="text-foreground font-medium tabular-nums">{chunks}</span>,
        })}
      </p>

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

      {notice ? (
        <Alerta
          tono="aviso"
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setNotice(null)}>
              <XIcon weight="bold" className="size-4" />
            </Button>
          }
        >
          {notice}
        </Alerta>
      ) : null}

      <TablaListado
        titulo={t("admin-inscripciones-titulo")}
        columnas={columns}
        filas={rows}
        clave={(r) => String(r.id)}
        cargando={loading}
        unidad={t("admin-inscripciones-unidad")}
        placeholderBusqueda={t("admin-inscripciones-buscar")}
        vacio={
          tab === "todos"
            ? t("admin-inscripciones-vacio")
            : t("admin-inscripciones-vacio-estado", { estado: estadoLabel(tab) })
        }
        filtros={
          <GrupoBotones
            etiqueta={t("admin-inscripciones-filtrar-estado")}
            valor={tab}
            onCambio={setTab}
            opciones={TABS.map((v) => ({
              valor: v,
              etiqueta: (
                <>
                  {v === "todos" ? t("simulacros-todos") : estadoLabel(v)}
                  <span className="bg-secondary text-etiqueta text-muted-foreground rounded-full px-1.5 font-semibold tabular-nums">
                    {contar(v)}
                  </span>
                </>
              ),
            }))}
          />
        }
      />

      <Dialog
        open={dialog.open}
        onOpenChange={(open) => {
          if (!open && !dialog.submitting) closeDialog();
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={aprobando ? CheckCircleIcon : XCircleIcon}
            tono={aprobando ? "ok" : "peligro"}
            titulo={aprobando ? t("admin-inscripciones-aprobar") : t("admin-inscripciones-rechazar")}
            descripcion={aprobando ? t("admin-inscripciones-aprobar-desc") : t("admin-inscripciones-rechazar-desc")}
          />

          <div className="grid gap-4 px-5 pb-2">
            {dialog.ins ? (
              <div className="bg-secondary rounded-xl p-3.5">
                <div className="flex items-center gap-3">
                  <Avatar nombre={dialog.ins.nombre_completo} tamano="lg" tono="accent" />
                  <div className="min-w-0">
                    <p className="text-control text-foreground truncate font-semibold">{dialog.ins.nombre_completo}</p>
                    <p className="text-etiqueta text-muted-foreground">
                      @{dialog.ins.username} ·{" "}
                      <span className="font-mono">{t("admin-inscripciones-dni", { dni: dialog.ins.dni })}</span>
                    </p>
                  </div>
                </div>
                <div className="mt-3 flex flex-wrap gap-1.5">
                  <Chip className="bg-card">{dialog.ins.grado}</Chip>
                  <Chip className="bg-card">{dialog.ins.especialidad}</Chip>
                </div>
                <div className="bg-card mt-3 rounded-lg px-3 py-2.5">
                  <p className="text-etiqueta text-texto-terciario">{t("simulacros-proceso")}</p>
                  <p className="text-control text-foreground font-medium">{dialog.ins.proceso_nombre}</p>
                  <p className="text-etiqueta text-muted-foreground">
                    {tipoLabel(dialog.ins.proceso_tipo)} ·{" "}
                    <span className="tabular-nums">{dialog.ins.proceso_anio}</span>
                  </p>
                </div>
              </div>
            ) : null}

            {dialog.accion === "rechazado" ? (
              <Campo id="ins-observacion" etiqueta={t("admin-usuarios-motivo-rechazo")}>
                <AreaTexto
                  id="ins-observacion"
                  rows={3}
                  required
                  autoFocus
                  value={dialog.observacion}
                  onChange={(e) => setDialog((d) => ({ ...d, observacion: e.target.value }))}
                  placeholder={t("admin-inscripciones-motivo-placeholder")}
                />
              </Campo>
            ) : null}
          </div>

          <PieDialogo>
            <button type="button" onClick={closeDialog} disabled={dialog.submitting} className={BOTON_SECUNDARIO}>
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={dialog.submitting}
              textoPendiente={t("comun-procesando")}
              onClick={confirm}
              disabled={dialog.accion === "rechazado" && !dialog.observacion.trim()}
              className={aprobando ? BOTON_PRIMARIO : BOTON_DESTRUCTIVO}
            >
              {aprobando ? t("admin-inscripciones-confirmar-aprobacion") : t("admin-inscripciones-confirmar-rechazo")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
