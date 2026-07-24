"use client";

import { RefreshCw, WifiOff } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { usePreferences } from "@/components/theme-provider";

export function PWARegister() {
  const { locale } = usePreferences();
  const [online, setOnline] = useState(true);
  const [waiting, setWaiting] = useState<ServiceWorker | null>(null);
  const updateAccepted = useRef(false);

  useEffect(() => {
    const updateStatus = () => setOnline(navigator.onLine);
    const reload = () => {
      if (updateAccepted.current) window.location.reload();
    };
    updateStatus();
    window.addEventListener("online", updateStatus);
    window.addEventListener("offline", updateStatus);
    if (process.env.NODE_ENV === "production" && "serviceWorker" in navigator) {
      void navigator.serviceWorker
        .register("/sw.js")
        .then((registration) => {
          if (registration.waiting) setWaiting(registration.waiting);
          registration.addEventListener("updatefound", () => {
            const installing = registration.installing;
            installing?.addEventListener("statechange", () => {
              if (installing.state === "installed" && navigator.serviceWorker.controller) {
                setWaiting(registration.waiting ?? installing);
              }
            });
          });
        })
        .catch(() => {
          // The application remains usable online when registration is unavailable.
        });
      navigator.serviceWorker.addEventListener("controllerchange", reload);
    }
    return () => {
      window.removeEventListener("online", updateStatus);
      window.removeEventListener("offline", updateStatus);
      navigator.serviceWorker?.removeEventListener("controllerchange", reload);
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
      <div className="pwa-update" role="status">
        <button
          className="pwa-status"
          onClick={() => {
            updateAccepted.current = true;
            waiting.postMessage({ type: "SKIP_WAITING" });
          }}
        >
          <RefreshCw />
          {locale === "es" ? "Actualizar Glazz" : "Update Glazz"}
        </button>
      </div>
    );
  }
  return null;
}
