# SHKeeper production-line local verification

Verified locally on 2026-09-27 (Asia/Shanghai). Feature verification and the 40-pair production UI comparison passed. The repository-wide `bun test`, lint, and format commands retain the baseline failures detailed below; this is not an assertion that every repository gate is green. No deployment, production database, live provider, blockchain, or funds were used.

## Source identity

- Production UI baseline: `27023295d21bcc824c5af1d6e9e36c22c54db3a5`.
- Task 5 input: `e6dfaf9e2f4dc7cafa19f24fae90c49f4b11bc63` on `codex/shkeeper-usdt-production-line`.
- Candidate implementation includes Tasks 1–4: `8841ef24b`, `b91a4099b`, `c337a2505`, `cc5b18f5f`, `1e040be18`, `0d4c8d5e9`, `cefcff28e`, `10c400374`, `9decb155a`, and `b5ee7fbf9`.
- Task 5 fix: `840679c52ee16e7236e4114002164d67024b1762` (`fix: preserve SHKeeper validation feedback before saving`). The documentation commit containing this file adds the final six locale files.
- Candidate worktree: `D:/newapi-710-yuapi/.worktrees/shkeeper-production-line`.
- Detached baseline worktree: `D:/newapi-710-yuapi/.worktrees/shkeeper-production-baseline`; `git check-ignore` verified `.worktrees/` before creation. Its tracked files remain clean.
- Candidate executable SHA-256: `49A01FF05ACE6433AF85179B68FC5E9BD6E9C7A646D730353EA5BFDF0069507A`.
- Both the committed and working-tree guards against the baseline passed for `web/default/src/features/yucore-brand/**` and `web/default/src/styles/**`.

## Review environment

| Process | Address | PID | State |
| --- | --- | --- | --- |
| Production-line candidate | `http://127.0.0.1:13037` | 33280 | Running for review |
| SHKeeper v2.5.32 fixture | `http://127.0.0.1:13038` | 39008 | Running; required by candidate review |
| Temporary production baseline | `http://127.0.0.1:13036` | 15636 (follow-up) | Stopped again at 2026-09-27 02:21:07 +08:00 |

Candidate sign-in: `http://127.0.0.1:13037/sign-in`. Wallet: `http://127.0.0.1:13037/wallet`. Payment settings: `http://127.0.0.1:13037/system-settings/billing/payment`, then SHKeeper.

Synthetic administrator: `review_admin` / `LocalReview-2026!SHK`. These credentials exist only in the isolated test databases. Initial quota was 100,000,000 in each database (displayed as $200). Two successful audit runs leave candidate quota 562,000,000 (displayed as $1,124), with synthetic order history retained as evidence.

Candidate packages are `10 → 66`, `50 → 330`, `100 → 660`, `200 → 1320`; all of `BNB-USDT`, `USDT`, and `POLYGON-USDT` are enabled. The first package has the synthetic label `Local review`. Expiry is 30 minutes; reconciliation interval is 60 seconds. Local write-only fixture credentials are API key `local-shkeeper-api-review-only` and backend key `local-shkeeper-backend-review-only`. The API key authenticates requests and signs webhook HMAC; the backend key authorizes walletnotify transaction rescans.

The executable is `.local-tests/shkeeper-production-line/candidate.exe`; its working directory and SQLite path are the same isolated directory, with `candidate.db`. Baseline used its own `.local-tests/shkeeper-production-line/baseline.db`. `SQL_DSN`, `LOG_SQL_DSN`, Redis URL, and frontend redirection were explicitly empty; `LISTEN_ADDRESS=127.0.0.1`. Both previews were started with hidden windows and task-local session/crypto secrets. Classic assets are a local unused placeholder; both databases explicitly select `theme.frontend=default`. Existing dependency caches were reused through junctions; no project dependency changes were made.

The initial visual pass hit the default 20-request critical limiter. Only these two isolated processes were restarted with critical/API/web request limits set to 100000 for the browser matrix. The final pass had no rate-limit failures. Initial and replacement PID records and process logs remain in the artifact directory. Port 13035, PID 16220, and its older `shkeeper-usdt` worktree were neither stopped nor changed.

## Localization and corrections

