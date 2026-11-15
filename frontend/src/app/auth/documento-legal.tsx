import { useLocale, useTranslations } from "next-intl";
import type { ReactNode } from "react";

import { avisarSoloEspanol } from "@/app/auth/documento-legal-idioma";
import { TarjetaAuth } from "@/app/auth/tarjeta-auth";
import { Alerta } from "@/components/feedback/alerta";

/**
 * Página legal pública (términos, privacidad): tarjeta de lectura con título, aviso de idioma, introducción y secciones
 * numeradas. Con la interfaz en otro idioma, el aviso va antes de la introducción y dice que el documento solo está en español.
 */
export function DocumentoLegal({
  titulo,
  intro,
  children,
}: {
  titulo: ReactNode;
  intro: ReactNode;
  children: ReactNode;
}) {
  const t = useTranslations("dashboard");
  const locale = useLocale();
  return (
    <TarjetaAuth ancho="documento" titulo={titulo}>
      {avisarSoloEspanol(locale) ? <Alerta tono="info">{t("auth-documento-solo-espanol")}</Alerta> : null}
      <p className="text-cuerpo text-muted-foreground leading-relaxed">{intro}</p>
      <div className="border-border/60 flex flex-col gap-6 border-t pt-6">{children}</div>
    </TarjetaAuth>
  );
}

/** Sección de un documento legal: título y cuerpo con párrafos y listas con viñeta. */
export function SeccionLegal({ titulo, children }: { titulo: ReactNode; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-titulo text-foreground">{titulo}</h2>
      <div className="text-cuerpo text-muted-foreground [&_ul]:marker:text-texto-terciario flex flex-col gap-2 leading-relaxed [&_li]:ms-5 [&_li]:ps-1 [&_ul]:flex [&_ul]:list-disc [&_ul]:flex-col [&_ul]:gap-1">
        {children}
      </div>
    </section>
  );
}
