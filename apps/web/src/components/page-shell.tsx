"use client";

import { ArrowLeft, Moon, Sun } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";

export function PageShell({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  const { theme, setTheme } = usePreferences();
  return (
    <main className="settings-shell">
      <header className="settings-topbar">
        <Link href="/" className="settings-back" aria-label="Volver al chat">
          <ArrowLeft />
        </Link>
        <Link href="/" className="wordmark">
          <span aria-hidden="true">G</span>
          Glazz
        </Link>
        <Button
          variant="ghost"
          size="icon"
          className="ml-auto"
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          aria-label="Cambiar apariencia"
        >
          {theme === "dark" ? <Sun /> : <Moon />}
        </Button>
      </header>
      <div className="settings-layout">
        <div className="settings-heading">
          <h1>{title}</h1>
          <p>{description}</p>
        </div>
        {children}
      </div>
    </main>
  );
}
