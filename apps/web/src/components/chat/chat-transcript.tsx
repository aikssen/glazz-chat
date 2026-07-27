"use client";

import { AlertCircle, ArrowDown, RotateCcw } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";
import { streamAnnouncement } from "@/lib/stream-announcement";
import type { Message } from "@/lib/types";
import { GlazzMark } from "./glazz-brand";
import { MessageContent } from "./message-content";

type Turn = { user?: Message; assistants: Message[] };

export function ChatTranscript({
  title,
  messages,
  streaming,
  fallbackModelName,
  onActiveTurn,
  onRetry,
}: {
  title?: string;
  messages: Message[];
  streaming: boolean;
  fallbackModelName: string;
  onActiveTurn: (turn: number) => void;
  onRetry: () => void;
}) {
  const end = useRef<HTMLDivElement>(null);
  const [atBottom, setAtBottom] = useState(true);
  const [announcement, setAnnouncement] = useState("");
  const atBottomRef = useRef(true);
  const wasStreaming = useRef(false);
  const { locale } = usePreferences();
  const t = dictionary(locale);
  const turns = useMemo(() => groupTurns(messages), [messages]);

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
    const root = end.current?.closest(".chat-scroll");
    const nodes = root?.querySelectorAll<HTMLElement>("[data-turn]");
    if (!root || !nodes?.length) return;
    const visibility = new Map<number, number>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const turn = Number((entry.target as HTMLElement).dataset.turn);
          visibility.set(turn, entry.intersectionRatio);
        }
        const active = [...visibility.entries()].sort((a, b) => b[1] - a[1])[0];
        if (active?.[1] > 0) onActiveTurn(active[0]);
      },
      { root, rootMargin: "-18% 0px -52%", threshold: [0, 0.15, 0.4, 0.75] },
    );
    nodes.forEach((node) => observer.observe(node));
    return () => observer.disconnect();
  }, [onActiveTurn, turns.length]);

  useEffect(() => {
    const lastAssistant = messages.findLast((message) => message.role === "assistant");
    const next = streamAnnouncement(wasStreaming.current, streaming, lastAssistant?.status, locale);
    wasStreaming.current = streaming;
    if (next) setAnnouncement(next);
  }, [locale, messages, streaming]);

  if (messages.length === 0) {
    return (
      <div className="empty-chat">
        <GlazzMark size={48} />
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
      <h1 className="transcript-title">{title}</h1>
      <p className="sr-only" role="status" aria-live="polite" aria-atomic="true">
        {announcement}
      </p>
      {turns.map((turn, index) => {
        const turnNumber = index + 1;
        return (
          <section
            id={`turn-${turnNumber}`}
            className="conversation-turn"
            data-turn={turnNumber}
            key={turn.user?.id ?? turn.assistants[0]?.id ?? turnNumber}
          >
            <div className="turn-number" aria-hidden="true">
              {String(turnNumber).padStart(2, "0")}
            </div>
            <div className="turn-content">
              {turn.user ? (
                <article className="message message--user">
                  <div className="message__meta">{locale === "es" ? "TÚ" : "YOU"}</div>
                  <div className="message__body">
                    <MessageContent content={turn.user.content} />
                  </div>
                </article>
              ) : null}
              {turn.assistants.map((message) => (
                <article
                  key={message.id}
                  className={`message message--assistant message--${message.status}`}
                >
                  <div className="message__meta">
                    <strong>GLAZZ</strong>
                    <span>{message.modelName ?? fallbackModelName}</span>
                  </div>
                  <div className="message__body">
                    <MessageContent content={message.content || (streaming ? " " : "")} />
                    {message.status === "failed" || message.status === "cancelled" ? (
                      <div className="message-error">
                        <AlertCircle />
                        <span>
                          {message.status === "cancelled"
                            ? locale === "es"
                              ? "Cancelaste esta respuesta."
                              : "You cancelled this response."
                            : locale === "es"
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
            </div>
          </section>
        );
      })}
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

function groupTurns(messages: Message[]) {
  const turns: Turn[] = [];
  for (const message of messages) {
    if (message.role === "user") {
      turns.push({ user: message, assistants: [] });
      continue;
    }
    if (!turns.length) turns.push({ assistants: [] });
    turns[turns.length - 1].assistants.push(message);
  }
  return turns;
}
