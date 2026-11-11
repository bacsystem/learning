"use client";

import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Suspense, useEffect } from "react";

import Loading from "@/app/loading";
import { BarraSuperior } from "@/components/app/barra-superior";
import { SidebarApp } from "@/components/app/sidebar-app";
import { TourApp } from "@/components/app/tour-app";
import { Alerta } from "@/components/feedback/alerta";
import { Shell } from "@/components/nav/shell";
import { DEFAULTS } from "@/config";
import { useAuth } from "@/lib/auth/auth-context";

export default function DashboardLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const { token, loading, user } = useAuth();
  const t = useTranslations("dashboard");
  const router = useRouter();
  const pathname = usePathname();

  // Sesión con token pero perfil aún sin cargar (p. ej. `/auth/me` falló por red o 5xx; el contexto lo reintenta):
  // no se redirige a ningún sitio, se espera con el indicador de carga.
  const perfilPendiente = !!token && !user;

  useEffect(() => {
    if (loading || perfilPendiente) return;
    if (!token) {
      router.replace("/auth/sign-in");
      return;
    }
    // Las rutas /admin/* son solo para administradores.
    if (pathname.startsWith("/admin") && user?.rol !== "admin") {
      router.replace(DEFAULTS.appRoot);
    }
  }, [loading, perfilPendiente, token, user, pathname, router]);

  if (loading) return <Loading />;
  // La sesión se restauró pero `/auth/me` falla (p. ej. 503 con la BD caída): la sesión se conserva y se avisa en vez
  // de dejar solo el indicador de carga. El contexto reintenta cada 5 s y, al responder, se muestra la app.
  if (perfilPendiente) {
    return (
      <div className="grid min-h-dvh place-items-center p-4">
        <Alerta tono="aviso" titulo={t("sesion-servidor-no-disponible-titulo")} className="max-w-md">
          {t("sesion-servidor-no-disponible-texto")}
        </Alerta>
      </div>
    );
  }
  if (!token) return null;
  // Mientras el efecto redirige, no se montan las páginas /admin/* para un no-admin (evita sus peticiones 403).
  if (pathname.startsWith("/admin") && user?.rol !== "admin") return <Loading />;

  return (
    <Shell sidebar={<SidebarApp />} topBar={<BarraSuperior />}>
      <Suspense fallback={<Loading />}>{children}</Suspense>
      <TourApp />
    </Shell>
  );
}
