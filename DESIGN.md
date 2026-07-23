# Glazz Product Design

## 1. Design thesis

Glazz is a public-facing AI chat, not a marketing page wrapped around a demo. The
first viewport is the usable conversation surface. Familiar chat structure lowers
the learning cost; a distinct visual system gives the product its own identity.

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

### 3.1 Concept: Signal Orange

Glazz uses a bright orange signal for action and live generation, grounded by
neutral paper/ink surfaces and a restrained teal counterpoint for success and
connection. Orange is spent on primary actions and the streaming signature, not
washed across the whole interface.

The intentional design risk is a thin **live signal rail** attached to the active
assistant response. It grows with streamed content and shifts from orange to teal
when generation completes. This makes system state spatially stable and recognizable
without a typing gimmick, orb, or decorative animation.

### 3.2 Brand expression

- Product name: `Glazz`
- Wordmark direction: custom-feeling typographic wordmark based on Outfit, with a
  compact aperture detail in the `G`; do not ship an improvised logo as final art.
- Voice: concise, neutral, direct, warm without pretending to be a person.
- Icon family: Lucide, consistent 1.75-2px stroke.

## 4. Design tokens

All application colors are semantic CSS variables consumed through Tailwind and
shadcn/ui. Components do not hardcode palette values.

### 4.1 Light theme

| Token | Value | Use |
| --- | --- | --- |
| `--background` | `#FAFAF8` | Main paper surface |
| `--foreground` | `#191817` | Primary text |
| `--surface` | `#FFFFFF` | Menus, dialogs, composer |
| `--surface-subtle` | `#F2F0EC` | Secondary bands and hover |
| `--border` | `#D9D5CE` | Dividers and controls |
| `--muted-foreground` | `#625E57` | Secondary text |
| `--primary` | `#C74312` | Primary action, active signal |
| `--primary-foreground` | `#FFFFFF` | Text on primary |
| `--brand-bright` | `#FF6A2A` | Large accent and streaming rail |
| `--success` | `#0F766E` | Connected/completed |
| `--warning` | `#A16207` | Quota warning |
| `--destructive` | `#B42318` | Destructive actions |
| `--ring` | `#C74312` | Focus ring |

### 4.2 Dark theme

| Token | Value | Use |
| --- | --- | --- |
| `--background` | `#151412` | Main background |
| `--foreground` | `#F5F3EF` | Primary text |
| `--surface` | `#201E1B` | Menus, dialogs, composer |
| `--surface-subtle` | `#292622` | Secondary bands and hover |
| `--border` | `#454039` | Dividers and controls |
| `--muted-foreground` | `#B8B2A8` | Secondary text |
| `--primary` | `#FF7A45` | Primary action |
| `--primary-foreground` | `#21120C` | Text on primary |
| `--brand-bright` | `#FF8B5C` | Streaming rail |
| `--success` | `#5EEAD4` | Connected/completed |
| `--warning` | `#F4C95D` | Quota warning |
| `--destructive` | `#FF7B72` | Destructive actions |
| `--ring` | `#FF9B73` | Focus ring |

Validate all token combinations against WCAG 2.2 AA. Normal text requires at least
4.5:1; large text and meaningful UI graphics require at least 3:1. Color is never
the only status cue.

### 4.3 Typography

| Role | Family | Weights | Notes |
| --- | --- | --- | --- |
| Display/brand | Outfit | 500, 600, 700 | Restrained use in product name and major screen titles |
| Interface/body | Work Sans | 400, 500, 600 | High legibility for messages and controls |
| Code/data | JetBrains Mono | 400, 500 | Code blocks, token/diagnostic values |

Use `next/font` or self-hosted assets to avoid layout shift and third-party runtime
font requests. Letter spacing is `0`; do not use negative tracking. Body text starts
at 16px with a 1.5-1.65 line height. Chat content measure is 68-76 characters.

Suggested scale:

