"use client";

import { Check, Copy } from "lucide-react";
import { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Button } from "@/components/ui/button";
import { usePreferences } from "@/components/theme-provider";
import { dictionary } from "@/lib/i18n";
import { highlightSyntax } from "@/lib/syntax-highlight";

export function MessageContent({ content }: { content: string }) {
  return (
    <div className="message-content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href, children }) => (
            <a href={href} target="_blank" rel="noreferrer noopener">
              {children}
              <span className="sr-only"> (abre en una pestaña nueva)</span>
            </a>
          ),
          pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
          table: ({ children }) => (
            <div className="my-4 max-w-full overflow-x-auto">
              <table>{children}</table>
            </div>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

function CodeBlock({ children }: { children?: React.ReactNode }) {
  const [copied, setCopied] = useState(false);
  const { locale } = usePreferences();
  const t = dictionary(locale);
  const value = extractText(children);
  const language = findLanguage(children) || "code";
  const highlighted = highlightSyntax(value, language);

  async function copy() {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  }

  return (
    <div className="code-block">
      <div className="code-block__bar">
        <span>{language}</span>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={copy}
          aria-label={copied ? t.copied : t.copy}
          title={copied ? t.copied : t.copy}
        >
          {copied ? <Check /> : <Copy />}
        </Button>
      </div>
      <pre>
        {highlighted ? (
          <code
            className={`hljs language-${highlighted.language}`}
            dangerouslySetInnerHTML={{ __html: highlighted.html }}
          />
        ) : (
          children
        )}
      </pre>
    </div>
  );
}

function extractText(node: React.ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(extractText).join("");
  if (node && typeof node === "object" && "props" in node) {
    return extractText((node as React.ReactElement<{ children?: React.ReactNode }>).props.children);
  }
  return "";
}

function findLanguage(node: React.ReactNode) {
  if (!node || typeof node !== "object" || !("props" in node)) return "";
  const className = (node as React.ReactElement<{ className?: string }>).props.className ?? "";
  return className.match(/language-([\w-]+)/)?.[1] ?? "";
}
