import { redirect } from "next/navigation";

/** `/auth` no tiene contenido propio: lleva al inicio de sesión. */
export default function Auth() {
  redirect("/auth/sign-in");
}
