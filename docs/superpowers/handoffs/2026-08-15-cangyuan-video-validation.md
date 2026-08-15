# Cangyuan Video Refresh Validation Handoff

Date: 2026-08-15

Branch: `codex/cangyuan-video-refresh-20260815`

Implementation base before the final authenticated re-audit: `78ca6e12e`

This handoff is intentionally redacted. It contains no credentials, API keys,
cookies, real user identifiers, provider task identifiers, account balances,
private asset URLs, server addresses, container names, or image digests.

## Scope and release state

- The embedded video catalog contains exactly 13 enabled, priced models and
  seven hidden probe models.
- Video task billing remains isolated from GPT token usage, cache accounting,
  and stream interruption behavior.
- No production configuration, channel, container, image, Caddy route, or user
  traffic was changed during this validation phase.
- The current production application must remain running throughout candidate
  preparation, traffic switch, observation, and any rollback.
- Public traffic switching and replacement-channel activation remain blocked on
  a separate explicit approval after the private candidate is verified.

## Audited inventory

Enabled and priced:

```text
grok-video
grok-video-1.5
happyhouse-1.0
happyhouse-1.1
minimax-h3-2k
omni-fast
omni-fast-no-water
omni-v2v
omni-v2v-no-water
sd7-seedance-2.0-1080p
sd7-seedance-2.0-720p
sd8-seedance-2.0
seedance-2.0
```

Hidden probes without an approved price or transport contract:

```text
sd4-seedance-2.0
sd4-seedance-2.0-fast
sd8-seedance-2.0-fast
seedance-2.0-fast
seedance-2.0-mini
seedance-2.0-mini-8s
veo-clean
```

The enabled-model upstream subtotal for one minimum accepted task per model is
`31.973`. The seven probe models must not receive paid requests.

## Authenticated upstream re-audit

Immediately before paid probing, the authenticated VIDEO token and public
pricing surface were read again without writing or displaying credentials. The
upstream had changed after the initial implementation:

- The VIDEO token listed 21 model identifiers: 20 video models plus one
  out-of-scope audio model.
- Public VIDEO-group pricing listed 14 rows: 13 video models plus the same
  out-of-scope audio model.
- `sd4-seedance-2.0` and `sd4-seedance-2.0-fast` remained token-visible but no
  longer had a published price, so they moved from enabled to hidden probe.
- `seedance-2.0` became token-visible and priced at `3.9` per generation, with
  fixed 720p output, 4-15 second duration, generated audio, and limits of five
  images, three videos, three audios, and 11 references total.
- The retained 12 priced video rows had no price drift.
- The upstream VIDEO group ratio remained `1.0`.

The paid batch was stopped before any creation request or debit, as required by
the inventory-drift gate. Tests, catalog, pricing, docs, runbook, and local
candidate were then refreshed to the 13/7 baseline before paid work resumed.

## Pricing evidence

Base prices use `ceil(upstream_cost * 1.20 * 10000) / 10000`, ensuring every
base price is at least 20% above the observed upstream price.

| Model                    | Upstream | Base price | `下游多模态` 1.0 | `多模态创作` 1.2 |
| ------------------------ | -------: | ---------: | ---------------: | ---------------: |
| `grok-video`             |     0.69 |      0.828 |            0.828 |           0.9936 |
| `grok-video-1.5`         |     1.39 |      1.668 |            1.668 |           2.0016 |
| `happyhouse-1.0`         |      4.5 |        5.4 |              5.4 |             6.48 |
| `happyhouse-1.1`         |      2.9 |       3.48 |             3.48 |            4.176 |
| `minimax-h3-2k`          |      3.5 |        4.2 |              4.2 |             5.04 |
| `omni-fast`              |   0.6624 |     0.7949 |           0.7949 |          0.95388 |
| `omni-fast-no-water`     |     0.81 |      0.972 |            0.972 |           1.1664 |
| `omni-v2v`               |   0.8856 |     1.0628 |           1.0628 |          1.27536 |
| `omni-v2v-no-water`      |    1.035 |      1.242 |            1.242 |           1.4904 |
| `sd7-seedance-2.0-1080p` |      4.9 |       5.88 |             5.88 |            7.056 |
| `sd7-seedance-2.0-720p`  |      3.9 |       4.68 |             4.68 |            5.616 |
| `sd8-seedance-2.0`       |      2.9 |       3.48 |             3.48 |            4.176 |
| `seedance-2.0`           |      3.9 |       4.68 |             4.68 |            5.616 |

