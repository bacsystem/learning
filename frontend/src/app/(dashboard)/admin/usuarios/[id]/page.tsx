"use client";

import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";

import { ListaDatos } from "@/components/datos/lista-datos";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton, SkeletonMetricas } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { AreaTexto, Campo, Entrada } from "@/components/formularios/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import type { Tono } from "@/components/patrones/metricas";
import { FilaMetricas, Metrica } from "@/components/patrones/metricas";
import { PillEstado } from "@/components/patrones/pill-estado";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { adminGetUsuario, adminPatchUsuarioRol, type UsuarioDetalle } from "@/lib/admin/api";
import { excede, LIMITES } from "@/lib/admin/limites";
import { useAuth } from "@/lib/auth/auth-context";
import {
  ACCION_SECUNDARIA,
  BOTON_DESTRUCTIVO,
  BOTON_PRIMARIO,
  BOTON_SECUNDARIO,
  CONTENEDOR_PAGINA,
  TARJETA,
} from "@/lib/estilos";
import { formatearFechaHora } from "@/lib/formato";
import { inscripcionesApi } from "@/lib/inscripciones/api";
import { cn } from "@/lib/utils";
import {
  BookOpenIcon,
  ChartLineIcon,
  CheckCircleIcon,
  EnvelopeSimpleIcon,
  IdentificationCardIcon,
  ProhibitIcon,
  ShieldCheckIcon,
  SlidersHorizontalIcon,
  TrophyIcon,
  UserIcon,
  XIcon,
} from "@phosphor-icons/react";

const ESTADO_TONO: Record<string, Tono> = {
  pendiente: "aviso",
  activo: "ok",
  rechazado: "error",
  inactivo: "neutro",
};

const ROLES = ["usuario", "admin"];
const ESTADOS = Object.keys(ESTADO_TONO);

function FilaAccion({ etiqueta, valor, children }: { etiqueta: string; valor: string; children: React.ReactNode }) {
  return (
    <div className="border-border/60 flex flex-col gap-2 border-b py-3 first:pt-0 last:border-0 last:pb-0">
      <p className="text-control text-muted-foreground">
        {etiqueta}: <strong className="text-foreground font-semibold break-all">{valor}</strong>
      </p>
      {children}
    </div>
  );
}

