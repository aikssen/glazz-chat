"use client";

import { AlertCircle, Bot, RotateCcw, UserRound } from "lucide-react";
import { useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";
import type { Message } from "@/lib/types";
import { MessageContent } from "./message-content";

export function ChatTranscript({
  messages,
  streaming,
  onRetry,
}: {
  messages: Message[];
  streaming: boolean;
  onRetry: () => void;
}) {
  const end = useRef<HTMLDivElement>(null);
  const { locale } = usePreferences();
  const t = dictionary(locale);

  useEffect(() => {
    end.current?.scrollIntoView({ block: "end", behavior: streaming ? "auto" : "smooth" });
  }, [messages, streaming]);

  if (messages.length === 0) {
    return (
      <div className="empty-chat">
        <div className="brand-mark" aria-hidden="true">
          G
        </div>
        <h1>{t.emptyTitle}</h1>
        <p>{t.emptyBody}</p>
      </div>
    );
  }

  return (
    <div className="transcript" aria-live="polite" aria-busy={streaming}>
      {messages.map((message) => (
        <article
          key={message.id}
          className={`message message--${message.role} ${
            message.role === "assistant" ? `message--${message.status}` : ""
          }`}
        >
          <div className="message__identity" aria-hidden="true">
            {message.role === "assistant" ? <Bot /> : <UserRound />}
          </div>
          <div className="message__body">
            <span className="sr-only">
              {message.role === "assistant" ? "Glazz" : locale === "es" ? "Tú" : "You"}
            </span>
            <MessageContent content={message.content || (streaming ? " " : "")} />
            {message.status === "failed" ? (
              <div className="message-error">
                <AlertCircle />
                <span>
                  {locale === "es"
                    ? "La respuesta se detuvo antes de terminar."
                    : "The response stopped before it finished."}
                </span>
                <Button variant="outline" size="sm" onClick={onRetry}>
                  <RotateCcw data-icon="inline-start" />
                  {t.retry}
                </Button>
              </div>
            ) : null}
          </div>
        </article>
      ))}
      <div ref={end} />
    </div>
  );
}
