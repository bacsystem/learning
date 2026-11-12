"use client";

import { InsigniaOficial, PillEstadoFuente, TextoProfesiones } from "../componentes";
import { DialogoEdicion, DialogoMarca, DialogoMotivo, DialogoVersion } from "../dialogos";
import { useOpcionesFuente } from "../use-opciones-fuente";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useFormatter, useTranslations } from "next-intl";
import { useState } from "react";

import { BotonReintentar } from "@/components/app/boton-reintentar";
import { ListaDatos } from "@/components/datos/lista-datos";
import { type EventoTimeline, Timeline } from "@/components/datos/timeline";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { DialogoConfirmacion } from "@/components/navegacion/dialogo-confirmacion";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { useCarga } from "@/hooks/use-carga";
import { useSondeo } from "@/hooks/use-sondeo";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import { type FuenteDetalle, fuentesApi, type VersionFuente } from "@/lib/serums-ia/fuentes/api";
import { mensajeErrorFuente } from "@/lib/serums-ia/fuentes/errores";
import { accionesDisponibles, type AccionFuente, enProceso, REFRESCO_MS } from "@/lib/serums-ia/fuentes/reglas";
import { cn } from "@/lib/utils";
import {
  ArchiveIcon,
  ArrowClockwiseIcon,
  ArrowCounterClockwiseIcon,
  ArrowLeftIcon,
  BooksIcon,
  ClockCounterClockwiseIcon,
  FileArrowDownIcon,
  FileArrowUpIcon,
  PencilSimpleIcon,
  SealCheckIcon,
} from "@phosphor-icons/react";

type Dialogo = "version" | "editar" | "retirar" | "restaurar" | "reintentar" | "marcar" | "desmarcar" | null;

/** Botones de acción en orden; cada uno se muestra solo si `accionesDisponibles` lo permite. */
const BOTONES: { accion: AccionFuente; dialogo: Exclude<Dialogo, null>; icono: IconoComponente }[] = [
  { accion: "nuevaVersion", dialogo: "version", icono: FileArrowUpIcon },
  { accion: "reintentar", dialogo: "reintentar", icono: ArrowClockwiseIcon },
  { accion: "restaurar", dialogo: "restaurar", icono: ArrowCounterClockwiseIcon },
  { accion: "editar", dialogo: "editar", icono: PencilSimpleIcon },
  { accion: "marcarOficial", dialogo: "marcar", icono: SealCheckIcon },
  { accion: "desmarcarOficial", dialogo: "desmarcar", icono: SealCheckIcon },
  { accion: "retirar", dialogo: "retirar", icono: ArchiveIcon },
];

const TONO_VERSION: Record<VersionFuente["estado"], EventoTimeline["tono"]> = {
  lista: "ok",
  procesando: "aviso",
  fallida: "error",
  archivada: "neutro",
};

