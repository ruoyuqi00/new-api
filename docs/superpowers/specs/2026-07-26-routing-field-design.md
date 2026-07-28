# Routing Field Visual Redesign

## Goal

Improve the authenticated dashboard routing field so it reads as a compact,
professional operations panel instead of a collection of overlapping floating
cards. Preserve the existing YuCore motion language and live account data.

## Scope

- Update `YucoreOpsPulse` and its dedicated styles only.
- Preserve request count, remaining quota, used quota, and selected model data.
- Preserve the radar rings, sweep, scan, core pulse, and lane motion.
- Support dark and light brand themes, desktop, and mobile layouts.
- Do not change dashboard APIs, routing behavior, or other homepage panels.

## Layout

1. Use a compact header for live-operation context, routing-field title, and
   health status.
2. Keep one restrained circular operations core as the primary visual.
3. Move the four metrics into a normal-flow 2-by-2 grid below the core so text
   and long values cannot collide with the animation.
4. Render latency, failover, and spend as aligned status lanes at the bottom.
5. Use stable grid tracks and minimum heights so dynamic values do not resize
   the surrounding dashboard hero.

## Visual Treatment

- Reduce ring and scan contrast behind text.
- Use compact rectangular metric cells with clear label/value hierarchy.
- Use cyan for routing activity, emerald for healthy state, and amber only for
  quota/spend accents.
- Avoid additional nested decorative cards and oversized rounded geometry.
- Keep all visible values readable in both brand themes.

## Behavior

- The component remains read-only.
- Existing live data props and fallback model text remain unchanged.
- Animation continues to respect the existing console motion budget and
  reduced-motion rules.

## Verification

- Run targeted frontend lint, formatting, type checking, tests, and production
  build.
- Capture local screenshots for ordinary-user and administrator dashboards in
  light and dark themes at desktop and mobile widths.
- Confirm no text overlap, horizontal overflow, blank canvas, or layout shift.
