"use client";

import { type ReactNode, useMemo, useState } from "react";

import { type Columna, TablaDatos } from "@/components/datos/tabla-datos";
import { Buscador } from "@/components/formularios/buscador";
import {
  Menu,
  MenuCheckboxItem,
  MenuContent,
  MenuGroup,
  MenuGroupLabel,
  MenuItem,
  MenuTrigger,
} from "@/components/ui/menu";
import { ACCION_SECUNDARIA } from "@/lib/estilos";
import { type ColumnaExportable, descargarCsv, imprimirTabla } from "@/lib/exportar";
import { ordenarFilas, type OrdenTabla } from "@/lib/ordenar";
import { ColumnsIcon, DownloadSimpleIcon, FileCsvIcon, PrinterIcon } from "@phosphor-icons/react";

/**
 * Columna de un listado: la de `TablaDatos` más cómo se busca y se exporta.
 * - `texto`: el valor plano de la celda para buscar, exportar e imprimir. Sin él se usa `valor` o `fila[id]`, **no** lo
 *   que dibuja `render`: una columna con `render` (un estado como píldora, un monto con formato) debe dar su `texto`
 *   para que se busque y se exporte lo mismo que se ve.
 * - `tituloTexto`: el nombre de la columna en el CSV, la impresión y el selector cuando `titulo` no es un texto (un
 *   icono, un nodo). Sin un nombre de texto, la columna no se exporta ni se puede ocultar.
 * - `exportable: false` la saca del CSV y de la impresión (p. ej. acciones); `ocultable: false` la deja siempre visible.
 */
export type ColumnaListado<T> = Columna<T> & {
  texto?: (fila: T) => string | number | null | undefined;
  tituloTexto?: string;
  exportable?: boolean;
  /** `false`: no aparece en el selector de columnas (siempre visible). */
  ocultable?: boolean;
  /** Oculta la columna al inicio (el usuario puede mostrarla). Solo vale para columnas que se pueden ocultar. */
  ocultaInicial?: boolean;
};

function textoDe<T>(c: ColumnaListado<T>, f: T): string | number | null | undefined {
  if (c.texto) return c.texto(f);
  if (c.valor) return c.valor(f);
  const v = (f as Record<string, unknown>)[c.id];
  return typeof v === "string" || typeof v === "number" ? v : v == null ? "" : String(v);
}

function nombreDe<T>(c: ColumnaListado<T>): string | null {
  if (c.tituloTexto) return c.tituloTexto;
  return typeof c.titulo === "string" && c.titulo !== "" ? c.titulo : null;
}

/** Minúsculas y sin marcas diacríticas (tildes, diéresis; la ñ queda n): «jose» encuentra «José». */
function normalizar(s: string): string {
  return s.normalize("NFD").replace(/\p{M}/gu, "").toLocaleLowerCase("es");
}

/** Textos de la barra del listado, para traducirlos (por defecto en castellano). */
export type TextosTablaListado = {
  buscar: string;
  columnas: string;
  columnasVisibles: string;
  exportar: string;
  exportarCsv: string;
  imprimir: string;
};

const TEXTOS_ES: TextosTablaListado = {
  buscar: "Buscar…",
  columnas: "Columnas",
  columnasVisibles: "Columnas visibles",
  exportar: "Exportar",
  exportarCsv: "Exportar CSV",
  imprimir: "Imprimir",
};

/**
 * Listado completo en cliente sobre `TablaDatos`: buscador (sin distinguir mayúsculas ni tildes), selector de columnas
 * visibles, exportar CSV e imprimir —en el orden que se ve en pantalla y con las columnas visibles—, más el orden y la
 * paginación de la tabla (que vuelve a la página 1 al buscar). `filtros` va debajo del buscador (tabs, selects);
 * `acciones` a la derecha. La barra y los filtros llevan `data-sin-imprimir`, para ocultarlos con `@media print` si se
 * imprime la página entera. Si el navegador bloquea la ventana de impresión se llama a `onImpresionBloqueada` (p. ej.
 * para avisar con un toast).
 */
