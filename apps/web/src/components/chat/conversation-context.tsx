"use client";

import { Archive, PanelRightClose, SquarePen, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import type { Conversation, Message } from "@/lib/types";

type ContextTab = "outline" | "details";

export function ConversationContext({
  open,
  tab,
  conversation,
  messages,
  modelName,
  activeTurn,
  onTab,
  onClose,
  onTurn,
  onRename,
  onArchive,
  onDelete,
}: {
  open: boolean;
  tab: ContextTab;
  conversation?: Conversation;
  messages: Message[];
  modelName: string;
  activeTurn: number;
  onTab: (tab: ContextTab) => void;
  onClose: () => void;
  onTurn: (turn: number) => void;
  onRename: () => void;
  onArchive: () => void;
  onDelete: () => void;
}) {
  const { locale } = usePreferences();
  const userMessages = messages.filter((message) => message.role === "user");
  const formatDate = (value?: string) =>
    value
      ? new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(
          new Date(value),
        )
      : "—";

  return (
    <>
      {open ? (
        <button
          className="context-backdrop"
          aria-label={locale === "es" ? "Cerrar contexto" : "Close context"}
          onClick={onClose}
        />
      ) : null}
      <aside
        className={`context-lane ${open ? "context-lane--open" : ""}`}
        aria-label={locale === "es" ? "Contexto de conversación" : "Conversation context"}
      >
        <div className="context-header">
          <span className="context-eyebrow">CONVERSATION</span>
          <strong className="context-title">
            {conversation?.title ?? (locale === "es" ? "Nueva conversación" : "New conversation")}
          </strong>
          <span className="context-model">{modelName}</span>
          <Button
            variant="ghost"
            size="icon-sm"
            className="context-collapse"
            onClick={onClose}
            aria-label={locale === "es" ? "Cerrar contexto" : "Close context"}
            title={locale === "es" ? "Cerrar contexto" : "Close context"}
          >
            <PanelRightClose className="context-close-desktop" />
            <X className="context-close-mobile" />
          </Button>
          <div
            className="context-tabs"
            role="tablist"
            aria-label={locale === "es" ? "Vistas de contexto" : "Context views"}
          >
            <button role="tab" aria-selected={tab === "outline"} onClick={() => onTab("outline")}>
              OUTLINE
            </button>
            <button role="tab" aria-selected={tab === "details"} onClick={() => onTab("details")}>
              DETAILS
            </button>
          </div>
        </div>
        {tab === "outline" ? (
          <nav className="outline-list" aria-label="Outline">
            {!userMessages.length ? (
              <p className="context-empty">
                {locale === "es"
                  ? "Los turnos aparecerán aquí."
                  : "Conversation turns will appear here."}
              </p>
            ) : (
              userMessages.map((message, index) => {
                const label = outlineLabel(message.content, locale);
                const excerpt = outlineExcerpt(message.content, locale);
                return (
                  <button
                    key={message.id}
                    aria-current={activeTurn === index + 1 ? "location" : undefined}
                    onClick={() => onTurn(index + 1)}
                  >
                    <span>{String(index + 1).padStart(2, "0")}</span>
                    <span className="outline-copy">
                      <strong>{label}</strong>
                      {excerpt !== label ? <small>{excerpt}</small> : null}
                    </span>
                  </button>
                );
              })
            )}
          </nav>
        ) : (
          <div className="context-details">
            <dl>
              <div>
                <dt>{locale === "es" ? "Título" : "Title"}</dt>
                <dd>
                  {conversation?.title ??
                    (locale === "es" ? "Nueva conversación" : "New conversation")}
                </dd>
              </div>
              <div>
                <dt>{locale === "es" ? "Modelo" : "Model"}</dt>
                <dd>{modelName}</dd>
              </div>
              <div>
                <dt>{locale === "es" ? "Turnos" : "Turns"}</dt>
                <dd>{userMessages.length}</dd>
              </div>
              <div>
                <dt>{locale === "es" ? "Actualizada" : "Updated"}</dt>
                <dd>{formatDate(conversation?.updatedAt)}</dd>
              </div>
            </dl>
            {conversation ? (
              <div className="context-actions">
                <Button variant="outline" size="sm" onClick={onRename}>
                  <SquarePen data-icon="inline-start" />
                  {locale === "es" ? "Renombrar" : "Rename"}
                </Button>
                <Button variant="outline" size="sm" onClick={onArchive}>
                  <Archive data-icon="inline-start" />
                  {conversation.status === "archived"
                    ? locale === "es"
                      ? "Restaurar"
                      : "Restore"
                    : locale === "es"
                      ? "Archivar"
                      : "Archive"}
                </Button>
                <Button variant="destructive" size="sm" onClick={onDelete}>
                  <Trash2 data-icon="inline-start" />
                  {locale === "es" ? "Eliminar" : "Delete"}
                </Button>
              </div>
            ) : null}
          </div>
        )}
      </aside>
    </>
  );
}

function outlineExcerpt(content: string, locale: "es" | "en") {
  const clean = content
    .replace(/[#_*`>\n]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (!clean) return locale === "es" ? "Sin contenido" : "No content";
  return clean.length > 78 ? `${clean.slice(0, 77).trimEnd()}…` : clean;
}

function outlineLabel(content: string, locale: "es" | "en") {
  const clean = content
    .replace(/[#_*`>\n]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (!clean) return locale === "es" ? "Turno sin título" : "Untitled turn";
  return clean.length > 58 ? `${clean.slice(0, 57).trimEnd()}…` : clean;
}