All values are per accepted generation. Status polling, content retrieval, and
asset download must not charge again. The exact runtime configuration is in
`docs/superpowers/runbooks/2026-08-15-cangyuan-video-production-refresh.md`.

## Automated verification

Backend formatting and tests:

```text
gofmt on all touched Go files                                      PASS
go test ./model ./service ./controller ./relay ./constant -count=1 PASS
go test -p 1 ./... -count=1                                        PASS
```

Default frontend checks:

```text
bun run typecheck                                                   PASS
bun run build                                                       PASS
bunx oxlint on the three touched TypeScript/TSX files               PASS
bunx oxfmt --check on all touched docs, TS/TSX, and locale files    PASS
bun run i18n:sync                                                   PASS
```

Classic frontend build:

```text
bun run build                                                       PASS
```

Repository-wide frontend lint baselines are not clean on the base branch. The
default full lint emits existing findings outside the touched files; its full
format check likewise reports existing unrelated files. The classic full lint
reports 122 existing source/generated-file style failures. These baseline
findings were not changed or suppressed. The touched default files pass the
targeted lint and format checks above, and both frontend production builds pass.

Documentation checks:

- The 13 base and final price values match the audited formula exactly.
- All seven fenced JSON examples and six JSON request bodies parse.
- Removed full video model identifiers are absent from the refreshed developer
  guide and production runbook.
- A generic credential/address scan found no secret-like value in the refreshed
  documentation.
- The optional operator-local private-pattern scan was not run because
  `YUAPI_PRIVATE_PATTERN_FILE` was not configured in the local environment. It
  remains a required pre-release server check.

## Local private-candidate evidence

An isolated local application was built from this branch and started at
`localhost:31845` with a disposable SQLite database, a synthetic local user,
and a fake channel exposing the 13 enabled models. It used no production or
upstream credential and sent no generation request.

Playwright exercised home, sign-in, console, pricing, Studio, Canvas, and
developer docs at desktop and mobile sizes. Results:

```text
page errors                                                        0
same-origin failed responses                                       1
expected anonymous /api/user/auth/refresh response                 401
desktop horizontal overflow                                        none
mobile horizontal overflow                                         none
Studio model buttons                                                13
Pricing model badges                                                13
720p control                                                        present
1080p control                                                       present
Native audio switch                                                 checked
Native audio switch accessibility                                  PASS
```

Screenshot files retained outside Git:

```text
home-desktop.png
sign-in-desktop.png
console-desktop.png
pricing-desktop.png
studio-video-desktop.png
studio-video-mobile.png
canvas-mobile.png
docs-desktop.png
```

Visual inspection confirmed that the video model list, per-generation prices,
resolution controls, duration controls, audio switch, action area, and results
area render without overlap at the tested desktop and mobile viewports.

## Remaining gates

1. Re-read the authenticated upstream VIDEO group and stop on any inventory or
   price drift.
2. Run exactly one smallest valid paid task for each of the 13 enabled models,
   reusing accepted mapping probes and never probing the seven unpriced models.
3. Record only redacted mapping, status, media metadata, and debit/charge
   comparisons; encode any proven contract correction test-first.
4. Push the fully verified branch and re-audit production read-only.
5. Capture scoped server-local rollback artifacts, then build and privately
   verify a blue-green candidate without changing public routing or stopping the
   current application.
6. Present the exact candidate and rollback facts and wait for explicit traffic
   switch approval before any Caddy reload or replacement-channel activation.
