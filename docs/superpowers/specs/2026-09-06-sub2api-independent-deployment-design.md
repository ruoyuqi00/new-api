# Independent Sub2API Pool Deployment Design

**Date:** 2026-09-06
**Status:** Implemented 2026-09-07
**Target repository:** `https://github.com/ruoyuqi00/sub2api-provider-adapters.git`
**Upstream:** `https://github.com/Wei-Shaw/sub2api.git`, branch `main`
**Public administration domain:** `https://sub2.yuaiapi.com`

## Objective

Restore Sub2API as a separately maintained account-pool service for YuAPI. The
new deployment follows the current official Sub2API `main` branch, starts with
fresh PostgreSQL, Redis, and application data, exposes only the administration
surface to the public Internet, and exposes model relay endpoints only to YuAPI
over a private Docker network.

The Sub2API source is stored in the user-owned repository on a dedicated
`sub2api` branch. That branch is an upstream mirror line, not a YuAPI feature
branch, and must never be merged into the YuAPI/NewAPI `main` history.

## Current Facts

- The official Sub2API `main` commit observed during design was
  `ab99d56e9626e6cd731592dae8553c9758a0efa2`, corresponding to version `0.2.1`.
- The old application under `/opt/sub2api` was stopped in July 2026, while its
  PostgreSQL, Redis, Caddy, configuration, and volumes were retained.
- The new deployment must not reuse or delete that old state.
- At design time, `sub2.yuaiapi.com` returned HTTP `525` because the origin TLS
  virtual host did not exist. The implemented Caddy host now serves the login
  page successfully through Cloudflare.
- The deployment target is `199.231.85.194`; key-based SSH access as `root`
  has been verified. The previously inspected `154.219.122.197` host was not
  the current production target and must not be modified. Live container,
  port, proxy, and network state must still be audited before any production
  mutation.

## Non-Goals

- Do not import or restore any old Sub2API accounts, groups, keys, database
  rows, Redis data, or application configuration.
- Do not delete the old `/opt/sub2api` directory, containers, images, volumes,
  configuration, or backups.
- Do not remove the account-pool implementation already present in YuAPI.
- Do not merge the Sub2API source branch into YuAPI/NewAPI `main`.
- Do not expose Sub2API model relay endpoints through the public domain.
- Do not automatically deploy every upstream commit without validation.

## Source-Control Design

### Branch topology

The remote branch `sub2api` in `ruoyuqi00/sub2api-provider-adapters` is created directly from
`Wei-Shaw/sub2api:main`, preserving official history. It is intentionally
unrelated to the YuAPI/NewAPI `main` line in the same GitHub repository.

The initial push is allowed only if `refs/heads/sub2api` does not already exist.
If it exists at execution time, its current object ID and ancestry must be
audited before any update. It must not be force-pushed merely to make it match
the design.

The following separation rules apply:

| Branch | Contents | Allowed updates | Forbidden operation |
| --- | --- | --- | --- |
| `main` and YuAPI feature branches | YuAPI/NewAPI source and documentation | Existing YuAPI workflow | Merge or replace with Sub2API history |
| `sub2api` | Exact official Sub2API source history | Fast-forward from official `main` after review | Add production secrets, YuAPI files, or private deployment overlays |

### Controlled branch synchronization

Each update follows this sequence:

1. Fetch official `Wei-Shaw/sub2api:main` and record its commit SHA.
2. Review release notes, migrations, Compose changes, and gateway route changes.
3. Confirm the user-owned `sub2api` branch is an ancestor of official `main`.
4. Fast-forward the user-owned branch; never merge the YuAPI branch into it.
5. Build an image tagged with the full or short source SHA, for example
   `yuapi-sub2api:ab99d56e`.
6. Validate the image and database migration against a disposable or backed-up
   environment before replacing the running image.
7. Pin the production Compose file to that immutable tag. Never deploy the
   floating `latest` tag.

Pushing the mirror branch does not itself trigger production deployment.

## Production Isolation

### New deployment identity

- Directory: `/opt/yuapi-sub2api-v2`
- Compose project: `yuapi-sub2api-v2`
- Application container: `yuapi-sub2api-v2-app`
- PostgreSQL container: `yuapi-sub2api-v2-postgres`
- Redis container: `yuapi-sub2api-v2-redis`
- Private application network: `yuapi-sub2api-v2-internal`
- Shared YuAPI data-plane network: existing external network `sub2api_sub2api-network`
- Sub2API alias on the shared network: `sub2api-internal`
- Host administration listener: `127.0.0.1:18473 -> app:8080`