| Token | Size / line-height |
| --- | --- |
| `display` | 32 / 38 |
| `title` | 24 / 32 |
| `heading` | 18 / 26 |
| `body` | 16 / 26 |
| `ui` | 14 / 20 |
| `caption` | 12 / 18 |

Do not scale type with viewport width.

### 4.4 Shape, spacing, and elevation

- Base spacing unit: 4px
- Common rhythm: 8, 12, 16, 24, 32, 48px
- Control height: 40px desktop, minimum 44px touch target
- Composer radius: 8px
- Dialog/card radius: 8px maximum
- Message content is unframed; user messages may use a subtle surface band
- Shadows are limited to floating menus, dialogs, and the sticky composer
- Page sections are not floating cards

## 5. Information architecture

### 5.1 Desktop

```text
+----------------------+-----------------------------------------------+
| Glazz        [new]   | Top bar: model | usage | connection | account|
| Search conversations |-----------------------------------------------|
|                      |                                               |
| Today                |          conversation transcript              |
| Conversation title   |          readable centered measure            |
| Conversation title   |                                               |
|                      |                                               |
| Archived             |-----------------------------------------------|
| Settings             | [ attach absent ] Ask Glazz...          [send]|
+----------------------+-----------------------------------------------+
```

The sidebar uses the shadcn Sidebar primitive and is resizable only if testing
shows a real need. Default width is stable. The transcript, not the navigation,
dominates the viewport.

### 5.2 Mobile

```text
+----------------------------------+
| [menu] Glazz       [model] [user]|
|----------------------------------|
|                                  |
|        transcript                |
|                                  |
|                                  |
|----------------------------------|
| Ask Glazz...               [send]|
+----------------------------------+
```

The conversation list opens as a sheet. The composer respects the visual viewport,
safe-area inset, and mobile keyboard. Scroll content includes bottom space so the
sticky composer never hides the last message.

## 6. Core screens

### 6.1 Guest chat

- The chat is immediately usable.
- Empty state contains one short prompt and optional starter suggestions, not a
  feature explanation.
- Remaining free messages appear near the composer after the first response.
- At exhaustion, keep the transcript visible and replace send capability with a
  focused Google sign-in gate.
- Explain that signing in preserves the current conversation.

### 6.2 Registered chat

- Conversation list groups recent and archived items.
- Search is a clear command with keyboard access.
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
- Show the live signal rail in orange while streaming
- Transition rail to teal and static when complete
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

- `Sidebar`, `Sheet`, `Dialog`, `AlertDialog`
- `DropdownMenu`, `Command`, `Popover`, `Tooltip`
- `Button`, `Textarea`, `Input`, `Label`
- `Tabs`, `Select`, `Switch`, `Checkbox`
- `Table`, `ScrollArea`, `Skeleton`, `Separator`
- `Toast`/`Sonner` for transient peripheral feedback

Product components:

- `ConversationList`
- `ConversationSearch`
- `ChatTranscript`
- `MessageBlock`
- `CodeBlock`
- `StreamingSignalRail`
- `ChatComposer`
- `ModelSelector`
- `UsageIndicator`
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
- Sidebar/sheet has an accessible name and deterministic focus behavior
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

- [ ] Chat is the first screen, not a landing hero
- [ ] Product name is a clear first-viewport signal
- [ ] Light and dark tokens pass contrast checks
- [ ] 375/768/1024/1440 screenshots show no overlap or clipped controls
- [ ] Mobile keyboard does not cover composer or last message
- [ ] Long Markdown, tables, code, and unbroken strings remain contained
- [ ] Keyboard, screen reader, 200% zoom, and reduced-motion paths pass
- [ ] Streaming, cancellation, retry, reconnect, quota, and maintenance states are
      visually distinct
- [ ] Icon buttons have labels/tooltips and stable 44px targets
- [ ] No nested cards, decorative orbs, emoji icons, or generic purple AI styling
- [ ] Browser console has no errors or hydration warnings
- [ ] Playwright visual checks cover guest, authenticated, dark, mobile, and admin

