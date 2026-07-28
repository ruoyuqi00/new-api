# Production Maintenance Cutover Design

## Status

Approved for local implementation on 2026-07-27. Production execution remains
a separate operation and requires explicit authorization.

## Goal

Replace the current production application image during a short overnight
maintenance window. Prefer a small, deterministic interruption over a more
complex cross-version frontend blue-green path.

The target interruption for new requests is one to three minutes. The
maintenance window may be announced as 15 to 20 minutes so validation and
rollback have room to complete.

## Decisions

- Existing relay, streaming, image, and video requests may be terminated after
  a five-second application stop grace period. Short interruption is more
  important than draining every active request.
- MySQL, Redis, and Caddy remain running throughout the replacement.
- The candidate image, checksum, backup, and isolated slave health probe are
  completed before maintenance begins.
- The candidate does not require cross-version static asset fallback. Users
  with an already-open page may need to refresh after maintenance.
- The current application image and Compose file remain available for direct
  rollback.
- No channel, account-pool, group, model, price, token, or Cloudflare setting is
  changed as part of the cutover.

## Scope

This design changes the deployment helper and production cutover runbook. It
does not change application routing, billing, account-pool behavior, database
models, frontend behavior, production data, or the experimental UI.

## Architecture

### Preparation

Build an immutable `linux/amd64` image from the accepted commit and export it
with its image ID, commit, and SHA-256 checksum. Transfer and load the image
before the maintenance window.

The production preflight must:

- validate the current immutable image ID and Caddy configuration;
- require the exact `newapi-mysql` container instead of discovering the first
  MySQL image on the host;
- compare the running `newapi` container with rendered Compose configuration
  without printing secret environment values;
- confirm the production database schema has already converged;
- snapshot channel, account-pool, provider-account, task, and relevant Redis
  state before any paid probe;
- create and validate a transaction-consistent MySQL backup;
- start the new image as an isolated slave with recurring master work disabled;
- validate status, authentication, private groups, mapping privacy, billing,
  and the smallest practical relay request against the slave.

The server has Python 3 but not `jq`. JSON probes and configuration comparison
therefore use a repository-owned Python 3 helper with no third-party packages.

### Maintenance Gate

Before stopping the old application, start a disposable maintenance container
from the already-present `nginx:1.27-alpine` image on the shared application
network. It returns HTTP 503 and a `Retry-After` header for every request.

Create and validate a temporary Caddyfile that replaces every audited
`newapi:3000` upstream with `yucore-maintenance:80`. Reload Caddy atomically.
New requests then receive an explicit maintenance response while connections
accepted by the previous Caddy configuration may remain attached to the old
application.

Stop the old application with a five-second timeout. Any remaining application
connections are deliberately terminated after that timeout.

### Master Replacement

Update only the application image reference in the production Compose file.
Validate the rendered Compose configuration, then recreate only the `newapi`
service. The final container starts as the normal master with the same
environment, bind mount, network, health check, restart policy, and `nofile`
limit as the current production container.

Do not use the slave candidate as the final master. It remains isolated and
available for diagnostics until the final master is healthy.

### Traffic Restoration

Keep Caddy on the maintenance upstream until the final master is running,
healthy, has zero restarts, uses the expected immutable image, and returns a
successful status response both from the host port and from the Caddy network.

Restore the original production Caddyfile and reload atomically. Immediately
probe the public, global, and VIP status endpoints, then check ordinary relay,
streaming relay, private-group discovery, mapping privacy, login, profile,
wallet, usage logs, and one minimal VIP media request.

Users with an old loaded frontend may receive a stale-chunk error. A refresh is
the accepted maintenance-window recovery. The deployment does not add a
cross-version asset proxy.

## Rollback

Rollback begins behind the maintenance gate:

1. Keep or return Caddy to `yucore-maintenance:80`.
2. Restore the saved Compose file and verify the old immutable image ID.
3. Recreate only the `newapi` service from the old image.
4. Wait for healthy status with zero restarts.
5. Restore the original Caddyfile and repeat public status and relay probes.

Do not roll back MySQL or Redis. The candidate has no schema delta relative to
the documented production source baseline, and the old image remains the
database rollback target.

## State Protection

The isolated slave is not read-only. It can write its `system_instances` row,
logs, rate-limit state, affinity state, and state produced by explicit probes.
The runbook must record bounded before-and-after snapshots and stop on any
unexpected channel or provider-account status change.

Paid probes use disposable credentials and the smallest supported request.
They never print keys, DSNs, session secrets, or complete container
environments. Probe failures must not silently leave a channel or account in a
different status.

## Error Handling

- A topology, image, Compose-drift, backup, schema, or Caddy validation failure
  blocks maintenance before the old application is stopped.
- Failure to enter the maintenance gate blocks the application stop.
- Failure of the new master triggers rollback while maintenance remains active.
- Failure to restore the old master keeps maintenance active and stops further
  automated changes for manual diagnosis.
- Cleanup never removes the old image, backups, shared network, database
  volumes, Redis data, or Caddy certificate data.

## Local Verification

Local acceptance must prove:

- the Python guard validates successful status JSON without `jq`;
- secret environment values never appear in drift output;
- an environment, mount, network, image, health-check, restart, or `nofile`
  drift returns a nonzero exit code;
- a real Caddy container can switch from an old marker upstream to maintenance,
  then to a new marker upstream, and back to the old marker;
- the maintenance upstream returns 503 plus `Retry-After`;
- forward and rollback reloads produce no Caddy configuration error;
- application source tests and production builds remain unchanged and pass;
- the tracked worktree is clean after temporary rehearsal resources are
  removed.

## Acceptance Criteria

- The production replacement can be executed without `jq`.
- The script selects `newapi-mysql` explicitly and refuses topology drift.
- Only the application container is stopped or recreated.
- New requests receive maintenance responses during replacement.
- Remaining old application requests are terminated after five seconds.
- The final master inherits existing routing, pricing, group, account-pool,
  session, and Redis configuration.
- The old image can be restored behind maintenance without a database rollback.
- No production operation occurs during local implementation or rehearsal.