Ran `bun run i18n:sync` before translation and inspected its report. Scanning the 31 changed Task 3/4 TS/TSX files found 241 literal translation keys and 106 missing English keys. Added 106 keys to each of en/zh/fr/ja/ru/vi (636 entries) exclusively through `web/default/scripts/add-missing-keys.mjs`, then ran `node scripts/add-missing-keys.mjs` and `bun run i18n:sync`. The script payload was restored; temporary discovery/apply scripts were removed. No locale JSON was manually edited, and no runtime-only static keys were needed.

All 241 used keys exist in every locale; placeholders match exactly. Final sync reports missing/extras zero in all six locales. Remaining untranslated heuristics are en 0, fr 6, ja 11, ru 11, vi 6, zh 2: existing unrelated untranslated copy plus the intentionally unchanged SHKeeper brand name. Six-language 360×740 browser captures have dialog width/scrollWidth 328/328, with the close action reachable by scrolling.

Browser verification found and fixed two linked settings-save failures in the feature-owned `shkeeper-settings-section.tsx`: throwing inside React Hook Form's invalid callback prevented error publication, and entering the save lifecycle before validation invalidated status ownership even when no save could occur. Validation now finishes before the persistence lifecycle starts, and the operation rejects only after the form publishes errors. The same mounted browser sequence changed from missing inline errors / corrected-save no-op to passing last-package, duplicate, corrected-save, and Save-all checks. `settings-array-error-red.json`, the subsequent failure artifact, and `settings-audit.json` preserve the evidence. The abandoned subscription-only experiment was reverted.

The API/backend key descriptions were also corrected to match the actual authentication and HMAC/walletnotify behavior, including all six locale values.

## Automated verification

| Command / gate | Result |
| --- | --- |
| `go test ./... -count=1` | PASS, exit 0; complete repository run |
| `git diff --check` | PASS |
| `git diff --exit-code 27023295d..HEAD -- web/default/src/features/yucore-brand web/default/src/styles` | PASS |
| Same immutable guard including uncommitted working-tree state | PASS |
| `bun run i18n:sync` | PASS; missing/extras zero |
| `bun run typecheck` | PASS after final source fix |
| `bun run build` | PASS for baseline and final candidate |
| Go executable builds for baseline/candidate | PASS |
| Scoped oxlint on all 37 changed TS/TSX/JSON files | PASS, zero errors; one existing Safari `prefer-includes` warning |
| Scoped oxfmt on the same files | PASS |
| `bun test src tests scripts` | PASS, 263 tests across 60 files |
| Exact `bun test` | 263 pass, 2 fail / 2 loader errors: existing Playwright specs call `test.beforeEach()` under Bun |
| Exact `bun test` on untouched baseline | 232 pass, same 2 fail / loader errors |
| Existing Playwright specs, installed Edge, desktop/mobile | PASS: 4 passed, 4 expected project/capture skips |
| Full lint | Baseline debt: candidate 436 errors / 94 warnings; baseline 448 errors / 98 warnings; no changed-file errors |
| `bun run format:check` | Existing Windows wrapper failed restoring `public/developer-docs/yucore-api.en.md` with `UNKNOWN` open error |
| Direct full `bunx oxfmt --check .` after restoration | Baseline drift: candidate 45 files, baseline 46; changed files clean |

The format wrapper writes temporary changes even in check mode; its failed restoration left unrelated formatting side effects. The worktree was clean before this task, so only those verified task-generated side effects were restored from HEAD. Protected renderer/style guards passed afterwards, and candidate frontend/binary were rebuilt before visual collection. Full-format baseline drift was recorded, not reformatted into this change.

Final candidate frontend output is approximately 57,171.0 kB / 16,651.2 kB gzip; baseline is 57,085.4 kB / 16,626.1 kB gzip. These are whole-application Rsbuild output totals, not a claim about initial transferred JavaScript.

## Global UI and motion comparison

Before configuring SHKeeper or opening payment UI, compared baseline/candidate sign-in, docs, wallet shell, home, and system settings at 1440×900, 1024×768, 390×844, and 360×740, in both light and dark themes: 40 pairs / 80 screenshots. Equivalent default theme, synthetic user, settings, and wallet state were seeded through local APIs.

