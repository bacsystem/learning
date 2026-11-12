import { claveErrorModulo } from "@/lib/serums-ia/errores";

/** Clave i18n de un error de las operaciones sobre preguntas (citas y estado). `tiene` es `t.has` de next-intl. */
export const claveErrorPregunta = (err: unknown, tiene: (clave: string) => boolean): string =>
  claveErrorModulo(err, "preguntas", tiene);
