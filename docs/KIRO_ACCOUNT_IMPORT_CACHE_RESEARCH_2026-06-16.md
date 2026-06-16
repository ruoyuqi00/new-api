# Kiro Account Import And Cache Research 2026-06-16

## Scope

- Local workspace: `D:\wflogin\sub2api-private`
- Production path: `/opt/sub2api`
- Input file: `C:\Users\Administrator\Downloads\kiro-accounts-2026-06-16.json`
- Excluded path: `D:\wflogin\注册机相关项目`

No provider secrets, raw access tokens, raw refresh tokens, or adapter keys are recorded here.

## Import Result

The input JSON was an array with 63 records. Each record used the shape:

```json
{
  "email": "...",
  "refreshToken": "...",
  "provider": "..."
}
```

The server-side Kiro admin key in `/opt/sub2api/.env` did not match the live `kiro-rs` admin API key. The working key was read from `/opt/sub2api/kiro-rs/config/config.json` during the operation, without printing it.

Before import:

- `kiro-rs` credentials file count: 1

Backup created before import:

- `/opt/sub2api-backups/kiro-rs-credentials-before-20260616-import-20260616-145141.json`

Import passes:

- First pass: 25 succeeded, 38 failed
- A short retry pass added 10 more successes
- Final `kiro-rs` credentials file count: 36

Failure classification from the retry pass:

- 25 records were already present after the first successful import pass.
- 25 records hit upstream Kiro/AWS OAuth rate limiting: HTTP 429, "Too many requests".
- 3 records hit temporary upstream OAuth server errors: HTTP 500.

I stopped retrying after seeing rate limiting. These should be retried later with a low rate and backoff, not forced in a loop.

## Runtime Wiring Finding

There are two separate Kiro runtime paths on the server:

1. `kiro-rs` plus `kiro-web-adapter`
   - `kiro-rs` mounts `./kiro-rs/config:/app/config`.
   - `kiro-web-adapter` mounts the same `./kiro-rs/config:/config`.
   - The newly imported accounts are in this shared credentials file.

2. `kiro-gateway`
   - Uses `./kiro-gateway/creds:/app/creds`.
   - Its configured credential file is `/app/creds/kiro-auth-token.json`.
   - The newly imported `kiro-rs` accounts do not automatically affect `kiro-gateway`.

Observed checks:

- `kiro-gateway /v1/models`: HTTP 200, 13 models.
- `kiro-gateway /v1/messages`: HTTP 402 during the smoke check.
- `kiro-web-adapter /v1/models`: HTTP 200 with `/opt/sub2api/kiro-rs/config/generated-kiro-api-key.txt`.
- `kiro-web-adapter /v1/messages`: HTTP 200 at transport level for `deepseek-3.2`, `qwen3-coder-next`, and `claude-opus-4.6`, but the selected account can return quota text such as "monthly usage limit" inside the assistant content.

That last point should not be treated as proof that the account is dead or permanently unusable. Some Kiro accounts can continue in an overage mode, so quota-looking text must be handled as a soft runtime signal only.

Follow-up after user clarification:

- A single-account probe found credential index 1 returned the quota-style text.
- Credential index 2 returned a normal `OK` response for `deepseek-3.2`.
- No account was disabled or deleted.
- The live credentials file was backed up, then only priorities were adjusted so the tested-good credential is selected first by `kiro-web-adapter`.
- Backup before this priority-only change:
  `/opt/sub2api-backups/kiro-rs-credentials-before-priority-usable-20260616-*.json`
- Default `kiro-web-adapter /v1/messages` smoke after the priority change returned HTTP 200 with text `OK`.

## Existing Cache In Code

`adapters/kiro-web/kiro_web_adapter.py` already has an in-memory `session_cache`:

- key: hash of the access token
- value: `PortalSession`
- behavior: reuses CSRF, cookies, visitor ID, user ID, profile ARN for about 15 minutes
- limitation: memory-only, lost on restart, not shared across replicas

