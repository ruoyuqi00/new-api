# Production Cross-Server Migration Design

## Status

Approved for local preparation on 2026-07-28. Production execution requires a
second explicit authorization after the new server address and access details
are supplied.

## Goal

Move the production YuCore gateway from the current server to a new server
during a short overnight maintenance window. Preserve all production data,
configuration, routing behavior, Cloudflare behavior, direct VIP access, and
the accepted local application fixes.

Daytime preparation must not change production containers, databases, files,
DNS, Cloudflare, channels, accounts, prices, or traffic.

## Current Evidence

The read-only audit on 2026-07-28 found:

- MySQL 8.4 stores approximately 1.35 GB of application data. The `logs` table
  accounts for approximately 1.33 GB and about 805,000 rows.
- The existing compressed full database backup is approximately 59 MB. The
  scheduled backup completes in about six seconds on the current server.
- Redis stores approximately 57 MB of AOF/RDB data and has about 2,200 keys.
- `/opt/newapi/data` is approximately 1.96 GB, almost entirely historical
  application log files. Runtime application data outside logs is below 1 MB.
- Caddy certificate and configuration storage is below 1 MB.
- The current immutable application image is approximately 78 MB.
- The production database already contains the candidate account-pool,
  private-group, mapping, billing, task, session, and media schema.
- `yuaiapi.com`, `api.yuaiapi.com`, and `global.yuaiapi.com` are Cloudflare
  proxied. `vip.yuaiapi.com` is DNS-only and currently has a 300-second TTL.
- The current source has MySQL ROW binlogs enabled without GTID. The small
  logical-backup duration makes live replication unnecessary for this move.

Treat these values as an audit snapshot. Repeat all size, image, schema, DNS,
and topology guards before the maintenance window.

## Chosen Strategy

Use pre-staging plus one final full logical database backup during maintenance.
Do not introduce MySQL replication, temporary public database exposure, or a
physical copy of the live MySQL data directory.

Most work completes before downtime:

- provision and validate the new server;
- transfer and verify the immutable candidate image;
- transfer production deployment configuration without committing secrets;
- restore a recent MySQL snapshot and Redis snapshot on the isolated new host;
- pre-copy application data, Caddy storage, static brand files, backup tooling,
  and historical logs;
- start the candidate privately and verify the restored snapshot.

The maintenance window performs only the final database dump, Redis flush,
small file delta, final restore, master start, private probes, and origin
switch.

## Daytime Preparation Boundary

Before the new server is supplied, local work may create and verify:

- an immutable image build and checksum workflow;
- old-server and new-server read-only preflight commands;
- hardware, filesystem, RAID, Docker, firewall, time-sync, and kernel checks;
- secret-safe Compose drift comparison;
- MySQL, Redis, filesystem, Caddy, and Cloudflare snapshot helpers;
- data transfer, restore, row-count, checksum, cutover, and rollback steps;
- local marker rehearsals for maintenance, forwarding, and rollback.

No daytime preparation command may write to the current production server.
Mutating commands require all of these gates:

1. the new server coordinates are present;
2. the accepted candidate commit and image digest match;
3. the operator enters an explicit maintenance confirmation string;
4. fresh backups and rollback prerequisites pass;
5. the user explicitly authorizes production execution.

## New Server Provisioning

After delivery, validate the advertised CPU topology, memory, NVMe identity and
health, RAID mode, network capacity, public IPs, and sustained stability before
copying production secrets.

The production layout preserves the current trust boundaries:

- Caddy exposes only ports 80 and 443;
- the application binds only to loopback and the private Docker network;
- MySQL and Redis are not published to the public Internet;
- application, MySQL, Redis, Caddy, and static-brand storage use explicit host
  bind mounts on mirrored persistent storage;
- the application, Caddy, and host receive appropriate file-descriptor and
  connection limits;
