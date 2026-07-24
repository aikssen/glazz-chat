"use client";

import { AlertCircle, ArrowDown, Bot, RotateCcw, UserRound } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";
import { streamAnnouncement } from "@/lib/stream-announcement";
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
  const [atBottom, setAtBottom] = useState(true);
  const [announcement, setAnnouncement] = useState("");
  const atBottomRef = useRef(true);
  const wasStreaming = useRef(false);
  const { locale } = usePreferences();
  const t = dictionary(locale);

  useEffect(() => {
    if (atBottomRef.current) {
      end.current?.scrollIntoView({ block: "end", behavior: streaming ? "auto" : "smooth" });
    }
  }, [messages, streaming]);

  useEffect(() => {
    const marker = end.current;
    const root = marker?.closest(".chat-scroll");
    if (!marker || !root) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        atBottomRef.current = entry.isIntersecting;
        setAtBottom(entry.isIntersecting);
      },
      { root, threshold: 0.75 },
    );
    observer.observe(marker);
    return () => observer.disconnect();
  }, [messages.length]);

  useEffect(() => {
    const lastAssistant = messages.findLast((message) => message.role === "assistant");
    const next = streamAnnouncement(wasStreaming.current, streaming, lastAssistant?.status, locale);
    wasStreaming.current = streaming;
    if (next) setAnnouncement(next);
  }, [locale, messages, streaming]);

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
    <div
      className="transcript"
      role="log"
      aria-label={locale === "es" ? "Transcripción de la conversación" : "Conversation transcript"}
      aria-live="off"
      aria-busy={streaming}
    >
      <p className="sr-only" role="status" aria-live="polite" aria-atomic="true">
        {announcement}
      </p>
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
      <div ref={end} className="transcript-end" />
      {!atBottom ? (
        <Button
          className="jump-latest"
          variant="outline"
          size="icon"
          onClick={() => {
            atBottomRef.current = true;
            end.current?.scrollIntoView({ behavior: "smooth", block: "end" });
          }}
          aria-label={locale === "es" ? "Ir al mensaje más reciente" : "Jump to latest message"}
          title={locale === "es" ? "Ir al mensaje más reciente" : "Jump to latest message"}
        >
          <ArrowDown />
        </Button>
      ) : null}
    </div>
  );
}