Before creation, execution must prove that the directory, Compose project,
container names, network names, volume names, and host port are unused. A name
or port collision stops deployment; the process must not remove, rename, or
take over another resource automatically.

### Storage and networks

The new project creates three new named volumes with the Compose project prefix:

- `yuapi-sub2api-v2_sub2api_data`
- `yuapi-sub2api-v2_postgres_data`
- `yuapi-sub2api-v2_redis_data`

The exact Docker-generated names must be confirmed with `docker volume ls`
after Compose validation. PostgreSQL and Redis join only the private application
network and expose no host ports. The application joins both its private
network and the existing `sub2api_sub2api-network` data plane.

The running YuAPI and edge containers were already members of
`sub2api_sub2api-network`, so neither was attached to a new network. The new
application joins that network with alias `sub2api-internal`, but never exposes
PostgreSQL or Redis to it. YuAPI can therefore reach the application without
access to Sub2API's data stores.

### Configuration and secrets

The deployment uses a server-only `.env` file owned by root with mode `0600`.
It contains independently generated values for:

- PostgreSQL password
- Redis password
- initial administrator password (removed after the user changed it)
- JWT secret
- TOTP encryption key

The initial administrator identity is `admin@yuapi.internal` unless the user
chooses another address before deployment. The generated password is delivered
once after deployment and must be changed on first login. Registration,
password recovery, third-party login, and payment features are disabled. TOTP
support retains a fixed encryption key; the administrator should enroll 2FA
after first login.

No secret is printed in deployment logs, committed to Git, placed in Caddy,
or included in the final operations document. A root-readable credential file
may be kept temporarily on the server until the user confirms successful login,
then removed with explicit user approval.

## Public Administration Plane

The existing edge proxy that owns ports 80/443 receives a new virtual host for
`sub2.yuaiapi.com`; a second proxy must not compete for those ports. The proxy
terminates valid origin TLS so Cloudflare no longer returns `525`, and forwards
allowed traffic through the existing private Docker network to
`sub2api-internal:8080`. The loopback listener remains available for local
health and administrative checks only.

The public host uses a positive allowlist:

- frontend document and static assets required by the embedded administration UI;
- administration routes under `/admin`;
- login and session calls under `/api/v1/auth/*`;
- public settings under `/api/v1/settings/public`;
- authenticated administration calls under `/api/v1/admin/*`;
- `/health` and `/setup/status` where required for availability checks.

Non-GET/HEAD browser routes outside the control plane are denied. Model and
gateway surfaces are denied at the proxy, including current forms under `/v1`,
`/v1beta`, `/responses`, `/models`, `/chat`, `/embeddings`, `/images`,
`/videos`, `/backend-api`, `/alpha`, `/messages`, `/antigravity`, `/tts`,
`/stt`, `/custom-voices`, `/realtime`, `/web_search`, and `/x_search`.

The allowlist is the security boundary: an unknown path added by a future
Sub2API release stays denied until deliberately classified. Cloudflare Access
is not required by the approved design. Cloudflare proxying, Sub2API login,
registration disablement, rate limiting, HTTPS, a strong unique password, and
administrator 2FA remain the protection layers.

Direct access to `18473` from the Internet is impossible because Docker binds
it to loopback. Firewall and socket inspection must confirm there is no public
listener for the app, PostgreSQL, or Redis.

## YuAPI Data Plane

Sub2API is initialized with an empty `yuapi-internal` group and an API key held
only by YuAPI. The internal base URL is:

```text
http://sub2api-internal:8080
```

The container's internal `8080` port does not conflict with host port `18473` or
other containers. Docker service discovery scopes the name to the existing
`sub2api_sub2api-network` data plane.

The deployed group platform is OpenAI, so YuAPI uses its existing standard
OpenAI-compatible channel adapter. This covers the group's `/v1/models`, chat,
and Responses contracts without rebuilding or replacing the running YuAPI
image. No Sub2API-specific adapter or YuAPI source change was needed. Future
Anthropic or Gemini pools should use separate Sub2API groups, keys, and matching
YuAPI channel types rather than combining protocol boundaries.

The new YuAPI channel is created disabled and with no active abilities. Empty
Sub2API groups must never receive live YuAPI traffic. After the user adds
accounts, execution fetches the actual model list, enables only verified model
abilities, assigns a lower gray-release priority, and runs real protocol smoke
tests before normal routing is allowed.

