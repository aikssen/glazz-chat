"use client";

import { type RefObject, useLayoutEffect } from "react";

const focusableSelector = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"]):not([disabled])',
].join(",");

export function useDialogFocus(
  open: boolean,
  dialog: RefObject<HTMLElement | null>,
  onClose: () => void,
  returnFocus?: RefObject<HTMLElement | null>,
) {
  useLayoutEffect(() => {
    if (!open || !dialog.current) return;
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const focusReturnTarget = returnFocus?.current ?? previous;
    const element = dialog.current;
    const focusable = () => Array.from(element.querySelectorAll<HTMLElement>(focusableSelector));
    (focusable()[0] ?? element).focus();

    function handleKeyDown(event: KeyboardEvent) {
      if (element.inert || element.getAttribute("aria-hidden") === "true") return;
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }
      if (event.key !== "Tab") return;
      const items = focusable();
      if (items.length === 0) {
        event.preventDefault();
        element.focus();
        return;
      }
      const first = items[0];
      const last = items.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      focusReturnTarget?.focus();
    };
  }, [dialog, onClose, open, returnFocus]);
}
