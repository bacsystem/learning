import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Marca } from "@/components/app/marca";
import { DEFAULTS } from "@/config";
import { ACCION_PRINCIPAL, TARJETA } from "@/lib/estilos";
import { HouseIcon } from "@phosphor-icons/react/dist/ssr";

export default async function NotFound() {
  const t = await getTranslations("dashboard");
  return (
    <main className="bg-background flex min-h-dvh items-center justify-center p-4">
      <div className={`${TARJETA} flex w-full max-w-md flex-col items-center gap-5 px-8 py-12 text-center`}>
        <Marca />
        <div className="flex flex-col gap-1.5">
          <h1 className="text-titulo-pagina text-foreground">{t("not-found-titulo")}</h1>
          <p className="text-cuerpo text-muted-foreground">{t("not-found-detalle")}</p>
        </div>
        <Link href={DEFAULTS.appRoot} className={ACCION_PRINCIPAL}>
          <HouseIcon className="size-4" />
          {t("auth-volver-inicio")}
        </Link>
      </div>
    </main>
  );
}
