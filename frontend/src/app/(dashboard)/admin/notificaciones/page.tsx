"use client";
import { useTranslations } from "next-intl";
import React, { useCallback, useEffect, useMemo, useState } from "react";

import { DialogContent } from "@/components/app/dialogo";
import { type ColumnaListado, TablaListado } from "@/components/app/tabla-listado";
import { Alerta } from "@/components/feedback/alerta";
import { useToast } from "@/components/feedback/toast";
import { AreaTexto, Campo, Entrada } from "@/components/formularios/campo";
import { SelectCampo } from "@/components/formularios/select-campo";
import { MenuAcciones } from "@/components/navegacion/menu-acciones";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraDialogo, PieDialogo } from "@/components/patrones/cabecera-dialogo";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Chip } from "@/components/patrones/pill-estado";
import { Dialog } from "@/components/ui/dialog";
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
import type { IconoComponente } from "@/lib/iconos";
import {
  CreateNotificacionPayload,
  NotifDestinatario,
  Notificacion,
  notificacionesApi,
  NotifTipo,
} from "@/lib/notifications/api";
import { cn } from "@/lib/utils";
import {
  ArrowClockwiseIcon,
  BellIcon,
  MonitorIcon,
  PaperPlaneTiltIcon,
  SparkleIcon,
  TrashIcon,
  WarningCircleIcon,
  WrenchIcon,
} from "@phosphor-icons/react";

const TIPOS: NotifTipo[] = ["sistema", "mantenimiento", "funcionalidad", "alerta"];

const TIPO_CLASES: Record<NotifTipo, string> = {
  sistema: "bg-info text-info-foreground",
  mantenimiento: "bg-warning text-warning-foreground",
  funcionalidad: "bg-success text-success-foreground",
  alerta: "bg-destructive-suave text-destructive-texto",
};

const TIPO_ICON: Record<NotifTipo, IconoComponente> = {
  sistema: MonitorIcon,
  mantenimiento: WrenchIcon,
  funcionalidad: SparkleIcon,
  alerta: WarningCircleIcon,
};

// CreateRequest (backend/internal/notifications/dto.go): titulo min=3,max=200 · cuerpo min=3 (en runas).
const TITULO_MIN = 3;
const TITULO_MAX = 200;
const CUERPO_MIN = 3;
const largo = (s: string) => Array.from(s.trim()).length;

const DEFAULT_FORM: CreateNotificacionPayload = {
  titulo: "",
  cuerpo: "",
  tipo: "sistema",
  destinatario: "todos",
};

/** Pill de tipo: tono e icono propios de cada categoría. */
function PillTipo({ tipo, etiqueta }: { tipo: NotifTipo; etiqueta: string }) {
  const Icono = TIPO_ICON[tipo];
  return (
    <span
      className={cn(
        "text-etiqueta inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-medium whitespace-nowrap",
        TIPO_CLASES[tipo] ?? "bg-secondary text-muted-foreground",
      )}
    >
      {Icono ? <Icono aria-hidden className="size-3.5" /> : null}
      {etiqueta}
    </span>
  );
}

