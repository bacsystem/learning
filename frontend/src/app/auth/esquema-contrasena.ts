import { useTranslations } from "next-intl";
import * as yup from "yup";

type Traducir = (clave: string) => string;

/** bcrypt solo usa los primeros 72 BYTES: el backend valida `maxbytes=72` (VerifyEmailRequest, ResetPasswordRequest). */
export const MAX_BYTES_CONTRASENA = 72;

export function bytesUtf8(valor: string): number {
  return new TextEncoder().encode(valor).length;
}

/**
 * Reglas de contraseña nueva, iguales a las del backend: requerida, 8+ caracteres y como mucho 72 bytes UTF-8 (las
 * letras con tilde o ñ ocupan 2), más la confirmación. Lo usan el registro (verify-email) y el restablecimiento.
 */
export function esquemaContrasena(t: Traducir) {
  return {
    password: yup
      .string()
      .required(t("auth-contrasena-requerida"))
      .min(8, t("auth-contrasena-min"))
      .test("max-bytes", t("auth-contrasena-max-bytes"), (v) => (v ? bytesUtf8(v) <= MAX_BYTES_CONTRASENA : true)),
    confirm_password: yup
      .string()
      .required(t("auth-contrasena-confirmar-requerida"))
      .oneOf([yup.ref("password")], t("auth-contrasena-no-coinciden")),
  };
}

/** Textos traducidos del campo `Contrasena` de khipu (botón mostrar/ocultar y medidor de fuerza). */
export function useEtiquetasContrasena() {
  const t = useTranslations("dashboard");
  return {
    etiquetaMostrar: t("auth-mostrar-contrasena"),
    etiquetaOcultar: t("auth-ocultar-contrasena"),
    etiquetasFuerza: { debil: t("auth-fuerza-debil"), media: t("auth-fuerza-media"), fuerte: t("auth-fuerza-fuerte") },
  };
}