`visual-audit.json` records root/body/main dimensions, overflow, DOM renderer classes, canvas counts and bounds, graphics renderer, nontransparent/nonblank pixels, frame hashes, animation state, console/page errors, failed requests, and HTTP errors for every page. Hardware renderer was `ANGLE (AMD, AMD Radeon(TM) Graphics (0x00001636) Direct3D11 vs_5_0 ps_5_0, D3D11)`. Sign-in/docs/home each had two matching WebGL canvases; wallet and settings each had zero in both builds. All 96 canvas samples were nonblank and changed across animation-frame observations. Because WebGL buffers are not preserved, samples were taken across animation frames until actual drawn pixels and a second differing frame were observed, without changing renderer options.

All 80 pages had zero horizontal overflow. Renderer class selection, canvas bounds and counts, static fallback elements, and root widths matched. There were zero page errors and failed requests. The only HTTP/console errors were 16 expected anonymous `/api/user/auth/refresh` 401s, one per baseline/candidate sign-in capture. No software fallback was forced; this machine supplied hardware rendering. Contact sheets were opened and visually inspected, including all screen/size/theme combinations. Existing light-mode docs headings have low contrast in both builds; it is unchanged baseline behavior.

The final source corrections affect only payment settings save handling and credential descriptions; the global matrix used the same immutable renderer/style files and shell. Final corrected binary was then used for the complete settings/payment audits and six-language review.

### Mobile Home height follow-up (Fix Round 1)

Review identified a gap in the original matrix: its assertions covered widths and renderer bounds, while three mobile Home pairs had body/root/main height differences of −3264, −3264, and +3264 pixels. Top-only captures could not prove below-fold equivalence. These saved measurements remain in `visual-audit.json`; they were explicitly rechecked as the RED evidence.

The focused follow-up used the unchanged baseline executable/database and final candidate. After fonts and Home details were available, each page was scrolled to its actual bottom and back in viewport-sized increments, recalculating the bottom after each materialization, awaiting animation frames, visible image decoding, and finite reveal transitions. Root/body/main and both detail-section heights then had to remain identical across eight consecutive animation frames. It took 20 down/up steps at 390×844 and 24 at 360×740. No arbitrary sleeps were used.

| Mobile Home pair | Original candidate − baseline body height | Baseline and candidate root/body/main height after traversal | Root/body/main scrollHeight | Width / scrollWidth | Final height difference |
| --- | --- | --- | --- | --- | --- |
| 390×844 light | −3264 px | 9245.46875 CSS px | 9245 px | 390 / 390 px | 0 px |
| 390×844 dark | −3264 px | 9245.46875 CSS px | 9245 px | 390 / 390 px | 0 px |
| 360×740 dark | +3264 px | 9340.703125 CSS px | 9341 px | 360 / 360 px | 0 px |

Explicit assertions allow at most 1 CSS pixel for subpixel rounding at identical viewport/font/content settings; the measured differences were exactly zero for all recorded root/body/main width, height, client and scroll dimensions. Both detail sections' bounds/text and renderer/canvas bounds also matched. All 12 hardware canvases were nonblank and moving; console/page/request/HTTP errors were empty across the six pages.

This confirms a materialization/timing artifact in the initial measurements, consistent with the unchanged `.yucore-home-details` CSS `content-visibility:auto` and `contain-intrinsic-size:auto 3200px`, rather than a candidate layout change. The initial fresh heights in the follow-up were also provisional and changed when the details were traversed. No source or immutable CSS change was needed.

Chrome full-page screenshot capture can re-skip offscreen `content-visibility:auto` sections after returning to the top. Therefore, **after the unmodified layout measurements**, the capture pass temporarily set only those two DOM sections to `content-visibility:visible`, repeated traversal/reveal completion, asserted the body height stayed unchanged, captured the full page, then restored the inline values. This identical browser-only capture adjustment is recorded in JSON and was never applied to source or global renderer elements. Original/fresh/after top contact sheets, full-page pairs, and readable 1000-pixel panels were opened and visually inspected through the enterprise/footer content. Animated globe/preview-tab phases may differ, while the complete content and layout match.

Artifacts under `.local-tests/shkeeper-production-line/`: `mobile-home-materialization.mjs`, `home-materialization.log`, `home-followup/result.json`, and `home-followup/home-{light,dark}-{390x844,360x740}-{before-after,full-pair}.jpg` for the three cases, plus `*-full-panel-*.jpg` and original PNGs. The unused light 360 case is not implied by the filename pattern. The focused command exits 0 with `HOME MATERIALIZATION PASS: 3/3 pairs, complete heights and widths`.

## Mounted settings and payment evidence

