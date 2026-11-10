import type { CnFunction } from "cn";
import { createCn } from "cn/config";

/**
 * Une clases y resuelve conflictos de Tailwind. Se configura con la escala tipográfica con nombre de src/styles/theme.css
 * (`text-etiqueta`, `text-control`, `text-campo`, `text-cuerpo`, `text-titulo`, `text-titulo-pagina`, `text-metrica`, `text-metrica-texto`): sin esto `cn`
 * las toma por colores de texto y borra el tamaño o el color al combinarlas (`cn("text-etiqueta text-muted-foreground")`
 * perdía `text-etiqueta`).
 * Todo componente importa `cn` desde aquí, nunca desde el paquete "cn" directo.
 */
export const cn: CnFunction = createCn({
  extend: {
    classGroups: {
      "font-size": [
        { text: ["etiqueta", "control", "campo", "cuerpo", "titulo", "titulo-pagina", "metrica", "metrica-texto"] },
      ],
    },
  },
});
