// Comprueba que es.json y en.json tienen exactamente las mismas claves, sin valores vacíos y con los mismos
// placeholders ICU ({n}, {nombre}…). Sale con código 1 si encuentra alguna diferencia. Uso: node scripts/check-i18n.mjs
import console from "node:console";
import { readFileSync } from "node:fs";
import process from "node:process";
import { URL } from "node:url";

const leer = (locale) =>
  JSON.parse(readFileSync(new URL(`../src/i18n/messages/${locale}.json`, import.meta.url), "utf8"));

/** Aplana `{ dashboard: { clave: "texto" } }` a `{ "dashboard.clave": "texto" }`. */
function aplanar(obj, prefijo = "", salida = {}) {
  for (const [k, v] of Object.entries(obj)) {
    const clave = prefijo ? `${prefijo}.${k}` : k;
    if (v && typeof v === "object") aplanar(v, clave, salida);
    else salida[clave] = v;
  }
  return salida;
}

// Solo los nombres de argumento de primer nivel (`{n}`, `{n, plural, …}`), no el texto de las ramas del plural.
const placeholders = (texto) =>
  [...new Set([...String(texto).matchAll(/\{\s*(\w+)\s*[,}]/g)].map((m) => m[1]))].sort().join(",");

const es = aplanar(leer("es"));
const en = aplanar(leer("en"));
const errores = [];

for (const k of Object.keys(es)) if (!(k in en)) errores.push(`solo en es.json: ${k}`);
for (const k of Object.keys(en)) if (!(k in es)) errores.push(`solo en en.json: ${k}`);
for (const [locale, mensajes] of [
  ["es", es],
  ["en", en],
])
  for (const [k, v] of Object.entries(mensajes))
    if (!String(v).trim()) errores.push(`valor vacío en ${locale}.json: ${k}`);
for (const k of Object.keys(es))
  if (k in en && placeholders(es[k]) !== placeholders(en[k]))
    errores.push(`placeholders distintos en ${k}: es {${placeholders(es[k])}} / en {${placeholders(en[k])}}`);

if (errores.length) {
  console.error(`i18n: ${errores.length} problema(s)\n  ${errores.join("\n  ")}`);
  process.exit(1);
}
console.log(`i18n: es.json y en.json coinciden (${Object.keys(es).length} claves).`);
