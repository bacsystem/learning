const SIMBOLOS: Record<string, string> = { PEN: "S/", USD: "$" };

/** Idiomas de las fechas escritas: castellano (por defecto) e inglés. */
export const LOCALES_FECHA = ["es", "en"] as const;
export type LocaleFecha = (typeof LOCALES_FECHA)[number];

/** ¿Es un idioma que sabemos escribir? Para acotar lo que llega de fuera (cookie, `useLocale()`, JS sin tipos). */
export const esLocaleFecha = (valor: unknown): valor is LocaleFecha => LOCALES_FECHA.some((l) => l === valor);

const MESES: Record<LocaleFecha, readonly string[]> = {
  es: ["Ene", "Feb", "Mar", "Abr", "May", "Jun", "Jul", "Ago", "Set", "Oct", "Nov", "Dic"],
  en: ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"],
};

/** Número con separador de miles y 2 decimales (formato SUNAT: 1,234.56). */
export function formatearNumero(valor: number): string {
  return Number(valor).toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

export function formatearMonto(moneda: string, valor: number): string {
  const simbolo = SIMBOLOS[moneda] ?? moneda;
  return `${simbolo} ${formatearNumero(valor)}`;
}

/** Hoy en hora local, ISO `YYYY-MM-DD` (valor de `<input type="date">` y presets de `EntradaRangoFechas`). */
export function isoHoy(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** Suma (o resta, con negativos) días a una fecha ISO `YYYY-MM-DD`, en hora local. */
export function sumarDias(iso: string, dias: number): string {
  const [anio, mes, dia] = iso.split("-").map(Number);
  const d = new Date(anio ?? 1970, (mes ?? 1) - 1, dia ?? 1);
  d.setDate(d.getDate() + dias);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** Fecha ISO `YYYY-MM-DD` a texto: «15 Nov 2026» (`es`, por defecto) o «Nov 15, 2026» (`en`). Si no es una fecha, la devuelve tal cual. */
export function formatearFecha(iso: string, locale: LocaleFecha = "es"): string {
  // Un idioma desconocido (`fr`, `null`) cae en castellano en ejecución, aunque el tipo no lo permita.
  const idioma = esLocaleFecha(locale) ? locale : "es";
  const [anio, mes, dia] = iso.split("-").map(Number);
  const nombreMes = MESES[idioma][mes - 1];
  if (!anio || !mes || !dia || !nombreMes) return iso;
  return idioma === "en" ? `${nombreMes} ${dia}, ${anio}` : `${dia} ${nombreMes} ${anio}`;
}

/** Instante ISO (UTC) a fecha y hora de Lima: «15 Set 2026, 10:32» (`es`, por defecto) o «Sep 15, 2026, 10:32» (`en`). */
export function formatearFechaHora(iso: string, locale: LocaleFecha = "es"): string {
  const fecha = new Date(iso);
  if (Number.isNaN(fecha.getTime())) return iso;
  const partes = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Lima",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(fecha);
  const v = (tipo: string) => partes.find((p) => p.type === tipo)?.value ?? "";
  return `${formatearFecha(`${v("year")}-${v("month")}-${v("day")}`, locale)}, ${v("hour")}:${v("minute")}`;
}

const SEGUNDOS_POR_MINUTO = 60;

/**
 * Segundos a `mm:ss` (`65 → "01:05"`) para una cuenta regresiva. Los minutos no se limitan a 2 dígitos (`6000 → "100:00"`);
 * un valor negativo o no finito (`NaN`, `Infinity`) da `00:00` y los decimales se truncan.
 */
export function formatearMmSs(segundos: number): string {
  const total = Number.isFinite(segundos) && segundos > 0 ? Math.floor(segundos) : 0;
  const dos = (n: number) => String(n).padStart(2, "0");
  return `${dos(Math.floor(total / SEGUNDOS_POR_MINUTO))}:${dos(total % SEGUNDOS_POR_MINUTO)}`;
}
