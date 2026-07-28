# Routing Field Telemetry Ledger Redesign

## Goal

Replace the cramped right-side routing panel with a full-width telemetry ledger
integrated into the bottom of the authenticated dashboard hero. Remove the
glowing circular core and nested black card while preserving useful account
data, brand motion, and clear routing-health context.

## Approved Direction

The approved visual direction is **C: Telemetry Ledger**.

- The hero no longer reserves a narrow right column for routing status.
- The routing field becomes a horizontal section inside the hero's normal flow.
- Typography, separators, and restrained signal motion establish hierarchy.
- No glowing sphere, radar target, large nested panel, or floating metric cards.
- The ledger adapts to the active light or dark brand theme.

## Scope

### In Scope

- Refactor `YucoreOpsPulse` into an unframed telemetry ledger.
- Update the overview hero layout so content uses the full available width.
- Preserve request count, remaining quota, used quota, and selected model data.
- Preserve the existing translated labels and live prop contract.
- Add theme-aware ledger styling and responsive geometry.
- Replace circular radar motion with lightweight signal and scan motion.

### Out of Scope

- Dashboard APIs, queries, billing, routing, or model-selection behavior.
- Other overview panels and creative-studio content.
- Production deployment, server configuration, or the experimental UI.
- New telemetry fields that the current dashboard does not actually provide.

## Desktop Layout

1. The hero uses one full-width content column instead of a copy/route split.
2. The existing brand mark, gateway status, headline, supporting copy, actions,
   and three setup signals remain in their current semantic order.
3. The three setup signals use a stable three-column row when space allows.
4. The telemetry ledger follows those signals at the bottom of the hero.
5. A single top divider separates the ledger from the main hero content. The
   ledger does not have its own outer card background, shadow, or large radius.
6. The ledger lead occupies roughly one quarter of the row and contains:
   - `Live operations`
   - `routing field` plus the existing stable state
   - a compact animated signal-bar strip
7. The remaining width contains four metric columns separated by thin vertical
   rules: Requests, Quota, Used, and Model.

## Mobile Layout

- The hero remains a single normal-flow column.
- The ledger lead becomes a full-width row above the metrics.
- Metrics use a stable 2-by-2 grid with no horizontal scrolling.
- Numeric values remain fully visible for ordinary large quotas such as
  `100,000,000`.
- Long model names may truncate with a `title` value for full inspection.
- No element uses absolute positioning for primary content.

## Visual System

### Structure

- Use one horizontal divider above the ledger and lightweight internal rules.
- Do not place metric cards inside the hero card.
- Keep corners square or minimally rounded only where the surrounding hero
  already requires them.
- Maintain stable row heights so loading or updated values do not shift the
  hero layout.

### Color

- In light mode, use the current light hero surface, dark text, muted cool-gray
  labels, cyan signal bars, and emerald healthy-state accents.
- In dark mode, inherit the current dark hero surface, use light text, subdued
  separators, cyan signal bars, and the same emerald healthy-state accent.
- Amber may identify quota/spend information but must not dominate the ledger.
- Do not introduce a permanently black inset panel in light mode.

### Typography

- The routing label is compact and subordinate to the hero headline.
- Metric labels are small and muted; values use tabular numerals and a clear
  medium weight.
- Do not use viewport-scaled type, negative letter spacing, or oversized labels.

## Motion

- Replace the radar rings, rotating sweep, and breathing sphere with a short
  signal-bar strip and a subtle horizontal scan accent.
- Motion uses CSS transforms and opacity only; no React state, timers, canvas,
  or animation-frame loop is added.
- Animations run only under the existing YuCore console motion budget.
- Existing reduced-motion behavior disables the new signal and scan animation.
- Motion remains visible but secondary to the numbers and routing status.

## Data And Behavior

`YucoreOpsPulse` keeps its current public props:

- `requestCount`
- `quota`
- `usedQuota`
- `modelName`
- `className`

The component remains read-only. It performs no fetching and introduces no new
state. The existing `ready` model fallback remains unchanged. Existing
translation keys are reused; any new visible copy must be added through the
project i18n workflow for all supported locales.

## Accessibility

- Use a semantic section or header relationship for the routing ledger.
- Mark signal bars and scan accents as decorative with `aria-hidden`.
- Keep status and metrics available as ordinary text.
- Maintain readable contrast in both themes.
- Truncated model values retain a text-equivalent title.

## Files And Boundaries

Expected implementation files:

- `web/default/src/features/dashboard/components/overview/overview-dashboard.tsx`
- `web/default/src/features/yucore-brand/components/yucore-ops-pulse.tsx`
- `web/default/src/styles/index.css`

No backend file, API contract, global theme preset, or unrelated dashboard
component should change.

## Verification

- Add a browser geometry regression that fails on the current right-column
  layout and passes when the ledger spans the hero normally.
- Verify desktop at `1440x900` and mobile at `390x844`.
- Capture ordinary-user and administrator dashboards in light and dark themes.
- Confirm no metric overlap, horizontal overflow, text collision, blank region,
  or hero layout shift.
- Confirm signal/scan motion is active with normal motion and disabled by the
  existing reduced-motion policy.
- Run targeted formatting and lint, TypeScript checking, the frontend test
  suite, and the production build check.

## Acceptance Criteria

- The circular routing core and right-side black nested panel are gone.
- The hero copy is no longer constrained by a permanent routing sidebar.
- The telemetry ledger reads as part of the hero, not as a card inside a card.
- All four existing live values remain visible and correctly labeled.
- Light and dark themes each use appropriate surface and text colors.
- Desktop and mobile layouts remain stable and readable.
- Brand motion remains present without competing with the data.
