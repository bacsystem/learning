"use client";

import { useFormatter, useTranslations } from "next-intl";

import { Alerta } from "@/components/feedback/alerta";
import { ProgresoLineal } from "@/components/feedback/progreso";
import { Metrica } from "@/components/patrones/metricas";
import { TARJETA } from "@/lib/estilos";
import type { ConsumoIa } from "@/lib/serums-ia/ia/api";
import { porcentajeConsumo } from "@/lib/serums-ia/ia/reglas";
import { cn } from "@/lib/utils";
import { UserCircleIcon } from "@phosphor-icons/react";

/** Consumo del día de quien consulta (regla 9 de E6-02): para un admin, solo la ventana diaria; `mes` llega `null`. */
export function TarjetaConsumo({ consumo }: { consumo: ConsumoIa }) {
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const { dia } = consumo;
  const reinicia = formato.dateTime(new Date(dia.reinicia_at), {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "America/Lima",
  });
  const porcentaje = porcentajeConsumo(dia);

  return (
    <section className={cn(TARJETA, "flex flex-col gap-4 p-4")} aria-labelledby="ia-consumo">
      <div>
        <h2 id="ia-consumo" className="text-titulo text-foreground">
          {t("serumsIa.ia.consumo.titulo")}
        </h2>
        <p className="text-control text-muted-foreground">{t("serumsIa.ia.consumo.ayuda")}</p>
      </div>
      <Metrica
        etiqueta={t("serumsIa.ia.consumo.usadas")}
        icono={UserCircleIcon}
        tono={dia.disponibles === 0 ? "error" : "neutro"}
        ayuda={t("serumsIa.ia.consumo.reinicia", { fecha: reinicia })}
        sufijo={t("serumsIa.ia.consumo.disponibles", { disponibles: dia.disponibles })}
      >
        {t("serumsIa.ia.consumo.usados", { usados: dia.usados, limite: dia.limite })}
      </Metrica>
      <ProgresoLineal
        valor={porcentaje}
        tono={dia.disponibles === 0 ? "destructive" : "primary"}
        etiqueta={t("serumsIa.ia.consumo.avance")}
      />
      {consumo.presupuesto_agotado ? <Alerta tono="error">{t("serumsIa.ia.consumo.agotado")}</Alerta> : null}
    </section>
  );
}
