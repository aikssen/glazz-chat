"use client";

import { LogIn, MessageSquare, Plus, Search, Settings } from "lucide-react";
import Link from "next/link";
import type { RefObject } from "react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";
import type { CurrentUser } from "@/lib/types";
import { GlazzMark } from "./glazz-brand";

export function NavigationRail({
  user,
  onNew,
  onCurrent,
  onHistory,
  historyTrigger,
  onLogin,
}: {
  user: CurrentUser | null;
  onNew: () => void;
  onCurrent: () => void;
  onHistory: () => void;
  historyTrigger: RefObject<HTMLButtonElement | null>;
  onLogin: () => void;
}) {
  const { locale } = usePreferences();
  const t = dictionary(locale);

  return (
    <nav
      className="navigation-rail"
      aria-label={locale === "es" ? "Navegación principal" : "Primary navigation"}
    >
      <Link className="rail-brand" href="/" aria-label="Glazz">
        <GlazzMark size={38} />
      </Link>
      <Button
        size="icon"
        className="rail-action rail-action--primary"
        onClick={onNew}
        aria-label={t.newChat}
        title={t.newChat}
      >
        <Plus />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="rail-action"
        onClick={onCurrent}
        aria-label={locale === "es" ? "Chat actual" : "Current chat"}
        title={locale === "es" ? "Chat actual" : "Current chat"}
      >
        <MessageSquare />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="rail-action"
        onClick={onHistory}
        ref={historyTrigger}
        aria-label={t.menu}
        title={t.menu}
      >
        <Search />
      </Button>
      <span className="rail-spacer" />
      <Button
        variant="ghost"
        size="icon"
        className="rail-action"
        render={<Link href={user?.role === "admin" ? "/admin" : "/settings"} />}
        aria-label={user?.role === "admin" ? t.admin : t.settings}
        title={user?.role === "admin" ? t.admin : t.settings}
      >
        <Settings />
      </Button>
      {!user ? (
        <Button
          variant="ghost"
          size="icon"
          className="rail-action"
          onClick={onLogin}
          aria-label={t.login}
          title={t.login}
        >
          <LogIn />
        </Button>
      ) : null}
    </nav>
  );
}