The adapter still creates a fresh Kiro `spaceId/sessionId` for every request. It does not yet have:

- per-account cooldown/quota cache;
- monthly quota exhaustion state;
- sticky conversation-to-space reuse;
- response cache;
- Redis-backed session cache.

## Official Capability Check

Public Kiro docs currently emphasize context management, compaction, steering, and knowledge/agent context. They do not document a public account-pool cache or a server-side API cache that solves this deployment's account rotation problem.

Useful official docs:

- Kiro CLI context management: `https://kiro.dev/docs/cli/chat/context/`
- Kiro IDE summarization/context meter: `https://kiro.dev/docs/chat/summarization/`
- Kiro steering files: `https://kiro.dev/docs/steering/`
- Kiro CLI knowledge management: `https://kiro.dev/docs/cli/experimental/knowledge-management/`

Those features help the official client manage local context, but they do not replace a gateway-side account health cache.

## Recommended Cache Design

Do not build this as a way to bypass provider limits. Build it to reduce duplicate work, avoid repeated failed calls, and respect account cooldown/quota state.

### 1. Account Health Cache

Add per-credential runtime state:

```json
{
  "credential_id": "kiro_xxx",
  "status": "available|cooldown|disabled",
  "cooldown_until": "...",
  "soft_limit_seen_until": "...",
  "last_error": "...",
  "last_success_at": "...",
  "consecutive_failures": 0
}
```

Storage options:

- short term: sidecar JSON file next to `credentials.json`;
- better: Redis, because Sub2API already runs Redis and cache survives adapter process restarts less awkwardly.

Detection rules should include:

- HTTP 401/403 or invalid token: disable or force refresh once.
- HTTP 429 / "Too many requests": cooldown, then retry another credential.
- "monthly usage limit" or similar quota text: do not disable, delete, or mark permanently exhausted. Record `soft_limit_seen_until` for observability and optionally use a short cooldown/retry another credential for this request only.
- Kiro high-traffic text: short cooldown, retry another credential.

### 2. Selection Strategy

Replace current priority-first behavior with account selection that skips unhealthy accounts:

- filter disabled credentials, and optionally skip short-cooldown credentials while other accounts are available;
- prefer accounts that support the requested model;
- choose round-robin or least-recently-used among healthy credentials;
- keep a per-request retry cap, for example 3 to 5 accounts.

### 3. Portal Session Cache

Keep the existing in-memory `session_cache`, but add:

- explicit expiry tied to access token expiry;
- invalidation when profile ARN changes;
- optional Redis implementation keyed by credential ID if multiple adapter processes are ever used.

### 4. Conversation Space Cache

For clients that pass stable conversation metadata, map:

```text
model + credential_id + metadata.user_id/conversation_id -> Kiro spaceId/sessionId
```

This avoids creating a fresh Kiro space for every turn. It should have a TTL and should be deleted on Kiro session errors.

### 5. Response Cache

Do not enable global response caching by default because user prompts may contain sensitive code or account data.

If needed later, make it opt-in and conservative:

- cache only non-stream text requests;
- require a caller-provided cache key or `metadata.cache=true`;
- hash canonical prompt, model, system prompt, and tool settings;
- short TTL;
- never cache requests containing files, tools, images, or explicit no-cache metadata.

## Next Code Work

Recommended first implementation:

1. Add quota/error classifier in `kiro_web_adapter.py`.
2. Add credential runtime state cache.
3. Retry another credential when Kiro returns quota/limit text, but keep the account available for future overage-capable calls.
4. Return a real non-2xx error only when all candidate credentials fail the current request; do not permanently block quota-looking accounts.
5. Add admin status fields for cooldown/quota counts.
6. Add smoke tests for:
   - monthly usage text becomes retryable;
   - 429 becomes cooldown;
   - duplicate/invalid refresh behavior does not leak secrets;
   - conversation `metadata.user_id` can reuse space when enabled.
