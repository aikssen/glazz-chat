"use client";

import { ArrowUp, Square } from "lucide-react";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";

export function ChatComposer({
  disabled,
  streaming,
  reason,
  onSend,
  onStop,
}: {
  disabled?: boolean;
  streaming: boolean;
  reason?: string;
  onSend: (content: string) => Promise<boolean>;
  onStop: () => void;
}) {
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const textarea = useRef<HTMLTextAreaElement>(null);
  const composing = useRef(false);
  const { locale } = usePreferences();
  const t = dictionary(locale);

  async function submit() {
    const content = draft.trim();
    if (!content || disabled || sending || streaming) return;
    setSending(true);
    const accepted = await onSend(content);
    if (accepted) {
      setDraft("");
      if (textarea.current) textarea.current.style.height = "";
    }
    setSending(false);
  }

  return (
    <div className="composer-wrap">
      <div className="composer">
        <label htmlFor="chat-message" className="sr-only">
          {t.composer}
        </label>
        <textarea
          ref={textarea}
          id="chat-message"
          rows={1}
          value={draft}
          disabled={disabled && !streaming}
          placeholder={t.composer}
          onCompositionStart={() => {
            composing.current = true;
          }}
          onCompositionEnd={() => {
            composing.current = false;
          }}
          onChange={(event) => {
            setDraft(event.target.value);
            event.currentTarget.style.height = "auto";
            event.currentTarget.style.height = `${Math.min(event.currentTarget.scrollHeight, 180)}px`;
          }}
          onKeyDown={(event) => {
            if (
              event.key === "Enter" &&
              !event.shiftKey &&
              !composing.current &&
              !event.nativeEvent.isComposing
            ) {
              event.preventDefault();
              void submit();
            }
          }}
        />
        <Button
          type="button"
          size="icon-lg"
          onClick={streaming ? onStop : () => void submit()}
          disabled={!streaming && (disabled || sending || !draft.trim())}
          aria-label={streaming ? t.stop : t.send}
          title={streaming ? t.stop : t.send}
          className={streaming ? "bg-foreground text-background hover:bg-foreground/80" : ""}
        >
          {streaming ? <Square className="fill-current" /> : <ArrowUp />}
        </Button>
      </div>
      {reason ? <p className="composer-reason">{reason}</p> : null}
    </div>
  );
}
