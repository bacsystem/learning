export const LOCALES = ["es", "en"] as const;
export type LocaleOption = (typeof LOCALES)[number];

const storagePrefix = process.env.NEXT_PUBLIC_STORAGE_PREFIX || "capacitate";
export const COOKIE_KEYS = { locale: `${storagePrefix}-locale` };
