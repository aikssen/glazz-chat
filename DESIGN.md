# Glazz Product Design

> This is the canonical interaction and visual specification. Product capabilities
> and implementation state are summarized in
> [`docs/product-capabilities.md`](./docs/product-capabilities.md).

## 1. Design thesis

Glazz is a public-facing AI chat, not a marketing page wrapped around a demo. The
first viewport is the usable conversation surface. Its interaction remains
immediately understandable, but its spatial model must be recognizably Glazz
rather than another wide-sidebar chat clone.

The experience should feel:

- Fresh, direct, and modern
- Calm while reading
- Energetic only at moments of action
- Dense enough for repeated use without feeling administrative
- Honest about connection, generation, limits, and errors

It should not look like:

- A purple AI template
- A collection of floating cards
- A dark blue developer dashboard
- A clone of ChatGPT
- A clone of Claude
- A terminal or telemetry console
- A marketing hero followed by the actual application

## 2. Audience and primary job

The audience is the general public across desktop and mobile. Users may not know
what tokens, context windows, or provider protocols are. Product language describes
what they can do and what happened, not how the backend works.

The primary job is:

> Ask a question and follow the response without losing context or control.

Secondary jobs are finding prior conversations, changing the model, understanding
remaining usage, managing sessions, and deleting the account.

## 3. Visual direction

### 3.1 Concept: Signal Workspace

Glazz uses cool graphite surfaces, ice-white text, and a restrained electric-cyan
signal. Cyan identifies focus, navigation position, primary action, and active
generation; it is never used as a decorative wash.

The product's signature is a numbered **conversation spine** connecting user turns
and assistant responses. A synchronized outline in the right context lane acts as
a minimap for long conversations. This creates a distinctive navigation model
without making the product feel like a terminal or operational dashboard.

A thin **live signal rail** marks the active assistant response. It remains
spatially stable during streaming and becomes a static completed-state indicator
without a typing gimmick, orb, or decorative animation.

### 3.2 Brand expression

- Product name: `Glazz`
- Brand mark: reuse the exact `G` geometry shown in the original orange Glazz UI,
  including its existing contrasting notch. Do not redraw, reinterpret, simplify,
  or replace it with a generic open `G`.
- Brand recoloring: ice-white primary geometry and electric-cyan notch. The
  wordmark remains `Glazz`/`GLAZZ` according to its existing asset.
- The mark has no glow, gradient, or permanent animation.
- Voice: concise, neutral, direct, warm without pretending to be a person.
- Icon family: Lucide, consistent 1.75-2px stroke.

## 4. Design tokens

All application colors are semantic CSS variables consumed through Tailwind and
shadcn/ui. Components do not hardcode palette values.

### 4.1 Light theme

| Token                  | Value     | Use                             |
| ---------------------- | --------- | ------------------------------- |
| `--background`         | `#F5FAFB` | Main cool-white canvas          |
| `--foreground`         | `#0C171A` | Primary text                    |
| `--surface`            | `#FFFFFF` | Menus, dialogs, composer        |
| `--surface-subtle`     | `#E9F1F3` | Prompt bands and hover          |
| `--border`             | `#C7D6DA` | Dividers and controls           |
| `--muted-foreground`   | `#52666D` | Secondary text                  |
| `--primary`            | `#007F8C` | Primary action and active state |
| `--primary-foreground` | `#FFFFFF` | Text on primary                 |
| `--brand-bright`       | `#00AFC0` | Brand notch and streaming rail  |
| `--success`            | `#447A00` | Confirmed healthy/completed     |
| `--warning`            | `#8A5A00` | Quota warning                   |
| `--destructive`        | `#B42332` | Destructive actions             |
| `--ring`               | `#007F8C` | Focus ring                      |

### 4.2 Dark theme

