"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { AcercaDe } from "@/components/app/acerca-de";
import { MarcaSidebar } from "@/components/app/marca";
import { gruposDelSidebar, ICONO_PERFIL } from "@/components/app/navegacion";
import { useTextosMenuUsuario, useTextosNav } from "@/components/app/use-textos-khipu";
import { MenuUsuario } from "@/components/nav/menu-usuario";
import { SidebarContent } from "@/components/nav/sidebar";
import { useAuth } from "@/lib/auth/auth-context";
import { InfoIcon } from "@phosphor-icons/react";

/** Sidebar de la app: grupos de NAVEGACION traducidos y filtrados por rol, y el menú de usuario al pie. */
export function SidebarApp() {
  const t = useTranslations("dashboard");
  const textos = useTextosMenuUsuario();
  const textosNav = useTextosNav();
  const { user, logout } = useAuth();
  const router = useRouter();
  const [acercaDe, setAcercaDe] = useState(false);
  const esAdmin = user?.rol === "admin";

  const grupos = gruposDelSidebar(esAdmin, t);

  const nombre = [user?.nombres, user?.apellido_paterno].filter(Boolean).join(" ") || undefined;

  return (
    <SidebarContent
      marca={<MarcaSidebar />}
      grupos={grupos}
      textosNav={textosNav}
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
              textos={textos}
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
