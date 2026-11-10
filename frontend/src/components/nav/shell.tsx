import type { ReactNode } from "react";

/**
 * Esqueleto de la app privada: sidebar flotante de 248px (md+) + columna con top bar y main.
 * `sidebar` es el `SidebarContent`; `topBar` la `TopBar` (que a su vez recibe `menuMovil` con el mismo sidebar).
 */
export function Shell({ sidebar, topBar, children }: { sidebar: ReactNode; topBar: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-h-screen">
      {/* Sidebar flotante: tarjeta de 248px con 12px de margen sobre el lienzo. */}
      <aside className="hidden w-[260px] shrink-0 py-3 pl-3 md:sticky md:top-0 md:block md:h-screen">
        <div className="border-borde-tarjeta bg-sidebar text-sidebar-foreground shadow-tarjeta h-full overflow-x-hidden overflow-y-auto rounded-xl border">
          {sidebar}
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        {topBar}
        <main className="min-w-0 flex-1 overflow-x-auto p-3">{children}</main>
      </div>
    </div>
  );
}

/** Contenedor de página: bloques separados por gap-3 y ancho máximo 1520px. */
export function Pagina({ children }: { children: ReactNode }) {
  return <div className="mx-auto grid w-full max-w-[1520px] min-w-0 grid-cols-1 gap-3">{children}</div>;
}
