"use client";

import {
  Archive,
  ArchiveRestore,
  Ellipsis,
  LogIn,
  MessageSquare,
  PanelLeftClose,
  Search,
  Settings,
  ShieldCheck,
  SquarePen,
  Trash2,
  X,
} from "lucide-react";
import Link from "next/link";
import { type RefObject, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";
import { useDialogFocus } from "@/lib/use-dialog-focus";
import type { Conversation, CurrentUser } from "@/lib/types";
import { GlazzWordmark } from "./glazz-brand";

export function ConversationSidebar({
  open,
  conversations,
  selected,
  search,
  user,
  modalActive,
  returnFocus,
  onClose,
  onSearch,
  onSelect,
  onRename,
  onArchive,
  onDelete,
  hasMore,
  loadingMore,
  onLoadMore,
  onLogin,
}: {
  open: boolean;
  conversations: Conversation[];
  selected?: string;
  search: string;
  user: CurrentUser | null;
  modalActive?: boolean;
  returnFocus: RefObject<HTMLElement | null>;
  onClose: () => void;
  onSearch: (value: string) => void;
  onSelect: (id: string) => void;
  onRename: (conversation: Conversation) => void;
  onArchive: (conversation: Conversation) => void;
  onDelete: (conversation: Conversation) => void;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  onLogin: () => void;
}) {
  const { locale } = usePreferences();
  const t = dictionary(locale);
  const sidebarRef = useRef<HTMLElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const [mobile, setMobile] = useState(false);
  const active = conversations.filter((item) => item.status === "active");
  const archived = conversations.filter((item) => item.status === "archived");
  const interactiveModal = open && !modalActive;
  useDialogFocus(interactiveModal, sidebarRef, onClose, returnFocus, searchRef);

  useEffect(() => {
    const query = window.matchMedia("(max-width: 1023px)");
    const update = () => setMobile(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);

  return (
    <>
      {open ? <button className="sidebar-backdrop" aria-label={t.close} onClick={onClose} /> : null}
      <aside
        ref={sidebarRef}
        className={`conversation-sidebar ${open ? "conversation-sidebar--open" : ""}`}
        role={interactiveModal ? "dialog" : undefined}
        aria-modal={interactiveModal ? "true" : undefined}
        aria-hidden={modalActive || (mobile && !interactiveModal) ? "true" : undefined}
        aria-label={locale === "es" ? "Conversaciones" : "Conversations"}
        tabIndex={mobile ? -1 : undefined}
        inert={modalActive || (mobile && !interactiveModal)}
      >
        <div className="sidebar-brand">
          <GlazzWordmark />
          <Button
            variant="ghost"
            size="icon"
            onClick={onClose}
            aria-label={t.close}
            title={t.close}
            className="lg:hidden"
          >
            <X />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={onClose}
            aria-label={t.close}
            title={t.close}
            className="hidden lg:inline-flex"
          >
            <PanelLeftClose />
          </Button>
        </div>
        <label className="sidebar-search">
          <span className="sr-only">{t.search}</span>
          <Search aria-hidden="true" />
          <input
            ref={searchRef}
            type="search"
            value={search}
            onChange={(event) => onSearch(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Escape") onClose();
              if (event.key !== "ArrowDown") return;
              event.preventDefault();
              sidebarRef.current
                ?.querySelector<HTMLButtonElement>(".conversation-item > button")
                ?.focus();
            }}
            placeholder={t.search}
          />
        </label>
        <nav className="conversation-nav" aria-label={t.search}>
          {!conversations.length ? (
            <p className="sidebar-empty" role="status">
              {search
                ? locale === "es"
                  ? "No hay conversaciones que coincidan."
                  : "No conversations match your search."
                : locale === "es"
                  ? "Tus conversaciones aparecerán aquí."
                  : "Your conversations will appear here."}
            </p>
          ) : null}
          <ConversationGroup
            label={t.today}
            items={active}
            selected={selected}
            locale={locale}
            onSelect={onSelect}
            actions={Boolean(user)}
            onRename={onRename}
            onArchive={onArchive}
            onDelete={onDelete}
          />
          {archived.length ? (
            <ConversationGroup
              label={t.archived}
              items={archived}
              selected={selected}
              locale={locale}
              onSelect={onSelect}
              archived
              actions={Boolean(user)}
              onRename={onRename}
              onArchive={onArchive}
              onDelete={onDelete}
            />
          ) : null}
          {hasMore ? (
            <Button
              className="sidebar-load-more"
              variant="outline"
              size="sm"
              disabled={loadingMore}
              onClick={onLoadMore}
            >
              {loadingMore
                ? locale === "es"
                  ? "Cargando..."
                  : "Loading..."
                : locale === "es"
                  ? "Cargar más"
                  : "Load more"}
            </Button>
          ) : null}
        </nav>
        <div className="sidebar-footer">
          {user ? (
            <>
              {user.role === "admin" ? (
                <Link href="/admin" className="sidebar-link">
                  <ShieldCheck />
                  {t.admin}
                </Link>
              ) : null}
              <Link href="/settings" className="sidebar-link">
                <Settings />
                {t.settings}
              </Link>
              <div className="sidebar-account">
                <Avatar user={user} />
                <span>
                  <strong>{user.displayName}</strong>
                  <small>{user.email}</small>
                </span>
              </div>
            </>
          ) : (
            <button className="sidebar-link" onClick={onLogin}>
              <LogIn />
              {t.login}
            </button>
          )}
        </div>
      </aside>
    </>
  );
}

function ConversationGroup({
  label,
  items,
  selected,
  locale,
  archived,
  actions,
  onSelect,
  onRename,
  onArchive,
  onDelete,
}: {
  label: string;
  items: Conversation[];
  selected?: string;
  locale: "es" | "en";
  archived?: boolean;
  actions: boolean;
  onSelect: (id: string) => void;
  onRename: (conversation: Conversation) => void;
  onArchive: (conversation: Conversation) => void;
  onDelete: (conversation: Conversation) => void;
}) {
  if (!items.length) return null;
  return (
    <section className="conversation-group">
      <h2>{label}</h2>
      {items.map((item) => (
        <div className="conversation-item" key={item.id}>
          <button
            onClick={() => onSelect(item.id)}
            aria-current={selected === item.id ? "page" : undefined}
          >
            {archived ? <Archive /> : <MessageSquare />}
            <span>{item.title}</span>
          </button>
          {actions ? (
            <details>
              <summary
                aria-label={`${locale === "es" ? "Opciones para" : "Options for"} ${item.title}`}
              >
                <Ellipsis />
              </summary>
              <div className="conversation-menu">
                <button onClick={() => onRename(item)}>
                  <SquarePen />
                  {locale === "es" ? "Renombrar" : "Rename"}
                </button>
                <button onClick={() => onArchive(item)}>
                  {archived ? <ArchiveRestore /> : <Archive />}
                  {archived
                    ? locale === "es"
                      ? "Restaurar"
                      : "Restore"
                    : locale === "es"
                      ? "Archivar"
                      : "Archive"}
                </button>
                <button className="conversation-menu__delete" onClick={() => onDelete(item)}>
                  <Trash2 />
                  {locale === "es" ? "Eliminar" : "Delete"}
                </button>
              </div>
            </details>
          ) : null}
        </div>
      ))}
    </section>
  );
}

function Avatar({ user }: { user: CurrentUser }) {
  if (user.avatarUrl) {
    // Google profile URLs are user-controlled remote media; keep a text fallback.
    return <span className="avatar">{user.displayName.slice(0, 1).toUpperCase()}</span>;
  }
  return <span className="avatar">{user.displayName.slice(0, 1).toUpperCase()}</span>;
}
