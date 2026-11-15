"use client";

import { usePathname } from "next/navigation";
import { type ReactNode, useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { OBJETIVO_TACTIL } from "@/lib/estilos";
import { ListIcon } from "@phosphor-icons/react";

/**
 * Hamburguesa de la top bar que abre el mismo `SidebarContent` en un cajón lateral; se cierra al navegar. Es una columna de la altura del cajón: solo la navegación se desplaza (`SidebarContent`), así que en una pantalla baja el usuario del pie
 * (tema, apariencia, cerrar sesión) sigue a la vista. `etiqueta` es el nombre accesible del botón.
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
        {children}
      </SheetContent>
    </Sheet>
  );
}
