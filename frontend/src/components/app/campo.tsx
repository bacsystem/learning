"use client";

import { useTranslations } from "next-intl";
import type { ComponentProps } from "react";

import { Campo as CampoKhipu } from "@/components/formularios/campo";

/** `Campo` de khipu con el «(opcional)» traducido. */
export function Campo(props: ComponentProps<typeof CampoKhipu>) {
  const t = useTranslations("dashboard");
  return <CampoKhipu textoOpcional={t("comun-opcional")} {...props} />;
}
