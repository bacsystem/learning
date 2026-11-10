import {
  codigoScriptApariencia,
  type Fuente,
  type Paleta,
  type PreferenciasApariencia,
  type TonoOscuro,
} from "@/components/apariencia-opciones";
import { CLAVE_DISENO, type Diseno, DISENO_POR_DEFECTO, DISENOS } from "@/components/disenos";

/**
 * Script en línea para el `<head>` del layout raíz: aplica el diseño, la paleta, el tono oscuro y la tipografía guardados a `<html>`
 * antes del primer pintado, para que no se vea un instante el aspecto por defecto. Server Component; las listas deben ser
 * las mismas que reciben `DisenoProvider` y `AparienciaProvider`, y `inicial` el mismo que el de `AparienciaProvider`.
 */
export function ScriptDiseno({
  disenos = DISENOS,
  porDefecto = DISENO_POR_DEFECTO,
  paletas,
  tonosOscuro,
  fuentes,
  inicial,
}: {
  disenos?: Diseno[];
  porDefecto?: string;
  paletas?: Paleta[];
  tonosOscuro?: TonoOscuro[];
  fuentes?: Fuente[];
  inicial?: Partial<PreferenciasApariencia>;
}) {
  const validos = JSON.stringify(disenos.map((d) => d.valor));
  const diseno = `try{var v=localStorage.getItem(${JSON.stringify(CLAVE_DISENO)});document.documentElement.dataset.diseno=${validos}.indexOf(v)>-1?v:${JSON.stringify(porDefecto)}}catch(e){document.documentElement.dataset.diseno=${JSON.stringify(porDefecto)}}`;
  const codigo = `${diseno};${codigoScriptApariencia({ paletas, tonosOscuro, fuentes, inicial })}`;
  return <script dangerouslySetInnerHTML={{ __html: codigo }} />;
}
