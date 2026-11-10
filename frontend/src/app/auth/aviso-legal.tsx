import Link from "next/link";
import { useTranslations } from "next-intl";

import { ENLACE } from "@/app/auth/tarjeta-auth";

/** Línea «Al continuar aceptas…» con enlaces a los términos y a la política de privacidad. */
export function AvisoLegal() {
  const t = useTranslations("dashboard");
  return (
    <p className="text-etiqueta text-muted-foreground text-center leading-relaxed">
      {t.rich("auth-acepta-terminos", {
        terminos: (texto) => (
          <Link href="/auth/terms-and-conditions" className={ENLACE}>
            {texto}
          </Link>
        ),
        privacidad: (texto) => (
          <Link href="/auth/privacy-policy" className={ENLACE}>
            {texto}
          </Link>
        ),
      })}
    </p>
  );
}
