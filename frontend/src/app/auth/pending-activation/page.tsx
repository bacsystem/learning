import Link from "next/link";
import { getTranslations } from "next-intl/server";
import React from "react";

import { TarjetaAuth } from "@/app/auth/tarjeta-auth";
import { BOTON_SECUNDARIO } from "@/lib/estilos";
import { ArrowLeftIcon, HourglassMediumIcon } from "@phosphor-icons/react/dist/ssr";

export default async function Page() {
  const t = await getTranslations("dashboard");
  return (
    <TarjetaAuth centrado>
      <HourglassMediumIcon weight="duotone" aria-hidden className="text-warning-foreground size-10" />
      <header className="flex flex-col gap-2">
        <h1 className="text-titulo-pagina text-foreground text-balance">{t("pending-activation-titulo")}</h1>
        <p className="text-cuerpo text-foreground leading-relaxed">{t("pending-activation-detalle")}</p>
        <p className="text-control text-muted-foreground">{t("pending-activation-consulta")}</p>
      </header>
      <Link href="/auth/sign-in" className={BOTON_SECUNDARIO}>
        <ArrowLeftIcon weight="bold" className="size-4" />
        {t("auth-volver-sign-in")}
      </Link>
    </TarjetaAuth>
  );
}