- one application instance is the master; any future traffic nodes are slaves;
- `SESSION_SECRET`, `CRYPTO_SECRET`, database credentials, Redis credentials,
  and upstream keys are transferred through a mode-0600 runtime secret file and
  are never committed or printed.

Resource limits are derived from the accepted hardware instead of copied
blindly. On a verified 128 GB host, reserve memory for the OS, filesystem cache,
MySQL, Redis, and Caddy before setting the application container limit and
`GOMEMLIMIT`.

## Candidate Image Guarantee

Build the image from a clean `git archive` of the accepted local production
branch. Record the commit, image ID, image digest, archive SHA-256, build
platform, and frontend asset identifiers.

The new server loads that exact archive. It does not run `git pull`, use a
floating image tag, build from an untracked worktree, or copy the experimental
UI. The locally verified routing, account-pool, private-group, mapped-model,
billing, usage-log, theme, and UI performance fixes are therefore part of the
new application container.

## Pre-Staging And Private Acceptance

Restore a recent logical MySQL backup and Redis snapshot on the new server.
Pre-copy `/opt/newapi/data`, Caddy storage, Caddy configuration, static brand
files, backup scripts, and systemd backup units. Historical application logs
may be archived separately because the MySQL `logs` table is authoritative for
user-visible usage history.

Start the candidate without public DNS. Use a unique node name, disable
recurring master work, and probe it through loopback and `curl --resolve` with
the real production hostnames. Verify:

- all three frontend roles and both themes;
- login, profile, wallet, usage logs, users, channels, and account pools;
- public and private group discovery;
- model mapping privacy and public-model billing;
- ordinary and streaming relay;
- the smallest supported image and video request through the VIP hostname;
- SMTP registration, Turnstile hostname acceptance, and any OAuth callback
  whose provider allows pre-cutover testing;
- database row counts, key option hashes, Redis key counts, and file manifests.

Snapshot testing uses disposable credentials and the smallest paid request.

## Maintenance Cutover

### 1. Freeze The Old Origin

Start a disposable maintenance upstream and atomically reload the old Caddy
configuration so new requests receive HTTP 503 plus `Retry-After`. Give the old
application five seconds to exit, then terminate remaining relay, stream,
image, and video requests as explicitly authorized.

Stop only the old application. MySQL, Redis, and Caddy remain available for the
final export and maintenance response.

### 2. Capture The Final State

Create a new transaction-consistent full MySQL dump without exposing the
password. Record its row-count manifest, maximum IDs, uncompressed byte count,
compressed SHA-256, and source timestamp.

Flush Redis persistence, stop the old Redis container, and archive the complete
RDB/AOF directory with a checksum. Run a final file delta for runtime data and
Caddy storage. Do not copy the live MySQL data directory.

Transfer the final database, Redis archive, runtime delta, manifests, and
checksums directly between the two servers over SSH.

### 3. Restore The New Origin

Replace the pre-staged target database with the final dump, restore Redis, and
apply the final file delta. Start MySQL and Redis, validate persistence and
counts, then start the accepted application image as the final master.

Do not expose public traffic until the final master is healthy, has zero
restarts, uses the expected image ID, connects to MySQL and Redis, and passes
the complete private acceptance set through `curl --resolve`.

### 4. Bridge And Change Origins

Once the new origin is accepted, change the old Caddy maintenance upstream to
forward the production hostnames to the new origin. This prevents stale
Cloudflare origin routing and cached `vip.yuaiapi.com` DNS from writing to the
frozen old database.

Snapshot Cloudflare DNS and relevant zone settings immediately before change.
Modify only the origin A records:

- keep the apex, API, and global records proxied;
- keep the VIP record DNS-only;
- preserve TTL, SSL/TLS mode, WAF, rulesets, cache behavior, Turnstile,
  redirects, and all unrelated records.

During the 300-second VIP TTL window, clients that still reach the old IP are
forwarded to the new origin. There is no active-active database period.

