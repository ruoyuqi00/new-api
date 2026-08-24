# Parent Feature Audit

> **Scope:** read-only comparison against the configured `origin/main` parent repository. This audit does not authorize production changes or a wholesale merge.

## Baseline rule

YuAPI remains the source of truth. Parent features may be considered only as isolated, reviewed patches. No parent frontend, routing tree, billing state machine, or configuration is merged wholesale, and the current YuCore brand/UI baseline is preserved.

## Already present in YuAPI

The current baseline already contains equivalent or stronger implementations for:

- automatic group selection and group-aware model listing;
- critical rate limiting on existing sensitive routes;
- DeepSeek Responses request handling;
- tool pricing and usage billing;
- authenticated session cookies, refresh, origin checks, and session-limit recovery;
- channel/account-pool affinity, cache-key scoping, stream recovery, accepted/ambiguous billing, and actual response model logging;
- domestic/Grok media routing, private asset proxying, and task billing.

These parent features are not to be re-merged.

## Candidate patches

### 1. User-scoped critical rate limit — recommended, separate patch

Parent commit `1da23d6b3` adds a user-keyed critical limiter to access-token generation and affiliate-quota transfer routes. YuAPI currently has the global critical limiter but not this narrower user scope. This is unrelated to model dispatch and does not touch the brand UI or billing settlement. If enabled, it should be ported as a backend-only patch with tests for per-user isolation, Redis/in-memory parity, and unchanged response codes.

### 2. Ali `top_p` omission fix — conditional compatibility patch

Parent commit `2399de97d` stops the Ali adapter from injecting `top_p` when the client omitted it. YuAPI still normalizes an omitted value to a small non-zero value in the Ali adapter. This should be ported only if an Ali/Qwen channel is in active scope; it is not needed for the current OpenAI-compatible Herohao channel. If ported, the test must distinguish omitted `top_p` from explicit `0` and preserve explicit values according to the relay request pointer rules.

### 3. Per-channel HTTP transport controls — defer

Parent commit `e99a9bd86` introduces per-channel transport policy and sharded clients. YuAPI currently has custom HTTP/2 keep-alive, proxy caching, origin-bound credentials, stream recovery, channel cooldown, and conservative retry/billing logic. A wholesale port could change response-header waits, redirect behavior, connection reuse, retry timing, or cache affinity. It should be reconsidered only after stage timing proves a channel-specific transport bottleneck and after dedicated no-double-submit tests pass.

### 4. Parent tiered billing retry fixes — do not merge wholesale

Parent commits `cfaba1dd6` and `df43f8015` address group-switch settlement in the parent state machine. YuAPI has a newer durable claim/finalization path for accepted and ambiguous submissions, frozen billing snapshots, and violation-fee handling. The parent patch must not replace this path. Any future comparison must be a contract-level test review, not a source merge.

### 5. Request compression and model categorization — low priority

Parent zstd request decompression (`0f9f668c6`) and Qwen model categorization fixes (`823e26304`) are independent of the domestic model channel. They can be considered separately; neither is required to expose or bill the six requested models.

## Recommendation for this model-group work

Do not block the domestic model groups on parent feature migration. First implement the groups using the approved YuAPI-native model mapping and billing expression paths. Track the user-scoped critical limiter as an independent hardening patch, and keep the Ali omission fix conditional on actual Ali channel usage. Defer transport-control changes until measured evidence exists.

## Verification gate for any borrowed patch

Every borrowed feature must have:

1. a YuAPI-specific design note and isolated commit;
2. a RED test proving the missing behavior;
3. backend tests across SQLite, MySQL, and PostgreSQL-compatible query paths where persistence is involved;
4. no UI baseline or production configuration changes;
5. a rollback commit or compensating configuration;
6. a fresh `/api/pricing` snapshot if the patch can affect model visibility or pricing.

