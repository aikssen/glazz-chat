"use client";

import {
  ChevronDown,
  CircleAlert,
  Menu,
  MessageCircle,
  Moon,
  PanelLeftOpen,
  Sun,
  Wifi,
  WifiOff,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { APIError, API_URL, api, websocketURL } from "@/lib/api";
import { selectedModelUnavailable, usagePresentation } from "@/lib/chat-presentation";
import { dictionary } from "@/lib/i18n";
import { clientEvent } from "@/lib/realtime-client";
import { appendDelta, finishAssistant, startAssistant } from "@/lib/streaming-reducer";
import type { Conversation, CurrentUser, GuestAllowance, Message, Model, Usage } from "@/lib/types";
import { useDialogFocus } from "@/lib/use-dialog-focus";
import { newUUID } from "@/lib/uuid";
import { ChatComposer } from "./chat-composer";
import { ChatTranscript } from "./chat-transcript";
import { ConversationSidebar } from "./conversation-sidebar";

type Connection = "connecting" | "connected" | "offline";
const sessionRecoveryKey = "glazz-session-recovery";

type ServerEvent = {
  type: string;
  sequence?: number;
  requestId?: string;
  payload: Record<string, unknown>;
};

const conversationPageSize = 20;

export function ChatApp() {
  const { locale, setLocale, theme, setTheme } = usePreferences();
  const t = dictionary(locale);
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [allowance, setAllowance] = useState<GuestAllowance | null>(null);
  const [usage, setUsage] = useState<Usage | null>(null);
  const [models, setModels] = useState<Model[]>([]);
  const [defaultModel, setDefaultModel] = useState("");
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [conversationCursor, setConversationCursor] = useState<string | null>(null);
  const [loadingMoreConversations, setLoadingMoreConversations] = useState(false);
  const [conversationID, setConversationID] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [sidebar, setSidebar] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [search, setSearch] = useState("");
  const [connection, setConnection] = useState<Connection>("connecting");
  const [error, setError] = useState("");
  const [maintenance, setMaintenance] = useState(false);
  const [ready, setReady] = useState(false);
  const [reconnectAttempt, setReconnectAttempt] = useState(0);
  const [loginDialog, setLoginDialog] = useState(false);
  const [termsAccepted, setTermsAccepted] = useState(false);
  const [privacyAccepted, setPrivacyAccepted] = useState(false);
  const [generation, setGeneration] = useState<{
    id: string;
    conversationId: string;
  } | null>(null);
  const socket = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<number | null>(null);
  const lastSequence = useRef(0);
  const conversationIDRef = useRef("");
  const pendingCommands = useRef(new Map<string, "chat.generate" | "chat.cancel">());
  const loginDialogRef = useRef<HTMLDivElement>(null);
  const loginTriggerRef = useRef<HTMLElement>(null);
  const closeLoginDialog = useCallback(() => {
    setLoginDialog(false);
    window.requestAnimationFrame(() => loginTriggerRef.current?.focus());
  }, []);
  useDialogFocus(loginDialog, loginDialogRef, closeLoginDialog, loginTriggerRef);

  const openLoginDialog = useCallback(() => {
    loginTriggerRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setLoginDialog(true);
  }, []);

  useEffect(() => {
    const location = new URL(window.location.href);
    if (location.searchParams.get("authError") !== "access_denied") return;
    queueMicrotask(() =>
      setError(
        locale === "es"
          ? "Cancelaste el acceso con Google. Puedes continuar como invitado o intentarlo de nuevo."
          : "You cancelled Google access. You can continue as a guest or try again.",
      ),
    );
    location.searchParams.delete("authError");
    window.history.replaceState({}, "", `${location.pathname}${location.search}`);
  }, [locale]);

  const loadConversation = useCallback(async (id: string) => {
    setConversationID(id);
    setMessages([]);
    setSidebar(false);
    const page = await api<{ items: Message[] }>(`/api/v1/conversations/${id}/messages?limit=100`);
    setMessages([...page.items].sort((a, b) => a.sequence - b.sequence));
  }, []);

  const refreshLists = useCallback(async () => {
    const [conversationPage, currentUsage] = await Promise.all([
      api<{ items: Conversation[]; nextCursor?: string | null }>(
        `/api/v1/conversations?limit=${conversationPageSize}${
          search ? `&query=${encodeURIComponent(search)}` : ""
        }`,
      ),
      api<Usage>("/api/v1/usage"),
    ]);
    setConversations(conversationPage.items);
    setConversationCursor(conversationPage.nextCursor ?? null);
    setUsage(currentUsage);
    if (!user) {
      const current = await api<GuestAllowance>("/api/v1/guest-sessions/current");
      setAllowance(current);
    }
  }, [search, user]);

  async function loadMoreConversations() {
    if (!conversationCursor || loadingMoreConversations) return;
    setLoadingMoreConversations(true);
    try {
      const page = await api<{ items: Conversation[]; nextCursor?: string | null }>(
        `/api/v1/conversations?limit=${conversationPageSize}&after=${encodeURIComponent(
          conversationCursor,
        )}${search ? `&query=${encodeURIComponent(search)}` : ""}`,
      );
      setConversations((current) => {
        const known = new Set(current.map((conversation) => conversation.id));
        return [...current, ...page.items.filter((conversation) => !known.has(conversation.id))];
      });
      setConversationCursor(page.nextCursor ?? null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Conversations could not be loaded");
    } finally {
      setLoadingMoreConversations(false);
    }
  }

  const handleEvent = useCallback(
    (event: ServerEvent) => {
      if (event.sequence) lastSequence.current = Math.max(lastSequence.current, event.sequence);
      if (event.type === "heartbeat.ping") {
        const pong = clientEvent("heartbeat.pong", {
          heartbeatId: String(event.payload.heartbeatId ?? ""),
        });
        socket.current?.send(JSON.stringify(pong));
        return;
      }
      if (event.type === "command.acknowledged") {
        pendingCommands.current.delete(String(event.payload.commandEventId ?? ""));
        return;
      }
      if (event.type === "command.rejected") {
        const commandEventID = String(event.payload.commandEventId ?? "");
        const command = pendingCommands.current.get(commandEventID);
        pendingCommands.current.delete(commandEventID);
        if (command !== "chat.generate") {
          return;
        }
        setGeneration(null);
        const code = String(event.payload.code ?? "");
        setError(
          locale === "es"
            ? code === "quota_exceeded"
              ? "Alcanzaste el límite disponible. Revisa tu consumo e inténtalo después."
              : code === "concurrency_exceeded"
                ? "Ya tienes una respuesta en curso. Espera a que termine e inténtalo de nuevo."
                : "No fue posible iniciar la respuesta. Inténtalo de nuevo."
            : code === "quota_exceeded"
              ? "You reached the available limit. Check your usage and try again later."
              : code === "concurrency_exceeded"
                ? "You already have a response in progress. Wait for it to finish and try again."
                : "The response could not be started. Try again.",
        );
        void refreshLists();
        return;
      }
      if (event.type === "connection.resync_required") {
        void refreshLists();
        if (conversationIDRef.current) {
          void loadConversation(conversationIDRef.current);
        }
        return;
      }
      if (event.type === "conversation.updated") {
        void refreshLists();
        return;
      }
      if (event.type === "chat.started") {
        const assistantID = String(event.payload.assistantMessageId);
        const generationID = String(event.payload.generationId);
        const targetConversation = String(event.payload.conversationId);
        setGeneration({ id: generationID, conversationId: targetConversation });
        setMessages((current) =>
          startAssistant(current, {
            assistantMessageId: assistantID,
            generationId: generationID,
            conversationId: targetConversation,
          }),
        );
      }
      if (event.type === "chat.delta") {
        const generationID = String(event.payload.generationId);
        const text = String(event.payload.text ?? "");
        setMessages((current) =>
          appendDelta(current, {
            generationId: generationID,
            offset: Number(event.payload.offset),
            text,
          }),
        );
      }
      if (["chat.completed", "chat.cancelled", "chat.failed"].includes(event.type)) {
        const generationID = String(event.payload.generationId);
        const status =
          event.type === "chat.completed"
            ? "complete"
            : event.type === "chat.cancelled"
              ? "cancelled"
              : "failed";
        setMessages((current) => finishAssistant(current, generationID, status));
        setGeneration(null);
        void refreshLists();
      }
    },
    [loadConversation, locale, refreshLists],
  );

  useEffect(() => {
    conversationIDRef.current = conversationID;
  }, [conversationID]);

  useEffect(() => {
    const viewport = window.visualViewport;
    const updateHeight = () => {
      document.documentElement.style.setProperty(
        "--app-height",
        `${Math.round(viewport?.height ?? window.innerHeight)}px`,
      );
    };
    updateHeight();
    viewport?.addEventListener("resize", updateHeight);
    viewport?.addEventListener("scroll", updateHeight);
    window.addEventListener("resize", updateHeight);
    return () => {
      viewport?.removeEventListener("resize", updateHeight);
      viewport?.removeEventListener("scroll", updateHeight);
      window.removeEventListener("resize", updateHeight);
      document.documentElement.style.removeProperty("--app-height");
    };
  }, []);

  const connect = useCallback(async () => {
    try {
      const ticket = await api<{ ticket: string }>("/api/v1/auth/ws-ticket", {
        method: "POST",
      });
      setConnection("connecting");
      const next = new WebSocket(websocketURL(ticket.ticket));
      socket.current = next;
      next.onopen = () => {
        window.sessionStorage.removeItem(sessionRecoveryKey);
        setConnection("connected");
        if (lastSequence.current > 0) {
          next.send(
            JSON.stringify(
              clientEvent("connection.resume", { lastSequence: lastSequence.current }),
            ),
          );
        }
      };
      next.onmessage = (message) => {
        try {
          handleEvent(JSON.parse(String(message.data)) as ServerEvent);
        } catch {
          setError(
            locale === "es" ? "Se recibió un evento inválido." : "An invalid event was received.",
          );
        }
      };
      next.onclose = () => {
        setConnection("offline");
        reconnectTimer.current = window.setTimeout(
          () => setReconnectAttempt((value) => value + 1),
          1500,
        );
      };
    } catch (cause) {
      if (
        cause instanceof APIError &&
        [401, 403].includes(cause.status) &&
        !window.sessionStorage.getItem(sessionRecoveryKey)
      ) {
        window.sessionStorage.setItem(sessionRecoveryKey, "attempted");
        try {
          await api<void>("/api/v1/auth/refresh", { method: "POST" });
        } catch {
          // The refresh response clears cookies signed by an obsolete deployment key.
        }
        window.location.reload();
        return;
      }
      setConnection("offline");
      reconnectTimer.current = window.setTimeout(
        () => setReconnectAttempt((value) => value + 1),
        2500,
      );
    }
  }, [handleEvent, locale]);

  useEffect(() => {
    let active = true;
    async function bootstrap() {
      try {
        const config = await api<{
          maintenance: boolean;
          guestPolicy: { messageLimit: number; outputTokenLimit: number };
        }>("/api/v1/config/public");
        if (!active) return;
        setMaintenance(config.maintenance);
        let currentUser: CurrentUser | null = null;
        try {
          currentUser = await api<CurrentUser>("/api/v1/me");
        } catch (cause) {
          if (cause instanceof APIError && cause.status === 401) {
            try {
              await api<void>("/api/v1/auth/refresh", { method: "POST" });
              currentUser = await api<CurrentUser>("/api/v1/me");
            } catch {
              currentUser = null;
            }
          }
        }
        if (!currentUser) {
          const guest = await api<GuestAllowance>("/api/v1/guest-sessions", {
            method: "POST",
          });
          if (!active) return;
          setAllowance(guest);
        }
        if (currentUser) setLocale(currentUser.locale);
        setUser(currentUser);
        const [catalog, conversationPage, currentUsage] = await Promise.all([
          api<{ items: Model[]; defaultModelId: string }>("/api/v1/models"),
          api<{ items: Conversation[]; nextCursor?: string | null }>(
            `/api/v1/conversations?limit=${conversationPageSize}`,
          ),
          api<Usage>("/api/v1/usage"),
        ]);
        if (!active) return;
        setModels(catalog.items);
        setDefaultModel(catalog.defaultModelId);
        setConversations(conversationPage.items);
        setConversationCursor(conversationPage.nextCursor ?? null);
        setUsage(currentUsage);
        const requestedConversation = new URLSearchParams(window.location.search).get(
          "conversation",
        );
        const first =
          conversationPage.items.find((item) => item.id === requestedConversation) ??
          conversationPage.items[0];
        if (first) await loadConversation(first.id);
        setReady(true);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : "Startup failed");
        setReady(true);
      }
    }
    void bootstrap();
    return () => {
      active = false;
      socket.current?.close();
      if (reconnectTimer.current) window.clearTimeout(reconnectTimer.current);
    };
  }, [loadConversation, setLocale]);

  useEffect(() => {
    if (!ready) return;
    const timer = window.setTimeout(() => void connect(), 0);
    return () => {
      window.clearTimeout(timer);
      if (socket.current) {
        socket.current.onclose = null;
        socket.current.close();
      }
      if (reconnectTimer.current) window.clearTimeout(reconnectTimer.current);
    };
  }, [connect, ready, reconnectAttempt]);

  useEffect(() => {
    function handleOffline() {
      setConnection("offline");
      if (socket.current) {
        socket.current.onclose = null;
        socket.current.close();
        socket.current = null;
      }
    }
    function handleOnline() {
      if (reconnectTimer.current) {
        window.clearTimeout(reconnectTimer.current);
        reconnectTimer.current = null;
      }
      setConnection("connecting");
      setReconnectAttempt((value) => value + 1);
    }
    window.addEventListener("offline", handleOffline);
    window.addEventListener("online", handleOnline);
    return () => {
      window.removeEventListener("offline", handleOffline);
      window.removeEventListener("online", handleOnline);
    };
  }, []);

  useEffect(() => {
    if (!ready) return;
    const timer = window.setTimeout(() => void refreshLists(), 250);
    return () => window.clearTimeout(timer);
  }, [search, ready, refreshLists]);

  async function createConversation() {
    setError("");
    const conversation = await api<Conversation>("/api/v1/conversations", {
      method: "POST",
      body: JSON.stringify(defaultModel ? { modelId: defaultModel } : {}),
    });
    setConversations((current) => [conversation, ...current]);
    setConversationID(conversation.id);
    setMessages([]);
    setSidebar(false);
    return conversation;
  }

  async function updateConversation(id: string, patch: Record<string, unknown>) {
    const updated = await api<Conversation>(`/api/v1/conversations/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    });
    setConversations((current) => current.map((item) => (item.id === updated.id ? updated : item)));
  }

  async function renameConversation(conversation: Conversation) {
    const title = window.prompt(
      locale === "es" ? "Nuevo nombre de la conversación" : "New conversation name",
      conversation.title,
    );
    if (!title?.trim() || title.trim() === conversation.title) return;
    await updateConversation(conversation.id, { title: title.trim() });
  }

  async function archiveConversation(conversation: Conversation) {
    await updateConversation(conversation.id, {
      archived: conversation.status !== "archived",
    });
  }

  async function deleteConversation(conversation: Conversation) {
    const confirmed = window.confirm(
      locale === "es"
        ? `¿Eliminar "${conversation.title}"? Esta acción no se puede deshacer.`
        : `Delete "${conversation.title}"? This cannot be undone.`,
    );
    if (!confirmed) return;
    await api<void>(`/api/v1/conversations/${conversation.id}`, { method: "DELETE" });
    const remaining = conversations.filter((item) => item.id !== conversation.id);
    setConversations(remaining);
    if (conversationID === conversation.id) {
      setConversationID("");
      setMessages([]);
      if (remaining[0]) await loadConversation(remaining[0].id);
    }
  }

  async function send(content: string) {
    try {
      setError("");
      const conversation =
        conversations.find((item) => item.id === conversationID) ?? (await createConversation());
      const userMessage: Message = {
        id: newUUID(),
        conversationId: conversation.id,
        role: "user",
        content,
        status: "complete",
        sequence: messages.length + 1,
        createdAt: new Date().toISOString(),
      };
      setMessages((current) => [...current, userMessage]);
      const event = clientEvent("chat.generate", {
        conversationId: conversation.id,
        content,
      });
      pendingCommands.current.set(event.eventId, "chat.generate");
      socket.current?.send(JSON.stringify(event));
      return true;
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Message failed");
      return false;
    }
  }

  function stop() {
    if (!generation) return;
    const event = clientEvent("chat.cancel", {
      conversationId: generation.conversationId,
      generationId: generation.id,
    });
    pendingCommands.current.set(event.eventId, "chat.cancel");
    socket.current?.send(JSON.stringify(event));
  }

  async function retry() {
    if (!conversationID) return;
    try {
      await api(`/api/v1/conversations/${conversationID}/retry`, {
        method: "POST",
        headers: { "Idempotency-Key": `retry-${newUUID()}` },
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Retry failed");
    }
  }

  function login() {
    const returnTo = encodeURIComponent(conversationID ? `/?conversation=${conversationID}` : "/");
    window.location.assign(
      `${API_URL}/api/v1/auth/google/start?termsAccepted=true&privacyAccepted=true&returnTo=${returnTo}`,
    );
  }

  const selected = conversations.find((item) => item.id === conversationID);
  const usageState = usagePresentation(usage);
  const exhausted = allowance?.exhausted || usageState.exhausted;
  const disabled = maintenance || connection !== "connected" || Boolean(exhausted);
  const { remainingMessages, warning: usageWarning } = usageState;
  const reason = maintenance
    ? locale === "es"
      ? "El servicio está en mantenimiento. Tu borrador se conservará."
      : "The service is under maintenance. Your draft will be preserved."
    : connection !== "connected"
      ? connection === "connecting"
        ? t.reconnecting
        : t.offline
      : exhausted
        ? locale === "es"
          ? `Alcanzaste el límite disponible${
              usage?.messages.resetAt
                ? `. Se restablece ${new Intl.DateTimeFormat(locale, {
                    dateStyle: "medium",
                    timeStyle: "short",
                  }).format(new Date(usage.messages.resetAt))}`
                : "."
            }`
          : `You reached the available limit${
              usage?.messages.resetAt
                ? `. It resets ${new Intl.DateTimeFormat(locale, {
                    dateStyle: "medium",
                    timeStyle: "short",
                  }).format(new Date(usage.messages.resetAt))}`
                : "."
            }`
        : "";

  return (
    <main className={`chat-shell ${sidebarCollapsed ? "chat-shell--collapsed" : ""}`}>
      <a className="skip-link" href="#chat-transcript">
        {locale === "es" ? "Saltar a la conversación" : "Skip to conversation"}
      </a>
      <ConversationSidebar
        open={sidebar}
        conversations={conversations}
        selected={conversationID}
        search={search}
        user={user}
        modalActive={loginDialog}
        onClose={() => {
          setSidebar(false);
          setSidebarCollapsed(true);
        }}
        onSearch={setSearch}
        onNew={() => void createConversation()}
        onSelect={(id) => void loadConversation(id)}
        onRename={(conversation) => void renameConversation(conversation)}
        onArchive={(conversation) => void archiveConversation(conversation)}
        onDelete={(conversation) => void deleteConversation(conversation)}
        hasMore={Boolean(conversationCursor)}
        loadingMore={loadingMoreConversations}
        onLoadMore={() => void loadMoreConversations()}
        onLogin={openLoginDialog}
      />
      <section className="chat-main">
        <header className="chat-topbar">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setSidebar(true);
              setSidebarCollapsed(false);
            }}
            aria-label={t.menu}
            title={t.menu}
          >
            {sidebarCollapsed ? <PanelLeftOpen /> : <Menu />}
          </Button>
          <div className="mobile-wordmark">Glazz</div>
          <label className="model-select">
            <span className="sr-only">{t.model}</span>
            <select
              value={selected?.modelId ?? defaultModel}
              disabled={!user || !selected || selected.generationState !== "idle"}
              onChange={async (event) => {
                if (!selected) return;
                const updated = await api<Conversation>(`/api/v1/conversations/${selected.id}`, {
                  method: "PATCH",
                  body: JSON.stringify({ modelId: event.target.value }),
                });
                setConversations((current) =>
                  current.map((item) => (item.id === updated.id ? updated : item)),
                );
              }}
            >
              {selected && selectedModelUnavailable(selected.modelId, models) ? (
                <option value={selected.modelId} disabled>
                  {locale === "es" ? "Modelo no disponible" : "Model unavailable"}
                </option>
              ) : null}
              {models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.name}
                </option>
              ))}
            </select>
            <ChevronDown aria-hidden="true" />
          </label>
          <div
            className={`connection connection--${connection}`}
            role="status"
            title={connection === "connected" ? t.connected : t.reconnecting}
          >
            {connection === "connected" ? <Wifi /> : <WifiOff />}
            <span>{connection === "connected" ? t.connected : t.reconnecting}</span>
          </div>
          {usage ? (
            <div
              className={`usage-pill ${usageWarning ? "usage-pill--warning" : ""}`}
              title={`${usage.outputTokens.used} / ${usage.outputTokens.limit}`}
              aria-label={
                locale === "es"
                  ? `${remainingMessages} mensajes disponibles`
                  : `${remainingMessages} messages available`
              }
            >
              <MessageCircle aria-hidden="true" />
              {remainingMessages}
            </div>
          ) : null}
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            aria-label={t.appearance}
            title={t.appearance}
          >
            {theme === "dark" ? <Sun /> : <Moon />}
          </Button>
        </header>
        {error ? (
          <div className="global-error" role="alert">
            <CircleAlert />
            <span>{error}</span>
            <button onClick={() => setError("")}>{t.close}</button>
          </div>
        ) : null}
        <div id="chat-transcript" className="chat-scroll" tabIndex={-1}>
          {!ready ? (
            <LoadingTranscript locale={locale} />
          ) : (
            <ChatTranscript messages={messages} streaming={Boolean(generation)} onRetry={retry} />
          )}
        </div>
        <footer className="chat-footer">
          {exhausted && !user ? (
            <div className="guest-gate">
              <div>
                <strong>
                  {locale === "es" ? "Continúa tu conversación" : "Continue your conversation"}
                </strong>
                <span>{t.preserve}</span>
              </div>
              <Button onClick={openLoginDialog}>
                <GoogleMark />
                {t.login}
              </Button>
            </div>
          ) : (
            <>
              <ChatComposer
                disabled={disabled}
                streaming={Boolean(generation)}
                reason={reason}
                onSend={send}
                onStop={stop}
              />
              {!user && allowance && allowance.messagesUsed > 0 ? (
                <p className="guest-allowance">
                  {Math.max(allowance.messageLimit - allowance.messagesUsed, 0)} {t.freeLeft}
                </p>
              ) : null}
            </>
          )}
        </footer>
      </section>
      {loginDialog ? (
        <div className="dialog-backdrop" role="presentation">
          <div
            ref={loginDialogRef}
            className="dialog login-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="login-title"
            tabIndex={-1}
          >
            <GoogleMark />
            <h2 id="login-title">{t.login}</h2>
            <p>
              {locale === "es"
                ? "Tu conversación actual se conservará al crear la cuenta."
                : "Your current conversation will be kept when the account is created."}
            </p>
            <label className="consent-check">
              <input
                type="checkbox"
                checked={termsAccepted}
                onChange={(event) => setTermsAccepted(event.target.checked)}
              />
              <span>
                {locale === "es" ? "Acepto los " : "I accept the "}
                <a href="/legal/terms" target="_blank">
                  {locale === "es" ? "Términos" : "Terms"}
                </a>
                {locale === "es"
                  ? " y confirmo que tengo 18 años."
                  : " and confirm I am 18 or older."}
              </span>
            </label>
            <label className="consent-check">
              <input
                type="checkbox"
                checked={privacyAccepted}
                onChange={(event) => setPrivacyAccepted(event.target.checked)}
              />
              <span>
                {locale === "es" ? "Acepto la " : "I accept the "}
                <a href="/legal/privacy" target="_blank">
                  {locale === "es" ? "Política de privacidad" : "Privacy Policy"}
                </a>
                .
              </span>
            </label>
            <div className="dialog-actions">
              <Button variant="ghost" onClick={closeLoginDialog}>
                {locale === "es" ? "Cancelar" : "Cancel"}
              </Button>
              <Button disabled={!termsAccepted || !privacyAccepted} onClick={login}>
                <GoogleMark />
                {t.login}
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </main>
  );
}

function LoadingTranscript({ locale }: { locale: "es" | "en" }) {
  return (
    <div
      className="transcript-skeleton"
      role="status"
      aria-label={locale === "es" ? "Cargando conversación" : "Loading conversation"}
    >
      <span />
      <span />
      <span />
    </div>
  );
}

function GoogleMark() {
  return (
    <span className="google-mark" aria-hidden="true">
      G
    </span>
  );
}