| Token                  | Value     | Use                       |
| ---------------------- | --------- | ------------------------- |
| `--background`         | `#07090A` | Main graphite canvas      |
| `--foreground`         | `#EAFBFF` | Primary ice-white text    |
| `--surface`            | `#11171A` | Menus, dialogs, composer  |
| `--surface-subtle`     | `#182125` | Prompt bands and hover    |
| `--border`             | `#263238` | Dividers and controls     |
| `--muted-foreground`   | `#8A9AA1` | Secondary text            |
| `--primary`            | `#10D7E8` | Primary action and signal |
| `--primary-foreground` | `#031012` | Text on primary           |
| `--brand-bright`       | `#10D7E8` | Brand notch/streaming     |
| `--success`            | `#B7F34A` | Confirmed completion      |
| `--warning`            | `#F3BE4F` | Quota warning             |
| `--destructive`        | `#FF5D68` | Destructive actions       |
| `--ring`               | `#10D7E8` | Focus ring                |

Validate all token combinations against WCAG 2.2 AA. Normal text requires at least
4.5:1; large text and meaningful UI graphics require at least 3:1. Color is never
the only status cue.

### 4.3 Typography

| Role           | Family         | Weights       | Notes                                                  |
| -------------- | -------------- | ------------- | ------------------------------------------------------ |
| Display/brand  | Outfit         | 500, 600, 700 | Restrained use in product name and major screen titles |
| Interface/body | Work Sans      | 400, 500, 600 | High legibility for messages and controls              |
| Code/data      | JetBrains Mono | 400, 500      | Code blocks, token/diagnostic values                   |

Use `next/font` or self-hosted assets to avoid layout shift and third-party runtime
font requests. Letter spacing is `0`; do not use negative tracking. Body text starts
at 16px with a 1.5-1.65 line height. Chat content measure is 68-76 characters.

Suggested scale:

| Token     | Size / line-height |
| --------- | ------------------ |
| `display` | 32 / 38            |
| `title`   | 24 / 32            |
| `heading` | 18 / 26            |
| `body`    | 16 / 26            |
| `ui`      | 14 / 20            |
| `caption` | 12 / 18            |

Do not scale type with viewport width.

### 4.4 Shape, spacing, and elevation

- Base spacing unit: 4px
- Common rhythm: 8, 12, 16, 24, 32, 48px
- Control height: 40px desktop, minimum 44px touch target
- Composer radius: 6px
- Dialog/card radius: 8px maximum; compact controls prefer 4-6px
- Message content is unframed; user messages may use a subtle surface band
- Shadows are limited to floating menus, dialogs, and the sticky composer
- Page sections are not floating cards

## 5. Information architecture

### 5.1 Desktop

```text
+------+-----------------------------------------+----------------------+
|Glazz | Top bar: connection | theme | account                          |
+------+-----------------------------------------+----------------------+
| [+]  | Conversation title                      | CONVERSATION      [<]|
| chat |                                         | OUTLINE | DETAILS    |
| find | 01 -- user prompt                       | 01 Topic             |
|      |  |   assistant response                 | 02 Active topic      |
| gear | 02 -- user prompt                       | 03 Topic             |
|      |  |   assistant response                 |                      |
|      |-----------------------------------------|                      |
|      | Ask Glazz...  model  guest usage [send]|                      |
+------+-----------------------------------------+----------------------+
```

The 72px navigation rail is stable and does not resize. Its first action is the
Lucide `Plus` icon for **New chat**; the top header must not duplicate this action.
Conversation history is not permanently visible. The search icon opens a
left-aligned searchable drawer and every unfamiliar icon has a tooltip.

The right context lane contains exactly two views: `Outline` and `Details`.
`Outline` is the default and dominant view. Its dense, independently scrollable
minimap links each numbered entry to a transcript turn and highlights the currently
visible turn. The collapse control is integrated inside the panel header. At
intermediate widths the lane collapses before reducing the transcript below its
readable measure.

### 5.2 Mobile

```text
+----------------------------------+
| Glazz          [history] [user]  |
|----------------------------------|
|                                  |
| 01 | transcript                  |
|                                  |
|                                  |
|----------------------------------|
| model  Ask Glazz...        [send]|
+----------------------------------+
```

