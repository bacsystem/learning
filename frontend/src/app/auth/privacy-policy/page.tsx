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
      titulo={t("auth-privacidad")}
      intro="Esta política explica qué datos personales recoge Capacitate, para qué los usa y cómo los protege, conforme a la Ley N.º 29733, Ley de Protección de Datos Personales, y su reglamento."
    >
      <SeccionLegal titulo="1. Datos que recogemos">
        <ul>
          <li>Datos de registro: nombres, apellidos, documento de identidad, grado, correo electrónico y teléfono.</li>
          <li>Datos de uso: prácticas y simulacros realizados, respuestas, puntajes y tiempos.</li>
          <li>Opiniones y testimonios que decidas enviar.</li>
          <li>Datos técnicos para mantener tu sesión y tus preferencias.</li>
          <li>
            Por cada sesión y en el registro de auditoría guardamos la dirección IP y el user-agent (el navegador y el
            dispositivo que usas), con fines de seguridad y auditoría. Los administradores de la plataforma pueden
            verlos.
          </li>
        </ul>
      </SeccionLegal>

      <SeccionLegal titulo="2. Para qué los usamos">
        <ul>
          <li>Crear y activar tu cuenta y verificar tu identidad.</li>
          <li>Mostrarte tu progreso, estadísticas y resultados.</li>
          <li>Enviarte avisos sobre tu cuenta, como la activación o el cambio de contraseña.</li>
          <li>Mejorar los bancos de preguntas y el funcionamiento de la plataforma.</li>
        </ul>
      </SeccionLegal>

      <SeccionLegal titulo="3. Con quién los compartimos">
        <p>
          No vendemos ni alquilamos tus datos. Solo los compartimos con los proveedores que nos dan el servicio técnico
          (alojamiento y envío de correos), que los tratan por encargo nuestro, y con las autoridades cuando la ley lo
          exige.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="4. Cookies y almacenamiento local">
        <p>
          Usamos una cookie para recordar el idioma que eliges y el almacenamiento local del navegador para mantener tu
          sesión iniciada y tus preferencias de apariencia. No usamos cookies de publicidad ni de seguimiento de
          terceros.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="5. Seguridad">
        <p>
          Protegemos tus datos con medidas técnicas y organizativas: contraseñas almacenadas con un hash irreversible,
          acceso restringido por roles y conexiones seguras. Aun así, ningún sistema es completamente infalible.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="6. Cuánto tiempo los conservamos">
        <p>
          Conservamos tus datos mientras tu cuenta esté activa. Si pides la baja, tu cuenta se desactiva y deja de poder
          usarse. La eliminación de tus datos se atiende por solicitud, salvo los que la ley nos obligue a conservar.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="7. Tus derechos">
        <p>Puedes ejercer en cualquier momento tus derechos de:</p>
        <ul>
          <li>Acceso a los datos que tenemos sobre ti.</li>
          <li>Rectificación de datos inexactos o desactualizados.</li>
          <li>Cancelación de tus datos y oposición a su tratamiento.</li>
          <li>Revocación del consentimiento que nos diste.</li>
        </ul>
        <p>
          Si consideras que no atendimos tu solicitud, puedes acudir a la Autoridad Nacional de Protección de Datos
          Personales.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="8. Cambios en esta política">
        <p>
          Publicaremos en esta página cualquier cambio a esta política y, si es relevante, te lo avisaremos dentro de la
          plataforma.
        </p>
      </SeccionLegal>

      <SeccionLegal titulo="9. Contacto">
        <p>
          Para cualquier consulta sobre tus datos o para ejercer tus derechos, escríbenos a{" "}
          <a href={`mailto:${CONTACT_EMAIL}`} className={ENLACE}>
            {CONTACT_EMAIL}
          </a>
          .
        </p>
      </SeccionLegal>
    </DocumentoLegal>
  );
}
