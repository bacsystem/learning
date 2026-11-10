"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { AcercaDe } from "@/components/app/acerca-de";
import { MarcaSidebar } from "@/components/app/marca";
import { ICONO_PERFIL, NAVEGACION } from "@/components/app/navegacion";
import { MenuUsuario } from "@/components/nav/menu-usuario";
import { SidebarContent } from "@/components/nav/sidebar";
import type { GrupoNav } from "@/components/nav/sidebar-nav";
import { useAuth } from "@/lib/auth/auth-context";
import { InfoIcon } from "@phosphor-icons/react";

/** Sidebar de la app: grupos de NAVEGACION traducidos y filtrados por rol, y el menú de usuario al pie. */
export function SidebarApp() {
  const t = useTranslations("dashboard");
  const { user, logout } = useAuth();
  const router = useRouter();
  const [acercaDe, setAcercaDe] = useState(false);
  const esAdmin = user?.rol === "admin";

  const grupos: GrupoNav[] = NAVEGACION.filter((g) => !g.soloAdmin || esAdmin).map((g) => ({
    titulo: t(g.titulo),
    items: g.items.map((i) => ({ href: i.href, label: t(i.etiqueta), icon: i.icono, exacto: i.exacto })),
  }));

  const nombre = [user?.nombres, user?.apellido_paterno].filter(Boolean).join(" ") || undefined;

  return (
    <SidebarContent
      marca={<MarcaSidebar />}
      grupos={grupos}
      usuario={
        user ? (
          // `data-tour="settings"`: paso «Tu perfil y estadísticas» del tour (antes `id="settings"` del menú de la plantilla).
          <div data-tour="settings">
            <MenuUsuario
              usuario={{ nombre, email: user.username, rol: user.grado || user.rol }}
              acciones={[
                { label: t("user-profile"), icon: ICONO_PERFIL, href: "/settings" },
                { label: t("acerca-de"), icon: InfoIcon, onClick: () => setAcercaDe(true) },
              ]}
              textoCerrarSesion={t("user-sign-out")}
              mostrarApariencia
              onCerrarSesion={async () => {
                await logout();
                router.replace("/auth/sign-in");
              }}
            />
            <AcercaDe open={acercaDe} onOpenChange={setAcercaDe} />
          </div>
        ) : null
      }
    />
  );
}
