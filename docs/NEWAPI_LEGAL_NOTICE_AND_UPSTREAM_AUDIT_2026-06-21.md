# NewAPI Legal Notice And Upstream Audit - 2026-06-21

This note records the production legal notice update and the upstream protocol
audit performed on 2026-06-21. Do not put API keys, passwords, refresh tokens,
access tokens, or full account payloads in this file.

## Production Legal Configuration

Production NewAPI was updated without restarting NewAPI.

Updated option keys:

| Key | Purpose | Result |
| --- | --- | --- |
| `Notice` | Login/site announcement | bilingual CN/EN notice live |
| `legal.user_agreement` | User agreement page and login/register consent | bilingual CN/EN agreement live |
| `legal.privacy_policy` | Privacy policy page and login/register consent | bilingual CN/EN privacy/compliance notice live |

Verification:

- `https://dtrljm.com/api/status` returned `user_agreement_enabled=true`.
- `https://dtrljm.com/api/status` returned `privacy_policy_enabled=true`.
- `https://dtrljm.com/api/notice` returned HTTP 200.
- `https://dtrljm.com/api/user-agreement` returned HTTP 200.
- `https://dtrljm.com/api/privacy-policy` returned HTTP 200.

The legal content states, in Chinese and English:

- Mainland China normal web access to login, registration, user console, and
  admin pages is blocked or challenged.
- Hong Kong and Taiwan are not included in the mainland web-block rule.
- Authorized API clients should use `https://api.dtrljm.com/v1`.
- Users are responsible for their own requests, content, downstream users,
  network access method, applicable laws, upstream provider terms, and site
  rules.
- If a user bypasses regional access controls through VPN, proxy, accelerator,
  overseas network, or similar methods, that is the user's own action and
  responsibility.
- Illegal content, reverse engineering, credential abuse, mass registration,
  rate-limit or risk-control circumvention, account/key resale, attacks,
  scraping, fraud, and upstream policy violations are prohibited.
- The site may restrict, suspend, terminate, or delete accounts, keys, and
  requests for risk-control, upstream-policy, compliance, abuse-prevention, or
  stability reasons.
- The external recharge portal is `https://pay.ldxp.cn/shop/yuqi`.

Operational reminder: these terms are a practical compliance boundary and user
responsibility notice. They are not a magic liability shield; keep Cloudflare or
origin-side mainland web restrictions enabled for the web/admin hosts.

## Access Boundary

Keep this split:

| Hostname | Role | Mainland normal access |
| --- | --- | --- |
| `api.dtrljm.com` | public API endpoint | open |
| `dtrljm.com` | NewAPI web/user console | block or challenge |
| `www.dtrljm.com` | NewAPI web alias | block or challenge |
| `admin.dtrljm.com` | NewAPI admin/login entry | block or challenge |
| `newapi.dtrljm.com` | NewAPI compatibility/admin entry | block or challenge |

Do not add a broad country block to the whole `dtrljm.com` zone, because that
would also block `api.dtrljm.com` and break mainland API clients.

## Sub2API Upstream Audit

Fetched `upstream/main` from `https://github.com/Wei-Shaw/sub2api.git`.

Observed upstream head:

```text
4a5665da chore: sync VERSION to 0.1.137 [skip ci]
```

Useful upstream changes currently ahead of our private branch include:

- `6c7203d8` - preserve SSE `event:error` body so ops logs reflect real upstream
  errors.
- `7fb3e1aa` - protocol-aware reasoning/thinking handling for
  Anthropic-compatible upstreams.
- `de38d623` - Anthropic 429 window reset handling.
- `9e9e154f` / `56c62c59` - improve IP ACL denial messages.
- pricing fallbacks for several newer Chinese/provider models.

Do not merge upstream wholesale right now. The upstream diff is very large and
would remove private provider adapters, deployment snippets, and local
operations documentation that this repo intentionally keeps.

## Selected Patch Merged

Merged only the focused SSE diagnostics patch from upstream:

```text
6c7203d8 fix(gateway): preserve SSE event:error body so ops logs reflect real upstream errors
```

Why this patch first:

- It directly improves the class of "stream return error" problems seen during
  image/API debugging.
- It preserves the real upstream SSE error JSON in `UpstreamFailoverError`.
- It adds a `stream_error` ops event so production logs can show whether the
  upstream was overloaded, rate-limited, or returned another structured error.
- It keeps the old `"have error in stream"` string for log-search
  compatibility.

Focused verification was run in an isolated server temp checkout, not against
the production container:

```bash
docker run --rm \
  -v /tmp/sub2api-test-sse/backend:/src \
  -w /src \
  golang:1.26.4 \
  go test -v -count=1 -tags unit \
  -run 'TestHandleStreamingResponse_SSEErrorEvent|TestHandleStreamingResponse_FailoverBodyDoesNotLeakAddresses' \
  ./internal/service
```

Result:

```text
PASS
ok github.com/Wei-Shaw/sub2api/internal/service
```

## Sub2API Production Deployment

Sub2API was rebuilt and redeployed after the focused unit tests passed.

New production image:

```text
sub2api-provider-adapters:sse-stream-error-body-20260621-b5389943
```

Previous production image:

```text
sub2api-provider-adapters:grok-gemini-model-map-20260620-214218
```

Compose backup on the server:

```text
/opt/sub2api/docker-compose.yml.bak-sse-error-body-20260621-b5389943
```

Deployment scope:

- recreated only the `sub2api` container;
- did not restart NewAPI;
- did not recreate NewAPI MySQL or Redis;
- did not change NewAPI channels or user-facing API keys.

Post-deploy verification:

- `sub2api` image was `sub2api-provider-adapters:sse-stream-error-body-20260621-b5389943`.
- `sub2api` container was `running healthy`.
- `newapi`, `newapi-mysql`, and `newapi-redis` remained healthy.
- `https://api.vyywcw.cn/health` returned HTTP 200.
- `https://api.dtrljm.com/api/status` returned HTTP 200.
- NewAPI legal endpoints continued to return HTTP 200.

## NewAPI Upstream Audit

Fetched `https://github.com/QuantumNous/new-api.git`.

Observed latest tag:

```text
v1.0.0-rc.14
```

Production NewAPI is still running the upstream image equivalent to
`v1.0.0-rc.10`. Do not upgrade NewAPI during active traffic without a separate
maintenance window. The rc.10 custom recharge-entry patch is still prepared in:

```text
patches/newapi/rc10-recharge-entry.patch
```

NewAPI legal/notice settings did not require a code deploy because the existing
options system already supports `Notice`, `legal.user_agreement`, and
`legal.privacy_policy`, and runtime option sync picked up database updates.

## Follow-Up Backlog

- Evaluate Sub2API `7fb3e1aa` reasoning/thinking protocol changes separately;
  do not combine with unrelated deployment changes.
- Evaluate `de38d623` Anthropic 429 window reset once we can run the targeted
  rate-limit tests in the same isolated workflow.
- Plan a low-traffic NewAPI maintenance window before moving from rc.10 to a
  newer upstream tag.
- Keep `api.dtrljm.com` open while keeping web/admin hosts blocked or
  challenged for mainland normal access.
