# GPT Codex Prelude Production Handoff

> Status: deployed and under observation
>
> Recorded: 2026-08-16 (Asia/Shanghai)
>
> Scope: GPT-only `/v1/responses` Codex metadata prelude and upstream
> `codex.rate_limits` filtering. No video, image, Claude, Gemini, billing,
> database, authentication, or frontend behavior was intentionally changed.

## 1. Source provenance

The runtime found immediately before this release did not match the older
2026-08-14 handoff. Caddy was actually routing to application source commit
`d6605a79a0ccb34c1d89e982f82b8b10058e8c53` through a retained network alias.
That commit contains the current video, documentation, and managed sample-asset
work.

The GPT patch was first committed as `00e5c32b1` on
`codex/scanner-cache-key-safety-20260813`. Deploying that branch directly would
have reverted the newer media work, so only that single patch was cherry-picked
onto the exact running application source commit. The production release commit
is:

```text
b6a6157afb9bb085872b10416203e80031e77b0a
```

Release branch:

```text
codex/gpt-codex-prelude-production-20260816
```

The unrelated parent commit `6837fdc20` from the original GPT development
branch is not part of this production release.

## 2. Current production runtime

| Item | Current value |
| --- | --- |
| Application commit | `b6a6157afb9bb085872b10416203e80031e77b0a` |
| Image | `yuapi:production-20260816-b6a6157af-gpt-codex-rc1` |
| Image ID | `sha256:5e3863d2b6f40f240b1030c29aaf3eeacdd1747ed683f4d4b3ebdcb1a3ce59ca` |
| Running container | `newapi-candidate-20260816-b6a6157af-gpt-codex-rc1` |
| Private binding | `127.0.0.1:13019 -> 3000/tcp` |
| Primary application network alias | `yuapi-gpt-codex-candidate-b6a6157af` |
| Caddy release-network alias | `newapi-candidate-20260816-b6a6157af-gpt-codex-rc1` |
| Caddy target count | Exactly two references to the Caddy release-network alias |
| Caddy backup | `/opt/edge/Caddyfile.pre-20260816T144334Z-d6605a79a-to-b6a6157af` |
| Build directory | `/opt/yuapi-builds/b6a6157af-gpt-codex-prelude` |

The container is attached to `sub2api_sub2api-network` for application
dependencies and to `yuapi-release-61527e8f9` for Caddy connectivity. The
temporary environment export used to create the container was deleted after
startup. Do not write environment values or container secrets into Git.

The production homepage currently references these static assets:

- `/static/css/6189.315a58962e.css`
- `/static/css/index.16fc389747.css`
- `/static/js/6189.b849ad6f8b.js`
- `/static/js/index.134936448f.js`
- `/static/js/lib-react.a6dd11adaa.js`
- `/static/js/vendor-tanstack.7425bb6434.js`
- `/static/js/vendor-ui-primitives.f8cdb75d06.js`

## 3. Retained rollback runtime

The immediately previous traffic target remains running, healthy, and has
restart count zero:

```text
newapi-candidate-20260816-d6605a79a-media-docs-rc2
```

It remains bound to `127.0.0.1:13018` and still owns the old Caddy target alias
`newapi-production-20260814-auth-origin-290db8f25` on
`yuapi-release-61527e8f9`. This is intentional: Caddy generations created before
the graceful reload can continue to serve in-flight streams without a 502.

Do not stop, rename, remove, or disconnect either the current container or this
retained container during the observation period. The older
`newapi-production-20260814-61527e8f9` container also remains untouched but is
not the immediate rollback target for this release.

## 4. Behavior contract

The new behavior is limited to successful streaming `/v1/responses` requests
whose original downstream model name starts with `gpt-`.

For a request with a Codex client signal in `Originator`, `User-Agent`,
`X-Codex-Turn-Metadata`, or `X-Codex-Beta-Features`, the gateway flushes this
event after upstream HTTP 200 and before reading the upstream SSE body:

```text
event: codex.response.metadata
data: {"type":"codex.rate_limits","plan_type":"pro","rate_limits":{"allowed":true,"limit_reached":false,"primary":null,"secondary":null},"credits":null}
```

For GPT Responses traffic, upstream events with JSON type
`codex.rate_limits` are discarded. Ordinary GPT clients do not receive the
synthetic event. Non-GPT models retain their previous injection and filtering
behavior exactly. Affinity, response IDs, terminal usage, billing, incomplete
stream handling, and token accounting remain on their existing paths.

## 5. Verification evidence

Before deployment:

- the exact running source `d6605a79a` passed `go test ./... -count=1`;
- the integrated release passed `go test -p 1 ./... -count=1`;
- the focused GPT/Codex ordering, client-signal, ordinary-client, non-GPT
  pass-through, header, usage, and filtering tests passed;
- the source archive SHA-256 matched before and after transfer;
- the Docker build completed the default frontend, classic frontend, and Go
  backend stages;
- the candidate reached healthy with restart count zero before Caddy changed.

The first integrated full test attempt exhausted the local Windows linker
memory while building an unchanged Gemini test package. The same Gemini package
passed alone, the diff contained no Gemini file, and the complete suite passed
with build parallelism limited to one. This was an environment resource failure,
not a test assertion or code failure.

Before cutover, the candidate and previous live instance had identical homepage
SHA-256 values and identical static JS/CSS fingerprints. Their public status and
unauthenticated media-catalog status codes also matched.

After graceful reload:

- the main API returned 200 for two server-side runs of 20 and 50 consecutive
  requests;
- the VIP API returned 200 for the same two runs;
- the homepage returned 200 for 10 consecutive requests and retained its
  pre-cutover SHA-256;
- independent external checks returned 200 for ten requests each to the main
  API, VIP API, and global homepage when transient client-network errors were
  retried;
- exact Caddy `status=502` count was zero;
- candidate Caddy dial, connection, and name-resolution error count was zero;
- candidate panic, fatal, scanner, migration, and database error count was zero;
- the candidate received production `/v1/responses` traffic;
- both the current and retained containers remained healthy with restart count
  zero.

An unauthenticated Codex-shaped request returned 401 and did not receive the
synthetic event, confirming that the event is not emitted before successful
request authentication and upstream acceptance. No real user identifier,
request metadata, credential, or payload is recorded in this handoff.

## 6. Immediate rollback

Rollback changes only Caddy routing. Do not stop the current container first.

1. Confirm `newapi-candidate-20260816-d6605a79a-media-docs-rc2` is healthy and
   reachable from `yuapi-caddy` through
   `newapi-production-20260814-auth-origin-290db8f25:3000`.
2. Copy
   `/opt/edge/Caddyfile.pre-20260816T144334Z-d6605a79a-to-b6a6157af` over
   `/opt/edge/Caddyfile`.
3. Run `caddy validate` inside `yuapi-caddy` against
   `/etc/caddy/Caddyfile`.
4. Run a graceful `caddy reload`; do not restart the Caddy container.
5. Verify the main API, VIP API, homepage, Caddy 502 count, previous-container
   health, and database error count.
6. Keep the `b6a6157af` container and image for investigation until an operator
   explicitly approves cleanup.

Do not restore a database snapshot or alter user balances for this rollback;
this release contains no database migration or billing change.

## 7. Cleanup gate

No production container, image, Caddy backup, release-network alias, or build
directory should be removed merely because the initial checks passed. Cleanup
requires a later observation review and explicit operator approval.
