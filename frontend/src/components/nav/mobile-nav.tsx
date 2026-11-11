"use client";

import { usePathname } from "next/navigation";
import { type ReactNode, useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { OBJETIVO_TACTIL } from "@/lib/estilos";
import { ListIcon } from "@phosphor-icons/react";

/**
 * Hamburguesa de la top bar que abre el mismo `SidebarContent` en un cajón lateral; se cierra al navegar. El cajón se desplaza: en una pantalla baja el usuario del pie
 * (tema, apariencia, cerrar sesión) quedaba fuera. `etiqueta` es el nombre accesible del botón.
 */
export function MobileNav({ children, etiqueta = "Abrir menú" }: { children: ReactNode; etiqueta?: string }) {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();

  useEffect(() => {
    setOpen(false);
  }, [pathname]);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger render={<Button variant="ghost" size="icon-sm" className={OBJETIVO_TACTIL} />}>
        <ListIcon weight="bold" className="size-5" />
        <span className="sr-only">{etiqueta}</span>
      </SheetTrigger>
      <SheetContent
        side="left"
        showCloseButton={false}
        className="bg-sidebar text-sidebar-foreground w-[248px] max-w-[85vw] p-0"
      >
        {/* `SidebarContent` es `h-full overflow-hidden`: se le da un contenedor que se desplaza y se lo deja crecer al contenido. */}
        <div className="h-full overflow-y-auto [&>div]:h-auto [&>div]:min-h-full [&>div]:overflow-visible">
          {children}
        </div>
      </SheetContent>
    </Sheet>
  );
}
