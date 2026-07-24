import type { Metadata } from "next";
import { LegalDocument } from "@/components/legal-document";

export const metadata: Metadata = { title: "Terms · Términos" };

export default function TermsPage() {
  return (
    <LegalDocument
      es={{
        title: "Términos de uso",
        intro:
          "Estos términos son un borrador de desarrollo y requieren revisión legal antes del lanzamiento público.",
        sections: [
          {
            heading: "Uso del servicio",
            body: "Glazz ofrece respuestas generadas por inteligencia artificial que pueden contener errores. No uses el servicio como única fuente para decisiones médicas, legales, financieras o de seguridad.",
          },
          {
            heading: "Cuenta y acceso",
            body: "Debes tener al menos 18 años y mantener segura tu cuenta de Google. El acceso puede limitarse para proteger el servicio y a sus usuarios.",
          },
          {
            heading: "Contenido",
            body: "Conservas la responsabilidad sobre lo que envías. No uses Glazz para infringir derechos, vulnerar sistemas o causar daño.",
          },
        ],
      }}
      en={{
        title: "Terms of use",
        intro: "These terms are a development draft and require legal review before public launch.",
        sections: [
          {
            heading: "Using the service",
            body: "Glazz provides AI-generated responses that may contain errors. Do not use it as the sole source for medical, legal, financial, safety, or security decisions.",
          },
          {
            heading: "Account and access",
            body: "You must be at least 18 and keep your Google account secure. Access may be limited to protect the service and its users.",
          },
          {
            heading: "Content",
            body: "You remain responsible for what you submit. Do not use Glazz to violate rights, compromise systems, or cause harm.",
          },
        ],
      }}
    />
  );
}