export default function AdminNotificacionesPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const [rows, setRows] = useState<Notificacion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [form, setForm] = useState<CreateNotificacionPayload>(DEFAULT_FORM);
  const [submitting, setSubmitting] = useState(false);

  const [confirmDialog, setConfirmDialog] = useState<{
    open: boolean;
    row: Notificacion | null;
    submitting: boolean;
  }>({ open: false, row: null, submitting: false });

  const tipoLabel = useMemo<Record<NotifTipo, string>>(
    () => ({
      sistema: t("admin-notif-tipo-sistema"),
      mantenimiento: t("admin-notif-tipo-mantenimiento"),
      funcionalidad: t("admin-notif-tipo-funcionalidad"),
      alerta: t("admin-notif-tipo-alerta"),
    }),
    [t],
  );
  const destinatarioLabel = useMemo<Record<NotifDestinatario, string>>(
    () => ({
      todos: t("admin-notif-dest-todos"),
      "rol:admin": t("admin-notif-dest-admin"),
      "rol:usuario": t("admin-notif-dest-user"),
    }),
    [t],
  );

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const data = await notificacionesApi.adminList(token);
      setRows(data);
    } catch {
      setError(t("comun-error-cargar"));
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    load();
  }, [load]);

  const tituloValido = largo(form.titulo) >= TITULO_MIN && largo(form.titulo) <= TITULO_MAX;
  const cuerpoValido = largo(form.cuerpo) >= CUERPO_MIN;
  const puedeEnviar = tituloValido && cuerpoValido;

  const handleSend = async () => {
    if (!token || !puedeEnviar) return;
    setSubmitting(true);
    try {
      await notificacionesApi.adminCreate(token, form);
      toast.ok(t("admin-notif-enviada"));
      setForm(DEFAULT_FORM);
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      toast.error(err.message ?? t("comun-error-guardar"));
    } finally {
      setSubmitting(false);
    }
  };

  const openConfirm = useCallback((row: Notificacion) => setConfirmDialog({ open: true, row, submitting: false }), []);
  const closeConfirm = () => setConfirmDialog((d) => ({ ...d, open: false }));

  const submitDelete = async () => {
    if (!token || !confirmDialog.row) return;
    setConfirmDialog((d) => ({ ...d, submitting: true }));
    try {
      await notificacionesApi.adminDelete(token, confirmDialog.row.id);
      toast.ok(t("admin-notif-eliminada"));
      closeConfirm();
      load();
    } catch (e: unknown) {
      const err = e as { message?: string };
      toast.error(err.message ?? t("comun-error-eliminar"));
      setConfirmDialog((d) => ({ ...d, submitting: false }));
    }
  };

  const columns = useMemo<ColumnaListado<Notificacion>[]>(
    () => [
      {
        id: "tipo",
        titulo: t("admin-notif-tipo-label"),
        ordenable: true,
        texto: (r) => tipoLabel[r.tipo] ?? r.tipo,
        render: (r) => <PillTipo tipo={r.tipo} etiqueta={tipoLabel[r.tipo] ?? r.tipo} />,
      },
      {
        id: "titulo",
        titulo: t("admin-notif-titulo-label"),
        ordenable: true,
        ocultable: false,
        render: (r) => <span className="text-foreground font-medium">{r.titulo}</span>,
      },
      {
        id: "cuerpo",
        titulo: t("admin-notif-col-mensaje"),
        className: "max-w-0 w-full whitespace-normal",
        render: (r) => (
          <span className="text-muted-foreground line-clamp-2" title={r.cuerpo}>
            {r.cuerpo}
          </span>
        ),
      },
      {
        id: "destinatario",
        titulo: t("admin-notif-destinatario-label"),
        ordenable: true,
        texto: (r) => destinatarioLabel[r.destinatario] ?? r.destinatario,
        render: (r) => <Chip>{destinatarioLabel[r.destinatario] ?? r.destinatario}</Chip>,
      },
      {
        id: "created_at",
        titulo: t("admin-notif-col-enviada"),
        ordenable: true,
        className: "tabular-nums text-muted-foreground",
        texto: (r) => (r.created_at ? formatearFechaHora(r.created_at) : ""),
        valor: (r) => r.created_at,
        render: (r) => (r.created_at ? formatearFechaHora(r.created_at) : ""),
      },
      {
        id: "acciones",
        titulo: <span className="sr-only">{t("practicas-acciones")}</span>,
        exportable: false,
        ocultable: false,
        alinear: "derecha",
        ancho: "3rem",
        render: (r) => (
          <MenuAcciones
            etiqueta={t("admin-notif-acciones-de", { titulo: r.titulo })}
            acciones={[
              { label: t("comun-eliminar"), icon: TrashIcon, destructiva: true, onClick: () => openConfirm(r) },
            ]}
          />
        ),
      },
    ],
    [openConfirm, t, tipoLabel, destinatarioLabel],
  );

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error ? (
        <Alerta
          tono="error"
          accion={
            <button type="button" onClick={load} className={ACCION_SECUNDARIA}>
              <ArrowClockwiseIcon weight="bold" className="size-4" />
              {t("admin-catalogos-reintentar")}
            </button>
          }
        >
          {error}
        </Alerta>
      ) : null}

      {/* Compose form */}
      <section className={TARJETA}>
        <CabeceraSeccion icon={BellIcon} titulo={t("admin-nueva-notificacion")} />
        <form
          className="grid gap-4 px-4 pt-2 pb-4 md:grid-cols-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (!submitting) handleSend();
          }}
        >
          <Campo id="notif-tipo" etiqueta={t("admin-notif-tipo-label")}>
            <SelectCampo
              id="notif-tipo"
              valor={form.tipo}
              onCambio={(v) => setForm((f) => ({ ...f, tipo: v as NotifTipo }))}
              opciones={TIPOS.map((tipo) => ({ value: tipo, label: tipoLabel[tipo] }))}
              required
            />
          </Campo>
          <Campo id="notif-destinatario" etiqueta={t("admin-notif-destinatario-label")}>
            <SelectCampo
              id="notif-destinatario"
              valor={form.destinatario}
              onCambio={(v) => setForm((f) => ({ ...f, destinatario: v as NotifDestinatario }))}
              opciones={[
                { value: "todos", label: t("admin-notif-dest-todos") },
                { value: "rol:usuario", label: t("admin-notif-dest-user") },
                { value: "rol:admin", label: t("admin-notif-dest-admin") },
              ]}
              required
            />
          </Campo>
          <Campo
            id="notif-titulo"
            etiqueta={t("admin-notif-titulo-label")}
            ayuda={
              <span className="tabular-nums">
                {t("admin-catalogos-minimo-caracteres", { n: TITULO_MIN })} · {form.titulo.length} / {TITULO_MAX}
              </span>
            }
            error={form.titulo && !tituloValido ? t("admin-catalogos-minimo-caracteres", { n: TITULO_MIN }) : undefined}
            className="md:col-span-2"
          >
            <Entrada
              id="notif-titulo"
              required
              maxLength={TITULO_MAX}
              invalido={!!form.titulo && !tituloValido}
              value={form.titulo}
              onChange={(e) => setForm((f) => ({ ...f, titulo: e.target.value }))}
            />
          </Campo>
          <Campo
            id="notif-cuerpo"
            etiqueta={t("admin-notif-cuerpo-label")}
            ayuda={t("admin-catalogos-minimo-caracteres", { n: CUERPO_MIN })}
            error={form.cuerpo && !cuerpoValido ? t("admin-catalogos-minimo-caracteres", { n: CUERPO_MIN }) : undefined}
            className="md:col-span-2"
          >
            <AreaTexto
              id="notif-cuerpo"
              required
              rows={3}
              aria-invalid={(!!form.cuerpo && !cuerpoValido) || undefined}
              value={form.cuerpo}
              onChange={(e) => setForm((f) => ({ ...f, cuerpo: e.target.value }))}
            />
          </Campo>
          <div className="flex flex-wrap items-center justify-between gap-3 md:col-span-2">
            <p className="text-etiqueta text-texto-terciario flex items-center gap-2">
              <span>{t("admin-notif-vista-previa")}</span>
              <PillTipo tipo={form.tipo} etiqueta={tipoLabel[form.tipo]} />
              <span aria-hidden>→</span>
              <span className="text-muted-foreground">{destinatarioLabel[form.destinatario]}</span>
            </p>
            <BotonAsync
              type="submit"
              pendiente={submitting}
              textoPendiente={t("admin-enviando-notificacion")}
              icon={<PaperPlaneTiltIcon className="size-4" />}
              disabled={!puedeEnviar}
              className={BOTON_PRIMARIO}
            >
              {t("admin-enviar-notificacion")}
            </BotonAsync>
          </div>
        </form>
      </section>

      {/* Sent notifications list */}
      <TablaListado
        titulo={t("admin-notificaciones-titulo")}
        columnas={columns}
        filas={rows}
        clave={(r) => String(r.id)}
        cargando={loading && rows.length === 0}
        exportar={false}
        placeholderBusqueda={t("tabla-buscar")}
        unidad={t("admin-notif-unidad")}
        ordenInicial={{ id: "created_at", dir: "desc" }}
        vacio={t("admin-notif-vacio")}
        acciones={
          <span className="text-control text-muted-foreground tabular-nums">
            {t("admin-notificaciones-subtitulo", { count: rows.length })}
          </span>
        }
      />

      {/* Delete confirm dialog */}
      <Dialog
        open={confirmDialog.open}
        onOpenChange={(open) => {
          if (!open && !confirmDialog.submitting) closeConfirm();
        }}
      >
        <DialogContent className="gap-0 p-0 sm:max-w-md">
          <CabeceraDialogo
            icon={TrashIcon}
            tono="peligro"
            titulo={t("admin-eliminar-notif-titulo")}
            descripcion={t("admin-eliminar-notif-confirm", { titulo: confirmDialog.row?.titulo ?? "" })}
          />
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
              textoPendiente={t("comun-eliminando")}
              onClick={submitDelete}
              className={BOTON_DESTRUCTIVO}
            >
              {t("comun-eliminar")}
            </BotonAsync>
          </PieDialogo>
        </DialogContent>
      </Dialog>
    </div>
  );
}