export default function FuenteDetallePage() {
  const { id } = useParams<{ id: string }>();
  const fuenteId = Number(id);
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const toast = useToast();
  const opciones = useOpcionesFuente(token);
  const [dialogo, setDialogo] = useState<Dialogo>(null);

  const carga = useCarga(token && fuenteId > 0 ? `fuente-${fuenteId}` : null, () =>
    fuentesApi.detalle(token ?? "", fuenteId),
  );
  const fuente = carga.datos;
  useSondeo(Boolean(fuente && enProceso(fuente)), carga.refrescar, REFRESCO_MS);

  const fecha = (iso: string) =>
    formato.dateTime(new Date(iso), { dateStyle: "medium", timeStyle: "short", timeZone: "America/Lima" });

  /** Aplica la respuesta de una acción (la fuente actualizada) y cierra el diálogo. */
  function aplicar(actualizada: FuenteDetalle, aviso: string) {
    carga.reemplazar(actualizada);
    setDialogo(null);
    toast.ok(aviso);
  }

  if (carga.error) {
    return (
      <div className={CONTENEDOR_PAGINA}>
        <Alerta tono="error" accion={<BotonReintentar onReintentar={carga.recargar} />}>
          {mensajeErrorFuente(carga.error, t, t("serumsIa.fuentes.errorCargarDetalle"))}
        </Alerta>
      </div>
    );
  }
  if (!fuente || !token) {
    return (
      <div className={CONTENEDOR_PAGINA} aria-busy="true">
        <Skeleton className="h-40 w-full rounded-xl" />
        <Skeleton className="h-56 w-full rounded-xl" />
      </div>
    );
  }

  const acciones = accionesDisponibles(fuente);
  // Las profesiones ya asociadas se ofrecen aunque se hayan desactivado, para poder quitarlas.
  const activas = new Set(opciones.profesiones.map((p) => p.id));
  const profesionesEdicion = [
    ...opciones.profesiones,
    ...fuente.especialidades.filter((e) => !activas.has(e.id)).map((e) => ({ ...e, inactiva: true })),
  ];
  const ultimaFallida = fuente.versiones.find((v) => v.estado === "fallida");

  const eventos: EventoTimeline[] = fuente.versiones.map((v) => ({
    id: String(v.numero),
    titulo: (
      <span className="flex flex-wrap items-center gap-2">
        {t("serumsIa.fuentes.detalle.version", { numero: v.numero })}
        <span className="text-muted-foreground font-normal">· {t(`serumsIa.fuentes.versionEstado.${v.estado}`)}</span>
        {v.oficial_verificada ? <InsigniaOficial /> : null}
      </span>
    ),
    detalle: (
      <span className="grid gap-0.5">
        <span className="font-mono wrap-anywhere">{v.nombre_archivo}</span>
        <span>
          {t("serumsIa.fuentes.detalle.versionDatos", {
            pasajes: v.pasajes_count,
            lineas: v.lineas,
            kb: Math.ceil(v.tamano_bytes / 1024),
          })}
        </span>
        {v.oficial_entidad ? (
          <span>{t("serumsIa.fuentes.detalle.versionOficial", { entidad: v.oficial_entidad })}</span>
        ) : null}
        {v.motivo_fallo ? (
          <span className="text-destructive">{t(`serumsIa.fuentes.motivos.${v.motivo_fallo}`)}</span>
        ) : null}
      </span>
    ),
    fecha: fecha(v.created_at),
    tono: TONO_VERSION[v.estado],
    actual: fuente.version_vigente?.numero === v.numero,
  }));

  return (
    <div className={CONTENEDOR_PAGINA}>
      <Link
        href="/admin/fuentes"
        className="text-control text-muted-foreground hover:text-foreground inline-flex w-fit items-center gap-1.5"
      >
        <ArrowLeftIcon className="size-4" />
        {t("serumsIa.fuentes.detalle.volver")}
      </Link>

      <section className={cn(TARJETA, "flex flex-col")} aria-labelledby="fuente-titulo">
        <CabeceraSeccion
          icon={BooksIcon}
          titulo={
            <span id="fuente-titulo" className="wrap-anywhere">
              {fuente.titulo}
            </span>
          }
          derecha={<PillEstadoFuente estado={fuente.estado} />}
        />
        <div className="flex flex-col gap-4 p-4">
          {fuente.estado === "fallida" && ultimaFallida?.motivo_fallo ? (
            <Alerta tono="error" titulo={t("serumsIa.fuentes.detalle.fallida")}>
              {t(`serumsIa.fuentes.motivos.${ultimaFallida.motivo_fallo}`)}
            </Alerta>
          ) : null}
          {fuente.estado === "procesando" || fuente.version_en_proceso?.estado === "procesando" ? (
            <Alerta tono="info">{t("serumsIa.fuentes.detalle.procesando")}</Alerta>
          ) : null}
          {fuente.estado === "retirada" && fuente.motivo_retiro ? (
            <Alerta tono="aviso" titulo={t("serumsIa.fuentes.detalle.retirada")}>
              {fuente.motivo_retiro}
            </Alerta>
          ) : null}

          <ListaDatos
            columnas={2}
            datos={[
              { etiqueta: t("serumsIa.fuentes.col.tipo"), valor: t(`serumsIa.fuentes.tipo.${fuente.tipo}`) },
              {
                etiqueta: t("serumsIa.fuentes.col.profesiones"),
                valor: <TextoProfesiones aplicaATodas={fuente.aplica_a_todas} especialidades={fuente.especialidades} />,
              },
              {
                etiqueta: t("serumsIa.fuentes.col.version"),
                valor: fuente.version_vigente ? `v${fuente.version_vigente.numero}` : "—",
                mono: true,
              },
              {
                etiqueta: t("serumsIa.fuentes.col.pasajes"),
                valor: fuente.version_vigente ? formato.number(fuente.version_vigente.pasajes_count) : "—",
                mono: true,
              },
              { etiqueta: t("serumsIa.fuentes.campo.anio"), valor: fuente.anio ?? "—" },
              { etiqueta: t("serumsIa.fuentes.campo.proceso"), valor: fuente.proceso?.nombre ?? "—" },
              {
                etiqueta: t("serumsIa.fuentes.detalle.citantes"),
                valor: formato.number(fuente.contenidos_citantes),
                mono: true,
              },
            ]}
          />

          {fuente.oficial ? (
            <div className="bg-secondary grid gap-1 rounded-lg p-3">
              <InsigniaOficial />
              <p className="text-control text-foreground">
                {t("serumsIa.fuentes.detalle.oficialEntidad", { entidad: fuente.oficial.entidad_emisora })}
              </p>
              <p className="text-control text-muted-foreground whitespace-pre-line">{fuente.oficial.nota}</p>
            </div>
          ) : null}

          <div className="flex flex-wrap gap-2">
            {BOTONES.filter((b) => acciones.has(b.accion)).map((b, i) => (
              <button
                key={b.accion}
                type="button"
                onClick={() => setDialogo(b.dialogo)}
                className={i === 0 ? ACCION_PRINCIPAL : ACCION_SECUNDARIA}
              >
                <b.icono className="size-4" />
                {t(`serumsIa.fuentes.acciones.${b.accion}`)}
              </button>
            ))}
            {fuente.tipo === "balotario" && fuente.estado === "activa" ? (
              <Link href={`/admin/fuentes/${fuente.id}/balotario`} className={ACCION_SECUNDARIA}>
                <FileArrowDownIcon className="size-4" />
                {t("serumsIa.fuentes.acciones.importarBalotario")}
              </Link>
            ) : null}
          </div>
        </div>
      </section>

      <section className={cn(TARJETA, "flex flex-col")} aria-labelledby="fuente-versiones">
        <CabeceraSeccion
          icon={ClockCounterClockwiseIcon}
          titulo={<span id="fuente-versiones">{t("serumsIa.fuentes.detalle.versiones")}</span>}
        />
        <div className="p-4">
          <Timeline eventos={eventos} />
        </div>
      </section>

      {dialogo === "version" ? (
        <DialogoVersion
          titulo={fuente.titulo}
          onCerrar={() => setDialogo(null)}
          onSubir={async (archivo) =>
            aplicar(await fuentesApi.nuevaVersion(token, fuente.id, archivo), t("serumsIa.fuentes.version.ok"))
          }
        />
      ) : null}
      {dialogo === "editar" ? (
        <DialogoEdicion
          titulo={fuente.titulo}
          especialidadIds={fuente.especialidades.map((e) => e.id)}
          profesiones={profesionesEdicion}
          onCerrar={() => setDialogo(null)}
          onGuardar={async (titulo, ids) =>
            aplicar(await fuentesApi.editar(token, fuente.id, titulo, ids), t("serumsIa.fuentes.editar.ok"))
          }
        />
      ) : null}
      {dialogo === "retirar" ? (
        <DialogoMotivo
          titulo={t("serumsIa.fuentes.retirar.titulo")}
          descripcion={fuente.titulo}
          detalle={t("serumsIa.fuentes.retirar.citantes", { n: fuente.contenidos_citantes })}
          textoConfirmar={t("serumsIa.fuentes.acciones.retirar")}
          onCerrar={() => setDialogo(null)}
          onConfirmar={async (motivo) => {
            const r = await fuentesApi.cambiarEstado(token, fuente.id, { estado: "retirada", motivo });
            aplicar(r, t("serumsIa.fuentes.retirar.ok", { n: r.contenidos_marcados ?? 0 }));
          }}
        />
      ) : null}
      {dialogo === "desmarcar" ? (
        <DialogoMotivo
          titulo={t("serumsIa.fuentes.desmarcar.titulo")}
          descripcion={t("serumsIa.fuentes.desmarcar.efecto")}
          textoConfirmar={t("serumsIa.fuentes.acciones.desmarcarOficial")}
          onCerrar={() => setDialogo(null)}
          onConfirmar={async (motivo) =>
            aplicar(await fuentesApi.desmarcarOficial(token, fuente.id, motivo), t("serumsIa.fuentes.desmarcar.ok"))
          }
        />
      ) : null}
      {dialogo === "marcar" ? (
        <DialogoMarca
          titulo={fuente.titulo}
          onCerrar={() => setDialogo(null)}
          onConfirmar={async (entidad, nota) =>
            aplicar(await fuentesApi.marcarOficial(token, fuente.id, entidad, nota), t("serumsIa.fuentes.marcar.ok"))
          }
        />
      ) : null}
      <DialogoConfirmacion
        open={dialogo === "restaurar" || dialogo === "reintentar"}
        onOpenChange={(abierto) => !abierto && setDialogo(null)}
        icon={dialogo === "restaurar" ? ArrowCounterClockwiseIcon : ArrowClockwiseIcon}
        titulo={
          dialogo === "restaurar" ? t("serumsIa.fuentes.restaurar.titulo") : t("serumsIa.fuentes.reintentar.titulo")
        }
        descripcion={
          dialogo === "restaurar" ? t("serumsIa.fuentes.restaurar.efecto") : t("serumsIa.fuentes.reintentar.efecto")
        }
        textoConfirmar={
          dialogo === "restaurar" ? t("serumsIa.fuentes.acciones.restaurar") : t("serumsIa.fuentes.acciones.reintentar")
        }
        textoCancelar={t("comun-cancelar")}
        textoPendiente={t("comun-procesando")}
        etiquetaCerrar={t("comun-cerrar")}
        destructiva={false}
        onConfirmar={async () => {
          try {
            const destino = dialogo === "restaurar" ? "activa" : "procesando";
            const r = await fuentesApi.cambiarEstado(token, fuente.id, { estado: destino });
            carga.reemplazar(r);
            toast.ok(destino === "activa" ? t("serumsIa.fuentes.restaurar.ok") : t("serumsIa.fuentes.reintentar.ok"));
            return null;
          } catch (e) {
            return mensajeErrorFuente(e, t, t("comun-error-estado"));
          }
        }}
      />
    </div>
  );
}
