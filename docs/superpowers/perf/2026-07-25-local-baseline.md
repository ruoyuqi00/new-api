# Local Production Brand Performance Baseline

## Environment

- Source: `codex/local-production-brand-performance-20260725` at `99215cbfa`
- URL: `http://127.0.0.1:13000/`
- Browser: Microsoft Edge, headless, 1440x900 viewport
- Data: isolated local SQLite preview only; no production database, cache, API key, channel, or account-pool data was used.

## Public Home Measurements

Collected after network idle, the full 14-second brand entrance sequence, and a 120-frame steady-state sample:

| Measurement | Value |
| --- | ---: |
| First paint | 320 ms |
| First contentful paint | 3156 ms |
| Largest contentful paint | 7236 ms |
| Load event end | 509 ms |
| Network requests | 21 |
| Long tasks | 8 |
| Long-task duration | 1327 ms |
| Active canvases | 2 |
| Frame interval average | 6.13 ms |
| Frame interval p95 | 8.40 ms |

The high LCP is expected from the deliberate entrance sequence, but its long-task budget is too large for repeat visits. The 2-canvas steady state is smooth in the reference browser; neither renderer needs to lose its visual motion.

## Renderer Lifecycle Findings

- `YucoreWebglEarth` and `YucoreSignalFieldWebgl` both stop scheduling frames while `document.hidden` and restart with one frame when the document becomes visible.
- Neither renderer observes its own viewport intersection. A background can continue using WebGL while its page region is scrolled out of view.
- The public home activates both renderers and fetches its earth texture during the entrance sequence. This combines WebGL setup, image decoding, and the loaded brand animation during the LCP window.
- The dashboard starts independent API-key and model queries. Those requests use distinct stable cache keys, so any dashboard optimization must keep them independent and avoid adding another polling chain.

## Working Hypothesis

Adding intersection-aware frame ownership to the two WebGL renderers should stop offscreen GPU work without changing the visual sequence. The smallest measurable follow-up is to verify that an inactive or offscreen renderer owns no `requestAnimationFrame`, then preserve its existing visible frame rate and page-visible resume behavior.

## Lifecycle Verification

- `yucore-render-loop.test.ts` proves an inactive, hidden, or offscreen renderer queues no frame, and a visible active renderer owns exactly one queued frame.
- The two existing WebGL renderers now share that ownership rule and attach an `IntersectionObserver` to their canvas. The signal-field renderer resets its adaptive frame-time baseline when it re-enters the viewport, so intentional idle time is not interpreted as a slow render.
- Public-home visual inspection at 1440x900 found the same earth, signal-field, particle, route, and layout sequence in both light and dark mode after the lifecycle change.

## Post-Change Public Measurement

| Measurement | Baseline | Post-change |
| --- | ---: | ---: |
| First contentful paint | 3156 ms | 3528 ms |
| Largest contentful paint | 7236 ms | 7592 ms |
| Long tasks | 8 | 8 |
| Long-task duration | 1327 ms | 1355 ms |
| Active canvases while visible | 2 | 2 |

The visible-page startup sample is intentionally not treated as an improvement claim: the change targets offscreen GPU work, not the entrance animation's first-load time. The post-change sample has no material regression in the same local browser; the behavioral regression test is the direct evidence that offscreen animation work is removed.

## 2026-07-27 Renderer Handoff Acceptance

- Candidate: `7eb2d11f45` on `codex/local-production-brand-performance-20260725`
- Control: existing local production build on `http://127.0.0.1:13000/`
- Candidate: temporary production preview on `http://127.0.0.1:13001/`
- Browser: Microsoft Edge headless, 1440x900 desktop and 390x844 mobile
- Themes: light and dark

Three-run medians were collected serially on the same machine after the stable-home handoff:

| Measurement | Existing local control | Renderer-handoff candidate |
| --- | ---: | ---: |
| First contentful paint | 2784 ms | 2716 ms |
| Largest contentful paint | 6880 ms | 6880 ms |
| Long tasks | 8 | 8 |
| Long-task duration | 1122 ms | 1104 ms |
| Frame interval average | 6.94 ms | 6.94 ms |
| Frame interval p95 | 7.1 ms | 7.1 ms |
| Shader compilations after stable activation | 4 | 0 |

The comparable timing metrics are effectively unchanged. The accepted result is the renderer-lifetime contract: all three candidate runs activated the stable home with zero new shader compilations, while every control run compiled four shaders after activation. Both candidate canvases remained active and showed nonzero pixel motion.

