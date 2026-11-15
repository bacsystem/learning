/** Idioma del cuerpo legal: solo existe en español (borrador pendiente de revisión legal, sin traducción aprobada). */
const IDIOMA_DEL_CUERPO = "es";

/** ¿Hay que avisar que el documento solo está en español? Sí con la interfaz en cualquier otro idioma. */
export const avisarSoloEspanol = (locale: string): boolean => locale !== IDIOMA_DEL_CUERPO;
