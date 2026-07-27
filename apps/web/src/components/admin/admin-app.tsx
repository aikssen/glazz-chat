"use client";

import { Activity, Database, RefreshCw, Save, Settings2, ShieldCheck, Users } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { PageShell } from "@/components/page-shell";
import { usePreferences } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";
import { APIError, api } from "@/lib/api";
import type { AdminModel, AdminUser, RuntimeSetting } from "@/lib/types";
import { newUUID } from "@/lib/uuid";

type Tab = "models" | "settings" | "users" | "usage" | "audit";

interface AdminUsage {
  periodStart: string;
  periodEnd: string;
  generations: number;
  inputTokens: number;
  outputTokens: number;
  estimatedCost: number;
  currency: string;
}

interface AuditEvent {
  id: string;
  actorId?: string | null;
  action: string;
  targetType: string;
  targetId: string;
  occurredAt: string;
}

export function AdminApp() {
  const { locale } = usePreferences();
  const es = locale === "es";
  const [tab, setTab] = useState<Tab>("models");
  const [models, setModels] = useState<AdminModel[]>([]);
  const [settings, setSettings] = useState<RuntimeSetting[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [usage, setUsage] = useState<AdminUsage | null>(null);
  const [audit, setAudit] = useState<AuditEvent[]>([]);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setError("");
    try {
      if (tab === "models") {
        setModels((await api<{ items: AdminModel[] }>("/api/v1/admin/models")).items);
      }
      if (tab === "settings") {
        setSettings((await api<{ items: RuntimeSetting[] }>("/api/v1/admin/settings")).items);
      }
      if (tab === "users") {
        setUsers(
          (
            await api<{ items: AdminUser[] }>(
              `/api/v1/admin/users?limit=100${query ? `&query=${encodeURIComponent(query)}` : ""}`,
            )
          ).items,
        );
      }
      if (tab === "usage") {
        setUsage(await api<AdminUsage>("/api/v1/admin/usage"));
      }
      if (tab === "audit") {
        setAudit((await api<{ items: AuditEvent[] }>("/api/v1/admin/audit-log?limit=100")).items);
      }
    } catch (cause) {
      setError(
        cause instanceof APIError && cause.status === 403
          ? "No tienes acceso a administración."
          : cause instanceof Error
            ? cause.message
            : "No fue posible cargar esta vista.",
      );
    }
  }, [query, tab]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), tab === "users" ? 250 : 0);
    return () => window.clearTimeout(timer);
  }, [load, tab]);

  async function updateModel(model: AdminModel, patch: Partial<AdminModel>) {
    setBusy(true);
    try {
      const updated = await api<AdminModel>(`/api/v1/admin/models/${model.id}`, {
        method: "PATCH",
        headers: { "If-Match": `"${model.version}"` },
        body: JSON.stringify(patch),
      });
      if (patch.defaultFor) {
        await load();
      } else {
        setModels((current) => current.map((item) => (item.id === updated.id ? updated : item)));
      }
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "No fue posible guardar el modelo.";
      await load();
      setError(message);
    } finally {
      setBusy(false);
    }
  }

  async function makeDefault(model: AdminModel, actorType: "guest" | "user") {
    if (!model.enabled || model.defaultFor.includes(actorType)) return;
    const audience = model.audience.includes(actorType)
      ? model.audience
      : [...model.audience, actorType];
    await updateModel(model, {
      audience,
      defaultFor: [...model.defaultFor, actorType],
    });
  }

  async function updateSetting(setting: RuntimeSetting, value: unknown) {
    setBusy(true);
    try {
      const updated = await api<RuntimeSetting>(
        `/api/v1/admin/settings/${encodeURIComponent(setting.key)}`,
        {
          method: "PATCH",
          headers: { "If-Match": `"${setting.version}"` },
          body: JSON.stringify({ value }),
        },
      );
      setSettings((current) => current.map((item) => (item.key === updated.key ? updated : item)));
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "No fue posible guardar el ajuste.";
      await load();
      setError(message);
    } finally {
      setBusy(false);
    }
  }

  async function updateRole(user: AdminUser, role: AdminUser["role"]) {
    setBusy(true);
    try {
      const updated = await api<AdminUser>(`/api/v1/admin/users/${user.id}/role`, {
        method: "PATCH",
        headers: { "If-Match": `"${user.version}"` },
        body: JSON.stringify({ role }),
      });
      setUsers((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : "No fue posible cambiar el rol.";
      await load();
      setError(message);
    } finally {
      setBusy(false);
    }
  }

  async function synchronize() {
    setBusy(true);
    try {
      await api("/api/v1/admin/models/sync", {
        method: "POST",
        headers: { "Idempotency-Key": `model-sync-${newUUID()}` },
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "No fue posible sincronizar.");
    } finally {
      setBusy(false);
    }
  }

  const tabs: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: "models", label: es ? "Modelos" : "Models", icon: <Database /> },
    { id: "settings", label: es ? "Opciones" : "Settings", icon: <Settings2 /> },
    { id: "users", label: es ? "Usuarios" : "Users", icon: <Users /> },
    { id: "usage", label: es ? "Uso" : "Usage", icon: <Activity /> },
    { id: "audit", label: es ? "Auditoría" : "Audit", icon: <ShieldCheck /> },
  ];

  return (
    <PageShell
      title={es ? "Administración" : "Administration"}
      description={
        es
          ? "Control operativo sin acceso al contenido de las conversaciones."
          : "Operational controls without access to conversation content."
      }
    >
      <div
        className="admin-tabs"
        role="tablist"
        aria-label={locale === "es" ? "Secciones de administración" : "Administration sections"}
      >
        {tabs.map((item) => (
          <button
            key={item.id}
            role="tab"
            aria-selected={tab === item.id}
            onClick={() => setTab(item.id)}
          >
            {item.icon}
            {item.label}
          </button>
        ))}
      </div>
      {error ? <div className="settings-error">{error}</div> : null}
      {tab === "models" ? (
        <section className="admin-panel">
          <div className="admin-toolbar">
            <div>
              <h2>{es ? "Catálogo de modelos" : "Model catalog"}</h2>
              <p>
                {es
                  ? "Solo se pueden exponer modelos disponibles y compatibles."
                  : "Only available and supported models can be exposed."}
              </p>
            </div>
            <Button variant="outline" disabled={busy} onClick={() => void synchronize()}>
              <RefreshCw data-icon="inline-start" />
              {es ? "Sincronizar" : "Synchronize"}
            </Button>
          </div>
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>{es ? "Modelo" : "Model"}</th>
                  <th>{es ? "Estado" : "Status"}</th>
                  <th>{es ? "Audiencia" : "Audience"}</th>
                  <th>{es ? "Predeterminado" : "Default"}</th>
                  <th>{es ? "Expuesto" : "Exposed"}</th>
                </tr>
              </thead>
              <tbody>
                {models.map((model) => (
                  <tr key={model.id}>
                    <td>
                      <strong>{model.name}</strong>
                      <span>{model.description}</span>
                    </td>
                    <td>
                      <StatusLabel
                        ok={model.available && model.supported}
                        label={
                          model.available && model.supported
                            ? es
                              ? "Disponible"
                              : "Available"
                            : es
                              ? "No disponible"
                              : "Unavailable"
                        }
                      />
                    </td>
                    <td>{model.audience.join(", ") || "—"}</td>
                    <td>
                      <div className="model-default-controls">
                        {(["guest", "user"] as const).map((actorType) => {
                          const selected = model.defaultFor.includes(actorType);
                          const actorLabel =
                            actorType === "guest"
                              ? es
                                ? "Invitados"
                                : "Guests"
                              : es
                                ? "Usuarios"
                                : "Users";
                          return (
                            <button
                              key={actorType}
                              type="button"
                              className="model-default-button"
                              aria-pressed={selected}
                              disabled={
                                busy ||
                                !model.enabled ||
                                !model.available ||
                                !model.supported ||
                                selected
                              }
                              title={
                                !model.enabled
                                  ? es
                                    ? "Expón el modelo antes de seleccionarlo."
                                    : "Expose the model before selecting it."
                                  : undefined
                              }
                              onClick={() => void makeDefault(model, actorType)}
                            >
                              {actorLabel}
                            </button>
                          );
                        })}
                      </div>
                    </td>
                    <td>
                      <div className="model-exposure-control">
                        <label
                          className="switch-control"
                          title={
                            model.defaultFor.length > 0
                              ? es
                                ? "Selecciona otro modelo predeterminado antes de desactivarlo."
                                : "Select another default model before disabling this one."
                              : undefined
                          }
                        >
                          <input
                            type="checkbox"
                            checked={model.enabled}
                            disabled={
                              busy ||
                              (!model.enabled && (!model.available || !model.supported)) ||
                              model.defaultFor.length > 0
                            }
                            onChange={(event) =>
                              void updateModel(model, { enabled: event.target.checked })
                            }
                          />
                          <span />
                          <b className="sr-only">Exponer {model.name}</b>
                        </label>
                        {model.defaultFor.length > 0 ? (
                          <small>
                            {es ? "Cambia el predeterminado primero" : "Change the default first"}
                          </small>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}
      {tab === "settings" ? (
        <section className="admin-panel">
          <div className="admin-toolbar">
            <div>
              <h2>{es ? "Opciones en ejecución" : "Runtime settings"}</h2>
              <p>
                {es
                  ? "Los cambios se aplican sin reiniciar la aplicación."
                  : "Changes apply without restarting the application."}
              </p>
            </div>
          </div>
          <div className="setting-editor-list">
            {settings.map((setting) => (
              <SettingEditor
                key={`${setting.key}:${setting.version}`}
                setting={setting}
                busy={busy}
                locale={locale}
                onSave={(value) => void updateSetting(setting, value)}
              />
            ))}
          </div>
        </section>
      ) : null}
      {tab === "users" ? (
        <section className="admin-panel">
          <div className="admin-toolbar">
            <div>
              <h2>{es ? "Usuarios" : "Users"}</h2>
              <p>
                {es
                  ? "Busca por nombre o correo y administra roles."
                  : "Search by name or email and manage roles."}
              </p>
            </div>
            <input
              className="admin-search"
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={es ? "Buscar usuarios" : "Search users"}
            />
          </div>
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>{es ? "Usuario" : "User"}</th>
                  <th>{es ? "Estado" : "Status"}</th>
                  <th>{es ? "Rol" : "Role"}</th>
                  <th>{es ? "Alta" : "Joined"}</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={user.id}>
                    <td>
                      <strong>{user.displayName}</strong>
                      <span>{user.email}</span>
                    </td>
                    <td>
                      <StatusLabel ok={user.status === "active"} label={user.status} />
                    </td>
                    <td>
                      <select
                        value={user.role}
                        disabled={busy}
                        onChange={(event) =>
                          void updateRole(user, event.target.value as AdminUser["role"])
                        }
                      >
                        <option value="user">{es ? "Usuario" : "User"}</option>
                        <option value="admin">{es ? "Administrador" : "Administrator"}</option>
                      </select>
                    </td>
                    <td>
                      {new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(
                        new Date(user.createdAt),
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}
      {tab === "usage" ? (
        <section className="admin-panel">
          <div className="admin-toolbar">
            <div>
              <h2>{es ? "Uso agregado" : "Aggregate usage"}</h2>
              <p>
                {es
                  ? "Ventana de 30 días, sin mensajes ni datos identificables."
                  : "30-day window without messages or identifying data."}
              </p>
            </div>
          </div>
          <div className="usage-stats">
            <Metric
              label={es ? "Generaciones" : "Generations"}
              value={usage?.generations ?? 0}
              locale={locale}
            />
            <Metric
              label={es ? "Tokens de entrada" : "Input tokens"}
              value={usage?.inputTokens ?? 0}
              locale={locale}
            />
            <Metric
              label={es ? "Tokens de salida" : "Output tokens"}
              value={usage?.outputTokens ?? 0}
              locale={locale}
            />
            <Metric
              label={es ? "Costo estimado" : "Estimated cost"}
              locale={locale}
              value={new Intl.NumberFormat(locale, {
                style: "currency",
                currency: usage?.currency ?? "USD",
              }).format(usage?.estimatedCost ?? 0)}
            />
          </div>
        </section>
      ) : null}
      {tab === "audit" ? (
        <section className="admin-panel">
          <div className="admin-toolbar">
            <div>
              <h2>{es ? "Auditoría" : "Audit"}</h2>
              <p>
                {es
                  ? "Cambios administrativos, sin secretos ni contenido de chat."
                  : "Administrative changes without secrets or chat content."}
              </p>
            </div>
          </div>
          <div className="audit-list">
            {audit.map((event) => (
              <div key={event.id}>
                <ShieldCheck />
                <div>
                  <strong>{event.action}</strong>
                  <span>
                    {event.targetType} · {event.targetId}
                  </span>
                </div>
                <time dateTime={event.occurredAt}>
                  {new Intl.DateTimeFormat(locale, {
                    dateStyle: "medium",
                    timeStyle: "short",
                  }).format(new Date(event.occurredAt))}
                </time>
              </div>
            ))}
          </div>
        </section>
      ) : null}
    </PageShell>
  );
}

function StatusLabel({ ok, label }: { ok: boolean; label: string }) {
  return <span className={`status-label ${ok ? "status-label--ok" : ""}`}>{label}</span>;
}

function SettingEditor({
  setting,
  busy,
  locale,
  onSave,
}: {
  setting: RuntimeSetting;
  busy: boolean;
  locale: "es" | "en";
  onSave: (value: unknown) => void;
}) {
  const serialize = (value: unknown) => (typeof value === "string" ? value : JSON.stringify(value));
  const [value, setValue] = useState(serialize(setting.value));
  const isBoolean = typeof setting.value === "boolean";

  return (
    <div className="setting-editor">
      <div>
        <strong>{setting.key}</strong>
        <span>
          {locale === "es" ? "Versión" : "Version"} {setting.version}
        </span>
      </div>
      {isBoolean ? (
        <label className="switch-control">
          <input
            type="checkbox"
            checked={value === "true"}
            disabled={busy}
            onChange={(event) => {
              const next = String(event.target.checked);
              setValue(next);
              onSave(event.target.checked);
            }}
          />
          <span />
          <b className="sr-only">Cambiar {setting.key}</b>
        </label>
      ) : (
        <>
          {typeof setting.value === "string" && setting.key.includes("prompt") ? (
            <textarea value={value} onChange={(event) => setValue(event.target.value)} />
          ) : (
            <input value={value} onChange={(event) => setValue(event.target.value)} />
          )}
          <Button
            variant="outline"
            size="icon"
            disabled={busy}
            aria-label={`Guardar ${setting.key}`}
            onClick={() => {
              let parsed: unknown = value;
              if (typeof setting.value === "number") parsed = Number(value);
              if (Array.isArray(setting.value)) {
                try {
                  parsed = JSON.parse(value);
                } catch {
                  return;
                }
              }
              onSave(parsed);
            }}
          >
            <Save />
          </Button>
        </>
      )}
    </div>
  );
}

function Metric({
  label,
  value,
  locale,
}: {
  label: string;
  value: string | number;
  locale: "es" | "en";
}) {
  return (
    <div>
      <span>{label}</span>
      <strong>
        {typeof value === "number" ? new Intl.NumberFormat(locale).format(value) : value}
      </strong>
    </div>
  );
}