New chat remains available as a `Plus` action in the compact mobile navigation.
Conversation search/history and the context lane open as separate sheets. The
composer respects the visual viewport, safe-area inset, and mobile keyboard. Scroll
content includes bottom space so the sticky composer never hides the last message.

## 6. Core screens

### 6.1 Guest chat

- The chat is immediately usable.
- Empty state contains one short prompt and optional starter suggestions, not a
  feature explanation.
- Remaining free messages appear once as subdued metadata near the composer after
  the first response. They are never a meter, panel section, or central visual.
- At exhaustion, keep the transcript visible and replace send capability with a
  focused Google sign-in gate.
- Explain that signing in preserves the current conversation.

### 6.2 Registered chat

- Conversation list groups recent and archived items.
- Search is a clear rail command with keyboard access. It opens a 360-400px drawer,
  focuses the query field, groups results by date, highlights title matches,
  supports arrow-key navigation, and closes with `Escape`.
- Model selector shows only enabled models and a plain-language description.
- Transcript gives user and assistant content distinct rhythm without surrounding
  every message with a card.
- Actions appear on focus/hover and remain reachable by keyboard/touch.

### 6.3 Settings

Use an unframed settings layout with sections for profile, appearance, language,
sessions, privacy, and account deletion. Destructive account deletion is visually
separate, requires recent authentication, and uses a confirmation dialog with
specific consequences.

### 6.4 Admin

Admin is a quiet operational surface, not an analytics landing page.

- Table/list views for models, users, settings, and audit events
- Segmented filters and explicit status labels
- Inline validation and confirmation for high-impact changes
- Usage trends only where a chart improves comparison
- No access to conversation bodies

## 7. Chat interaction model

### 7.1 Composer

- Multiline textarea with visible label available to assistive technology
- `Enter` sends; `Shift+Enter` creates a new line
- Mobile behavior must not assume a hardware keyboard
- Send uses a Lucide arrow icon with an accessible name
- During generation, send becomes a square stop icon without changing bounds
- Disabled reasons are visible: offline, maintenance, quota, or empty message
- Draft remains if submission fails before acknowledgement

### 7.2 Streaming

- Insert an assistant response shell after `chat.started`
- Apply deltas without causing transcript-wide reflow
- Show the live signal rail in cyan while streaming
- Transition the rail to a static completed state without moving layout
- Do not auto-scroll if the user has intentionally scrolled away
- Provide a jump-to-latest control when new content is below the viewport
- Announce start, completion, cancellation, and errors; do not make a screen reader
  announce every token
- Respect `prefers-reduced-motion`; rail changes color without animated travel

### 7.3 Message content

- Sanitize Markdown
- Render tables inside accessible horizontal overflow containers
- Provide language label and copy button for fenced code
- Never render raw model HTML
- External links open safely and clearly indicate destination
- Long unbroken strings wrap without widening the layout

### 7.4 Failure and recovery

Errors state what happened and the available action:

- Connection lost: "Connection lost. Reconnecting..."
- Retryable generation: "The response stopped before it finished." + `Retry`
- Quota reached: show reset time + sign-in/plan action as applicable
- Maintenance: preserve drafts and explain that sending is temporarily unavailable

Toasts are used only for peripheral confirmation, such as a renamed conversation.
Chat failures remain inline where they occurred.

## 8. Component inventory

Build from shadcn/ui where appropriate:

- `Sheet`, `Dialog`, `AlertDialog`, `ScrollArea`
- `DropdownMenu`, `Command`, `Popover`, `Tooltip`
- `Button`, `Textarea`, `Input`, `Label`
- `Tabs`, `Select`, `Switch`, `Checkbox`
- `Table`, `ScrollArea`, `Skeleton`, `Separator`
- `Toast`/`Sonner` for transient peripheral feedback

Product components:

