import type { ReactNode } from "react";

import { TarjetaAuth } from "@/app/auth/tarjeta-auth";

/** Página legal pública (términos, privacidad): tarjeta de lectura con título, introducción y secciones numeradas. */
export function DocumentoLegal({
  titulo,
  intro,
  children,
}: {
  titulo: ReactNode;
  intro: ReactNode;
  children: ReactNode;
}) {
  return (
    <TarjetaAuth ancho="documento" titulo={titulo} descripcion={intro}>
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
