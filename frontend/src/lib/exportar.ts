/** Exportación de listados a CSV e impresión, sin dependencias. Las columnas dicen cómo se lee cada valor como texto. */
export type ColumnaExportable<T> = {
  titulo: string;
  valor: (fila: T) => string | number | null | undefined;
};

/** Un número negativo, con o sin separador de miles (punto, coma, espacio o espacio duro: `-3`, `-12.5`, `-1,234.56`, `-1 234`): no es una fórmula. */
const NEGATIVO = /^-(?:\d+|\d{1,3}(?:[., \u00a0]\d{3})+)(?:[.,]\d+)?$/;

/** Excel ejecuta como fórmula la celda que empieza con = + - @ (o tab/CR): se antepone ' para que quede como texto. */
function neutralizarFormula(s: string): string {
  return /^[=+\-@\t\r]/.test(s) && !NEGATIVO.test(s) ? `'${s}` : s;
}

export function celdaCsv(v: string | number | null | undefined): string {
  const s = neutralizarFormula(v == null ? "" : String(v));
  return /[",;\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

export function descargarCsv<T>(nombre: string, columnas: ColumnaExportable<T>[], filas: T[]) {
  const lineas = [
    columnas.map((c) => celdaCsv(c.titulo)).join(","),
    ...filas.map((f) => columnas.map((c) => celdaCsv(c.valor(f))).join(",")),
  ];
  // BOM para que Excel lea bien los acentos.
  const blob = new Blob(["﻿" + lineas.join("\r\n")], {
    type: "text/csv;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${nombre}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

function escaparHtml(s: string): string {
  return s.replace(
    /[&<>"']/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c] as string,
  );
}

/**
 * Abre una ventana con el listado y lanza el diálogo de impresión. Devuelve `false` si el navegador bloqueó la ventana,
 * para que quien llama avise (p. ej. con un toast).
 */
export function imprimirTabla<T>(titulo: string, columnas: ColumnaExportable<T>[], filas: T[]): boolean {
  const ventana = window.open("", "_blank", "noopener=no,width=1024,height=768");
  if (!ventana) return false;
  const cabecera = columnas.map((c) => `<th>${escaparHtml(c.titulo)}</th>`).join("");
  const cuerpo = filas
    .map((f) => `<tr>${columnas.map((c) => `<td>${escaparHtml(String(c.valor(f) ?? ""))}</td>`).join("")}</tr>`)
    .join("");
  // Colores fijos a propósito: se imprime sobre papel blanco aunque la app esté en modo oscuro (los tokens darían texto claro).
  ventana.document
    .write(`<!doctype html><html lang="es"><head><meta charset="utf-8"><title>${escaparHtml(titulo)}</title>
<style>body{font:13px/1.45 Inter,system-ui,sans-serif;color:#16181d;margin:24px}h1{font-size:18px;margin:0 0 12px}
table{border-collapse:collapse;width:100%}th,td{border-bottom:1px solid #dcdee4;padding:6px 8px;text-align:left;vertical-align:top}
th{font-size:12px;font-weight:600;color:#5d6170}</style></head>
<body><h1>${escaparHtml(titulo)}</h1><table><thead><tr>${cabecera}</tr></thead><tbody>${cuerpo}</tbody></table></body></html>`);
  ventana.document.close();
  ventana.focus();
  ventana.print();
  return true;
}