`settings-audit.json` records actual UI/API interactions: fresh empty configuration; null and omitted arrays injected at the status transport boundary; add/remove/last-package validation; duplicate USDT amount rejection without a POST; six decimal balance accepted and seventh rejected; correction followed by one-click save; secret fields empty with two Configured badges after save/reload; status payload omits raw keys; all three exact quotes ready; Polygon amount mismatch makes overall readiness false; and SHKeeper-only Save-all persists. Mobile light/dark package editor and bottom Save/Test controls were reachable without horizontal overflow.

`payment-audit.json` records the real mounted wallet/dialog and local provider:

- Package/network selection by keyboard arrows, one create POST despite duplicate same-session clicks, and an exact request body containing only `usdt_amount` and `crypto`.
- Exact requested 10 USDT / fixed 66 balance, invoice identity, address, rendered QR, and address clipboard copy. QR response payload equals displayed address and the rendered QR uses that value; Edge's BarcodeDetector is unavailable, so an independent optical decode was not performed.
- Controlled browser clock observes the actual registered 3000 ms interval, no request before its deadline, and a request at the deadline. This does not rely on wall-clock sleeps.
- Signed partial callback records 4 USDT and credits zero; exact full payment credits 66. Actual wallet `/api/user/self` refresh fires once for the first positive credit. Four additional polling intervals after terminal state produce no status reads. Callback replay adds no quota.
- A 55 USDT payment of the 50 package credits exactly 330; excess does not enlarge entitlement.
- HTTP-200 `success:false` status responses retain the last invoice/address after the bounded retry. Retry fetches the same order and issues no additional create.
- Transaction hash submission reaches the provider's walletnotify path with backend credentials and does not itself credit quota.
- A single synthetic order's expiry was moved into the past in the isolated candidate SQLite database. Its unpaid recovery remained available; provider-confirmed payment then produced `late` with fixed credit.
- Closing a held status request produces `net::ERR_ABORTED`; no later polling occurs. Reopening has no invoice, package/network selection, or recovery hash. All Task 4 deferred mounted lifecycle checks are therefore covered.
- 1440/390/360 screenshots in light/dark cover selection, invoice, copy, recovery, and footer. The native dialog scrolls; all actions are reachable and there is no marketing panel or alternate payment design.

The ordinary amount field preserves the previous valid integer when fractional/negative text is attempted; unsafe integers disable payment. A synthetic ordinary payment method was configured only to expose this control, and no ordinary payment was submitted. That configuration was cleared after the test; final review offers SHKeeper only.

Fixture boundaries: loopback HTTP only; v2.5.32 list/quote/create/lookup/walletnotify shapes; HMAC timestamp + raw body signed with local API key; callbacks restricted to the candidate loopback port. No live SHKeeper, chain, wallet app, funds, production DB, Caddy, Docker, or Sub2API was contacted or started.

## Artifacts and residual ledger

All raw screenshots, scripts, logs, JSON evidence, SQLite databases, and binaries are under the ignored directory `D:/newapi-710-yuapi/.worktrees/shkeeper-production-line/.local-tests/shkeeper-production-line/`. Key files:

- `go-test.log`, `bun-test-final.log`, `bun-unit-test-final.log`, `existing-playwright.log`, `scoped-lint-final.log`, `scoped-format-final.log`, `full-lint-final.log`, `full-format-final.log`, and baseline counterparts.
- `literal-keys.json`, `locale-verification.json`, `i18n-reports/final-sync-report.json`.
- `visual-audit.json`, `evidence-summary.json`, `screenshots/`, `contact-sheets/global-{light,dark}-{1440,1024,390,360}.jpg`.
- `settings-audit.json`, `settings-array-error-red.json`, `payment-audit.json`, `final-review.json`, `fixture-events.jsonl`, and `contact-sheets/payment-*.jpg`.
- `baseline-process.json`, `candidate-process.json`, `fixture-process.json`, `baseline-cleanup.json`, launch/restart scripts, and process stdout/stderr logs.

