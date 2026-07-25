"use client";

import { LogOut, Settings, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";
import type { CurrentUser } from "@/lib/types";

export function AccountMenu({
  user,
  onLogout,
}: {
  user: CurrentUser;
  onLogout: () => Promise<void>;
}) {
  const { locale } = usePreferences();
  const t = dictionary(locale);
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const firstItem = root.current?.querySelector<HTMLElement>('[role="menuitem"]');
    firstItem?.focus();

    function closeOnPointer(event: PointerEvent) {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    }

    function closeOnEscape(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      setOpen(false);
      trigger.current?.focus();
    }

    document.addEventListener("pointerdown", closeOnPointer);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnPointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [open]);

  return (
    <div className="account-menu" ref={root}>
      <button
        ref={trigger}
        className="topbar-account"
        type="button"
        aria-label={`${t.account}: ${user.displayName}`}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
      >
        {user.displayName.slice(0, 1).toUpperCase()}
      </button>
      {open ? (
        <div className="account-popover" role="menu" aria-label={t.account}>
          <div className="account-popover__identity">
            <strong>{user.displayName}</strong>
            <span>{user.email}</span>
          </div>
          {user.role === "admin" ? (
            <Link href="/admin" role="menuitem">
              <ShieldCheck />
              {t.admin}
            </Link>
          ) : null}
          <Link href="/settings" role="menuitem">
            <Settings />
            {t.settings}
          </Link>
          <button type="button" role="menuitem" onClick={() => void onLogout()}>
            <LogOut />
            {t.logout}
          </button>
        </div>
      ) : null}
    </div>
  );
}
