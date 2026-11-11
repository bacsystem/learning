"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

import { Alerta } from "@/components/feedback/alerta";
import { Casilla } from "@/components/formularios/casilla";
import { PillEstado } from "@/components/patrones/pill-estado";
import { Badge } from "@/components/ui/badge";
import { AYUDA_CAMPO, ETIQUETA_CAMPO } from "@/lib/estilos";
import type { EstadoFuente } from "@/lib/serums-ia/fuentes/api";
import type { FuenteDuplicada } from "@/lib/serums-ia/fuentes/errores";
import { SealCheckIcon } from "@phosphor-icons/react";

const TONO_ESTADO: Record<EstadoFuente, "ok" | "aviso" | "error" | "neutro"> = {
  activa: "ok",
  procesando: "aviso",
  fallida: "error",
  retirada: "neutro",
};

/** Estado de la fuente traducido (`dashboard.serumsIa.fuentes.estado.*`). */
export function PillEstadoFuente({ estado }: { estado: EstadoFuente }) {
  const t = useTranslations("dashboard");
  return <PillEstado tono={TONO_ESTADO[estado]}>{t(`serumsIa.fuentes.estado.${estado}`)}</PillEstado>;
}

/** Insignia «Oficial verificada» de la versión vigente. */
export function InsigniaOficial() {
  const t = useTranslations("dashboard");
  return (
    <Badge variant="secondary">
      <SealCheckIcon weight="fill" data-icon="inline-start" />
      {t("serumsIa.fuentes.oficial.insignia")}
    </Badge>
  );
}

/** Profesiones de la fuente en una celda o una ficha; sin ninguna, aplica a todas. */
export function TextoProfesiones({
  aplicaATodas,
  especialidades,
}: {
  aplicaATodas: boolean;
  especialidades: { nombre: string }[];
}) {
  const t = useTranslations("dashboard");
  return <>{aplicaATodas ? t("serumsIa.fuentes.todasProfesiones") : especialidades.map((e) => e.nombre).join(", ")}</>;
}

export interface OpcionProfesion {
  id: number;
  nombre: string;
  /** Asociada a la fuente pero ya no activa: se muestra para poder quitarla. */
  inactiva?: boolean;
}

/** Grupo de casillas de profesiones (0..N; ninguna = todas). */
export function CasillasProfesiones({
  id,
  opciones,
  valor,
  onCambio,
  error,
}: {
  id: string;
  opciones: OpcionProfesion[];
  valor: number[];
  onCambio: (ids: number[]) => void;
  error?: string;
}) {
  const t = useTranslations("dashboard");
  return (
    <fieldset className="grid gap-1.5 md:col-span-2" aria-describedby={`${id}-ayuda`}>
      <legend className={ETIQUETA_CAMPO}>{t("serumsIa.fuentes.campo.profesiones")}</legend>
      {error ? (
        <p id={`${id}-ayuda`} role="alert" className="text-control text-destructive">
          {error}
        </p>
      ) : (
        <p id={`${id}-ayuda`} className={AYUDA_CAMPO}>
          {t("serumsIa.fuentes.campo.profesionesAyuda")}
        </p>
      )}
      <div className="border-border grid max-h-48 gap-2 overflow-y-auto rounded-lg border p-3 sm:grid-cols-2">
        {opciones.map((o) => (
          <Casilla
            key={o.id}
            id={`${id}-${o.id}`}
            etiqueta={o.inactiva ? t("serumsIa.fuentes.campo.profesionInactiva", { nombre: o.nombre }) : o.nombre}
            checked={valor.includes(o.id)}
            onCheckedChange={(v) => onCambio(v ? [...valor, o.id] : valor.filter((x) => x !== o.id))}
          />
        ))}
      </div>
    </fieldset>
  );
}

/** 409 `fuente_duplicada`: la fuente existente con su estado y un enlace a su detalle (E2-07 regla 8). */
export function AvisoDuplicado({ duplicada }: { duplicada: FuenteDuplicada }) {
  const t = useTranslations("dashboard");
  const estado = duplicada.estado as EstadoFuente;
  return (
    <Alerta tono="aviso" titulo={t("serumsIa.fuentes.duplicada.titulo")}>
      <span className="flex flex-wrap items-center gap-2">
        <span>{t("serumsIa.fuentes.duplicada.texto", { titulo: duplicada.titulo })}</span>
        {estado in TONO_ESTADO ? <PillEstadoFuente estado={estado} /> : null}
        <Link
          href={`/admin/fuentes/${duplicada.fuenteId}`}
          className="text-acento-texto font-medium underline-offset-4 hover:underline"
        >
          {t("serumsIa.fuentes.duplicada.ver")}
        </Link>
      </span>
    </Alerta>
  );
}
