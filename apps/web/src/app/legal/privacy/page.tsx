import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "Privacidad" };

export default function PrivacyPage() {
  return (
    <main className="legal-page">
      <Link href="/">← Glazz</Link>
      <h1>Política de privacidad</h1>
      <p className="legal-version">Borrador · versión de desarrollo</p>
      <p>
        Esta política es un borrador de desarrollo y requiere revisión legal antes del lanzamiento
        público.
      </p>
      <h2>Datos que tratamos</h2>
      <p>
        Para usuarios registrados tratamos perfil básico de Google, sesiones, conversaciones y
        mediciones de uso. Los visitantes reciben una sesión anónima limitada.
      </p>
      <h2>Finalidad</h2>
      <p>
        Usamos estos datos para autenticarte, conservar conversaciones, generar respuestas, aplicar
        límites y proteger el servicio. Los mensajes no se escriben en logs operativos.
      </p>
      <h2>Eliminación</h2>
      <p>
        Puedes solicitar la eliminación desde Ajustes. Las sesiones se revocan de inmediato y los
        datos personales se eliminan dentro de 24 horas; permanecen agregados no identificables.
      </p>
    </main>
  );
}