- `NavigationRail`
- `ConversationSearchDrawer`
- `ConversationOutline`
- `ConversationDetails`
- `ChatTranscript`
- `ConversationSpine`
- `MessageBlock`
- `CodeBlock`
- `StreamingSignalRail`
- `ChatComposer`
- `ModelSelector`
- `GuestUsageMetadata`
- `ConnectionStatus`
- `GuestLimitGate`
- `SessionList`
- `AccountDeletionDialog`
- `AdminModelTable`
- `RuntimeSettingEditor`

Do not put cards inside cards. Use icons for familiar compact actions and tooltips
for unfamiliar icons. Use text or icon-plus-text for consequential actions.

## 9. Responsive and accessibility requirements

- Mobile-first CSS
- Test 375, 768, 1024, and 1440px widths
- Test mobile portrait and landscape
- No horizontal page scroll
- Minimum 44x44px touch targets
- Keyboard order follows visual order
- Skip link to transcript/main content
- Visible `:focus-visible` ring
- Errors use `role="alert"` or appropriate live regions
- Streaming status uses a polite live region with throttled announcements
- Dialogs trap focus and return it to their trigger
- Navigation rail, search drawer, and context sheet have accessible names and
  deterministic focus behavior
- Theme meets contrast requirements independently in light and dark modes
- Zoom up to 200% does not hide commands or overlap text
- Safe-area and visual viewport behavior is tested on mobile Safari

## 10. Motion

Motion is functional and limited:

- Control transitions: 150-200ms
- Dialog/sheet transitions: 200-250ms
- Streaming rail: restrained continuous progress only while generating
- Connection and completion state changes: color/opacity, no layout movement
- No scroll-reveal animation inside the application
- No bouncing message bubbles, token-by-token typewriter cursor, gradient orbs, or
  decorative background motion

All animation has a reduced-motion alternative.

## 11. Content and localization

- Spanish and English have equal feature coverage.
- Browser preference selects the initial locale; registered preference persists.
- English is the technical fallback for missing keys.
- UI strings use typed keys and interpolation, not concatenated fragments.
- Dates, times, counts, and reset times use locale-aware formatting.
- Model names remain proper nouns; capabilities and errors are translated.
- Buttons use consistent verbs: `Send`, `Stop`, `Retry`, `Rename`, `Archive`,
  `Delete`, `Sign in`.
- Avoid "tokens" in primary user-facing quota copy; use messages/usage first and
  offer technical detail second.

## 12. PWA behavior

- Installable manifest with Glazz name, theme colors, and final approved icons
- Cache application shell and versioned static assets
- Do not cache authenticated API responses or transcripts in the service worker
- Offline state permits reading only content already held in the current runtime;
  sending is disabled
- New deployment prompts a controlled refresh when version skew affects the client

## 13. Design QA checklist

- [x] Chat is the first screen, not a landing hero
- [x] Product name is a clear first-viewport signal
- [x] Light and dark tokens pass contrast checks
- [x] 375/768/1024/1440 screenshots show no overlap or clipped controls
- [x] Mobile keyboard does not cover composer or last message
- [ ] Long Markdown, tables, code, and unbroken strings remain contained
- [x] Keyboard, screen reader, 200% zoom, and reduced-motion paths pass
- [ ] Streaming, cancellation, retry, reconnect, quota, and maintenance states are
      visually distinct
- [x] Icon buttons have labels/tooltips and stable 44px targets
- [x] No nested cards, decorative orbs, emoji icons, or generic purple AI styling
- [ ] Browser console has no unexpected runtime errors or hydration warnings
- [ ] Playwright visual checks cover guest, authenticated, dark, mobile, and admin

These checks must be revalidated against the Signal Workspace implementation;
evidence from the superseded orange design does not satisfy them.

Signal Workspace evidence recorded on 2026-07-25:

- 20 production-mode visual baselines across 375, 768, 1024, and 1440px
- 9 structural E2E checks for rail, search focus, context lane/sheet, and composer model
- 25 active foundation, accessibility, PWA, and performance checks
- 29 unit tests plus clean format, TypeScript, ESLint, and production build
