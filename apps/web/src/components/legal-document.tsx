"use client";

import Link from "next/link";
import { usePreferences } from "@/components/theme-provider";

type Section = { heading: string; body: string };

export function LegalDocument({
  es,
  en,
}: {
  es: { title: string; intro: string; sections: Section[] };
  en: { title: string; intro: string; sections: Section[] };
}) {
  const { locale } = usePreferences();
  const document = locale === "es" ? es : en;
  return (
    <main className="legal-page">
      <Link href="/">← Glazz</Link>
      <h1>{document.title}</h1>
      <p className="legal-version">
        {locale === "es" ? "Borrador · versión de desarrollo" : "Draft · development version"}
      </p>
      <p>{document.intro}</p>
      {document.sections.map((section) => (
        <section key={section.heading}>
          <h2>{section.heading}</h2>
          <p>{section.body}</p>
        </section>
      ))}
    </main>
  );
}
