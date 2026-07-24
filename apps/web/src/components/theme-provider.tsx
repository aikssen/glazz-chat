"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { Locale, Theme } from "@/lib/types";

interface Preferences {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

const PreferencesContext = createContext<Preferences | null>(null);

export function PreferencesProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>("system");
  const [locale, setLocale] = useState<Locale>("es");
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const savedTheme = localStorage.getItem("glazz-theme") as Theme | null;
      const savedLocale = localStorage.getItem("glazz-locale") as Locale | null;
      if (savedTheme) setTheme(savedTheme);
      if (savedLocale === "en" || savedLocale === "es") {
        setLocale(savedLocale);
      } else {
        setLocale(navigator.language.toLowerCase().startsWith("es") ? "es" : "en");
      }
      setHydrated(true);
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (!hydrated) return;
    localStorage.setItem("glazz-theme", theme);
    const preference = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const dark = theme === "dark" || (theme === "system" && preference.matches);
      document.documentElement.classList.toggle("dark", dark);
      document.documentElement.style.colorScheme = dark ? "dark" : "light";
      document.documentElement.dataset.themeReady = "true";
    };
    apply();
    if (theme !== "system") return;
    preference.addEventListener("change", apply);
    return () => preference.removeEventListener("change", apply);
  }, [hydrated, theme]);

  useEffect(() => {
    if (!hydrated) return;
    localStorage.setItem("glazz-locale", locale);
    document.documentElement.lang = locale;
  }, [hydrated, locale]);

  const value = useMemo(() => ({ theme, setTheme, locale, setLocale }), [theme, locale]);
  return <PreferencesContext.Provider value={value}>{children}</PreferencesContext.Provider>;
}

export function usePreferences() {
  const value = useContext(PreferencesContext);
  if (!value) throw new Error("PreferencesProvider is missing");
  return value;
}
