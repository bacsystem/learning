"use client";

import pkg from "../../../package.json";
import { useTranslations } from "next-intl";
import type { ReactNode } from "react";

import { DialogContent } from "@/components/app/dialogo";
import { Marca } from "@/components/app/marca";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";

const CONTACTO = "bacsystem.sac@gmail.com";

/** Diálogo «Acerca de»: versión de la app y datos de soporte (antes en el pie del dashboard de la plantilla). */
export function AcercaDe({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const t = useTranslations("dashboard");

  return (
    <Dialog open={open} onOpenChange={onOpenChange} disablePointerDismissal={false}>
      <DialogContent className="gap-0 p-0 sm:max-w-sm">
        <div className="flex flex-col items-center gap-2 px-6 pt-8 pb-5 text-center">
          <Marca tamano="lg" />
          <DialogTitle className="sr-only">{t("acerca-de")}</DialogTitle>
          <Badge variant="outline">v{pkg.version}</Badge>
          <DialogDescription className="text-muted-foreground max-w-60">{t("acerca-de-descripcion")}</DialogDescription>
        </div>
        <Separator />
        <dl className="grid gap-3 px-6 py-5">
          <Fila etiqueta={t("acerca-de-desarrollador")}>Faustina Del Rosario</Fila>
          <Fila etiqueta={t("acerca-de-empresa")}>Bacsystem Solutions EIRL</Fila>
          <Fila etiqueta={t("acerca-de-celular")}>906 450 533</Fila>
          <Fila etiqueta={t("acerca-de-contacto")}>
            <a href={`mailto:${CONTACTO}`} className="text-primary hover:underline">
              {CONTACTO}
            </a>
          </Fila>
          <Fila etiqueta={t("acerca-de-stack")}>Next.js · Go · PostgreSQL</Fila>
        </dl>
        <Separator />
        <p className="text-etiqueta text-texto-terciario px-6 py-4 text-center">
          © {new Date().getFullYear()} Capacitate — {t("acerca-de-derechos")}
        </p>
      </DialogContent>
    </Dialog>
  );
}

function Fila({ etiqueta, children }: { etiqueta: string; children: ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <dt className="text-etiqueta text-muted-foreground">{etiqueta}</dt>
      <dd className="text-control text-foreground text-right">{children}</dd>
    </div>
  );
}
