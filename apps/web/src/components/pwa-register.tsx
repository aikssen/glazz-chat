"use client";

import { RefreshCw, WifiOff } from "lucide-react";
import { useEffect, useState } from "react";
import { usePreferences } from "@/components/theme-provider";

export function PWARegister() {
  const { locale } = usePreferences();
  const [online, setOnline] = useState(true);
  const [waiting, setWaiting] = useState<ServiceWorker | null>(null);

  useEffect(() => {
    const updateStatus = () => setOnline(navigator.onLine);
    updateStatus();
    window.addEventListener("online", updateStatus);
    window.addEventListener("offline", updateStatus);
    if (process.env.NODE_ENV === "production" && "serviceWorker" in navigator) {
      void navigator.serviceWorker.register("/sw.js").then((registration) => {
        if (registration.waiting) setWaiting(registration.waiting);
        registration.addEventListener("updatefound", () => {
          const installing = registration.installing;
          installing?.addEventListener("statechange", () => {
            if (installing.state === "installed" && navigator.serviceWorker.controller) {
              setWaiting(registration.waiting);
            }
          });
        });
      });
      navigator.serviceWorker.addEventListener("controllerchange", () => window.location.reload());
    }
    return () => {
      window.removeEventListener("online", updateStatus);
      window.removeEventListener("offline", updateStatus);
    };
  }, []);

  if (!online) {
    return (
      <div className="pwa-status" role="status">
        <WifiOff />
        {locale === "es" ? "Sin conexión" : "Offline"}
      </div>
    );
  }
  if (waiting) {
    return (
      <button className="pwa-status" onClick={() => waiting.postMessage({ type: "SKIP_WAITING" })}>
        <RefreshCw />
        {locale === "es" ? "Actualizar Glazz" : "Update Glazz"}
      </button>
    );
  }
  return null;
}
