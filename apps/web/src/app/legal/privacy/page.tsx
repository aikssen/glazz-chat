import type { Metadata } from "next";
import { LegalDocument } from "@/components/legal-document";

export const metadata: Metadata = { title: "Privacy · Privacidad" };

export default function PrivacyPage() {
  return (
    <LegalDocument
      es={{
        title: "Política de privacidad",
        intro:
          "Esta política es un borrador de desarrollo y requiere revisión legal antes del lanzamiento público.",
        sections: [
          {
            heading: "Datos que tratamos",
            body: "Para usuarios registrados tratamos perfil básico de Google, sesiones, conversaciones y mediciones de uso. Los visitantes reciben una sesión anónima limitada.",
          },
          {
            heading: "Finalidad",
            body: "Usamos estos datos para autenticarte, conservar conversaciones, generar respuestas, aplicar límites y proteger el servicio. Los mensajes no se escriben en logs operativos.",
          },
          {
            heading: "Eliminación",
            body: "Puedes solicitar la eliminación desde Ajustes. Las sesiones se revocan de inmediato y los datos personales se eliminan dentro de 24 horas; permanecen agregados no identificables.",
          },
        ],
      }}
      en={{
        title: "Privacy policy",
        intro: "This policy is a development draft and requires legal review before public launch.",
        sections: [
          {
            heading: "Data we process",
            body: "For registered users we process a basic Google profile, sessions, conversations, and usage measurements. Visitors receive a limited anonymous session.",
          },
          {
            heading: "Purpose",
            body: "We use this data to authenticate you, retain conversations, generate responses, apply limits, and protect the service. Messages are not written to operational logs.",
          },
          {
            heading: "Deletion",
            body: "You can request deletion in Settings. Sessions are revoked immediately and personal data is removed within 24 hours; only non-identifying aggregates remain.",
          },
        ],
      }}
    />
  );
}