Existing YuAPI channels, priorities, billing rules, direct credentials, and
user groups are not modified during empty-pool deployment. Existing YuAPI
account-pool bindings are backed up and disabled only when a replacement
Sub2API group has passed real traffic tests. The account-pool code, tables, and
history remain intact for rollback.

## Deployment Sequence

1. Restore administrative SSH access and perform a read-only server inventory.
2. Back up existing edge-proxy configuration and record all old Sub2API and
   YuAPI resources.
3. Verify all proposed names and `127.0.0.1:18473` are unused.
4. Create or fast-forward the GitHub `sub2api` branch from official `main`.
5. Clone the user-owned branch into `/opt/yuapi-sub2api-v2/src` at a recorded
   commit SHA.
6. Build `yuapi-sub2api:<sha>` locally on the server and record its image ID.
7. Create root-only environment and Compose overlay files outside the source
   checkout, with fresh volumes and isolated networks.
8. Start PostgreSQL and Redis, then start Sub2API and allow its automatic fresh
   migrations to finish.
9. Verify health, version, logs, database persistence, Redis authentication,
   restart behavior, and initial admin login.
10. Add the edge-proxy virtual host, validate proxy configuration, reload it,
    and verify Cloudflare HTTPS.
11. Create the empty `yuapi-internal` group/key and the disabled YuAPI channel.
12. Run public-boundary and private-network tests. Do not enable model traffic.

## Verification Matrix

| Check | Expected result |
| --- | --- |
| `sub2.yuaiapi.com` root/login | HTTPS 200 and administration UI loads |
| Admin login with generated credentials | Success; registration UI/API disabled |
| Public auth/settings/admin calls | Only expected authenticated control-plane behavior |
| Public `/v1/models` | `404` or `403` from edge policy |
| Public `/responses` and `/backend-api/codex/responses` | `404` or `403` from edge policy |
| Public image/video/Gemini routes | `404` or `403` from edge policy |
| Host listener | `127.0.0.1:18473` only; no `0.0.0.0:18473`/`[::]:18473` |
| PostgreSQL/Redis host listeners | None |
| YuAPI container to `sub2api-internal:8080/health` | HTTP 200 |
| YuAPI container on the existing data plane | Resolves `sub2api-internal`, health 200, models 200 |
| New Sub2API API key with empty pool | Model catalog available; no relay traffic because the YuAPI channel is disabled |
| Existing YuAPI smoke requests | Continue using existing channels successfully |
| Container restart | Admin login and fresh state persist; services become healthy |

## Rollback

Before every mutable production step, capture the current configuration or
database state required to undo that step.

For the initial empty deployment, rollback means:

1. Disable the new YuAPI channel if it was created.
2. Restore the previous edge-proxy configuration and reload it.
3. Stop the `yuapi-sub2api-v2` Compose project without deleting volumes.
4. Leave `sub2api_sub2api-network` intact; it predates this deployment and is
   shared by current production services.

Rollback must not run `docker compose down -v`, delete the new database, or
touch old `/opt/sub2api` resources. Removal of retained resources is a separate
destructive task requiring explicit approval.

For future upgrades, take a PostgreSQL logical backup and preserve the previous
image tag before running migrations. If an upgrade fails before an irreversible
migration, pin and restart the prior image. If schema compatibility is unknown,
restore the backup into a separate new volume instead of attempting an in-place
downgrade.

## Error Handling and Stop Conditions

Deployment stops without automatic repair when any of these conditions occurs:

- live server inventory cannot be completed;
- an intended name, directory, port, or network conflicts with another project;
- the remote `sub2api` branch exists with divergent or unknown history;
- image build or migration fails;
- Sub2API cannot restart cleanly with persistent state;
- edge-proxy validation fails or YuAPI's existing virtual host changes;
- a public gateway path returns anything other than the expected denial;
- existing YuAPI smoke traffic changes route or fails;
- the existing OpenAI channel contract no longer matches the configured
  Sub2API OpenAI group.

At a stop condition, preserve logs and state, report the exact failing boundary,
and obtain direction before expanding scope.

## Baseline Verification Note

The isolated YuAPI design worktree successfully downloaded Go modules. All Go
packages reached by `go test ./...` passed except the root package setup, which
could not compile because the ignored `web/classic/dist` embed artifact was not
present. The first attempt to install frontend dependencies for generating that
artifact exceeded 240 seconds without an error result. This pre-existing local
setup limitation is unrelated to this documentation-only design commit and must
not be represented as a product regression.
