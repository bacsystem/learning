export const LOCALES = ["es", "en"] as const;
export type LocaleOption = (typeof LOCALES)[number];

const storagePrefix = process.env.NEXT_PUBLIC_STORAGE_PREFIX || "capacitate";
export const COOKIE_KEYS = { locale: `${storagePrefix}-locale` };

/** Acota un texto (p. ej. el de `useLocale()`) a un idioma de la app; `undefined` si no es uno de `LOCALES`. */
export const comoLocale = (valor: string): LocaleOption | undefined => LOCALES.find((l) => l === valor);
