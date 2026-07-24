# Phase 9 Web Performance Audit

- Date: 2026-07-24
- Task: QA-009
- Result: Accepted
- Build: Next.js 16.2.11 production standalone

## Budgets and results

Playwright measures a fresh production page in Chromium and then lays out and
scrolls a deterministic 200-message transcript.

| Metric                    |          Budget | Mobile 375 | Wide 1440 |
| ------------------------- | --------------: | ---------: | --------: |
| Composer interactive      |      < 3,000 ms |   224.8 ms |  224.0 ms |
| First contentful paint    |      < 2,500 ms |      72 ms |     72 ms |
| CLS                       |           < 0.1 |     0.0311 |    0.0117 |
| Encoded JavaScript        | < 400,000 bytes |    238,867 |   238,867 |
| Encoded CSS               | < 100,000 bytes |     10,796 |    10,796 |
| 200-message layout/scroll |        < 250 ms |    17.0 ms |   13.7 ms |
| Horizontal overflow       |            None |       None |      None |
| Unexpected runtime errors |            None |       None |      None |

The gate is `apps/web/tests/e2e/performance.spec.ts`. It runs on the smallest and
largest supported viewports; the full visual matrix covers intermediate layout
breakpoints. Expected 401/403 resource messages from anonymous authentication and
administrator probes are excluded; page exceptions, hydration warnings, and other
console errors fail the gate.

## Dependency and rendering review

- Next.js statically prerenders all current routes.
- `next/font` self-hosts Outfit, Work Sans, and JetBrains Mono, avoiding runtime
  font-host connections and reducing font-driven layout shift.
- Markdown uses `react-markdown` plus GFM with explicit safe component rendering.
- No full syntax-highlighting catalog is shipped. `highlight.js` core registers
  only Bash, CSS, Go, JavaScript, JSON, Python, SQL, TypeScript, and XML.
- Adding real syntax highlighting increased encoded route JavaScript by 20,199
  bytes and kept the total at 59.7% of the 400 KB budget.
- Unknown code languages fall back to React plain-text rendering. Registered
  language output is tested to escape executable HTML.
- Lucide imports are per-icon and the UI has no charting, animation, editor, or
  general-purpose utility runtime.

The 200-message measurement is a deterministic DOM/layout stress case. Production
telemetry in M6 should add real-user Core Web Vitals and transcript-length
distributions; virtualization is not justified by the current measured budget.