Deferred items: network order is stable but not lexically sorted; a prior fixture-only snapshot test remains; concurrency/database evidence is SQLite only. The settings section remains larger than the frontend guideline. Omitted-array and six-decimal browser evidence now closes the prior unit-test coverage gaps. SHKeeper-only Save-all still also shows the legacy `No changes to save` toast, but the SHKeeper success toast and persisted data were verified; no workflow block. Unsafe ordinary integers show the legacy minimum copy (`Minimum topup amount: 1`) despite being invalid for range, while payment remains disabled; normal fractional/negative edits preserve the valid integer. Base UI radio arrow navigation works; Space on its current span composition did not select in the probe, so inherited primitive behavior is recorded without changing global components.

MySQL >=5.7.8 and PostgreSQL >=9.6 migration/concurrency verification and real-provider/real-chain confirmation remain external prerequisites. This local fixture cannot establish real network fees, confirmations, provider availability, or deployment readiness.

### SQLite contention observation

This is a **time-bounded observation**, not a lifetime event total. The refreshed sample cutoff is **2026-09-27 02:33:08.915 +08:00** (2026-09-26 18:33:08.915 UTC): seven `database is locked (5) (SQLITE_BUSY)` events were observed in candidate `candidate.stdout.log` through that cutoff. They occurred at `model/shkeeper_topup.go:453` (`tx.Save(order)`) on 2026-09-27 at 01:46:27, 01:47:27, 01:48:43, 01:56:57, 02:04:57, 02:06:27, and 02:23:27 +08:00. The first three occurred during payment/mobile/locale auditing; later observations occurred while the isolated preview and periodic reconciliation continued running. They affected unpaid, partial, and already credited late/paid orders. Candidate and fixture remain running; additional post-cutoff events may occur and do not invalidate this bounded statement. Consult the current `.local-tests/shkeeper-production-line/candidate.stdout.log` and `fixture-events.jsonl` for live state. Contention remains unresolved and is not claimed fixed or exhaustively reproduced.

The payment assertions passed partial-zero/full/replay/overpayment/late-credit invariants. After the refreshed log cutoff, a coherent read-only SQLite transaction captured the financial snapshot at **2026-09-27 02:33:08.931 +08:00**: user quota 562,000,000 = initial 100,000,000 + 462,000,000 summed from six positively credited order snapshots. Each credited order equals its fixed `package_quota`, and there are zero duplicate transaction rows within an order/network/hash identity. The seventh observed failure involved order ID 5, trade `USDT1Hn7f3PELyq1Tv274IhiHcrCa`; its snapshot remains `paid`, received USDT `10`, credited balance `66`, and credited/package quota 33,000,000. Partial and late-order invariants also remain intact. These are financial observations after the latest sampled failure, not proof that contention or a future failure is impossible.

`sqlite-contention-round-2.json` records the exact cutoff, seven sampled events, log-snapshot SHA-256, subsequent same-order provider lookups, persisted attempt timestamps, and the financial snapshot. The seventh event at 02:23:27 had attempted timestamp `1790447007`; its first later lookup was **02:24:42.364 +08:00**, and by the cutoff it had eight later lookups, most recently **02:32:27.354 +08:00**. Persisted `last_reconciled_at` advanced to `1790447547` (02:32:27), and task 51 at that time reported 24 processed / 0 credited / 0 failed. This proves a scheduler revisit and preserved credit for that order, not removal of the underlying lock contention. The condition-based observer allowed at most two configured 60-second scheduler intervals; the first observation already found the later lookup and advanced attempt, so it did not wait for elapsed time as a substitute for evidence.

The older `sqlite-contention-evidence.json` is retained as the historical 02:19:22 sample; its six observed events were only those captured at that time and are superseded by the new bounded observation above. `sqlite-contention-round-2-candidate.log` preserves the new raw snapshot, while `candidate.stdout.log` remains the live source. Code records an attempt before provider lookup and selects reconciliation candidates by `last_reconciled_at asc, id asc`; an individual failed settlement is counted and later scheduler passes revisit the order while rotating attempts. A task's overall `succeeded` label alone is not sufficient evidence, because per-order failures are reported in its result. No database settings, retry code, or process lifecycle were changed in this follow-up. SQLite preview contention and observed retries do not establish MySQL/PostgreSQL production concurrency or migration behavior.

## Cleanup prerequisites

Only the task-owned baseline PID 26780 was stopped after its exact executable path matched the detached baseline tree. The baseline worktree/database/artifacts remain for audit. Candidate PID 33280 and fixture PID 39008 stay running for review; their exact paths/command lines must be checked again before any later stop. The old 13035 preview belongs to Task 6 cleanup, which must separately inspect ownership and account for the baseline gate exceptions above. No hot switch or production action is authorized by this handoff.

