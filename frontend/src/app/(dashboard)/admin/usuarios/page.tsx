"use client";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { ListaDatos } from "@/components/datos/lista-datos";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { AreaTexto, Campo, Entrada } from "@/components/formularios/campo";
import { type AccionFila, MenuAcciones } from "@/components/navegacion/menu-acciones";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import type { Tono } from "@/components/patrones/metricas";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { GrupoBotones } from "@/components/ui/grupo-botones";
import { excede, LIMITES } from "@/lib/admin/limites";
import { useAuth } from "@/lib/auth/auth-context";
import { BOTON_DESTRUCTIVO, BOTON_PRIMARIO, BOTON_SECUNDARIO, CONTENEDOR_PAGINA } from "@/lib/estilos";
import { formatearFecha, formatearFechaHora } from "@/lib/formato";
import { inscripcionesApi, SesionHistorialItem, UsuarioPendiente } from "@/lib/inscripciones/api";
import { cn } from "@/lib/utils";
import {
  ArrowClockwiseIcon,
  CheckCircleIcon,
  ClockIcon,
  EnvelopeSimpleIcon,
  EyeIcon,
  ProhibitIcon,
  XCircleIcon,
  XIcon,
} from "@phosphor-icons/react";

const TABS = ["todos", "pendiente", "activo", "rechazado", "inactivo"] as const;
type TabVal = (typeof TABS)[number];
type Accion = "activo" | "rechazado" | "inactivo";

const ESTADO_TONO: Record<string, Tono> = {
  pendiente: "aviso",
  activo: "ok",
  rechazado: "error",
  inactivo: "neutro",
};

function FichaUsuario({ usuario }: { usuario: UsuarioPendiente }) {
  const t = useTranslations("dashboard");
  return (
    <div className="bg-secondary rounded-xl p-3.5">
      <div className="flex items-center gap-3">
        <Avatar nombre={usuario.nombre_completo} tamano="lg" tono="accent" />
        <div className="min-w-0">
          <p className="text-control text-foreground truncate font-semibold">{usuario.nombre_completo}</p>
          <p className="text-etiqueta text-muted-foreground">
            @{usuario.username} · {t("admin-usuarios-dni")} <span className="font-mono">{usuario.dni}</span>
          </p>
        </div>
      </div>
      <div className="mt-3 flex flex-wrap gap-1.5">
        <Chip className="bg-card">{usuario.grado}</Chip>
        <Chip className="bg-card">{usuario.especialidad}</Chip>
      </div>
    </div>
  );
}

