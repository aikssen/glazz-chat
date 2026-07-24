import type { Metadata } from "next";
import { SettingsApp } from "@/components/settings/settings-app";

export const metadata: Metadata = { title: "Ajustes" };

export default function SettingsPage() {
  return <SettingsApp />;
}
