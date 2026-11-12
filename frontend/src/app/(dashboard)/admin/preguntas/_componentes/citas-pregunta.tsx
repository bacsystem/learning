"use client";

import { TextoConClave } from "./texto-con-clave";
import { useTranslations } from "next-intl";

import { Acordeon } from "@/components/navegacion/acordeon";
import { PillEstado } from "@/components/patrones/pill-estado";
import { ACCION_SECUNDARIA, TARJETA } from "@/lib/estilos";
import type { CitaPregunta } from "@/lib/questions/api";
import { fuenteRetirada, MAX_CITAS } from "@/lib/serums-ia/revision/reglas";
import { CheckCircleIcon, TrashIcon, WarningIcon } from "@phosphor-icons/react";

/** Estado de la fuente de una cita: nunca solo color (icono + texto). */
function EstadoFuenteCita({ cita }: { cita: CitaPregunta }) {
  const t = useTranslations("dashboard");
  const retirada = fuenteRetirada(cita);
  return (
    <PillEstado tono={retirada ? "aviso" : "ok"} icon={retirada ? WarningIcon : CheckCircleIcon}>
      {t(retirada ? "serumsIa.revision.citas.fuenteRetirada" : "serumsIa.revision.citas.fuenteActiva")}
    </PillEstado>
  );
}

function ItemCita({
  cita,
  deshabilitada,
  onQuitar,
}: {
  cita: CitaPregunta;
  deshabilitada: boolean;
  onQuitar: (pasajeId: number) => void;
}) {
  const t = useTranslations("dashboard");
  const ubicacion = [
    t("serumsIa.revision.citas.version", { n: cita.version }),
    cita.seccion ? t("serumsIa.revision.citas.seccion", { seccion: cita.seccion }) : null,
    t("serumsIa.revision.citas.lineas", { inicio: cita.linea_inicio, fin: cita.linea_fin }),
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <li className={`${TARJETA} grid gap-2 p-4`}>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-cuerpo text-foreground font-semibold">{cita.fuente_titulo}</p>
          <p className="text-etiqueta text-muted-foreground">{ubicacion}</p>
        </div>
        <div className="flex items-center gap-2">
          <EstadoFuenteCita cita={cita} />
          <button
            type="button"
            className={ACCION_SECUNDARIA}
            disabled={deshabilitada}
            aria-label={t("serumsIa.revision.citas.quitarDe", { fuente: cita.fuente_titulo })}
            onClick={() => onQuitar(cita.pasaje_id)}
          >
            <TrashIcon className="size-4" />
            {t("serumsIa.revision.citas.quitar")}
          </button>
        </div>
      </div>
      <p className="text-control text-muted-foreground">
        <TextoConClave texto={cita.extracto} />
      </p>
      <Acordeon
        items={[
          {
            id: String(cita.pasaje_id),
            titulo: t("serumsIa.revision.citas.verTexto"),
            // React escapa el texto y `whitespace-pre-wrap` conserva los saltos de línea; solo `<mark>` se resalta.
            contenido: (
              <p className="text-foreground whitespace-pre-wrap">
                <TextoConClave texto={cita.texto} />
              </p>
            ),
          },
        ]}
      />
    </li>
  );
}

/** Citas actuales de la pregunta, con la opción de quitarlas y el texto completo de cada pasaje. */
export function CitasPregunta({
  citas,
  ocupado,
  onQuitar,
}: {
  citas: CitaPregunta[];
  ocupado: boolean;
  onQuitar: (pasajeId: number) => void;
}) {
  const t = useTranslations("dashboard");
  return (
    <section aria-labelledby="citas-titulo" className="grid gap-2">
      <div className="flex items-baseline justify-between gap-2">
        <h2 id="citas-titulo" className="text-titulo text-foreground">
          {t("serumsIa.revision.citas.titulo")}
        </h2>
        <span className="text-etiqueta text-muted-foreground">
          {t("serumsIa.revision.citas.contador", { n: citas.length, maximo: MAX_CITAS })}
        </span>
      </div>
      {citas.length === 0 ? (
        <p className="text-control text-muted-foreground">{t("serumsIa.revision.citas.vacio")}</p>
      ) : (
        <ul className="grid gap-2">
          {citas.map((c) => (
            <ItemCita key={c.pasaje_id} cita={c} deshabilitada={ocupado} onQuitar={onQuitar} />
          ))}
        </ul>
      )}
    </section>
  );
}
