"use client";

import { Laptop, LogOut, MonitorCog, ShieldAlert, Smartphone, Trash2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { PageShell } from "@/components/page-shell";
import { usePreferences } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";
import { APIError, api } from "@/lib/api";
import type { CurrentUser, Locale, Theme } from "@/lib/types";
import { useDialogFocus } from "@/lib/use-dialog-focus";

interface Session {
  id: string;
  current: boolean;
  deviceLabel?: string | null;
  createdAt: string;
  lastSeenAt: string;
  expiresAt: string;
}

export function SettingsApp() {
  const { locale, setLocale, theme, setTheme } = usePreferences();
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [error, setError] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [confirmation, setConfirmation] = useState("");
  const deletionDialogRef = useRef<HTMLDivElement>(null);
  const closeDeletionDialog = useCallback(() => setConfirming(false), []);
  useDialogFocus(confirming, deletionDialogRef, closeDeletionDialog);

  useEffect(() => {
    async function load() {
      try {
        const [current, sessionPage] = await Promise.all([
          api<CurrentUser>("/api/v1/me"),
          api<{ items: Session[] }>("/api/v1/me/sessions"),
        ]);
        if (current.locale !== locale) setLocale(current.locale);
        setUser(current);
        setSessions(sessionPage.items);
      } catch (cause) {
        setError(
          cause instanceof APIError && cause.status === 401
            ? locale === "es"
              ? "Inicia sesión para abrir los ajustes."
              : "Sign in to open settings."
            : cause instanceof Error
              ? cause.message
              : "Error",
        );
      }
    }
    void load();
  }, [locale, setLocale]);

  async function updateLocale(value: Locale) {
    try {
      if (user) {
        const current = await api<CurrentUser>("/api/v1/me", {
          method: "PATCH",
          body: JSON.stringify({ locale: value }),
        });
        setUser(current);
      }
      setLocale(value);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Preference update failed");
    }
  }

  async function revoke(session: Session) {
    await api<void>(`/api/v1/me/sessions/${session.id}`, { method: "DELETE" });
    if (session.current) {
      window.location.assign("/");
      return;
    }
    setSessions((current) => current.filter((item) => item.id !== session.id));
  }

  async function logout() {
    await api<void>("/api/v1/auth/logout", { method: "POST" });
    window.location.assign("/");
  }

  async function reauthenticate() {
    const result = await api<{ authorizationUrl: string }>("/api/v1/me/reauthenticate", {
      method: "POST",
    });
    window.location.assign(result.authorizationUrl);
  }

  async function deleteAccount() {
    try {
      await api("/api/v1/me", {
        method: "DELETE",
        body: JSON.stringify({ confirmation }),
      });
      window.location.assign("/?deleted=true");
    } catch (cause) {
      if (cause instanceof APIError && cause.code === "recent_auth_required") {
        await reauthenticate();
        return;
      }
      setError(cause instanceof Error ? cause.message : "Deletion failed");
    }
  }

  const copy =
    locale === "es"
      ? {
          title: "Ajustes",
          description: "Controla tu experiencia, tus sesiones y tus datos.",
          profile: "Perfil",
          appearance: "Apariencia",
          language: "Idioma",
          sessions: "Sesiones activas",
          privacy: "Privacidad y cuenta",
          delete: "Eliminar cuenta",
        }
      : {
          title: "Settings",
          description: "Control your experience, sessions, and data.",
          profile: "Profile",
          appearance: "Appearance",
          language: "Language",
          sessions: "Active sessions",
          privacy: "Privacy and account",
          delete: "Delete account",
        };

  return (
    <PageShell title={copy.title} description={copy.description}>
      {error ? <div className="settings-error">{error}</div> : null}
      <section className="settings-section">
        <h2>{copy.profile}</h2>
        <div className="profile-row">
          <span className="avatar">{user?.displayName.slice(0, 1).toUpperCase() ?? "G"}</span>
          <div>
            <strong>{user?.displayName ?? "—"}</strong>
            <span>{user?.email ?? "—"}</span>
          </div>
          {user ? (
            <Button variant="outline" onClick={logout}>
              <LogOut data-icon="inline-start" />
              {locale === "es" ? "Cerrar sesión" : "Sign out"}
            </Button>
          ) : null}
        </div>
      </section>
      <section className="settings-section">
        <h2>{copy.appearance}</h2>
        <div className="segmented-control" aria-label={copy.appearance}>
          {(["light", "dark", "system"] as Theme[]).map((value) => (
            <button key={value} aria-pressed={theme === value} onClick={() => setTheme(value)}>
              {value === "light"
                ? locale === "es"
                  ? "Claro"
                  : "Light"
                : value === "dark"
                  ? locale === "es"
                    ? "Oscuro"
                    : "Dark"
                  : locale === "es"
                    ? "Sistema"
                    : "System"}
            </button>
          ))}
        </div>
      </section>
      <section className="settings-section">
        <h2>{copy.language}</h2>
        <div className="segmented-control" aria-label={copy.language}>
          {(["es", "en"] as Locale[]).map((value) => (
            <button
              key={value}
              aria-pressed={locale === value}
              onClick={() => void updateLocale(value)}
            >
              {value === "es" ? "Español" : "English"}
            </button>
          ))}
        </div>
      </section>
      <section className="settings-section">
        <h2>{copy.sessions}</h2>
        <div className="session-list">
          {sessions.map((session) => (
            <div key={session.id} className="session-row" data-session-id={session.id}>
              {session.deviceLabel?.toLowerCase().includes("mobile") ? <Smartphone /> : <Laptop />}
              <div>
                <strong>
                  {session.deviceLabel || (locale === "es" ? "Navegador" : "Browser")}
                  {session.current ? ` · ${locale === "es" ? "Actual" : "Current"}` : ""}
                </strong>
                <span>
                  {locale === "es" ? "Último uso " : "Last used "}
                  {new Intl.DateTimeFormat(locale, {
                    dateStyle: "medium",
                    timeStyle: "short",
                  }).format(new Date(session.lastSeenAt))}
                </span>
              </div>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => void revoke(session)}
                aria-label={locale === "es" ? "Revocar sesión" : "Revoke session"}
              >
                <LogOut />
              </Button>
            </div>
          ))}
        </div>
      </section>
      <section className="settings-section settings-danger">
        <h2>{copy.privacy}</h2>
        <div className="danger-row">
          <ShieldAlert />
          <div>
            <strong>{copy.delete}</strong>
            <span>
              {locale === "es"
                ? "Revoca todas las sesiones y elimina conversaciones y datos personales."
                : "Revokes every session and removes conversations and personal data."}
            </span>
          </div>
          <Button variant="destructive" onClick={() => setConfirming(true)}>
            <Trash2 data-icon="inline-start" />
            {copy.delete}
          </Button>
        </div>
      </section>
      {confirming ? (
        <div className="dialog-backdrop" role="presentation">
          <div
            ref={deletionDialogRef}
            className="dialog"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="delete-title"
            tabIndex={-1}
          >
            <MonitorCog aria-hidden="true" />
            <h2 id="delete-title">{copy.delete}</h2>
            <p>
              {locale === "es"
                ? 'Esta acción no se puede deshacer. Escribe "DELETE" para continuar.'
                : 'This cannot be undone. Type "DELETE" to continue.'}
            </p>
            <label>
              <span>{locale === "es" ? "Confirmación" : "Confirmation"}</span>
              <input
                value={confirmation}
                onChange={(event) => setConfirmation(event.target.value)}
              />
            </label>
            <div className="dialog-actions">
              <Button variant="ghost" onClick={closeDeletionDialog}>
                {locale === "es" ? "Cancelar" : "Cancel"}
              </Button>
              <Button
                variant="destructive"
                disabled={confirmation !== "DELETE"}
                onClick={() => void deleteAccount()}
              >
                {copy.delete}
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </PageShell>
  );
}