Visual acceptance used viewport screenshots captured at each real scroll position instead of relying on one full-page screenshot. Across light/dark desktop and light/dark mobile, every reveal element meeting the component's 8 percent intersection threshold reached full opacity, both detail sections had a 0 px document-flow gap, both canvases remained moving, and no horizontal overflow was detected. The evidence directory contains 50 viewport screenshots plus the corresponding JSON reports, handoff traces, and CPU profiles under `local-production-role-audit-20260726/`.

The temporary candidate preview on port `13001` was stopped after its command line was verified as this worktree's `rsbuild preview`. Local backend `3000`, development frontend `3001`, and existing control `13000` remained listening.

## 2026-07-27 Full Local Acceptance Results

### Role and route matrix

- Source: `codex/local-production-brand-performance-20260725` at `5291d5061`
- Frontend: `http://127.0.0.1:3001`
- Backend: `http://127.0.0.1:3000`
- Profiles: light desktop at 1440x900 and dark mobile at 390x844
- Roles: anonymous, ordinary user, and super-admin
- Coverage: 38 route/profile cases spanning home, sign-in, pricing, overview, keys, usage logs, wallet, Studio, profile, flow, users, channels, account pools, private-group pricing, routing reliability, channel affinity, and site settings
- Result: 38 passed, with no page-level horizontal overflow, page exceptions, unexpected console errors, error-page redirects, or local API 5xx responses

The anonymous shell intentionally calls `/api/user/auth/refresh` once. A 401 from that endpoint is the existing anonymous-session contract and was recorded separately instead of being treated as a page failure. Any other 401 or console error remained a failure. Screenshots and the machine-readable report are stored in the local visualization directory under `local-production-role-audit-20260726/`.

Representative screenshot inspection confirmed readable light/dark surfaces for usage logs, Studio, users, group pricing, and channel affinity. The 390px channel-affinity action row remains visually dense, but it does not cause document or main-content overflow. The isolated local pricing seed has no enabled models, so its red routing-coverage warnings describe test data rather than a rendering failure.

A temporary local-only Claude consumption log verified cache accounting presentation end to end. The table displayed 1,200 cache-read tokens and 900 cache-write tokens; the details dialog displayed the 600-token 5m write, 300-token 1h write, cache pricing, group ratio, and total cost. The synthetic row was deleted immediately after the screenshots and assertions passed.

### Build and static checks

- Default frontend tests: 125 passed, 0 failed
- Default frontend type check: passed
- Default production build: passed
- Classic production build: passed
- Files changed by this optimization batch: 26 TypeScript files linted with 0 errors and 0 warnings
- Whole-default lint gate: blocked by 481 errors and 102 warnings outside this batch
- Whole-default format gate: blocked by 24 files; the three files changed by this batch were formatted separately

The whole-repository lint and format debt is therefore not reported as passing. It needs a separate scoped cleanup because applying whole-tree auto-fixes would touch unrelated authentication, billing, model, key, subscription, setup, and shared component code.

### Same-machine production-build comparison

Both columns are three-run medians collected serially in Microsoft Edge headless at 1440x900. The control is the existing local production container on port 13000. The candidate is the current production build served locally on port 13001. Each run waited through the entrance sequence and then sampled 120 animation frames.

| Measurement | Existing local control | Current candidate |
| --- | ---: | ---: |
| First paint | 396 ms | 428 ms |
| First contentful paint | 3764 ms | 3832 ms |
| Largest contentful paint | 7508 ms | 7560 ms |
| Load event end | 679 ms | 569 ms |
| Network requests | 22 | 22 |
| Async chunk requests | 6 | 6 |
| Long tasks | 9 | 10 |
| Long-task duration | 1961 ms | 2412 ms |
| Active canvases while visible | 2 | 2 |
| Frame interval average | 13.48 ms | 14.42 ms |
| Frame interval p95 | 21.0 ms | 25.1 ms |

FCP and LCP are effectively unchanged in this same-machine comparison, so there is no evidence of a material startup regression or a startup improvement. Long-task duration is higher in the candidate median, but the sample ranges overlap (control 1887-2435 ms; candidate 2122-3095 ms). This result is recorded as a remaining performance risk, not as a proven regression and not as an improvement claim. A future entrance-animation optimization should use a browser trace to identify the responsible long tasks before changing visible motion.
