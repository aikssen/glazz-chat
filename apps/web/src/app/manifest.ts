import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Glazz",
    short_name: "Glazz",
    description: "Chat de IA enfocado, rápido y bilingüe.",
    start_url: "/",
    display: "standalone",
    background_color: "#fafaf8",
    theme_color: "#c74312",
    icons: [{ src: "/favicon.ico", sizes: "any", type: "image/x-icon" }],
  };
}
