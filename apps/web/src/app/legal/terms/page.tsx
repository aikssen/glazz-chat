import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "Términos" };

export default function TermsPage() {
  return (
    <LegalPage title="Términos de uso">
      <p>
        Estos términos son un borrador de desarrollo y requieren revisión legal antes del
        lanzamiento público.
      </p>
      <h2>Uso del servicio</h2>
      <p>
        Glazz ofrece respuestas generadas por inteligencia artificial que pueden contener errores.
        No uses el servicio como única fuente para decisiones médicas, legales, financieras o de
        seguridad.
      </p>
      <h2>Cuenta y acceso</h2>
      <p>
        Debes tener al menos 18 años y mantener segura tu cuenta de Google. El acceso puede
        limitarse para proteger el servicio y a sus usuarios.
      </p>
      <h2>Contenido</h2>
      <p>
        Conservas la responsabilidad sobre lo que envías. No uses Glazz para infringir derechos,
        vulnerar sistemas o causar daño.
      </p>
    </LegalPage>
  );
}

function LegalPage({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <main className="legal-page">
      <Link href="/">← Glazz</Link>
      <h1>{title}</h1>
      <p className="legal-version">Borrador · versión de desarrollo</p>
      {children}
    </main>
  );
}
