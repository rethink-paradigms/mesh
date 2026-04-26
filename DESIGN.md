# Design System — AgentBodies

## Product Context
- **What this is:** AI-first agent body management interface. The user types plain English, AI proposes actions with visual confirmation, everything is reversible.
- **Who it's for:** Solo developer building world-class quality for themselves.
- **Space/industry:** Agent infrastructure / compute management. Peers: ClawHQ, HELIX, Daytona.
- **Project type:** Web app (chat-first with visual canvas)
- **Memorable thing:** "The interface that thinks with you — AI proposes, you confirm, everything reversible."

## Aesthetic Direction
- **Direction:** The Craftsman's Bench
- **Decoration level:** Minimal — typography and the copper accent do all the work. No decorative blobs, no gradients, no patterns. Depth through luminance steps, not shadows.
- **Mood:** Dark, warm, instrument-like. Workshop dark. The kind of dark where you can focus for hours because everything recedes except what matters. Like a watchmaker's bench at midnight: blackened surfaces, selective warm illumination, every tool has weight and purpose.
- **Reference sites:** Linear (surgical restraint), Raycast (macOS-native precision), Vercel v0 (chat-first AI interface where the UI disappears)

## Typography
- **Display/Hero:** Instrument Serif — Architectural, not geometric. Has soul without being precious. Every DevTool uses geometric sans; this is the risk that makes AgentBodies instantly recognizable.
- **Body/UI:** Geist Sans — Designed by Vercel for developer interfaces. Excellent readability at small sizes, tabular-nums for data, understated enough to let Instrument Serif carry the personality. Open source.
- **Data/Tables:** Geist Sans (tabular-nums) — same as body, with tabular figure variants for aligned numeric columns.
- **Code:** Berkeley Mono — Wide-set, generous spacing, unambiguous glyphs. When reading a body ID or snapshot hash at 2am, certainty beats personality.
- **Loading:** Google Fonts for Instrument Serif. Geist Sans and Berkeley Mono from their respective CDNs or self-hosted.
- **Scale:**
  - Display: 48px / 400 / 1.10
  - H1: 32px / 400 / 1.20 (Instrument Serif)
  - H2: 24px / 600 / 1.30 (Geist Sans)
  - H3: 20px / 400 / 1.40 (Instrument Serif)
  - Body: 16px / 400 / 1.60 (Geist Sans)
  - Small: 14px / 400 / 1.50 (Geist Sans)
  - Caption: 13px / 400 / 1.50 (Geist Sans)
  - Label: 12px / 600 / 1.40 (Geist Sans, uppercase, letter-spacing 0.06em)
  - Code: 14px / 400 / 1.60 (Berkeley Mono)
  - Meta: 11px / 400 / 1.50 (Berkeley Mono)

## Color
- **Approach:** Restrained. One accent + warm neutrals. Color is rare and meaningful. Accent on less than 5% of pixels.
- **Primary:** #C8956C (burnished copper) — the brand signal. Warm, material, workshop-like. Not blue, not indigo, not purple. Used sparingly: active states, brand elements, key CTAs.
- **Secondary:** #6B9DAD (gunmetal blue-teal) — info states, secondary interactive elements.
- **Neutrals:** Warm grays from lightest to darkest:
  - Primary text: #E8E4DD (warm off-white, like parchment under lamplight)
  - Secondary text: #A09A92 (warm gray for descriptions)
  - Muted text: #6B6660 (warm gray for timestamps, metadata)
  - Border: #2A2A2D (barely there)
  - Border strong: #3A3A3E (structural dividers)
  - Surface hover: #252528 (click targets)
  - Surface raised: #1C1C1F (hover states, active elements)
  - Surface: #141416 (cards, panels — depth through shadow, not border)
  - Background: #0A0A0B (warm near-black, like oiled tool steel)
- **Semantic:** success #7FB069 (sage green), warning #D4A843 (aged brass), error #C75C5C (oxidized red), info #6B9DAD (gunmetal blue-teal)
- **Dark mode:** Primary surface. Designed dark-first, not inverted light theme. Elevation through luminance steps: each surface is 6-8 points lighter than the previous. No pure black (#000000). Borders use rgba(255,255,255,0.04-0.08) for subtle light-edge elevation on dark surfaces.

## Spacing
- **Base unit:** 4px
- **Density:** Compact
- **Scale:** 2xs(2px) xs(4px) sm(8px) md(16px) lg(24px) xl(32px) 2xl(48px)

## Layout
- **Approach:** Chat-first with visual canvas (hybrid of Vercel v0 and Linear)
- **Three zones:**
  1. **The Prompt** (bottom, always present) — where the user talks. "What do you want to do?" A conversation, not a search bar.
  2. **The Canvas** (center, scrollable) — where AI renders status cards, timelines, agent maps, provision previews. Cards appear and disappear based on conversation context.
  3. **The Rail** (left, minimal, ~200px) — body names with colored status dots. Not full navigation. Just enough to see what exists without asking.
- **Grid:** Single column canvas with card grid (2-column at >640px). Rail is fixed-width.
- **Max content width:** 1280px (canvas area ~1080px)
- **Border radius:** 2px(badges) / 4px(tags) / 8px(cards, inputs, buttons) / 12px(modals) / 9999px(pills, tags)

## Motion
- **Approach:** Intentional. Every animation communicates something. No decorative animation.
- **The Preview Render:** Destructive actions don't use confirmation dialogs. Instead, the AI shows consequences before executing: the body card dims, a timeline entry appears in preview mode, connected bodies pulse to show dependency impact, and a 5-second countdown starts with an "undo" affordance. Confirmation is a demonstration, not a binary gate.
- **Easing:** enter(ease-out) / exit(ease-in) / move(ease-in-out)
- **Duration:** micro(50ms) / short(150ms) / medium(250ms) / long(400ms)
- **Card entrances:** spring physics (slight overshoot, settle) — cubic-bezier(0.34, 1.56, 0.64, 1)
- **State changes:** animate only the changed element, not the whole view

## Decisions Log
| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-24 | Initial design system created | Created by /design-consultation based on product context, competitive research (Linear, Raycast, Vercel/v0, Daytona), and outside design voices |
| 2026-04-24 | Serif display font (Instrument Serif) | Deliberate departure from DevTools category norm. Creates instant recognition. |
| 2026-04-24 | Copper accent (#C8956C) | Warm, material, zero-confusion with blue/indigo competitors. |
| 2026-04-24 | No dashboard, empty-start layout | Google Homepage principle. AI is the dashboard. Information on demand. |
| 2026-04-24 | Preview Render confirmation | Replace binary "are you sure?" with visual consequence demonstration. |