export function TablaListado<T>({
  columnas,
  filas,
  clave,
  titulo,
  buscar = true,
  placeholderBusqueda,
  exportar = true,
  filtros,
  acciones,
  textos: textosParciales,
  onImpresionBloqueada,
  ...resto
}: {
  columnas: ColumnaListado<T>[];
  filas: T[];
  clave: (fila: T) => string;
  /** Nombre del archivo CSV y del título impreso. */
  titulo: string;
  buscar?: boolean;
  placeholderBusqueda?: string;
  exportar?: boolean;
  filtros?: ReactNode;
  acciones?: ReactNode;
  textos?: Partial<TextosTablaListado>;
  onImpresionBloqueada?: () => void;
} & Omit<React.ComponentProps<typeof TablaDatos<T>>, "columnas" | "filas" | "clave">) {
  const textos = { ...TEXTOS_ES, ...textosParciales };
  const textoBusqueda = placeholderBusqueda ?? textos.buscar;
  const [busqueda, setBusqueda] = useState("");
  const [orden, setOrden] = useState<OrdenTabla>(resto.ordenInicial ?? null);

  const ocultables = columnas.filter((c) => c.ocultable !== false && nombreDe(c) !== null);
  const [ocultas, setOcultas] = useState<string[]>(() => ocultables.filter((c) => c.ocultaInicial).map((c) => c.id));

  const visibles = useMemo(() => columnas.filter((c) => !ocultas.includes(c.id)), [columnas, ocultas]);

  const filtradas = useMemo(() => {
    const q = normalizar(busqueda.trim());
    if (!q) return filas;
    // Solo en lo que el usuario ve: una fila no debe aparecer por un dato de una columna oculta.
    return filas.filter((f) => visibles.some((c) => normalizar(String(textoDe(c, f) ?? "")).includes(q)));
  }, [filas, visibles, busqueda]);

  // Volver a la página 1 al buscar, y también cuando cambia el `reinicioPagina` propio de quien llama (sus filtros).
  const reinicioPropio = resto.reinicioPagina;
  const reinicio = useMemo(() => [busqueda, reinicioPropio], [busqueda, reinicioPropio]);

  // Con las columnas visibles, como la tabla: si se ocultó la columna del orden, en pantalla ya no se ordena por ella.
  const paraExportar = useMemo(() => ordenarFilas(filtradas, visibles, orden), [filtradas, visibles, orden]);

  const exportables: ColumnaExportable<T>[] = visibles.flatMap((c) => {
    const nombre = nombreDe(c);
    return c.exportable !== false && nombre !== null ? [{ titulo: nombre, valor: (f: T) => textoDe(c, f) }] : [];
  });

  function imprimir() {
    if (!imprimirTabla(titulo, exportables, paraExportar)) onImpresionBloqueada?.();
  }

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {buscar || exportar || acciones || ocultables.length > 0 ? (
        <div className="flex flex-wrap items-center gap-2" data-sin-imprimir>
          {buscar ? (
            <div className="min-w-56 flex-1">
              <Buscador etiqueta={textoBusqueda} placeholder={textoBusqueda} valor={busqueda} onCambio={setBusqueda} />
            </div>
          ) : (
            <div className="flex-1" />
          )}
          {ocultables.length > 0 ? (
            <Menu>
              <MenuTrigger className={ACCION_SECUNDARIA} aria-label={textos.columnas} title={textos.columnas}>
                <ColumnsIcon className="size-4" />
                <span className="max-sm:sr-only">{textos.columnas}</span>
              </MenuTrigger>
              <MenuContent align="end" className="min-w-48">
                <MenuGroup>
                  <MenuGroupLabel>{textos.columnasVisibles}</MenuGroupLabel>
                  {ocultables.map((c) => (
                    <MenuCheckboxItem
                      key={c.id}
                      checked={!ocultas.includes(c.id)}
                      onCheckedChange={(visible) =>
                        setOcultas((o) => (visible ? o.filter((x) => x !== c.id) : [...o, c.id]))
                      }
                    >
                      {nombreDe(c)}
                    </MenuCheckboxItem>
                  ))}
                </MenuGroup>
              </MenuContent>
            </Menu>
          ) : null}
          {exportar ? (
            <Menu>
              <MenuTrigger className={ACCION_SECUNDARIA} aria-label={textos.exportar} title={textos.exportar}>
                <DownloadSimpleIcon className="size-4" />
                <span className="max-sm:sr-only">{textos.exportar}</span>
              </MenuTrigger>
              <MenuContent align="end" className="min-w-44">
                <MenuItem onClick={() => descargarCsv(titulo, exportables, paraExportar)}>
                  <FileCsvIcon className="size-4" />
                  {textos.exportarCsv}
                </MenuItem>
                <MenuItem onClick={imprimir}>
                  <PrinterIcon className="size-4" />
                  {textos.imprimir}
                </MenuItem>
              </MenuContent>
            </Menu>
          ) : null}
          {acciones}
        </div>
      ) : null}
      {filtros ? (
        <div className="flex flex-wrap items-center gap-2" data-sin-imprimir>
          {filtros}
        </div>
      ) : null}
      <TablaDatos
        columnas={visibles}
        filas={filtradas}
        clave={clave}
        {...resto}
        reinicioPagina={reinicio}
        onOrdenCambio={(o) => {
          setOrden(o);
          resto.onOrdenCambio?.(o);
        }}
      />
    </div>
  );
}
