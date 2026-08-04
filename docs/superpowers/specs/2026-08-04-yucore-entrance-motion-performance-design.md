# YuCore Entrance Motion Performance Design

## Goal

Reduce intermittent frame drops during the YuCore entrance sequence while preserving its recognizable particle field, ordered orbital motion, energy routes, globe handoff, and current layout.

## Current Bottleneck

The entrance overlaps three animated renderers for much of the 8.2-second sequence: a 30 FPS worker-backed 2D boot canvas, a WebGL signal field targeting 60 FPS, and the WebGL globe near handoff. The static home background is also prepared before the loader exits. Each renderer is individually bounded, but their combined frame and compositing cost creates short load spikes on integrated GPUs and high-DPI mobile devices.

## Considered Approaches

1. **Phase-budgeted adaptive rendering (selected).** Keep every visual motif, reduce duplicate particles, use deterministic lanes and golden-angle shells to retain visual order, and assign a frame budget to each entrance phase. This improves weak devices without changing the composition.
2. **Single renderer per phase.** Completely stop one renderer before starting the next. This gives the largest performance margin but makes the handoff visibly abrupt and risks changing the intended cinematic sequence.
3. **Static fallback for most devices.** Replace the particle field with CSS textures. This is cheapest but removes the depth and motion users recognize.

## Design

### Performance Profile

Introduce a small pure performance-profile module that classifies the browser as `full`, `balanced`, or `reduced` using stable capability signals: reduced-motion preference, logical CPU count, device memory when available, viewport size, and device pixel ratio. Unknown desktop devices default to `balanced`, not `full`.

The profile provides renderer budgets rather than exposing device checks throughout components:

- Boot particles, shards, and sphere points.
- Boot and signal-field target FPS.
- WebGL particle and route segment counts.
- Maximum render pixel ratio.

### Ordered Density Reduction

Particle generation remains deterministic. Reduced counts continue using the existing golden-angle sequence, fixed route lanes, shell bands, and seeded tones, so lowering density reveals a deliberate geometric structure instead of random gaps. No runtime randomness is added.

### Phase Budgeting

The boot canvas remains the dominant early renderer. The entrance signal field starts at a lower cadence and uses the profile budget instead of targeting 60 FPS unconditionally. The globe keeps its existing handoff timing but uses the same pixel-ratio ceiling. Background preparation remains inactive until the loader completes, so it may allocate resources without competing for animation frames.

### Adaptive Safety

The signal field keeps its existing slow-frame downgrade. The initial profile sets its starting quality and ceiling, so weak devices do not need several visibly slow frames before adapting. Hidden and offscreen render loops remain suspended. `prefers-reduced-motion` renders a stable low-cost frame.

## Scope

Only YuCore entrance/background renderer budgets and their tests change. Page layout, text, colors, billing, routing, API behavior, and authenticated console rendering remain unchanged.

## Verification

- Unit tests cover deterministic profile selection and budget ordering.
- Existing render-loop and prewarm tests remain green.
- Typecheck, targeted lint, and production frontend build pass.
- Playwright captures desktop and mobile entrance frames and verifies canvases are nonblank, correctly sized, and free of overlap.
- A browser trace compares animation-frame pacing before and after on an emulated low-capability viewport.
