"use client";

import { useTranslations } from "next-intl";
import type { ComponentProps } from "react";

import { DialogContent as DialogContentKhipu } from "@/components/ui/dialog";
import { SheetContent as SheetContentKhipu } from "@/components/ui/sheet";

/** `DialogContent` de khipu con el nombre accesible del botón de cerrar traducido. */
export function DialogContent(props: ComponentProps<typeof DialogContentKhipu>) {
  const t = useTranslations("dashboard");
  return <DialogContentKhipu etiquetaCerrar={t("comun-cerrar")} {...props} />;
}

/** `SheetContent` de khipu con el nombre accesible del botón de cerrar traducido. */
export function SheetContent(props: ComponentProps<typeof SheetContentKhipu>) {
  const t = useTranslations("dashboard");
  return <SheetContentKhipu etiquetaCerrar={t("comun-cerrar")} {...props} />;
}
