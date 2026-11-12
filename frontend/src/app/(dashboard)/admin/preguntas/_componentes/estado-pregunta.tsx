"use client";

import { useTranslations } from "next-intl";

import type { Tono } from "@/components/patrones/metricas";
import { PillEstado } from "@/components/patrones/pill-estado";
import type { IconoComponente } from "@/lib/iconos";
import type { EstadoPregunta } from "@/lib/questions/api";
import { claveEstadoPregunta } from "@/lib/questions/estados";
import { CheckCircleIcon, EyeIcon, NotePencilIcon, SealCheckIcon, TrashIcon, XCircleIcon } from "@phosphor-icons/react";

/** Tono e icono de cada estado (SIA-E3-02): el estado nunca se comunica solo con el color. */
const ASPECTO_ESTADO: Record<EstadoPregunta, { tono: Tono; icono: IconoComponente }> = {
  borrador: { tono: "neutro", icono: NotePencilIcon },
  en_revision: { tono: "aviso", icono: EyeIcon },
  aprobada: { tono: "ok", icono: CheckCircleIcon },
  activo: { tono: "ok", icono: SealCheckIcon },
  rechazada: { tono: "error", icono: XCircleIcon },
  eliminada: { tono: "neutro", icono: TrashIcon },
};

/** Estado traducido de una pregunta, con icono y color. */
export function PillEstadoPregunta({ estado }: { estado: EstadoPregunta }) {
  const t = useTranslations("dashboard");
  const aspecto = ASPECTO_ESTADO[estado];
  return (
    <PillEstado tono={aspecto.tono} icon={aspecto.icono}>
      {t(claveEstadoPregunta(estado))}
    </PillEstado>
  );
}
