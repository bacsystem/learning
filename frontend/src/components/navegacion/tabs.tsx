"use client";

import type { ReactNode } from "react";

import { SEGMENTADO, SEGMENTO_BASE } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";

export type TabItem<T extends string> = { id: T; etiqueta: ReactNode; contador?: number; disabled?: boolean };

// h-10 fijo (no padding vertical) para que el contador no altere la altura de la fila.
const PESTANA_LINEA =
  "inline-flex h-10 items-center gap-1.5 px-1 text-control font-medium whitespace-nowrap text-muted-foreground transition-colors duration-(--duracion-rapida) ease-salida outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 data-active:text-foreground";

// base-ui posiciona el indicador con --active-tab-left/top/width/height; la transición CSS lo desliza entre pestañas.
// `-z-10` dentro de una lista `isolate`: queda sobre el fondo de la lista y debajo del texto de las pestañas.
const INDICADOR_BASE =
  "absolute -z-10 left-(--active-tab-left) w-(--active-tab-width) transition-[left,width,top,height] duration-(--duracion-media) ease-salida motion-reduce:transition-none";
// La fila subrayada se desplaza a lo ancho cuando las pestañas no entran (en un teléfono, cinco pestañas miden más que la pantalla y empujaban la página entera),
// sin barra de desplazamiento a la vista. La línea de base es una sombra interior y no un `border-b`, y el indicador va en `bottom-0`: con `overflow-x-auto` el
// navegador recorta también en vertical, y un indicador en `-bottom-px` sobre el borde quedaba cortado.
const PESTANAS_LINEA =
  "flex items-center gap-4 overflow-x-auto [scrollbar-width:none] shadow-[inset_0_-1px_0_var(--color-border)] [&::-webkit-scrollbar]:hidden";

const INDICADOR = {
  linea: cn(INDICADOR_BASE, "bottom-0 h-0.5 rounded-full bg-foreground"),
  segmentado: cn(
    INDICADOR_BASE,
    "top-(--active-tab-top) h-(--active-tab-height) rounded-full bg-card shadow-sm ring-1 ring-border dark:bg-input",
  ),
};

/**
 * Pestañas de página. `estilo="segmentado"` es el control en caja de las barras de filtro; `"linea"` es la pestaña subrayada para secciones de detalle.
 * Los paneles se pasan como `render(id)` (`children`) o, desde un Server Component —que no puede pasarle una función a un componente de cliente—, ya
 * armados en `paneles`, uno por id. En los dos casos solo se monta el panel activo.
 */
export function Tabs<T extends string>({
  items,
  valor,
  defaultValor,
  onCambio,
  estilo = "linea",
  children,
  paneles,
  className,
}: {
  items: TabItem<T>[];
  valor?: T;
  defaultValor?: T;
  onCambio?: (v: T) => void;
  estilo?: "linea" | "segmentado";
  children?: (id: T) => ReactNode;
  paneles?: Partial<Record<T, ReactNode>>;
  className?: string;
}) {
  return (
    <TabsPrimitive.Root
      value={valor}
      defaultValue={defaultValor ?? items[0]?.id}
      onValueChange={(v) => onCambio?.(v as T)}
      className={className}
    >
      <TabsPrimitive.List className={cn("relative isolate", estilo === "linea" ? PESTANAS_LINEA : SEGMENTADO)}>
        {items.map((t) => (
          <TabsPrimitive.Tab
            key={t.id}
            value={t.id}
            disabled={t.disabled}
            className={estilo === "linea" ? PESTANA_LINEA : SEGMENTO_BASE}
          >
            {t.etiqueta}
            {t.contador != null ? (
              <span className="bg-secondary text-etiqueta text-muted-foreground in-data-active:bg-acento in-data-active:text-acento-texto rounded-full px-1.5 py-px font-semibold tabular-nums">
                {t.contador}
              </span>
            ) : null}
          </TabsPrimitive.Tab>
        ))}
        <TabsPrimitive.Indicator className={INDICADOR[estilo]} />
      </TabsPrimitive.List>
      {items.map((t) => (
        <TabsPrimitive.Panel key={t.id} value={t.id} className="pt-4 outline-none">
          {children ? children(t.id) : paneles?.[t.id]}
        </TabsPrimitive.Panel>
      ))}
    </TabsPrimitive.Root>
  );
}
