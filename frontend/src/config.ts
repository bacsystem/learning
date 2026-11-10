export const DEFAULTS = {
  appRoot: "/dashboards/analytics",
  locale: "es",
};

/** Correo público de contacto (privacidad, términos, pie de la landing). Siempre tiene valor:
 * NEXT_PUBLIC_CONTACT_EMAIL lo sobrescribe y, vacío o sin definir (el valor por defecto de compose, Makefile y
 * Dockerfile), se usa el de la empresa. */
export const CONTACT_EMAIL = process.env.NEXT_PUBLIC_CONTACT_EMAIL?.trim() || "contact@bacsystemsolutions.com";
