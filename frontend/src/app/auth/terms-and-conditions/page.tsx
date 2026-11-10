// PENDIENTE (revisión legal): texto base redactado durante la migración a khipu. Debe revisarlo un abogado
// antes de publicarse (ley aplicable: Perú; privacidad según la Ley N.º 29733).
// El cuerpo es un borrador jurídico en español y no se traduce; sí se traduce la interfaz que lo rodea (título).
import { getTranslations } from "next-intl/server";

import { DocumentoLegal, SeccionLegal } from "@/app/auth/documento-legal";
import { ENLACE } from "@/app/auth/tarjeta-auth";
import { CONTACT_EMAIL } from "@/config";

export default async function Page() {
  const t = await getTranslations("dashboard");
  return (
    <DocumentoLegal
      titulo={t("auth-terminos")}
      intro="Estos términos regulan el uso de Capacitate, la plataforma de práctica y simulacros para los exámenes de ascenso. Al crear una cuenta o usar la plataforma aceptas estos términos; si no estás de acuerdo con ellos, no la uses."
    >
      <SeccionLegal titulo="1. Objeto del servicio">
        <p>
          Capacitate ofrece bancos de preguntas, prácticas por curso y simulacros de examen para que el personal se
          prepare para los procesos de ascenso. El contenido es material de estudio y no reemplaza las bases, temarios
          ni comunicaciones oficiales de cada proceso.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="2. Cuentas de usuario">
        <ul>
          <li>Debes registrarte con datos verdaderos y mantenerlos actualizados.</li>
          <li>La cuenta es personal: no compartas tu contraseña ni permitas que otra persona la use.</li>
          <li>Las cuentas nuevas quedan pendientes hasta que un administrador las activa.</li>
          <li>Eres responsable de la actividad que se realice con tu cuenta.</li>
        </ul>
      </SeccionLegal>

      <SeccionLegal titulo="3. Uso permitido">
        <p>Al usar la plataforma te comprometes a no:</p>
        <ul>
          <li>Copiar, publicar, vender o distribuir las preguntas y materiales fuera de la plataforma.</li>
          <li>Extraer contenido de forma automatizada o masiva.</li>
          <li>Intentar vulnerar la seguridad, el funcionamiento o la integridad del sistema.</li>
          <li>Publicar contenido ofensivo, falso o que infrinja derechos de terceros en opiniones o comentarios.</li>
        </ul>
      </SeccionLegal>

      <SeccionLegal titulo="4. Propiedad intelectual">
        <p>
          Los bancos de preguntas, recursos, textos, diseño y software de Capacitate pertenecen a sus titulares y están
          protegidos por la normativa de propiedad intelectual. El acceso a la plataforma no te transfiere ningún
          derecho sobre ellos.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="5. Contenido que publicas">
        <p>
          Las opiniones y testimonios que envíes deben ser tuyos. Al publicarlos autorizas a Capacitate a mostrarlos
          dentro de la plataforma y, una vez aprobados por un administrador, también en la página pública de inicio, con
          tu nombre y la inicial de tu apellido. Los administradores pueden ocultar o retirar el contenido que incumpla
          estos términos.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="6. Resultados y disponibilidad">
        <p>
          Los puntajes de prácticas y simulacros son referenciales y no garantizan un resultado en el examen real.
          Procuramos que la plataforma esté disponible y que el contenido sea correcto, pero puede haber interrupciones
          por mantenimiento o errores que corregiremos en cuanto los detectemos.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="7. Suspensión de cuentas">
        <p>
          Podemos suspender o dar de baja una cuenta que incumpla estos términos, que use datos falsos o que ponga en
          riesgo la seguridad de la plataforma o de otros usuarios.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="8. Cambios en los términos">
        <p>
          Podemos actualizar estos términos. Publicaremos la versión vigente en esta página y, si el cambio es
          relevante, te lo avisaremos dentro de la plataforma.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="9. Ley aplicable">
        <p>
          Estos términos se rigen por las leyes de la República del Perú. Cualquier controversia se someterá a los
          jueces y tribunales competentes del Perú.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="10. Contacto">
        <p>
          Para cualquier consulta sobre estos términos, escríbenos a{" "}
          <a href={`mailto:${CONTACT_EMAIL}`} className={ENLACE}>
            {CONTACT_EMAIL}
          </a>
          .
        </p>
      </SeccionLegal>
    </DocumentoLegal>
  );
}
