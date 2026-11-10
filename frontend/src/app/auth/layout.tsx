import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";

import Loading from "@/app/loading";
import { Marca } from "@/components/app/marca";
import { ThemeToggle } from "@/components/nav/theme-toggle";
import { FOCO } from "@/lib/estilos";
import { cn } from "@/lib/utils";

/** Marco de las páginas públicas de auth: lienzo, marca arriba (vuelve al inicio), selector de tema y pie legal. */
export default async function AuthLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const t = await getTranslations("dashboard");
  return (
    <div className="bg-background flex min-h-dvh flex-col">
      <header className="flex items-center justify-between gap-3 px-4 py-4 md:px-8">
        <Link
          href="/"
          aria-label={t("auth-volver-inicio")}
          title={t("auth-volver-inicio")}
          className={cn("rounded-lg transition-opacity hover:opacity-80", FOCO)}
        >
          <Marca />
        </Link>
        <ThemeToggle />
      </header>

      <main className="flex flex-1 items-start justify-center px-4 pt-4 pb-10 sm:items-center sm:pt-0">
        <Suspense fallback={<Loading />}>{children}</Suspense>
      </main>

      <footer className="text-etiqueta text-texto-terciario flex flex-wrap items-center justify-center gap-x-4 gap-y-1 px-4 pb-6">
        <span>© {new Date().getFullYear()} Capacitate</span>
        <Link href="/auth/terms-and-conditions" className="hover:text-foreground rounded-sm">
          {t("auth-terminos")}
        </Link>
        <Link href="/auth/privacy-policy" className="hover:text-foreground rounded-sm">
          {t("auth-privacidad")}
        </Link>
      </footer>
    </div>
  );
}
