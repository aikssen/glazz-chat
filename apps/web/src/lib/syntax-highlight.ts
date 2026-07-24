import highlight from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import css from "highlight.js/lib/languages/css";
import go from "highlight.js/lib/languages/go";
import javascript from "highlight.js/lib/languages/javascript";
import json from "highlight.js/lib/languages/json";
import python from "highlight.js/lib/languages/python";
import sql from "highlight.js/lib/languages/sql";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";

highlight.registerLanguage("bash", bash);
highlight.registerLanguage("css", css);
highlight.registerLanguage("go", go);
highlight.registerLanguage("javascript", javascript);
highlight.registerLanguage("json", json);
highlight.registerLanguage("python", python);
highlight.registerLanguage("sql", sql);
highlight.registerLanguage("typescript", typescript);
highlight.registerLanguage("xml", xml);

const aliases: Record<string, string> = {
  html: "xml",
  js: "javascript",
  jsx: "javascript",
  py: "python",
  sh: "bash",
  shell: "bash",
  ts: "typescript",
  tsx: "typescript",
  zsh: "bash",
};

export function highlightSyntax(code: string, requestedLanguage: string) {
  const normalized = requestedLanguage.trim().toLowerCase();
  const language = aliases[normalized] ?? normalized;
  if (!language || !highlight.getLanguage(language)) return null;
  return {
    html: highlight.highlight(code, { language, ignoreIllegals: true }).value,
    language,
  };
}
