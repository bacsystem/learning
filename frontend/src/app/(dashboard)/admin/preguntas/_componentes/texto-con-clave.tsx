"use client";

import { useTranslations } from "next-intl";

import { nodosConClave } from "@/lib/serums-ia/revision/marcas";

/** Texto de un pasaje con la clave oficial (`<mark>` bien emparejado) resaltada; el resto del HTML queda como texto. */
export function TextoConClave({ texto }: { texto: string }) {
  const t = useTranslations("dashboard");
  return <>{nodosConClave(texto, t("serumsIa.revision.citas.claveOficial"))}</>;
}