export default function Page() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const router = useRouter();
  /** Etiqueta traducida de un estado de cuenta; si el backend manda uno desconocido, se muestra tal cual. */
  const etiquetaEstado = (estado: string) =>
    TABS.includes(estado as TabVal) ? t(`admin-usuarios-estado-${estado}`) : estado;
  const [tab, setTab] = useState<TabVal>("pendiente");
  const [allRows, setAllRows] = useState<UsuarioPendiente[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [onlineIds, setOnlineIds] = useState<Set<number>>(new Set());
  const [sesionesDialog, setSesionesDialog] = useState<{
    open: boolean;
    usuario: UsuarioPendiente | null;
    data: SesionHistorialItem[];
    loading: boolean;
  }>({ open: false, usuario: null, data: [], loading: false });

  const [dialog, setDialog] = useState<{
    open: boolean;
    usuario: UsuarioPendiente | null;
    accion: Accion | null;
    observacion: string;
    submitting: boolean;
  }>({ open: false, usuario: null, accion: null, observacion: "", submitting: false });

  const [emailDialog, setEmailDialog] = useState<{
    open: boolean;
    usuario: UsuarioPendiente | null;
    email: string;
    submitting: boolean;
  }>({ open: false, usuario: null, email: "", submitting: false });

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const data = await inscripcionesApi.listUsuarios(token);
      setAllRows(data);
    } catch {
      setError(t("admin-error-cargar-usuarios"));
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!token) return;
    const fetchOnline = () =>
      inscripcionesApi
        .listUsuariosOnline(token)
        .then((r) => setOnlineIds(new Set(r.data.map((u) => u.id))))
        .catch(() => {});
    fetchOnline();
    const interval = setInterval(fetchOnline, 30_000);
    return () => clearInterval(interval);
  }, [token]);

  // Solo se aplica la respuesta de la última petición de sesiones, y solo si el diálogo sigue abierto para ese mismo
  // usuario: si no, el diálogo de B podría mostrar las IP y el user-agent de las sesiones de A.
  const sesionesReq = useRef(0);
  const openSesiones = useCallback(
    async (usuario: UsuarioPendiente) => {
      if (!token) return;
      const reqId = ++sesionesReq.current;
      setSesionesDialog({ open: true, usuario, data: [], loading: true });
      const vigente = (d: typeof sesionesDialog) =>
        reqId === sesionesReq.current && d.open && d.usuario?.id === usuario.id;
      try {
        const r = await inscripcionesApi.listSesionesUsuario(token, usuario.id);
        setSesionesDialog((d) => (vigente(d) ? { ...d, data: r.data, loading: false } : d));
      } catch {
        setSesionesDialog((d) => (vigente(d) ? { ...d, loading: false } : d));
      }
    },
    [token],
  );

  const rows = useMemo(() => (tab === "todos" ? allRows : allRows.filter((r) => r.estado === tab)), [allRows, tab]);

  const openDialog = useCallback(
    (usuario: UsuarioPendiente, accion: Accion) =>
      setDialog({ open: true, usuario, accion, observacion: "", submitting: false }),
    [],
  );

  const closeDialog = () => setDialog((d) => ({ ...d, open: false, submitting: false }));

  const confirm = async () => {
    if (!token || !dialog.usuario || !dialog.accion) return;
    if (dialog.accion === "rechazado" && !dialog.observacion.trim()) return;
    if (excede(dialog.observacion, LIMITES.observacionEstado)) return;
    setDialog((d) => ({ ...d, submitting: true }));
    try {
      await inscripcionesApi.updateUsuarioEstado(token, dialog.usuario.id, dialog.accion, dialog.observacion);
      toast.ok(
        dialog.accion === "activo"
          ? t("admin-usuarios-cuenta-activada")
          : dialog.accion === "inactivo"
            ? t("admin-usuarios-cuenta-desactivada")
            : t("admin-usuarios-cuenta-rechazada"),
        dialog.usuario.nombre_completo,
      );
      closeDialog();
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      // H21: the page Alerta is hidden behind the modal; a toast is visible over it.
      toast.error(err.message || t("admin-error-actualizar"));
      setDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const openEmailDialog = useCallback(
    (usuario: UsuarioPendiente) =>
      setEmailDialog({ open: true, usuario, email: usuario.email ?? "", submitting: false }),
    [],
  );

  const closeEmailDialog = () => setEmailDialog((d) => ({ ...d, open: false }));

  const handleResetTour = useCallback(
    async (usuario: UsuarioPendiente) => {
      if (!token) return;
      try {
        await inscripcionesApi.resetTourUsuario(token, usuario.id);
        toast.ok(t("admin-usuarios-recorrido-habilitado"), `@${usuario.username}`);
      } catch (e: unknown) {
        const err = e as { message?: string };
        toast.error(err.message || t("admin-usuarios-error-recorrido"));
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [token],
  );

  const confirmEmail = async () => {
    if (!token || !emailDialog.usuario || !emailDialog.email.includes("@")) return;
    setEmailDialog((d) => ({ ...d, submitting: true }));
    try {
      await inscripcionesApi.updateUsuarioEmail(token, emailDialog.usuario.id, emailDialog.email);
      toast.ok(t("admin-usuarios-correo-actualizado"), `@${emailDialog.usuario.username}`);
      closeEmailDialog();
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      // H21: 409 (email in use) / 422 must be visible over the dialog.
      toast.error(err.message || t("admin-error-actualizar-correo"));
      setEmailDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const columns = useMemo<ColumnaListado<UsuarioPendiente>[]>(
    () => [
      {
        id: "nombre_completo",
        titulo: t("admin-usuarios-col-nombre"),
        ordenable: true,
        ocultable: false,
        render: (r) => {
          const enLinea = onlineIds.has(r.id);
          return (
            <div className="flex min-w-48 items-center gap-2.5">
              <span className="relative shrink-0">
                <Avatar nombre={r.nombre_completo} tono="accent" />
                {enLinea ? (
                  <span
                    title={t("admin-usuarios-en-linea")}
                    aria-label={t("admin-usuarios-en-linea")}
                    className="bg-success-solid ring-card absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full ring-2"
                  />
                ) : null}
              </span>
              <span className="text-foreground truncate font-medium">{r.nombre_completo}</span>
            </div>
          );
        },
      },
      { id: "dni", titulo: t("admin-usuarios-dni"), ordenable: true, className: "font-mono tabular-nums" },
      {
        id: "username",
        titulo: t("admin-usuarios-col-username"),
        ordenable: true,
        render: (r) => <span className="text-muted-foreground">@{r.username}</span>,
      },
      { id: "grado", titulo: t("perfil-grado"), ordenable: true },
      { id: "especialidad", titulo: t("perfil-especialidad"), ordenable: true, ocultaInicial: true },
      { id: "email", titulo: t("admin-usuarios-col-email"), ordenable: true, ocultaInicial: true },
      {
        id: "fecha_solicitud",
        titulo: t("admin-usuarios-col-fecha-registro"),
        ordenable: true,
        className: "whitespace-nowrap tabular-nums",
        render: (r) => formatearFecha(r.fecha_solicitud),
        texto: (r) => formatearFecha(r.fecha_solicitud),
      },
      {
        id: "estado",
        titulo: t("perfil-estado"),
        ordenable: true,
        render: (r) => <PillEstado tono={ESTADO_TONO[r.estado] ?? "neutro"}>{etiquetaEstado(r.estado)}</PillEstado>,
        texto: (r) => etiquetaEstado(r.estado),
      },
      {
        id: "actions",
        titulo: "",
        alinear: "derecha",
        exportable: false,
        ocultable: false,
        ancho: "3.5rem",
        render: (r) => {
          const { estado } = r;
          const acciones: AccionFila[] = [];
          if (estado === "pendiente" || estado === "rechazado" || estado === "inactivo") {
            acciones.push({
              label: t("admin-activar-cuenta"),
              icon: CheckCircleIcon,
              onClick: () => openDialog(r, "activo"),
            });
          }
          if (estado === "pendiente") {
            acciones.push({
              label: t("admin-rechazar-cuenta"),
              icon: XCircleIcon,
              destructiva: true,
              onClick: () => openDialog(r, "rechazado"),
            });
          }
          if (estado === "activo") {
            acciones.push({
              label: t("admin-desactivar-cuenta"),
              icon: ProhibitIcon,
              onClick: () => openDialog(r, "inactivo"),
            });
          }
          acciones.push(
            {
              label: t("admin-usuarios-corregir-correo"),
              icon: EnvelopeSimpleIcon,
              onClick: () => openEmailDialog(r),
              separador: true,
            },
            { label: t("admin-habilitar-recorrido"), icon: ArrowClockwiseIcon, onClick: () => handleResetTour(r) },
            { label: t("admin-usuarios-ver-accesos"), icon: ClockIcon, onClick: () => openSesiones(r) },
            // H29: the detail page had no entry point.
            {
              label: t("admin-usuarios-ver-detalle"),
              icon: EyeIcon,
              onClick: () => router.push(`/admin/usuarios/${r.id}`),
            },
          );
          return (
            <MenuAcciones
              etiqueta={t("admin-usuarios-acciones-para", { nombre: r.nombre_completo })}
              acciones={acciones}
            />
          );
        },
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [openDialog, openEmailDialog, handleResetTour, openSesiones, onlineIds, t, router],
  );

  const contar = (v: TabVal) => (v === "todos" ? allRows.length : allRows.filter((r) => r.estado === v).length);

  const accion = dialog.accion;
  const sesion = sesionesDialog.data[0] ?? null;
  const usuarioSesion = sesionesDialog.usuario;

  return (
    <div className={CONTENEDOR_PAGINA}>
      <p className="text-cuerpo text-muted-foreground">
        {t.rich("admin-usuarios-resumen", {
          n: allRows.length,
          b: (c) => <span className="text-foreground font-medium tabular-nums">{c}</span>,
        })}
        {onlineIds.size > 0 ? (
          <span className="text-etiqueta text-success-foreground ml-2 inline-flex items-center gap-1.5">
            <span className="bg-success-solid size-1.5 rounded-full" />
            {t.rich("admin-usuarios-en-linea-n", {
              n: onlineIds.size,
              b: (c) => <span className="tabular-nums">{c}</span>,
            })}
          </span>
        ) : null}
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

      <TablaListado
        titulo={t("admin-usuarios-titulo")}
        columnas={columns}
        filas={rows}
        clave={(r) => String(r.id)}
        cargando={loading}
        unidad={t("admin-usuarios-unidad")}
        placeholderBusqueda={t("admin-usuarios-buscar")}
        vacio={
          tab === "todos"
            ? t("admin-usuarios-vacio")
            : t("admin-usuarios-vacio-estado", { estado: etiquetaEstado(tab) })
        }
        filtros={
          <GrupoBotones
            etiqueta={t("admin-usuarios-filtrar-estado")}
            valor={tab}
            onCambio={setTab}
            className="max-w-full overflow-x-auto"
            opciones={TABS.map((v) => ({
              valor: v,
              etiqueta: (
                <>
                  {etiquetaEstado(v)}
                  <span className="bg-secondary text-etiqueta text-muted-foreground rounded-full px-1.5 font-semibold tabular-nums">
                    {contar(v)}
                  </span>
                </>
              ),
            }))}
          />
        }
      />

      {/* ── Dialog: cambio de estado ── */}
      <Dialog
        open={dialog.open}
        onOpenChange={(open) => {
          if (!open && !dialog.submitting) closeDialog();
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={accion === "activo" ? CheckCircleIcon : accion === "inactivo" ? ProhibitIcon : XCircleIcon}
            tono={accion === "activo" ? "ok" : accion === "inactivo" ? "aviso" : "peligro"}
            titulo={
              accion === "activo"
                ? t("admin-activar-cuenta")
                : accion === "inactivo"
                  ? t("admin-desactivar-cuenta")
                  : t("admin-rechazar-cuenta")
            }
            descripcion={
              accion === "activo"
                ? t("admin-usuarios-desc-activar")
                : accion === "inactivo"
                  ? t("admin-usuarios-desc-desactivar")
                  : t("admin-usuarios-desc-rechazar")
            }
          />

          <div className="grid gap-4 px-5 pb-2">
            {dialog.usuario ? <FichaUsuario usuario={dialog.usuario} /> : null}

            {accion === "rechazado" || accion === "inactivo" ? (
              <Campo
                id="usuario-observacion"
                etiqueta={
                  accion === "rechazado" ? t("admin-usuarios-motivo-rechazo") : t("admin-usuarios-motivo-desactivacion")
                }
                opcional={accion === "inactivo"}
                error={
                  excede(dialog.observacion, LIMITES.observacionEstado)
                    ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.observacionEstado })
                    : undefined
                }
              >
                <AreaTexto
                  id="usuario-observacion"
                  rows={3}
                  maxLength={LIMITES.observacionEstado}
                  invalido={excede(dialog.observacion, LIMITES.observacionEstado)}
                  required={accion === "rechazado"}
                  autoFocus
                  value={dialog.observacion}
                  onChange={(e) => setDialog((d) => ({ ...d, observacion: e.target.value }))}
                  placeholder={
                    accion === "rechazado"
                      ? t("admin-usuarios-placeholder-rechazo")
                      : t("admin-usuarios-placeholder-desactivacion")
                  }
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
              disabled={
                (accion === "rechazado" && !dialog.observacion.trim()) ||
                excede(dialog.observacion, LIMITES.observacionEstado)
              }
              className={accion === "activo" ? BOTON_PRIMARIO : BOTON_DESTRUCTIVO}
            >
              {accion === "activo"
                ? t("comun-confirmar-activacion")
                : accion === "inactivo"
                  ? t("comun-confirmar-desactivacion")
                  : t("admin-inscripciones-confirmar-rechazo")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>

      {/* ── Dialog: último acceso ── */}
      <Dialog open={sesionesDialog.open} onOpenChange={(open) => setSesionesDialog((d) => ({ ...d, open }))}>
        <DialogContent className="gap-0 p-0 sm:max-w-sm">
          <div className="flex items-center gap-3 px-5 pt-5 pr-14 pb-4">
            <span className="relative shrink-0">
              <Avatar nombre={usuarioSesion?.nombre_completo ?? "?"} tamano="lg" tono="accent" />
              {sesion?.activa ? (
                <span className="bg-success-solid ring-popover absolute right-0 bottom-0 size-2.5 rounded-full ring-2" />
              ) : null}
            </span>
            <div className="min-w-0 flex-1">
              <DialogTitle className="truncate">{usuarioSesion?.nombre_completo}</DialogTitle>
              <p className="text-etiqueta text-muted-foreground">@{usuarioSesion?.username}</p>
            </div>
            {!sesionesDialog.loading ? (
              <PillEstado tono={sesion?.activa ? "ok" : "neutro"}>
                {sesion?.activa ? t("admin-usuarios-en-linea") : t("admin-usuarios-desconectado")}
              </PillEstado>
            ) : null}
          </div>

          <div className={cn("bg-secondary mx-5 rounded-xl px-4 py-3.5")}>
            {sesionesDialog.loading ? (
              <div className="flex flex-col gap-3" aria-busy>
                {[0, 1, 2, 3].map((i) => (
                  <div key={i} className="flex justify-between gap-4">
                    <Skeleton className="h-3 w-24" />
                    <Skeleton className="h-3 w-32" />
                  </div>
                ))}
              </div>
            ) : !sesion ? (
              <div className="text-muted-foreground flex flex-col items-center gap-2 py-4 text-center">
                <ClockIcon aria-hidden className="size-6" />
                <p className="text-cuerpo">{t("admin-usuarios-sin-accesos")}</p>
              </div>
            ) : (
              <ListaDatos
                datos={[
                  {
                    etiqueta: t("admin-usuarios-ultimo-acceso"),
                    valor: <span className="tabular-nums">{formatearFechaHora(sesion.last_seen_at)}</span>,
                  },
                  {
                    etiqueta: t("admin-usuarios-inicio-sesion"),
                    valor: <span className="tabular-nums">{formatearFechaHora(sesion.created_at)}</span>,
                  },
                  { etiqueta: t("admin-usuarios-ip"), valor: sesion.ip || "—", mono: true },
                  {
                    etiqueta: t("admin-usuarios-dispositivo"),
                    valor: sesion.user_agent
                      ? sesion.user_agent
                          .replace(/\(.*?\)/g, "")
                          .split(" ")
                          .filter(Boolean)
                          .slice(0, 4)
                          .join(" ")
                      : "—",
                  },
                ]}
              />
            )}
          </div>

          <PieDialogo>
            <button
              type="button"
              onClick={() => setSesionesDialog((d) => ({ ...d, open: false }))}
              className={BOTON_SECUNDARIO}
            >
              {t("comun-cerrar")}
            </button>
          </PieDialogo>
        </DialogContent>
      </Dialog>

      {/* ── Dialog: corregir correo ── */}
      <Dialog
        open={emailDialog.open}
        onOpenChange={(open) => {
          if (!open && !emailDialog.submitting) closeEmailDialog();
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={EnvelopeSimpleIcon}
            titulo={t("admin-usuarios-corregir-correo")}
            descripcion={t("admin-usuarios-correo-desc")}
          />
          <form
            id="form-correo"
            className="grid gap-4 px-5 pb-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (!emailDialog.submitting) confirmEmail();
            }}
          >
            {emailDialog.usuario ? (
              <p className="text-control text-muted-foreground">
                {t.rich("admin-usuarios-usuario-label", {
                  usuario: emailDialog.usuario.username,
                  b: (c) => <strong className="text-foreground font-semibold">{c}</strong>,
                })}
              </p>
            ) : null}
            <Campo id="usuario-email" etiqueta={t("admin-nuevo-correo-electronico")}>
              <Entrada
                id="usuario-email"
                type="email"
                required
                autoFocus
                value={emailDialog.email}
                onChange={(e) => setEmailDialog((d) => ({ ...d, email: e.target.value }))}
                placeholder={t("admin-usuarios-correo-placeholder")}
              />
            </Campo>
          </form>
          <PieDialogo>
            <button
              type="button"
              onClick={closeEmailDialog}
              disabled={emailDialog.submitting}
              className={BOTON_SECUNDARIO}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              type="submit"
              form="form-correo"
              pendiente={emailDialog.submitting}
              textoPendiente={t("comun-guardando")}
              disabled={!emailDialog.email.includes("@")}
              className={BOTON_PRIMARIO}
            >
              {t("admin-guardar-correo")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
