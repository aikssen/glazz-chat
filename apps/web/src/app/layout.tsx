import type { Metadata } from "next";
import { JetBrains_Mono, Outfit, Work_Sans } from "next/font/google";
import "./globals.css";
import { PreferencesProvider } from "@/components/theme-provider";
import { PWARegister } from "@/components/pwa-register";

const outfit = Outfit({
  variable: "--font-outfit",
  subsets: ["latin"],
});

const workSans = Work_Sans({
  variable: "--font-work-sans",
  subsets: ["latin"],
});

const jetBrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: { default: "Glazz", template: "%s · Glazz" },
  description: "Chat de IA enfocado, rápido y bilingüe.",
  applicationName: "Glazz",
  manifest: "/manifest.webmanifest",
};

const themeBootstrap = `
(() => {
  try {
    const saved = localStorage.getItem("glazz-theme");
    const theme = saved === "light" || saved === "dark" || saved === "system" ? saved : "system";
    const dark = theme === "dark" ||
      (theme === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.classList.toggle("dark", dark);
    document.documentElement.style.colorScheme = dark ? "dark" : "light";
  } catch {}
})();
`;

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="es" className="h-full" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeBootstrap }} />
      </head>
      <body
        className={`${outfit.variable} ${workSans.variable} ${jetBrainsMono.variable} min-h-full antialiased`}
      >
        <PreferencesProvider>
          <PWARegister />
          {children}
        </PreferencesProvider>
      </body>
    </html>
  );
}