## Cloudflare And External Dependencies

Changing the origin IP does not recreate the Cloudflare zone. Preserve and
verify:

- DNS record names, record types, proxy flags, TTLs, and unrelated records;
- SSL/TLS mode, minimum TLS version, HTTP/2 and HTTP/3 behavior;
- WAF and custom rules, rate limits, cache rules, redirects, and transforms;
- Turnstile site and secret configuration for unchanged hostnames;
- OAuth callback URLs, SMTP sender behavior, payment and webhook callbacks;
- upstream provider IP allowlists and any egress-IP restrictions.

Cloudflare API credentials are supplied only at execution time through an
environment variable. The tooling stores a timestamped redacted snapshot and
never commits or logs the token.

## Data Validation

The cutover compares source and target evidence for:

- database name, table count, schema signatures, row counts, and maximum IDs;
- users, tokens, channels, abilities, options, groups, account pools, provider
  accounts, bindings, redemptions, tasks, sessions, logs, and media tasks;
- hashes of non-secret routing, group, mapping, billing, theme, and URL options;
- Redis DB size, persistence mode, key-prefix counts, and archive checksum;
- application data and static-brand file manifests;
- Caddy configuration and storage manifests;
- accepted image commit, ID, digest, and archive checksum.

Secret option values are compared by keyed or SHA-256 digest and are never
printed.

## Rollback

### Before Public Traffic

If private new-origin acceptance fails, leave Cloudflare unchanged, keep the
old origin in maintenance, restore the old application and Redis, and end the
window. The frozen old MySQL remains authoritative.

### After Public Traffic

Once any request can write to the new database, DNS-only rollback is forbidden.
To roll back without losing user data:

1. return both origins to maintenance;
2. stop the new application after five seconds;
3. export the new authoritative MySQL and Redis state;
4. restore that state to the old server;
5. validate counts, hashes, and the old immutable image;
6. start the old application;
7. make the old Caddy authoritative and revert only the changed Cloudflare A
   records;
8. keep the new origin available for forensic comparison.

Do not attempt bidirectional writes or merge two independently writable
databases.

## Observation And Cleanup

Observe the new origin at 1, 5, 15, 30, and 60 minutes, then daily while the
old server remains available. Monitor health, restarts, OOM events, memory,
CPU, connection counts, first-byte latency, streaming disconnects, HTTP
`5xx/521`, registration email, private groups, pricing, mapped-model privacy,
account-pool routing, and task settlement.

Keep the old server, frozen backups, immutable old image, Cloudflare snapshot,
and forwarding configuration for at least seven days. Cleanup requires a
separate confirmation. Never prune images, delete backups, or release the old
server as part of the cutover.

## Local Verification

Before production use, local tests and rehearsals must prove:

- mutating commands refuse to run without the new host and confirmation gate;
- no secret appears in command output, manifests, or Git;
- MySQL dump and restore preserve exact validation manifests;
- Redis RDB/AOF export and restore preserve expected key counts;
- file pre-sync plus final delta produces identical manifests;
- Caddy can move through old, maintenance, forwarding, new, and rollback
  states without an invalid configuration;
- Cloudflare dry-run changes only approved A records and preserves proxy flags;
- rollback before and after simulated writes preserves all accepted data;
- the candidate image passes the complete local production preflight;
- temporary containers, files, credentials, and networks are removed without
  touching the accepted worktree or experimental UI.

## Acceptance Criteria

- Daytime preparation causes no production state or traffic change.
- The maintenance window transfers only final mutable state and small deltas.
- The new container runs the exact accepted local candidate.
- All production data and configuration pass source-target validation.
- Cloudflare behavior and the VIP direct endpoint are preserved.
- No old and new database accept writes concurrently.
- Short forced termination is used instead of waiting for long requests.
- Both pre-traffic and post-traffic rollback preserve user data.
- The old server remains recoverable for at least seven days.
