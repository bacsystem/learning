import type { LocaleOption } from "@/constants";
import { authApi } from "@/lib/auth/auth-api";

/**
 * Guarda el idioma elegido en el perfil (`PUT /auth/idioma`, docs/api/plataforma-avisos-idioma.md §1.3 y §1.5).
 * No se espera la respuesta y un error se ignora: la interfaz ya cambió y es solo una preferencia. Sin sesión no llama.
 */
export function guardarIdiomaDelUsuario(token: string | null, idioma: LocaleOption): void {
  if (!token) return;
  authApi.setIdioma(token, idioma).catch(() => {});
}