Fix Round 1 temporarily restarted that same verified baseline executable with its existing isolated `baseline.db` as PID 15636 on 13036. Its database identity (synthetic admin, default theme, local ServerAddress) and clean detached HEAD were checked before restart. PID 15636 was stopped after the focused audit at 02:21:07 +08:00 following exact executable-path verification; `home-followup/baseline-cleanup.json` records `stillRunning:false`. Candidate 33280, fixture 39008, and old preview 16220 were left running and unchanged.

## Task 6 authorized local cleanup

Pre-deletion inspection on 2026-09-27 verified the clean candidate at `f1c702da08baa4d4204541735e5364eb05c4c0cf` on `codex/shkeeper-usdt-production-line`, HTTP 200 health, unchanged executable SHA-256, empty committed/working immutable UI diffs, and clean whitespace checks. Task 5's reproduced baseline Bun/lint/format exceptions do not indicate a candidate regression; the separately documented SQLite contention remains unresolved, with its bounded evidence and external production gates unchanged.

The exact obsolete worktree was `D:/newapi-710-yuapi/.worktrees/shkeeper-usdt`, branch `codex/shkeeper-usdt`, HEAD `79ac694b3bd6296c158c66a349c3a62c20ec7e1b`; status contained only untracked `.superpowers/`. Its port 13035 listener was PID 16220, executable `.local-tests/shkeeper-preview/yuapi-shkeeper-preview.exe` inside that exact tree. The detached comparison tree was `D:/newapi-710-yuapi/.worktrees/shkeeper-production-baseline` at `27023295d21bcc824c5af1d6e9e36c22c54db3a5`, clean, with no listener on 13036 and no process executable or command referencing it.

Before deletion, `.local-tests/shkeeper-production-line/task-6-cleanup-before.json` captured full worktree and local branch listings, both target statuses/heads, the obsolete branch's 23 feature commits since its original `d7697c098` base, untracked paths, exact listener process paths/commands, candidate identity/health, and the main checkout's existing dirty status. `task-6-preserved/` holds the stopped baseline's database, executable and logs plus the obsolete tree's local `.superpowers` evidence; all 39 copied files passed SHA-256 comparison against their sources, recorded in `task-6-preserved-manifest.json`. Existing comparison screenshots, logs and JSON evidence already reside in the candidate artifact directory.

Cleanup completed on 2026-09-27. PID 16220 was stopped at 02:48:56 +08:00 only after fresh exact executable-path and process-creation-time verification. The obsolete worktree's initial `git worktree remove --force` unregistered it but left two dangling workspace dependency junctions and `web/package.json`. Tool policy rejected the attempted PowerShell junction removal before execution; the controller completed fresh validation and nonrecursive junction/file/empty-directory cleanup entirely within `cmd.exe`. It also unlinked the comparison tree's two verified dependency junctions without traversing their main/candidate targets. The obsolete root is absent; after its original head was rechecked, `git worktree prune` and `git branch -D codex/shkeeper-usdt` succeeded at 03:02:04 +08:00. The clean, process-free detached comparison worktree was then removed with the exact authorized `git worktree remove --force` path and pruned at 03:02:43 +08:00.

Post-checks confirm both removed paths absent, the obsolete branch absent, no listeners on 13035/13036, and only the correct feature worktree/branch remaining among these three. Candidate PID 33280 and fixture PID 39008 retain their original executable/command/creation identities on loopback ports 13037/13038; both return HTTP 200, candidate status reports success, and the candidate executable hash is unchanged. The existing isolated candidate database and all Task 5 artifacts remain in place. Immutable committed/working diffs and whitespace checks pass. Full worktree/branch comparisons show every unrelated registration/ref unchanged, and the main checkout retains its pre-existing dirty status unchanged. `task-6-stop.json`, `task-6-removal-interruption.json`, `task-6-cleanup-blocked.json`, `task-6-residue-recovery.txt`, `task-6-cleanup-after.json`, and `task-6-cleanup-final.json` retain the sequence and full listing evidence. The documentation-only cleanup commit preserves this record; final candidate tracked-tree cleanliness is checked after committing. No production, Docker, Sub2API, Caddy, or remote branch action occurred. Real-provider/real-chain and MySQL/PostgreSQL production gates remain open.
