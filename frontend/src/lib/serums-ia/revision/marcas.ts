import { createElement, Fragment, type ReactNode } from "react";

/**
 * La clave oficial de un balotario viene marcada con `<mark>…</mark>` en el texto del pasaje (docs/api/…balotario.md §2.3).
 * Solo esa etiqueta, bien emparejada, se trata como resaltado; cualquier otro HTML sigue siendo texto plano y nada
 * se interpreta como HTML (se renderiza con React, que lo escapa).
 */
export interface SegmentoTexto {
  texto: string;
  /** Va entre `<mark>` y `</mark>` bien emparejados. */
  clave: boolean;
}

const ETIQUETAS = /(<\/?mark>)/gi;
const esApertura = (parte: string) => parte.toLowerCase() === "<mark>";
const esCierre = (parte: string) => parte.toLowerCase() === "</mark>";

/**
 * Parte el texto en tramos normales y tramos de clave. Una marca sin su pareja (un `<mark>` sin cerrar, un `</mark>`
 * suelto, o un `<mark>` dentro de otro) queda como texto, tal cual llegó: el texto nunca se pierde ni se reescribe.
 */
export function segmentarMarcas(texto: string): SegmentoTexto[] {
  const segmentos: SegmentoTexto[] = [];
  const agregar = (valor: string, clave: boolean) => {
    if (valor === "") return;
    const ultimo = segmentos[segmentos.length - 1];
    if (ultimo && ultimo.clave === clave) ultimo.texto += valor;
    else segmentos.push({ texto: valor, clave });
  };

  let abierto = false;
  let contenido = "";
  for (const parte of texto.split(ETIQUETAS)) {
    if (esApertura(parte)) {
      // Un segundo `<mark>` con otro abierto: el primero no tenía pareja y vuelve a ser texto.
      if (abierto) agregar(`<mark>${contenido}`, false);
      abierto = true;
      contenido = "";
    } else if (esCierre(parte)) {
      if (abierto) {
        agregar(contenido, true);
        abierto = false;
        contenido = "";
      } else {
        agregar(parte, false);
      }
    } else if (abierto) {
      contenido += parte;
    } else {
      agregar(parte, false);
    }
  }
  // `<mark>` sin cerrar al final: texto.
  if (abierto) agregar(`<mark>${contenido}`, false);
  return segmentos;
}

/**
 * Nodos de React del texto con la clave resaltada. `<mark>` es semántico y, además del color, lleva negrita, subrayado
 * y un texto solo para lectores de pantalla («clave oficial»), para que no se distinga solo por color.
 */
export function nodosConClave(texto: string, etiquetaClave: string): ReactNode {
  return createElement(
    Fragment,
    null,
    segmentarMarcas(texto).map((s, i) =>
      s.clave
        ? createElement(
            "mark",
            {
              key: i,
              className:
                "bg-warning text-warning-foreground rounded-sm px-0.5 font-semibold underline decoration-2 underline-offset-2",
            },
            createElement("span", { className: "sr-only" }, `${etiquetaClave}: `),
            s.texto,
          )
        : createElement(Fragment, { key: i }, s.texto),
    ),
  );
}
