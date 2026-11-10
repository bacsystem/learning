"use client";

import { usePathname, useRouter } from "next/navigation";
import { Suspense, useEffect } from "react";

import Loading from "@/app/loading";
import { BarraSuperior } from "@/components/app/barra-superior";
import { SidebarApp } from "@/components/app/sidebar-app";
import { TourApp } from "@/components/app/tour-app";
import { Shell } from "@/components/nav/shell";
import { DEFAULTS } from "@/config";
import { useAuth } from "@/lib/auth/auth-context";

export default function DashboardLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const { token, loading, user } = useAuth();
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

  if (loading || perfilPendiente) return <Loading />;
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