export default function UsuarioDetallePage() {
  const { id } = useParams<{ id: string }>();
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  /** Etiquetas traducidas; un valor desconocido del backend se muestra tal cual. */
  const etiquetaEstado = (estado: string) => (ESTADOS.includes(estado) ? t(`admin-usuarios-estado-${estado}`) : estado);
  const etiquetaRol = (rol: string) => (ROLES.includes(rol) ? t(`admin-usuarios-rol-${rol}`) : rol);
  const opcionesRol = ROLES.map((rol) => ({ value: rol, label: etiquetaRol(rol) }));

  const [usuario, setUsuario] = useState<UsuarioDetalle | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Dialogs
  const [rolDialog, setRolDialog] = useState<{ open: boolean; rol: string; submitting: boolean }>({
    open: false,
    rol: "",
    submitting: false,
  });

  const [emailDialog, setEmailDialog] = useState<{
    open: boolean;
    email: string;
    submitting: boolean;
  }>({ open: false, email: "", submitting: false });

  const [estadoSubmitting, setEstadoSubmitting] = useState(false);
  // H29: deactivating asks for confirmation, with the same dialog as the users list.
  const [desactivarDialog, setDesactivarDialog] = useState<{ open: boolean; observacion: string }>({
    open: false,
    observacion: "",
  });

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const data = await adminGetUsuario(token, Number(id));
      setUsuario(data);
    } catch (e: unknown) {
      const err = e as { message?: string };
      setError(err.message || t("admin-usuarios-error-cargar-usuario"));
    } finally {
      setLoading(false);
    }
  }, [token, id, t]);

  useEffect(() => {
    load();
  }, [load]);

  const handleCambiarRol = async () => {
    if (!token || !usuario) return;
    setRolDialog((d) => ({ ...d, submitting: true }));
    try {
      await adminPatchUsuarioRol(token, usuario.id, rolDialog.rol);
      setRolDialog({ open: false, rol: "", submitting: false });
      toast.ok(t("admin-usuarios-rol-actualizado"));
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      // H21: the page Alerta is hidden behind the modal.
      toast.error(err.message || t("admin-usuarios-error-rol"));
      setRolDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const handleCambiarEmail = async () => {
    if (!token || !usuario) return;
    setEmailDialog((d) => ({ ...d, submitting: true }));
    try {
      await inscripcionesApi.updateUsuarioEmail(token, usuario.id, emailDialog.email);
      setEmailDialog({ open: false, email: "", submitting: false });
      toast.ok(t("admin-usuarios-correo-actualizado"));
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      // H21: 409 (email in use) / 422 must be visible over the dialog.
      toast.error(err.message || t("admin-usuarios-error-correo"));
      setEmailDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const cambiarEstado = async (newEstado: "activo" | "inactivo", observacion: string) => {
    if (!token || !usuario) return;
    setEstadoSubmitting(true);
    try {
      await inscripcionesApi.updateUsuarioEstado(token, usuario.id, newEstado, observacion);
      toast.ok(newEstado === "activo" ? t("admin-usuarios-cuenta-activada") : t("admin-usuarios-cuenta-desactivada"));
      setDesactivarDialog({ open: false, observacion: "" });
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      toast.error(err.message || t("comun-error-estado"));
    } finally {
      setEstadoSubmitting(false);
    }
  };

  const handleToggleEstado = () => {
    if (!usuario) return;
    if (usuario.estado === "activo") setDesactivarDialog({ open: true, observacion: "" });
    else cambiarEstado("activo", "");
  };

  const volver = <EnlaceVolver href="/admin/usuarios">{t("menu-admin-usuarios")}</EnlaceVolver>;

  if (loading && !usuario) {
    return (
      <div className={CONTENEDOR_PAGINA} aria-busy>
        <div>{volver}</div>
        <div className="flex items-center gap-3">
          <Skeleton className="size-12 rounded-full" />
          <div className="flex flex-col gap-2">
            <Skeleton className="h-5 w-64" />
            <Skeleton className="h-3 w-40" />
          </div>
        </div>
        <SkeletonMetricas columnas={3} />
        <div className="grid grid-cols-1 gap-3 md:grid-cols-12">
          <Skeleton className="h-72 rounded-xl md:col-span-7" />
          <Skeleton className="h-72 rounded-xl md:col-span-5" />
        </div>
      </div>
    );
  }

  if (!usuario) {
    return (
      <div className={CONTENEDOR_PAGINA}>
        <div>{volver}</div>
        <EstadoVacio icon={UserIcon} titulo={t("admin-usuarios-no-encontrado")}>
          {error ?? t("admin-usuarios-no-encontrado-detalle")}
        </EstadoVacio>
      </div>
    );
  }

  const nombreCompleto = `${usuario.nombres} ${usuario.apellido_paterno} ${usuario.apellido_materno}`;
  const activo = usuario.estado === "activo";

  return (
    <div className={CONTENEDOR_PAGINA}>
      <CabeceraPagina
        volver={volver}
        titulo={
          <span className="flex items-center gap-3">
            <Avatar nombre={nombreCompleto} tamano="xl" tono="accent" />
            <span className="min-w-0">{nombreCompleto}</span>
          </span>
        }
        descripcion={
          <>
            @{usuario.username} · {t("admin-usuarios-id")} <span className="font-mono">{usuario.id}</span>
          </>
        }
        acciones={
          <PillEstado tono={ESTADO_TONO[usuario.estado] ?? "neutro"}>{etiquetaEstado(usuario.estado)}</PillEstado>
        }
      />

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

      {/* Métricas */}
      <FilaMetricas columnas={3}>
        <Metrica
          etiqueta={t("perfil-practicas-completadas")}
          icono={BookOpenIcon}
          ayuda={t("admin-usuarios-total-historico")}
        >
          {usuario.total_practicas}
        </Metrica>
        <Metrica
          etiqueta={t("perfil-simulacros-realizados")}
          icono={ChartLineIcon}
          ayuda={t("admin-usuarios-total-historico")}
        >
          {usuario.total_simulacros}
        </Metrica>
        <Metrica
          etiqueta={t("perfil-nota-promedio")}
          icono={TrophyIcon}
          tono="acento"
          destacada
          ayuda={t("admin-usuarios-promedio-simulacros")}
        >
          {usuario.nota_promedio.toFixed(1)}
        </Metrica>
      </FilaMetricas>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-12">
        {/* Datos personales */}
        <section className={cn(TARJETA, "md:col-span-7")}>
          <CabeceraSeccion icon={IdentificationCardIcon} titulo={t("admin-usuarios-datos-personales")} />
          <div className="px-4 pt-2 pb-4">
            <ListaDatos
              columnas={2}
              datos={[
                { etiqueta: t("admin-usuarios-dni"), valor: usuario.dni ?? "—", mono: true },
                { etiqueta: t("admin-usuarios-col-username"), valor: `@${usuario.username}` },
                { etiqueta: t("perfil-nombres"), valor: usuario.nombres ?? "—" },
                { etiqueta: t("perfil-apellido-paterno"), valor: usuario.apellido_paterno ?? "—" },
                { etiqueta: t("perfil-apellido-materno"), valor: usuario.apellido_materno ?? "—" },
                { etiqueta: t("perfil-grado"), valor: usuario.grado ?? "—" },
                { etiqueta: t("perfil-especialidad"), valor: usuario.especialidad ?? "—" },
                { etiqueta: t("perfil-institucion"), valor: usuario.institucion ?? "—" },
                { etiqueta: t("admin-usuarios-telefono"), valor: usuario.telefono || "—" },
                {
                  etiqueta: t("admin-usuarios-col-email"),
                  valor: <span className="break-all">{usuario.email ?? "—"}</span>,
                },
                { etiqueta: t("perfil-rol"), valor: usuario.rol ? etiquetaRol(usuario.rol) : "—" },
                { etiqueta: t("perfil-estado"), valor: usuario.estado ? etiquetaEstado(usuario.estado) : "—" },
                ...(usuario.activated_at
                  ? [
                      {
                        etiqueta: t("admin-usuarios-activado-el"),
                        valor: <span className="tabular-nums">{formatearFechaHora(usuario.activated_at)}</span>,
                      },
                    ]
                  : []),
                {
                  etiqueta: t("admin-usuarios-registrado-el"),
                  valor: <span className="tabular-nums">{formatearFechaHora(usuario.created_at)}</span>,
                },
              ]}
            />
          </div>
        </section>

        {/* Acciones */}
        <section className={cn(TARJETA, "md:col-span-5")}>
          <CabeceraSeccion icon={SlidersHorizontalIcon} titulo={t("admin-usuarios-acciones")} />
          <div className="px-4 pt-2 pb-4">
            <FilaAccion etiqueta={t("admin-usuarios-rol-actual")} valor={etiquetaRol(usuario.rol)}>
              <button
                type="button"
                className={cn(ACCION_SECUNDARIA, "justify-center")}
                onClick={() => setRolDialog({ open: true, rol: usuario.rol, submitting: false })}
              >
                <ShieldCheckIcon className="size-4" />
                {t("admin-usuarios-cambiar-rol")}
              </button>
            </FilaAccion>
            <FilaAccion etiqueta={t("admin-usuarios-col-email")} valor={usuario.email}>
              <button
                type="button"
                className={cn(ACCION_SECUNDARIA, "justify-center")}
                onClick={() => setEmailDialog({ open: true, email: usuario.email, submitting: false })}
              >
                <EnvelopeSimpleIcon className="size-4" />
                {t("admin-usuarios-cambiar-correo")}
              </button>
            </FilaAccion>
            <FilaAccion etiqueta={t("admin-usuarios-estado-actual")} valor={etiquetaEstado(usuario.estado)}>
              <BotonAsync
                pendiente={estadoSubmitting && !desactivarDialog.open}
                textoPendiente={t("comun-procesando")}
                onClick={handleToggleEstado}
                icon={activo ? <ProhibitIcon className="size-4" /> : <CheckCircleIcon className="size-4" />}
                className={cn(
                  ACCION_SECUNDARIA,
                  "justify-center",
                  activo
                    ? "text-destructive-texto hover:bg-destructive-suave"
                    : "text-success-foreground hover:bg-success",
                )}
              >
                {activo ? t("admin-desactivar-cuenta") : t("admin-activar-cuenta")}
              </BotonAsync>
            </FilaAccion>
          </div>
        </section>
      </div>

      {/* Dialog: Cambiar Rol */}
      <Dialog
        open={rolDialog.open}
        onOpenChange={(open) => {
          if (!open && !rolDialog.submitting) setRolDialog((d) => ({ ...d, open: false }));
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={ShieldCheckIcon}
            titulo={t("admin-usuarios-cambiar-rol")}
            descripcion={`@${usuario.username}`}
          />
          <div className="grid gap-4 px-5 pb-2">
            <Campo id="usuario-rol" etiqueta={t("perfil-rol")}>
              <SelectCampo
                id="usuario-rol"
                opciones={opcionesRol}
                valor={rolDialog.rol}
                onCambio={(v) => setRolDialog((d) => ({ ...d, rol: v }))}
              />
            </Campo>
          </div>
          <PieDialogo>
            <button
              type="button"
              onClick={() => setRolDialog((d) => ({ ...d, open: false }))}
              disabled={rolDialog.submitting}
              className={BOTON_SECUNDARIO}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={rolDialog.submitting}
              textoPendiente={t("comun-guardando")}
              disabled={!rolDialog.rol}
              onClick={handleCambiarRol}
              className={BOTON_PRIMARIO}
            >
              {t("comun-guardar")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>

      {/* Dialog: Cambiar Email */}
      <Dialog
        open={emailDialog.open}
        onOpenChange={(open) => {
          if (!open && !emailDialog.submitting) setEmailDialog((d) => ({ ...d, open: false }));
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={EnvelopeSimpleIcon}
            titulo={t("admin-usuarios-cambiar-correo")}
            descripcion={`@${usuario.username}`}
          />
          <form
            id="form-cambiar-email"
            className="grid gap-4 px-5 pb-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (!emailDialog.submitting && emailDialog.email.includes("@")) handleCambiarEmail();
            }}
          >
            <Campo id="usuario-nuevo-email" etiqueta={t("admin-nuevo-correo-electronico")}>
              <Entrada
                id="usuario-nuevo-email"
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
              onClick={() => setEmailDialog((d) => ({ ...d, open: false }))}
              disabled={emailDialog.submitting}
              className={BOTON_SECUNDARIO}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              type="submit"
              form="form-cambiar-email"
              pendiente={emailDialog.submitting}
              textoPendiente={t("comun-guardando")}
              disabled={!emailDialog.email.includes("@")}
              className={BOTON_PRIMARIO}
            >
              {t("comun-guardar")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>

      {/* Dialog: confirmar desactivación (mismo diálogo que la lista de usuarios) */}
      <Dialog
        open={desactivarDialog.open}
        onOpenChange={(open) => {
          if (!open && !estadoSubmitting) setDesactivarDialog((d) => ({ ...d, open: false }));
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={ProhibitIcon}
            tono="aviso"
            titulo={t("admin-desactivar-cuenta")}
            descripcion={t("admin-usuarios-desc-desactivar")}
          />
          <div className="grid gap-4 px-5 pb-2">
            <Campo
              id="usuario-detalle-observacion"
              etiqueta={t("admin-usuarios-motivo-desactivacion")}
              opcional
              error={
                excede(desactivarDialog.observacion, LIMITES.observacionEstado)
                  ? t("admin-catalogos-maximo-caracteres", { n: LIMITES.observacionEstado })
                  : undefined
              }
            >
              <AreaTexto
                id="usuario-detalle-observacion"
                rows={3}
                maxLength={LIMITES.observacionEstado}
                invalido={excede(desactivarDialog.observacion, LIMITES.observacionEstado)}
                autoFocus
                value={desactivarDialog.observacion}
                onChange={(e) => setDesactivarDialog((d) => ({ ...d, observacion: e.target.value }))}
                placeholder={t("admin-usuarios-placeholder-desactivacion")}
              />
            </Campo>
          </div>
          <PieDialogo>
            <button
              type="button"
              onClick={() => setDesactivarDialog((d) => ({ ...d, open: false }))}
              disabled={estadoSubmitting}
              className={BOTON_SECUNDARIO}
            >
              {t("comun-cancelar")}
            </button>
            <BotonAsync
              pendiente={estadoSubmitting}
              textoPendiente={t("comun-procesando")}
              onClick={() => cambiarEstado("inactivo", desactivarDialog.observacion)}
              disabled={excede(desactivarDialog.observacion, LIMITES.observacionEstado)}
              className={BOTON_DESTRUCTIVO}
            >
              {t("comun-confirmar-desactivacion")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
