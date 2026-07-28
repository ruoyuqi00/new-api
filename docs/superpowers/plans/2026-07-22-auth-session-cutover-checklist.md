# Dashboard Authentication Session Cutover Checklist

This runbook prepares a reversible production cutover from the current dashboard
cookie-session application to the server-side authentication/session candidate.
It is a command reference only. Nothing in this document authorizes production
access or execution.

## APPROVAL GATE

**STOP. Do not SSH to production, push, create a production backup, transfer an
image, load an image on production, start or replace a production container,
run a production migration, reload a proxy, or switch traffic until the user
gives explicit approval for those actions.** Approval must identify the host,
maintenance window, database dialect, container name, and traffic-switch method.

Approval record, to be completed before any production action:

```text
APPROVED_BY=<MANDATORY: user identity>
APPROVED_AT_UTC=<MANDATORY: ISO-8601 timestamp>
APPROVED_HOST=<MANDATORY: exact production host or fleet>
APPROVED_WINDOW=<MANDATORY: start/end UTC>
APPROVED_DB_DIALECT=<MANDATORY: mysql|postgresql|sqlite>
APPROVED_SWITCH_METHOD=<MANDATORY: proxy|container-replacement>
APPROVED_SCOPE=<MANDATORY: backup, transfer, candidate start, migration, switch, rollback authority>
```

Absence of any field is an abort condition. Approval for one phase does not
implicitly approve later phases.

## Immutable Inputs and Evidence Fields

Known immutable inputs:

| Input | Required value |
| --- | --- |
| Production baseline image | `newapi:production-console-20260722-739cb2775` |
| Production baseline commit | `739cb27751e5a89932597567356326d3a73a980f` |
| Candidate source provenance boundary | `0eabc2e884550b11536fffb4637bcdfe644101d1` |
| Candidate local image tag | `newapi:auth-session-candidate-20260722` |
| Candidate image ID/digest | `sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee` |
| Canonical health path | `/api/status` |
| Candidate loopback health URL | `http://127.0.0.1:33000/api/status` |
| Primary public health URL | `https://yuaiapi.com/api/status` |
| Global public health URL | `https://global.yuaiapi.com/api/status` |
| Exact candidate container name | `new-api-auth-session-candidate-20260722` |
| Exact parked old-container name | `new-api-production-pre-auth-session-20260722` |

The `/api/status` response is acceptable only when HTTP status is 200 and the
JSON field `.success` is `true`.

Mandatory production identifiers must be filled from the approved deployment
inventory, not guessed:

```text
PROD_HOST=<MANDATORY: exact approved host>
RUNBOOK_COMMIT=<MANDATORY: exact 40-hex final reviewed runbook commit>
EVIDENCE_DIR=<MANDATORY: exact unique absolute evidence directory; must not exist>
CANDIDATE_IMAGE_ID=sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee
PROD_CONTAINER=<MANDATORY: exact current production application container name>
PROD_NETWORK=<MANDATORY: exact Docker network name>
PROD_ENV_FILE=<MANDATORY: mode-0600 environment file rendered from the approved secret store>
PROD_DATA_SOURCE=<MANDATORY: exact application data bind source or volume name>
PROD_DATA_TARGET=/data
PROD_PROXY_CONFIG=<MANDATORY when proxy switching: exact config path>
PROD_PROXY_CONFIG_OWNER_UID=<MANDATORY when proxy switching: expected numeric owner UID>
PROD_PROXY_CONFIG_GROUP_GID=<MANDATORY when proxy switching: original numeric group GID>
PROD_PROXY_CONFIG_MODE=<MANDATORY when proxy switching: original 3- or 4-digit octal mode>
PROD_PROXY_UPSTREAM_OLD=<MANDATORY when proxy switching: exact old upstream>
PROD_PROXY_UPSTREAM_CANDIDATE=127.0.0.1:33000
PROD_PROXY_CANDIDATE_CONFIG=<MANDATORY when proxy switching: exact reviewed candidate config path>
PROD_PROXY_CANDIDATE_SHA256=<MANDATORY when proxy switching: exact reviewed config SHA-256>
PROD_CONTAINER_CREATE_SCRIPT=<MANDATORY when replacing: exact reviewed create script path>
PROD_CONTAINER_CREATE_SCRIPT_OWNER_UID=<MANDATORY when replacing: expected numeric owner UID>
PROD_CONTAINER_CREATE_SCRIPT_SHA256=<MANDATORY when replacing: exact reviewed script SHA-256>
PROD_TRAFFIC_CLOSE_SCRIPT=<MANDATORY for recovery/rollback: exact reviewed traffic-close script path>
PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID=<MANDATORY for recovery/rollback: expected numeric owner UID>
PROD_TRAFFIC_CLOSE_SCRIPT_SHA256=<MANDATORY for recovery/rollback: exact reviewed script SHA-256>
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT=<MANDATORY: exact reviewed read-only closure-check script path>
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID=<MANDATORY: expected numeric owner UID>
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256=<MANDATORY: exact reviewed closure-check script SHA-256>
PROD_TRAFFIC_CONTROL_REVIEW_RECORD=<MANDATORY: exact peer-review/approval record for both scripts>
PROD_DB_DIALECT=<MANDATORY: mysql|postgresql|sqlite>
PROD_DB_BACKUP_PATH=<MANDATORY after approved backup creation>
PROD_DB_BACKUP_SHA256=<MANDATORY pre-switch evidence; fill from sha256sum after approved backup creation>
```

`PROD_DB_BACKUP_SHA256` must remain unfilled until a backup is created after
approval. It must then be recorded and independently compared before any
migration or traffic change. No production database backup or checksum exists
from this checklist-preparation task.

The protected local file `production-b5514ebe1.tar` was not inspected or
modified. It is **not** an approved database backup, candidate image archive, or
rollback input unless its origin, contents, timestamp, and checksum are proven
independently under a separate approval. Do not touch it during this cutover.

## Required Production Environment

Every application node must receive one shared, high-entropy `SESSION_SECRET`
from the production secret store. Generate it from at least 32 cryptographically
random bytes. Never put its value in a command, shell history, terminal output,
ticket, chat, image, Compose file, or this runbook.

The following non-secret values are exact:

```env
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://yuaiapi.com,https://global.yuaiapi.com
TRUSTED_PROXIES=127.0.0.0/8,::1/128,172.16.0.0/12
```

Before starting any candidate node, verify through secret-store metadata, not by
printing values, that all nodes reference the same `SESSION_SECRET` version and
the same production primary database. Keep any existing `CRYPTO_SECRET` and
Redis configuration unchanged.

Old dashboard cookies intentionally become invalid at cutover. Users must log
in once to establish the new refresh cookie and server-side session. Existing
relay API keys are unaffected and must not be rotated.

## Operator Safety and Abort Gates

Run production commands from a dedicated shell with command tracing disabled:

```bash
set -Eeuo pipefail
set +x
umask 077
```

Do not paste secrets into the shell. Load them through the approved secret-store
integration or a mode-0600 file referenced by a command. Do not run `env`,
`printenv`, `docker inspect` without redirection, or any command that displays
container environment values. Keep raw evidence containing configuration in an
encrypted, access-controlled incident directory and never attach it to chat.

Abort before the next phase on any of these conditions:

- Approval is absent, incomplete, expired, or names a different host/window.
- The current image or container identity differs from the recorded baseline.
- The candidate image ID differs from the immutable digest.
- Required production identifiers or secret-store references are missing.
- The database backup is empty, fails dialect verification, or lacks a matching
  SHA-256 value.
- The candidate cannot start without public traffic, migration logs contain an
  error, or a second restart attempts incompatible schema changes.
- Candidate or public health is not HTTP 200 with `.success == true`.
- Login, refresh, 2FA, profile, wallet, logs, registration/email policy, session
  management, account switching, or relay checks regress.
- Authentication `401`, `409`, `429`, or `5xx` rates remain materially above the
  captured baseline, or database/proxy/container health degrades.

Never use `git reset --hard`, delete the baseline image, delete the parked old
container, prune Docker state, or remove the database backup during the window.

### Traffic Closure Script Contract

The exact peer-review record must prove both pinned scripts satisfy this contract:

- The close script is idempotent and may only atomically withdraw or replace the
  routes for `https://yuaiapi.com` and `https://global.yuaiapi.com`. It must not
  mutate a database, application/container process, image, volume, secret, or any
  unrelated route. It returns nonzero on incomplete or failed withdrawal and
  contains the exact line `# CUTOVER_CONTRACT: close-approved-domain-routes-v1`.
- The closure-check script is read-only. It independently verifies that both
  approved domains route to neither the candidate nor baseline application and
  returns nonzero unless closure is proven. It contains the exact line
  `# CUTOVER_CONTRACT: verify-approved-domain-routes-closed-v1`.
- Neither script accepts positional arguments or unresolved placeholders. Both
  are owner-pinned, mode 0700, SHA-256-pinned, and pass `bash -n` before use.

Every close-script invocation in this runbook is immediately followed by the
closure-check script. A failed or inconclusive postcondition is CRITICAL: keep
all traffic closed, make no traffic-opening change, and escalate.

## Phase 1: Local Preflight and Identity Checks

These local commands are read-only. Run them before packaging the image:

```powershell
$ErrorActionPreference = 'Stop'
Set-Location 'D:\yucore-api-export'

$candidateSourceCommit = '0eabc2e884550b11536fffb4637bcdfe644101d1'
$runbookCommit = '<FILL exact 40-hex final reviewed runbook commit>'
if ($runbookCommit -notmatch '^[0-9a-f]{40}$') { throw 'ABORT: RUNBOOK_COMMIT is not a 40-hex commit' }

git cat-file -e "$candidateSourceCommit^{commit}"
if ($LASTEXITCODE -ne 0) { throw 'ABORT: candidate source provenance commit is missing' }
git cat-file -e "$runbookCommit^{commit}"
if ($LASTEXITCODE -ne 0) { throw 'ABORT: reviewed runbook commit is missing' }
git merge-base --is-ancestor $candidateSourceCommit $runbookCommit
if ($LASTEXITCODE -ne 0) { throw 'ABORT: candidate source commit is not an ancestor of RUNBOOK_COMMIT' }
$currentHead = (git rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw 'ABORT: cannot resolve current HEAD' }
if ($currentHead -ne $runbookCommit) { throw 'ABORT: current HEAD must equal RUNBOOK_COMMIT exactly' }

$trackedStatus = @(git status --porcelain=v1 --untracked-files=no)
if ($LASTEXITCODE -ne 0 -or $trackedStatus.Count -ne 0) {
  throw 'ABORT: tracked worktree or index is not clean'
}

$candidateId = (docker image inspect --format '{{.Id}}' 'newapi:auth-session-candidate-20260722').Trim()
if ($LASTEXITCODE -ne 0) { throw 'ABORT: candidate image is unavailable' }
if ($candidateId -ne 'sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee') {
  throw 'ABORT: candidate image digest mismatch'
}

docker image inspect --format '{{.RepoTags}} {{.Id}} {{.Os}}/{{.Architecture}} {{with index .Config \"User\"}}{{.}}{{else}}root{{end}}' 'newapi:auth-session-candidate-20260722'
if ($LASTEXITCODE -ne 0) { throw 'ABORT: candidate image inspection failed' }
git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'ABORT: git status failed' }
```

The candidate image has no recorded OCI revision label. The immutable image ID
and `0eabc2e884550b11536fffb4637bcdfe644101d1` are therefore two independently
recorded boundaries, not proof that the image was built from that commit. Do not
claim a verified source-to-image link. If a signed provenance attestation is
later supplied, verify its signature, subject digest, and source commit before
recording any stronger attestation.

Expected image user output is empty or root. The current image running as root
is an inherited minor hardening item. It is non-blocking for this authentication
migration, must be tracked separately, and must not be changed during cutover.

**Gate:** stop unless the candidate source commit exists, is an ancestor of the
reviewed runbook commit, current HEAD equals that runbook commit exactly, the
tracked worktree and index are clean, and image architecture, tag, and digest
match. Protected untracked paths do not satisfy or bypass the clean tracked gate.

## Phase 2: Remote Identity and Current Configuration Capture

Do not run this phase before explicit approval. Establish the approved session
using the organization's normal host-verification process; do not disable SSH
host-key checking. On the approved production host, set the mandatory identifiers
without placing secret values in the shell:

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_CONTAINER='<FILL exact current production container name>'
EVIDENCE_DIR='<FILL exact unique absolute evidence directory>'
EVIDENCE_FILES=(
  'prod-container-inspect.restricted.json'
  'prod-container-sanitized.json'
  'prod-baseline-image.json'
  'prod-running-image.txt'
  'prod-container-state.txt'
  'prod-health-before.json'
)
MANIFEST_NAME='evidence.sha256'

test -n "$PROD_CONTAINER" || { echo 'ABORT: PROD_CONTAINER missing' >&2; exit 1; }
[[ "$EVIDENCE_DIR" == /* ]] || { echo 'ABORT: EVIDENCE_DIR must be absolute' >&2; exit 1; }
test ! -e "$EVIDENCE_DIR" || { echo 'ABORT: EVIDENCE_DIR already exists; choose a new unique path' >&2; exit 1; }
EVIDENCE_PARENT="$(dirname "$EVIDENCE_DIR")"
test -d "$EVIDENCE_PARENT" && test -w "$EVIDENCE_PARENT"
mkdir --mode=700 -- "$EVIDENCE_DIR"
test "$(stat -c '%a' "$EVIDENCE_DIR")" = '700'
for target in "${EVIDENCE_FILES[@]}" "$MANIFEST_NAME"; do
  test ! -e "$EVIDENCE_DIR/$target" || { echo "ABORT: evidence target already exists: $target" >&2; exit 1; }
done
docker container inspect "$PROD_CONTAINER" >/dev/null || { echo 'ABORT: exact production container not found' >&2; exit 1; }
docker image inspect 'newapi:production-console-20260722-739cb2775' >/dev/null || { echo 'ABORT: baseline image missing' >&2; exit 1; }

docker container inspect "$PROD_CONTAINER" >"$EVIDENCE_DIR/prod-container-inspect.restricted.json"
chmod 600 "$EVIDENCE_DIR/prod-container-inspect.restricted.json"

docker container inspect "$PROD_CONTAINER" | jq '.[0] | {
  Id, Name, Created,
  Config: {Image, Entrypoint, Cmd, User, WorkingDir, ExposedPorts},
  HostConfig: {Binds, PortBindings, RestartPolicy, NetworkMode, ReadonlyRootfs},
  Mounts,
  NetworkSettings: {Networks: .NetworkSettings.Networks}
}' >"$EVIDENCE_DIR/prod-container-sanitized.json"

docker image inspect 'newapi:production-console-20260722-739cb2775' >"$EVIDENCE_DIR/prod-baseline-image.json"
docker container inspect --format '{{.Config.Image}}' "$PROD_CONTAINER" >"$EVIDENCE_DIR/prod-running-image.txt"
docker container inspect "$PROD_CONTAINER" | jq '.[0] | {
  status: .State.Status,
  health: (.State.Health.Status // "not-configured"),
  restart_count: .RestartCount
}' >"$EVIDENCE_DIR/prod-container-state.txt"
curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' >"$EVIDENCE_DIR/prod-health-before.json"
jq -e '.success == true' "$EVIDENCE_DIR/prod-health-before.json" >/dev/null

MANIFEST="$EVIDENCE_DIR/$MANIFEST_NAME"
MANIFEST_TMP="$(mktemp "$EVIDENCE_DIR/.evidence.sha256.tmp.XXXXXX")"
manifest_complete=0
cleanup_manifest() {
  rm -f -- "$MANIFEST_TMP"
  if [[ "$manifest_complete" -ne 1 ]]; then
    rm -f -- "$MANIFEST"
  fi
}
trap cleanup_manifest EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$MANIFEST_TMP"
(
  cd "$EVIDENCE_DIR"
  sha256sum -- "${EVIDENCE_FILES[@]}"
) >"$MANIFEST_TMP"
mv -- "$MANIFEST_TMP" "$MANIFEST"
(
  cd "$EVIDENCE_DIR"
  sha256sum --check 'evidence.sha256'
)
manifest_complete=1
trap - EXIT INT TERM
```

The raw container inspect is restricted because it can contain secret environment
values. Do not display, diff in a terminal, commit, or transmit it. The sanitized
capture intentionally omits `.Config.Env`.

Record the exact current image reference and compare its image ID with the
baseline image. If the current container is intentionally pinned by image ID,
record that relationship. Abort if the running application is not the approved
baseline or if the loopback health port differs without an approved inventory
update.

## Phase 3: Production Database Backup and SHA-256

Do not run any backup command before explicit approval. Select exactly one
dialect block. Store the backup outside the application data directory and on
storage with enough free space. Commands rely on credentials already injected
by the approved secret-store integration; they never include or print passwords.

### MySQL 5.7 or Later

Required non-secret identifiers: `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, and
`MYSQL_DATABASE`. The approved secret integration sets `MYSQL_PWD` without
printing it.

```bash
set -Eeuo pipefail
set +x
umask 077
BACKUP_DIR='<FILL approved backup directory>'
BACKUP="$BACKUP_DIR/newapi-auth-session-precutover-20260722.mysql.sql"
mkdir -p "$BACKUP_DIR"
: "${MYSQL_HOST:?ABORT: MYSQL_HOST missing}"
: "${MYSQL_PORT:?ABORT: MYSQL_PORT missing}"
: "${MYSQL_USER:?ABORT: MYSQL_USER missing}"
: "${MYSQL_DATABASE:?ABORT: MYSQL_DATABASE missing}"
: "${MYSQL_PWD:?ABORT: MYSQL_PWD not injected}"
test -d "$BACKUP_DIR" && test -w "$BACKUP_DIR"
test ! -e "$BACKUP" && test ! -e "$BACKUP.sha256" || { echo 'ABORT: MySQL final backup path already exists' >&2; exit 1; }

BACKUP_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.mysql.sql.tmp.XXXXXX")"
SHA_TMP=''
backup_complete=0
cleanup_mysql_backup() {
  if [[ -n "$BACKUP_TMP" ]]; then rm -f -- "$BACKUP_TMP"; fi
  if [[ -n "$SHA_TMP" ]]; then rm -f -- "$SHA_TMP"; fi
  if [[ "$backup_complete" -ne 1 ]]; then
    rm -f -- "$BACKUP" "$BACKUP.sha256"
  fi
}
trap cleanup_mysql_backup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$BACKUP_TMP"

mysqldump \
  --host="$MYSQL_HOST" --port="$MYSQL_PORT" --user="$MYSQL_USER" \
  --single-transaction --quick --routines --events --triggers --hex-blob \
  --set-gtid-purged=OFF --databases "$MYSQL_DATABASE" >"$BACKUP_TMP"

test -s "$BACKUP_TMP"
grep -q '^-- MySQL dump' "$BACKUP_TMP"
grep -q '^-- Dump completed on' "$BACKUP_TMP"
mysql --host="$MYSQL_HOST" --port="$MYSQL_PORT" --user="$MYSQL_USER" \
  --batch --skip-column-names --execute='SELECT 1' | grep -qx '1'

mv -- "$BACKUP_TMP" "$BACKUP"
BACKUP_TMP=''
chmod 600 "$BACKUP"
SHA_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.mysql.sha256.tmp.XXXXXX")"
chmod 600 "$SHA_TMP"
(
  cd "$BACKUP_DIR"
  sha256sum -- "$(basename "$BACKUP")"
) >"$SHA_TMP"
mv -- "$SHA_TMP" "$BACKUP.sha256"
SHA_TMP=''
chmod 600 "$BACKUP.sha256"
(
  cd "$BACKUP_DIR"
  sha256sum --check "$(basename "$BACKUP.sha256")"
)
backup_complete=1
trap - EXIT INT TERM
unset MYSQL_PWD
```

For stronger restore verification, restore into a separately approved disposable
MySQL database with no production traffic, then run application read-only smoke
checks there. Never test a restore over the production database.

### PostgreSQL 9.6 or Later

Required non-secret identifiers: `PGHOST`, `PGPORT`, `PGUSER`, and `PGDATABASE`.
The approved secret integration sets `PGPASSWORD` without printing it.

```bash
set -Eeuo pipefail
set +x
umask 077
BACKUP_DIR='<FILL approved backup directory>'
BACKUP="$BACKUP_DIR/newapi-auth-session-precutover-20260722.postgresql.dump"
mkdir -p "$BACKUP_DIR"
: "${PGHOST:?ABORT: PGHOST missing}"
: "${PGPORT:?ABORT: PGPORT missing}"
: "${PGUSER:?ABORT: PGUSER missing}"
: "${PGDATABASE:?ABORT: PGDATABASE missing}"
: "${PGPASSWORD:?ABORT: PGPASSWORD not injected}"
test -d "$BACKUP_DIR" && test -w "$BACKUP_DIR"
test ! -e "$BACKUP" && test ! -e "$BACKUP.sha256" || { echo 'ABORT: PostgreSQL final backup path already exists' >&2; exit 1; }

BACKUP_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.postgresql.dump.tmp.XXXXXX")"
MANIFEST_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.postgresql.manifest.tmp.XXXXXX")"
SHA_TMP=''
backup_complete=0
cleanup_postgresql_backup() {
  if [[ -n "$BACKUP_TMP" ]]; then rm -f -- "$BACKUP_TMP"; fi
  if [[ -n "$MANIFEST_TMP" ]]; then rm -f -- "$MANIFEST_TMP"; fi
  if [[ -n "$SHA_TMP" ]]; then rm -f -- "$SHA_TMP"; fi
  if [[ "$backup_complete" -ne 1 ]]; then
    rm -f -- "$BACKUP" "$BACKUP.sha256"
  fi
}
trap cleanup_postgresql_backup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$BACKUP_TMP" "$MANIFEST_TMP"

pg_dump --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" \
  --format=custom --no-owner --no-privileges --file="$BACKUP_TMP" "$PGDATABASE"

test -s "$BACKUP_TMP"
pg_restore --list "$BACKUP_TMP" >"$MANIFEST_TMP"
test -s "$MANIFEST_TMP"
pg_isready --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --dbname="$PGDATABASE"

mv -- "$BACKUP_TMP" "$BACKUP"
BACKUP_TMP=''
chmod 600 "$BACKUP"
rm -f -- "$MANIFEST_TMP"
MANIFEST_TMP=''
SHA_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.postgresql.sha256.tmp.XXXXXX")"
chmod 600 "$SHA_TMP"
(
  cd "$BACKUP_DIR"
  sha256sum -- "$(basename "$BACKUP")"
) >"$SHA_TMP"
mv -- "$SHA_TMP" "$BACKUP.sha256"
SHA_TMP=''
chmod 600 "$BACKUP.sha256"
(
  cd "$BACKUP_DIR"
  sha256sum --check "$(basename "$BACKUP.sha256")"
)
backup_complete=1
trap - EXIT INT TERM
unset PGPASSWORD
```

For stronger restore verification, restore the custom archive into a separately
approved disposable PostgreSQL database with `pg_restore --exit-on-error`, then
run application read-only smoke checks there. Never test a restore over the
production database.

### SQLite

`PROD_SQLITE_PATH` must be the exact host path used by the production container's
captured bind mount. SQLite's online `.backup` command creates a consistent
snapshot without copying a live database file byte-for-byte.

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_SQLITE_PATH='<FILL exact host SQLite path from captured mount>'
BACKUP_DIR='<FILL approved backup directory>'
BACKUP="$BACKUP_DIR/newapi-auth-session-precutover-20260722.sqlite"
mkdir -p "$BACKUP_DIR"
test -f "$PROD_SQLITE_PATH" || { echo 'ABORT: SQLite source missing' >&2; exit 1; }
test -d "$BACKUP_DIR" && test -w "$BACKUP_DIR"
test ! -e "$BACKUP" && test ! -e "$BACKUP.sha256" || { echo 'ABORT: SQLite final backup path already exists' >&2; exit 1; }

BACKUP_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.sqlite.tmp.XXXXXX")"
FOREIGN_KEY_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.sqlite.foreign-key.tmp.XXXXXX")"
SHA_TMP=''
backup_complete=0
cleanup_sqlite_backup() {
  if [[ -n "$BACKUP_TMP" ]]; then rm -f -- "$BACKUP_TMP"; fi
  if [[ -n "$FOREIGN_KEY_TMP" ]]; then rm -f -- "$FOREIGN_KEY_TMP"; fi
  if [[ -n "$SHA_TMP" ]]; then rm -f -- "$SHA_TMP"; fi
  if [[ "$backup_complete" -ne 1 ]]; then
    rm -f -- "$BACKUP" "$BACKUP.sha256"
  fi
}
trap cleanup_sqlite_backup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$BACKUP_TMP" "$FOREIGN_KEY_TMP"
[[ "$BACKUP_TMP" != *"'"* ]] || { echo 'ABORT: SQLite temporary path contains a single quote' >&2; exit 1; }

sqlite3 "$PROD_SQLITE_PATH" ".timeout 30000" ".backup '$BACKUP_TMP'"
test -s "$BACKUP_TMP"
test "$(sqlite3 "$BACKUP_TMP" 'PRAGMA quick_check;')" = 'ok'
sqlite3 "$BACKUP_TMP" 'PRAGMA foreign_key_check;' >"$FOREIGN_KEY_TMP"
test ! -s "$FOREIGN_KEY_TMP"

mv -- "$BACKUP_TMP" "$BACKUP"
BACKUP_TMP=''
chmod 600 "$BACKUP"
rm -f -- "$FOREIGN_KEY_TMP"
FOREIGN_KEY_TMP=''
SHA_TMP="$(mktemp "$BACKUP_DIR/.newapi-auth-session-precutover-20260722.sqlite.sha256.tmp.XXXXXX")"
chmod 600 "$SHA_TMP"
(
  cd "$BACKUP_DIR"
  sha256sum -- "$(basename "$BACKUP")"
) >"$SHA_TMP"
mv -- "$SHA_TMP" "$BACKUP.sha256"
SHA_TMP=''
chmod 600 "$BACKUP.sha256"
(
  cd "$BACKUP_DIR"
  sha256sum --check "$(basename "$BACKUP.sha256")"
)
backup_complete=1
trap - EXIT INT TERM
```

After the selected block succeeds, copy the exact checksum text into the change
record and set:

```text
PROD_DB_BACKUP_PATH=<exact backup path>
PROD_DB_BACKUP_SHA256=<exact 64-hex SHA-256 from the approved backup>
BACKUP_VERIFIED_BY=<second operator>
BACKUP_VERIFIED_AT_UTC=<ISO-8601 timestamp>
```

**Gate:** a second operator must run `sha256sum --check` and confirm dialect
verification. Abort before image load, candidate start, migration, or traffic
change if this evidence is incomplete.

## Phase 4: Candidate Image Transfer, Load, and Verification

Do not run before explicit transfer approval. On the build host, package only the
exact candidate tag:

```powershell
$ErrorActionPreference = 'Stop'
$archive = 'newapi-auth-session-candidate-20260722.tar'
$archiveTemp = "$archive.partial-$PID"
$expectedImageId = 'sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
$candidateId = (docker image inspect --format '{{.Id}}' 'newapi:auth-session-candidate-20260722').Trim()
if ($LASTEXITCODE -ne 0) { throw 'ABORT: candidate image is unavailable' }
if ($candidateId -ne $expectedImageId) {
  throw 'ABORT: candidate image digest mismatch before save'
}
if (Test-Path -LiteralPath $archive) { throw 'ABORT: candidate archive already exists' }
try {
  docker save --output $archiveTemp 'newapi:auth-session-candidate-20260722'
  if ($LASTEXITCODE -ne 0) { throw 'ABORT: docker save failed' }
  if (-not (Test-Path -LiteralPath $archiveTemp) -or (Get-Item -LiteralPath $archiveTemp).Length -eq 0) {
    throw 'ABORT: candidate archive is empty'
  }
  Move-Item -LiteralPath $archiveTemp -Destination $archive
  Get-FileHash -Algorithm SHA256 $archive | Format-List
} finally {
  if (Test-Path -LiteralPath $archiveTemp) { Remove-Item -LiteralPath $archiveTemp -Force }
}
```

Record the archive SHA-256 as `CANDIDATE_ARCHIVE_SHA256`. Transfer the archive
with the separately approved transport to the exact approved host. For an
approved SSH transport, keep host verification enabled and use configured key or
agent authentication, never a password embedded in the command:

```bash
set -Eeuo pipefail
set +x
umask 077
LOCAL_ARCHIVE='newapi-auth-session-candidate-20260722.tar'
PROD_SSH_ALIAS='<FILL approved SSH alias>'
REMOTE_STAGING_DIR='<FILL approved mode-0700 staging directory>'
: "${PROD_SSH_ALIAS:?ABORT: approved SSH alias missing}"
: "${REMOTE_STAGING_DIR:?ABORT: remote staging directory missing}"
test -s "$LOCAL_ARCHIVE"
scp -- "$LOCAL_ARCHIVE" "${PROD_SSH_ALIAS}:${REMOTE_STAGING_DIR}/"
```

On the production host:

```bash
set -Eeuo pipefail
set +x
umask 077
STAGED_IMAGE='<FILL exact staging directory>/newapi-auth-session-candidate-20260722.tar'
CANDIDATE_ARCHIVE_SHA256='<FILL recorded archive SHA-256>'
: "${STAGED_IMAGE:?ABORT: staged image path missing}"
[[ "$CANDIDATE_ARCHIVE_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: archive SHA-256 is not 64 lowercase hex' >&2; exit 1; }
test -s "$STAGED_IMAGE"
test "$(sha256sum "$STAGED_IMAGE" | awk '{print $1}')" = "$CANDIDATE_ARCHIVE_SHA256" || { echo 'ABORT: archive checksum mismatch' >&2; exit 1; }

docker load --input "$STAGED_IMAGE"
test "$(docker image inspect --format '{{.Id}}' 'newapi:auth-session-candidate-20260722')" = \
  'sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee' || { echo 'ABORT: loaded image digest mismatch' >&2; exit 1; }
docker image inspect --format '{{.RepoTags}} {{.Id}} {{.Os}}/{{.Architecture}} {{with index .Config "User"}}{{.}}{{else}}root{{end}}' \
  'newapi:auth-session-candidate-20260722'
```

**Gate:** abort on archive, image ID, OS, or architecture mismatch. Do not retag
another image to make a failed comparison pass.

## Phase 5: Start the Candidate Without Public Traffic

Do not start until the approved database backup and its SHA-256 are verified.
Render `PROD_ENV_FILE` from the approved secret store with mode 0600. It must
contain the existing production environment plus the exact session settings in
this runbook. Do not display it. The mount syntax below assumes the captured
production `/data` mount; use the captured bind or volume form exactly.

For a bind mount:

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_ENV_FILE='<FILL mode-0600 secret-store-rendered env path>'
PROD_NETWORK='<FILL exact captured Docker network>'
PROD_DATA_SOURCE='<FILL exact captured host bind source>'
PROD_DB_BACKUP_PATH='<FILL exact approved backup path>'
PROD_DB_BACKUP_SHA256='<FILL exact approved backup SHA-256>'
CANDIDATE_IMAGE_ID='sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
: "${PROD_ENV_FILE:?ABORT: production env file missing}"
: "${PROD_NETWORK:?ABORT: production network missing}"
: "${PROD_DATA_SOURCE:?ABORT: production data source missing}"
: "${PROD_DB_BACKUP_PATH:?ABORT: database backup path missing}"
[[ "$PROD_DB_BACKUP_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: backup SHA-256 is not 64 lowercase hex' >&2; exit 1; }
test "$(stat -c '%a' "$PROD_ENV_FILE")" = '600' || { echo 'ABORT: env file permissions must be 600' >&2; exit 1; }
test -s "$PROD_DB_BACKUP_PATH" && test -s "$PROD_DB_BACKUP_PATH.sha256"
test "$(docker image inspect --format '{{.Id}}' 'newapi:auth-session-candidate-20260722')" = "$CANDIDATE_IMAGE_ID"
test "$(sha256sum "$PROD_DB_BACKUP_PATH" | awk '{print $1}')" = "$PROD_DB_BACKUP_SHA256" || { echo 'ABORT: backup checksum evidence mismatch' >&2; exit 1; }
(
  cd "$(dirname "$PROD_DB_BACKUP_PATH")"
  sha256sum --check "$(basename "$PROD_DB_BACKUP_PATH.sha256")"
)
docker container inspect 'new-api-auth-session-candidate-20260722' >/dev/null 2>&1 && { echo 'ABORT: candidate container name already exists' >&2; exit 1; }

docker run --detach \
  --name 'new-api-auth-session-candidate-20260722' \
  --restart=no \
  --env-file "$PROD_ENV_FILE" \
  --network "$PROD_NETWORK" \
  --mount "type=bind,src=$PROD_DATA_SOURCE,dst=/data" \
  --publish '127.0.0.1:33000:3000' \
  "$CANDIDATE_IMAGE_ID"
test "$(docker container inspect --format '{{.Image}}' 'new-api-auth-session-candidate-20260722')" = \
  "$CANDIDATE_IMAGE_ID"
```

For a named volume, replace only the mount command with the captured source:

```text
--mount "type=volume,src=$PROD_DATA_SOURCE,dst=/data"
```

If the production container has additional mounts, devices, capabilities,
ulimits, DNS settings, or non-default entrypoint/command, include their exact
captured values before execution. Do not infer or silently omit them. The
loopback-only publish is mandatory: `docker port` must show no public bind.

```bash
set -Eeuo pipefail
set +x
umask 077
docker container inspect --format '{{json .HostConfig.PortBindings}}' \
  'new-api-auth-session-candidate-20260722' | jq -e \
  '.["3000/tcp"] | length == 1 and .[0].HostIp == "127.0.0.1" and .[0].HostPort == "33000"' >/dev/null
```

**Gate:** abort if the candidate is externally reachable, uses a different
database/configuration than approved, or the old container changes state.

## Phase 6: Health, Migration, and Restart Checks

Capture logs to a restricted file because unexpected errors can contain
configuration data. Do not paste raw logs into chat or tickets.

```bash
set -Eeuo pipefail
set +x
umask 077
EVIDENCE_DIR='<FILL exact Phase 2 evidence directory>'
HEALTH_FINAL="$EVIDENCE_DIR/candidate-health-first.json"
LOG_FINAL="$EVIDENCE_DIR/candidate-first-start.restricted.log"
[[ "$EVIDENCE_DIR" == /* ]] || { echo 'ABORT: EVIDENCE_DIR must be absolute' >&2; exit 1; }
test -d "$EVIDENCE_DIR" && test "$(stat -c '%a' "$EVIDENCE_DIR")" = '700'
test -f "$EVIDENCE_DIR/evidence.sha256"
(
  cd "$EVIDENCE_DIR"
  sha256sum --check 'evidence.sha256'
)
test ! -e "$HEALTH_FINAL" && test ! -e "$LOG_FINAL" || { echo 'ABORT: first-start evidence target already exists' >&2; exit 1; }
HEALTH_TMP="$(mktemp "$EVIDENCE_DIR/.candidate-health-first.tmp.XXXXXX")"
LOG_TMP="$(mktemp "$EVIDENCE_DIR/.candidate-first-start.log.tmp.XXXXXX")"
cleanup_first_start_evidence() {
  if [[ -n "$HEALTH_TMP" ]]; then rm -f -- "$HEALTH_TMP"; fi
  if [[ -n "$LOG_TMP" ]]; then rm -f -- "$LOG_TMP"; fi
}
trap cleanup_first_start_evidence EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$HEALTH_TMP" "$LOG_TMP"
for attempt in $(seq 1 30); do
  if curl --fail --silent --show-error \
    'http://127.0.0.1:33000/api/status' >"$HEALTH_TMP" && \
    jq -e '.success == true' "$HEALTH_TMP" >/dev/null; then
    break
  fi
  sleep 2
done
jq -e '.success == true' "$HEALTH_TMP" >/dev/null || { echo 'ABORT: candidate health failed' >&2; exit 1; }
mv -- "$HEALTH_TMP" "$HEALTH_FINAL"
HEALTH_TMP=''

docker logs --since 10m 'new-api-auth-session-candidate-20260722' \
  >"$LOG_TMP" 2>&1
mv -- "$LOG_TMP" "$LOG_FINAL"
LOG_TMP=''
chmod 600 "$HEALTH_FINAL" "$LOG_FINAL"
docker container inspect --format '{{.State.Status}} {{.State.ExitCode}} {{.RestartCount}}' \
  'new-api-auth-session-candidate-20260722'
trap - EXIT INT TERM
```

An authorized operator must review the restricted log for migration errors,
panic, database lock/timeout, repeated DDL, or authentication configuration
rejection. Do not rely only on string matching.

Verify the relevant schema using the selected dialect's read-only metadata
commands. Expected tables include `user_sessions`, `auth_flows`, and
`external_identity_claims`; expected columns include `users.auth_version` and
`user_sessions.previous_refresh_hash`. Do not mutate schema manually to satisfy
these checks.

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_DB_DIALECT='<FILL mysql|postgresql|sqlite>'
EVIDENCE_DIR='<FILL exact Phase 2 evidence directory>'
SCHEMA_FINAL="$EVIDENCE_DIR/migration-schema-normalized.txt"
EXPECTED_SCHEMA=$'auth_flows\nexternal_identity_claims\nuser_sessions\nuser_sessions.previous_refresh_hash\nusers.auth_version'
[[ "$EVIDENCE_DIR" == /* ]] || { echo 'ABORT: EVIDENCE_DIR must be absolute' >&2; exit 1; }
test -d "$EVIDENCE_DIR" && test "$(stat -c '%a' "$EVIDENCE_DIR")" = '700'
test -f "$EVIDENCE_DIR/evidence.sha256"
(
  cd "$EVIDENCE_DIR"
  sha256sum --check 'evidence.sha256'
)
test ! -e "$SCHEMA_FINAL" || { echo 'ABORT: normalized schema evidence already exists' >&2; exit 1; }
SCHEMA_RAW="$(mktemp "$EVIDENCE_DIR/.migration-schema-raw.tmp.XXXXXX")"
SCHEMA_NORMALIZED="$(mktemp "$EVIDENCE_DIR/.migration-schema-normalized.tmp.XXXXXX")"
cleanup_schema_evidence() {
  if [[ -n "$SCHEMA_RAW" ]]; then rm -f -- "$SCHEMA_RAW"; fi
  if [[ -n "$SCHEMA_NORMALIZED" ]]; then rm -f -- "$SCHEMA_NORMALIZED"; fi
}
trap cleanup_schema_evidence EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$SCHEMA_RAW" "$SCHEMA_NORMALIZED"
case "$PROD_DB_DIALECT" in
  mysql)
    : "${MYSQL_HOST:?ABORT: MYSQL_HOST missing}"
    : "${MYSQL_PORT:?ABORT: MYSQL_PORT missing}"
    : "${MYSQL_USER:?ABORT: MYSQL_USER missing}"
    : "${MYSQL_DATABASE:?ABORT: MYSQL_DATABASE missing}"
    : "${MYSQL_PWD:?ABORT: MYSQL_PWD not injected}"
    mysql --host="$MYSQL_HOST" --port="$MYSQL_PORT" --user="$MYSQL_USER" "$MYSQL_DATABASE" \
      --batch --skip-column-names --raw --execute="SELECT entry FROM (SELECT table_name AS entry FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('auth_flows','external_identity_claims','user_sessions') UNION ALL SELECT CONCAT(table_name,'.',column_name) AS entry FROM information_schema.columns WHERE table_schema=DATABASE() AND ((table_name='user_sessions' AND column_name='previous_refresh_hash') OR (table_name='users' AND column_name='auth_version'))) AS expected_schema ORDER BY entry;" >"$SCHEMA_RAW"
    ;;
  postgresql)
    : "${PGHOST:?ABORT: PGHOST missing}"
    : "${PGPORT:?ABORT: PGPORT missing}"
    : "${PGUSER:?ABORT: PGUSER missing}"
    : "${PGDATABASE:?ABORT: PGDATABASE missing}"
    : "${PGPASSWORD:?ABORT: PGPASSWORD not injected}"
    psql --host="$PGHOST" --port="$PGPORT" --username="$PGUSER" --dbname="$PGDATABASE" \
      --tuples-only --no-align --command="SELECT entry FROM (SELECT table_name::text AS entry FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('auth_flows','external_identity_claims','user_sessions') UNION ALL SELECT table_name||'.'||column_name AS entry FROM information_schema.columns WHERE table_schema='public' AND ((table_name='user_sessions' AND column_name='previous_refresh_hash') OR (table_name='users' AND column_name='auth_version'))) AS expected_schema ORDER BY entry;" >"$SCHEMA_RAW"
    ;;
  sqlite)
    PROD_SQLITE_PATH='<FILL exact host SQLite path from captured mount>'
    test -f "$PROD_SQLITE_PATH"
    sqlite3 -batch -noheader "$PROD_SQLITE_PATH" "SELECT entry FROM (SELECT name AS entry FROM sqlite_master WHERE type='table' AND name IN ('auth_flows','external_identity_claims','user_sessions') UNION ALL SELECT 'user_sessions.'||name AS entry FROM pragma_table_info('user_sessions') WHERE name='previous_refresh_hash' UNION ALL SELECT 'users.'||name AS entry FROM pragma_table_info('users') WHERE name='auth_version') ORDER BY entry;" >"$SCHEMA_RAW"
    ;;
  *)
    echo 'ABORT: PROD_DB_DIALECT must be mysql, postgresql, or sqlite' >&2
    exit 1
    ;;
esac
tr -d '\r' <"$SCHEMA_RAW" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u >"$SCHEMA_NORMALIZED"
diff --unified --label expected-schema --label actual-schema \
  <(printf '%s\n' "$EXPECTED_SCHEMA") "$SCHEMA_NORMALIZED"
mv -- "$SCHEMA_NORMALIZED" "$SCHEMA_FINAL"
SCHEMA_NORMALIZED=''
rm -f -- "$SCHEMA_RAW"
SCHEMA_RAW=''
chmod 600 "$SCHEMA_FINAL"
trap - EXIT INT TERM
```

Restart once without changing configuration, then repeat health and log review:

```bash
set -Eeuo pipefail
set +x
umask 077
EVIDENCE_DIR='<FILL exact Phase 2 evidence directory>'
CANDIDATE_IMAGE_ID='sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
HEALTH_FINAL="$EVIDENCE_DIR/candidate-health-restart.json"
LOG_FINAL="$EVIDENCE_DIR/candidate-restart.restricted.log"
[[ "$EVIDENCE_DIR" == /* ]] || { echo 'ABORT: EVIDENCE_DIR must be absolute' >&2; exit 1; }
test -d "$EVIDENCE_DIR" && test "$(stat -c '%a' "$EVIDENCE_DIR")" = '700'
test -f "$EVIDENCE_DIR/evidence.sha256"
(
  cd "$EVIDENCE_DIR"
  sha256sum --check 'evidence.sha256'
)
test ! -e "$HEALTH_FINAL" && test ! -e "$LOG_FINAL" || { echo 'ABORT: restart evidence target already exists' >&2; exit 1; }
HEALTH_TMP="$(mktemp "$EVIDENCE_DIR/.candidate-health-restart.tmp.XXXXXX")"
LOG_TMP="$(mktemp "$EVIDENCE_DIR/.candidate-restart.log.tmp.XXXXXX")"
cleanup_restart_evidence() {
  if [[ -n "$HEALTH_TMP" ]]; then rm -f -- "$HEALTH_TMP"; fi
  if [[ -n "$LOG_TMP" ]]; then rm -f -- "$LOG_TMP"; fi
}
trap cleanup_restart_evidence EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 600 "$HEALTH_TMP" "$LOG_TMP"
docker restart 'new-api-auth-session-candidate-20260722'
test "$(docker container inspect --format '{{.Image}}' 'new-api-auth-session-candidate-20260722')" = \
  "$CANDIDATE_IMAGE_ID"
for attempt in $(seq 1 30); do
  if curl --fail --silent --show-error \
    'http://127.0.0.1:33000/api/status' >"$HEALTH_TMP" && \
    jq -e '.success == true' "$HEALTH_TMP" >/dev/null; then
    break
  fi
  sleep 2
done
jq -e '.success == true' "$HEALTH_TMP" >/dev/null || { echo 'ABORT: restart health failed' >&2; exit 1; }
mv -- "$HEALTH_TMP" "$HEALTH_FINAL"
HEALTH_TMP=''
docker logs --since 5m 'new-api-auth-session-candidate-20260722' \
  >"$LOG_TMP" 2>&1
mv -- "$LOG_TMP" "$LOG_FINAL"
LOG_TMP=''
chmod 600 "$HEALTH_FINAL" "$LOG_FINAL"
trap - EXIT INT TERM
```

**Gate:** abort unless the second start is healthy and shows no migration error
or repeated incompatible/type-changing DDL.

## Phase 7: Traffic Switch and 5-15 Minute Observation

Choose only the switch method named in the approval record. Capture baseline
request/error/latency, database, proxy, and container metrics immediately before
the switch. Keep the old container, old configuration, baseline image, backup,
and previous proxy configuration intact.

### Preferred: Reversible Proxy Upstream Switch

The candidate remains on loopback port 33000. A separately reviewed full Nginx
configuration must change only the application upstream from
`PROD_PROXY_UPSTREAM_OLD` to `127.0.0.1:33000`. Record its exact path, owner, mode,
and SHA-256 in the approval record. Do not change TLS, headers, timeouts, CDN, or
unrelated routes.

This runbook supports canonical absolute paths to regular files only. It rejects
symlink active/candidate paths and any proxy-container, generated-link, or other
topology; those require a separate reviewed runbook. Candidate content is never
allowed to supply ACLs, xattrs, ownership, mode, or SELinux context.

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_PROXY_CONFIG='<FILL exact active proxy config path>'
PROD_PROXY_CONFIG_OWNER_UID='<FILL expected numeric owner UID>'
PROD_PROXY_CONFIG_GROUP_GID='<FILL original numeric group GID>'
PROD_PROXY_CONFIG_MODE='<FILL original 3- or 4-digit octal mode>'
PROD_PROXY_CANDIDATE_CONFIG='<FILL exact reviewed candidate config path>'
PROD_PROXY_CANDIDATE_SHA256='<FILL exact reviewed candidate config SHA-256>'
PROD_TRAFFIC_CLOSE_SCRIPT='<FILL exact reviewed traffic-close script path>'
PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSE_SCRIPT_SHA256='<FILL exact reviewed traffic-close script SHA-256>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT='<FILL exact reviewed read-only closure-check script path>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256='<FILL exact reviewed closure-check script SHA-256>'
PROD_TRAFFIC_CONTROL_REVIEW_RECORD='<FILL exact peer-review/approval record for both scripts>'
CANDIDATE_IMAGE_ID='sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
ROLLBACK_CONFIG="$PROD_PROXY_CONFIG.pre-auth-session-20260722"
: "${PROD_PROXY_CONFIG:?ABORT: active proxy config missing}"
: "${PROD_PROXY_CANDIDATE_CONFIG:?ABORT: candidate proxy config missing}"
[[ "$PROD_PROXY_CONFIG_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: proxy owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_PROXY_CONFIG_GROUP_GID" =~ ^[0-9]+$ ]] || { echo 'ABORT: proxy group GID must be numeric' >&2; exit 1; }
[[ "$PROD_PROXY_CONFIG_MODE" =~ ^[0-7]{3,4}$ ]] || { echo 'ABORT: proxy mode must be octal' >&2; exit 1; }
[[ "$PROD_PROXY_CANDIDATE_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: candidate proxy SHA-256 is not 64 lowercase hex' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: traffic-close script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: traffic-close script SHA-256 is invalid' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: closure-check script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: closure-check script SHA-256 is invalid' >&2; exit 1; }
: "${PROD_TRAFFIC_CONTROL_REVIEW_RECORD:?ABORT: traffic-control peer-review record missing}"
[[ "$PROD_TRAFFIC_CONTROL_REVIEW_RECORD" != *'<FILL'* ]] || { echo 'ABORT: traffic-control review record unresolved' >&2; exit 1; }
test -f "$PROD_PROXY_CONFIG" && test -f "$PROD_PROXY_CANDIDATE_CONFIG"
test ! -L "$PROD_PROXY_CONFIG" && test ! -L "$PROD_PROXY_CANDIDATE_CONFIG" || { echo 'ABORT: proxy config symlinks are unsupported' >&2; exit 1; }
test "$PROD_PROXY_CONFIG" = "$(realpath -e -- "$PROD_PROXY_CONFIG")" || { echo 'ABORT: active proxy path must be absolute and canonical' >&2; exit 1; }
test "$PROD_PROXY_CANDIDATE_CONFIG" = "$(realpath -e -- "$PROD_PROXY_CANDIDATE_CONFIG")" || { echo 'ABORT: candidate proxy path must be absolute and canonical' >&2; exit 1; }
test "$(stat -c '%F' "$PROD_PROXY_CONFIG")" = 'regular file'
test "$(stat -c '%F' "$PROD_PROXY_CANDIDATE_CONFIG")" = 'regular file'
test ! -e "$ROLLBACK_CONFIG" || { echo 'ABORT: rollback config already exists' >&2; exit 1; }
test "$(stat -c '%u' "$PROD_PROXY_CONFIG")" = "$PROD_PROXY_CONFIG_OWNER_UID"
test "$(stat -c '%g' "$PROD_PROXY_CONFIG")" = "$PROD_PROXY_CONFIG_GROUP_GID"
test "$(stat -c '%a' "$PROD_PROXY_CONFIG")" = "$PROD_PROXY_CONFIG_MODE"
test "$(stat -c '%u' "$PROD_PROXY_CANDIDATE_CONFIG")" = "$PROD_PROXY_CONFIG_OWNER_UID"
test "$(stat -c '%a' "$PROD_PROXY_CANDIDATE_CONFIG")" = '600'
for script in "$PROD_TRAFFIC_CLOSE_SCRIPT" "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"; do
  test -f "$script" && test ! -L "$script"
  test "$(stat -c '%a' "$script")" = '700'
  bash -n "$script"
  if grep -Eq '<FILL|TODO|REPLACE_ME' "$script"; then
    echo 'ABORT: traffic-control script contains an unresolved placeholder' >&2
    exit 1
  fi
done
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSE_SCRIPT")" = "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID"
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT")" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID"
test "$(sha256sum "$PROD_TRAFFIC_CLOSE_SCRIPT" | awk '{print $1}')" = "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256"
test "$(sha256sum "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT" | awk '{print $1}')" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256"
grep -Fxq '# CUTOVER_CONTRACT: close-approved-domain-routes-v1' "$PROD_TRAFFIC_CLOSE_SCRIPT"
grep -Fxq '# CUTOVER_CONTRACT: verify-approved-domain-routes-closed-v1' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
test "$(sha256sum "$PROD_PROXY_CANDIDATE_CONFIG" | awk '{print $1}')" = \
  "$PROD_PROXY_CANDIDATE_SHA256" || { echo 'ABORT: candidate proxy config checksum mismatch' >&2; exit 1; }
test "$(docker container inspect --format '{{.Image}}' 'new-api-auth-session-candidate-20260722')" = \
  "$CANDIDATE_IMAGE_ID"

command -v realpath >/dev/null
command -v getfacl >/dev/null
command -v getfattr >/dev/null
nginx -t -c "$PROD_PROXY_CANDIDATE_CONFIG"
PROXY_DIR="$(dirname "$PROD_PROXY_CONFIG")"
ROLLBACK_SHA="$ROLLBACK_CONFIG.sha256"
ROLLBACK_STAT="$ROLLBACK_CONFIG.stat"
ROLLBACK_ACL="$ROLLBACK_CONFIG.acl"
ROLLBACK_XATTR="$ROLLBACK_CONFIG.xattr"
ROLLBACK_METADATA_SHA="$ROLLBACK_CONFIG.metadata.sha256"
for target in "$ROLLBACK_SHA" "$ROLLBACK_STAT" "$ROLLBACK_ACL" "$ROLLBACK_XATTR" "$ROLLBACK_METADATA_SHA"; do
  test ! -e "$target" || { echo 'ABORT: rollback evidence target already exists' >&2; exit 1; }
done
ROLLBACK_TMP="$(mktemp "$PROXY_DIR/.proxy-rollback.tmp.XXXXXX")"
ROLLBACK_SHA_TMP="$(mktemp "$PROXY_DIR/.proxy-rollback.sha256.tmp.XXXXXX")"
INSTALL_TMP="$(mktemp "$PROXY_DIR/.proxy-install.tmp.XXXXXX")"
RESTORE_TMP=''
ACTIVE_STAT="$(stat -c '%F|%u|%g|%a|%C' "$PROD_PROXY_CONFIG")"
printf '%s\n' "$ACTIVE_STAT" >"$ROLLBACK_STAT"
getfacl -cpn -- "$PROD_PROXY_CONFIG" >"$ROLLBACK_ACL"
getfattr --absolute-names --dump --match=- -- "$PROD_PROXY_CONFIG" | sed '/^# file:/d' >"$ROLLBACK_XATTR"
chmod 600 "$ROLLBACK_STAT" "$ROLLBACK_ACL" "$ROLLBACK_XATTR"
ROLLBACK_METADATA_SHA_TMP="$(mktemp "$PROXY_DIR/.proxy-rollback-metadata.sha256.tmp.XXXXXX")"
(
  cd "$PROXY_DIR"
  sha256sum -- "$(basename "$ROLLBACK_STAT")" "$(basename "$ROLLBACK_ACL")" "$(basename "$ROLLBACK_XATTR")"
) >"$ROLLBACK_METADATA_SHA_TMP"
mv -- "$ROLLBACK_METADATA_SHA_TMP" "$ROLLBACK_METADATA_SHA"
chmod 600 "$ROLLBACK_METADATA_SHA"
(
  cd "$PROXY_DIR"
  sha256sum --check "$(basename "$ROLLBACK_METADATA_SHA")"
)

cp --preserve=all "$PROD_PROXY_CONFIG" "$ROLLBACK_TMP"
chmod 600 "$ROLLBACK_TMP"
cmp --silent "$PROD_PROXY_CONFIG" "$ROLLBACK_TMP"
mv -- "$ROLLBACK_TMP" "$ROLLBACK_CONFIG"
chmod 600 "$ROLLBACK_SHA_TMP"
(
  cd "$PROXY_DIR"
  sha256sum -- "$(basename "$ROLLBACK_CONFIG")"
) >"$ROLLBACK_SHA_TMP"
mv -- "$ROLLBACK_SHA_TMP" "$ROLLBACK_SHA"
(
  cd "$PROXY_DIR"
  sha256sum --check "$(basename "$ROLLBACK_SHA")"
)
BASELINE_PROXY_SHA256="$(sha256sum "$ROLLBACK_CONFIG" | awk '{print $1}')"

converge_forward_proxy_to_baseline() {
  trap - EXIT INT TERM
  set +e
  recovery_rc=0
  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  closed_rc=$?
  if [[ "$close_rc" -ne 0 || "$closed_rc" -ne 0 ]]; then recovery_rc=1; fi
  if [[ -f "$PROD_PROXY_CONFIG" && ! -L "$PROD_PROXY_CONFIG" ]]; then
    active_sha="$(sha256sum "$PROD_PROXY_CONFIG" | awk '{print $1}')"
  else
    active_sha='missing-or-invalid'
    recovery_rc=1
  fi
  if [[ "$active_sha" != "$BASELINE_PROXY_SHA256" && -f "$PROD_PROXY_CONFIG" && ! -L "$PROD_PROXY_CONFIG" ]]; then
    RESTORE_TMP="$(mktemp "$PROXY_DIR/.proxy-restore.tmp.XXXXXX")"
    cp --preserve=all "$PROD_PROXY_CONFIG" "$RESTORE_TMP"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    cat -- "$ROLLBACK_CONFIG" >"$RESTORE_TMP"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    if [[ "$(sha256sum "$RESTORE_TMP" | awk '{print $1}')" = "$BASELINE_PROXY_SHA256" ]]; then
      mv -- "$RESTORE_TMP" "$PROD_PROXY_CONFIG"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    else
      recovery_rc=1
    fi
  fi
  if [[ -f "$PROD_PROXY_CONFIG" && ! -L "$PROD_PROXY_CONFIG" && \
    "$(sha256sum "$PROD_PROXY_CONFIG" | awk '{print $1}')" = "$BASELINE_PROXY_SHA256" ]]; then
    if [[ "$(stat -c '%F|%u|%g|%a|%C' "$PROD_PROXY_CONFIG")" != "$ACTIVE_STAT" ]]; then recovery_rc=1; fi
    getfacl -cpn -- "$PROD_PROXY_CONFIG" | cmp --silent - "$ROLLBACK_ACL"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    getfattr --absolute-names --dump --match=- -- "$PROD_PROXY_CONFIG" | sed '/^# file:/d' | cmp --silent - "$ROLLBACK_XATTR"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    if [[ "$recovery_rc" -eq 0 ]]; then
      nginx -t -c "$PROD_PROXY_CONFIG"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    fi
    if [[ "$recovery_rc" -eq 0 ]]; then
      nginx -s reload -c "$PROD_PROXY_CONFIG"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    fi
  else
    recovery_rc=1
  fi
  rm -f -- "$INSTALL_TMP" "$RESTORE_TMP"
  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  final_close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  final_closed_rc=$?
  if [[ "$final_close_rc" -ne 0 || "$final_closed_rc" -ne 0 ]]; then recovery_rc=1; fi
  if [[ "$recovery_rc" -ne 0 ]]; then
    echo 'CRITICAL: forward proxy recovery or traffic-closure proof failed; keep traffic closed and escalate' >&2
  else
    echo 'CRITICAL: forward proxy switch failed; baseline content restored and traffic remains closed pending review' >&2
  fi
  exit 90
}
trap converge_forward_proxy_to_baseline EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cp --preserve=all "$PROD_PROXY_CONFIG" "$INSTALL_TMP"
cat -- "$PROD_PROXY_CANDIDATE_CONFIG" >"$INSTALL_TMP"
test "$(sha256sum "$INSTALL_TMP" | awk '{print $1}')" = "$PROD_PROXY_CANDIDATE_SHA256"
test "$(stat -c '%F|%u|%g|%a|%C' "$INSTALL_TMP")" = "$ACTIVE_STAT"
getfacl -cpn -- "$INSTALL_TMP" | cmp --silent - "$ROLLBACK_ACL"
getfattr --absolute-names --dump --match=- -- "$INSTALL_TMP" | sed '/^# file:/d' | cmp --silent - "$ROLLBACK_XATTR"
mv -- "$INSTALL_TMP" "$PROD_PROXY_CONFIG"
nginx -t -c "$PROD_PROXY_CONFIG"
nginx -s reload -c "$PROD_PROXY_CONFIG"
trap - EXIT INT TERM
```

Immediately confirm both domains and the candidate container identity:

```bash
set -Eeuo pipefail
set +x
umask 077
CANDIDATE_IMAGE_ID='sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
curl --fail --silent --show-error 'https://yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
curl --fail --silent --show-error 'https://global.yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
test "$(docker container inspect --format '{{.Image}}' 'new-api-auth-session-candidate-20260722')" = \
  "$CANDIDATE_IMAGE_ID"
docker container inspect --format '{{.State.Status}} {{.RestartCount}}' 'new-api-auth-session-candidate-20260722'
```

This switch block supports a file-based Nginx deployment only. A proxy container
or control plane requires a separate reviewed runbook commit containing its exact
checksum-gated atomic install, validation, reload, and automatic rollback commands.

### Alternative: Exact-Name Container Replacement

Use only when explicitly approved and when proxy switching is unavailable. A
separately reviewed script must contain the complete exact `docker create`
command derived from the restricted container capture. It must preserve the
environment-file reference, mounts, networks, ports, restart policy, entrypoint,
command, capabilities, resource limits, and labels, changing only the image.
Never reconstruct secrets from terminal output. Record the script's exact path,
owner UID, mode, and SHA-256 in the approval record.

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_CONTAINER='<FILL exact current production container name>'
PROD_CONTAINER_CREATE_SCRIPT='<FILL exact reviewed create script path>'
PROD_CONTAINER_CREATE_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_CONTAINER_CREATE_SCRIPT_SHA256='<FILL exact reviewed create script SHA-256>'
PROD_TRAFFIC_CLOSE_SCRIPT='<FILL exact reviewed traffic-close script path>'
PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSE_SCRIPT_SHA256='<FILL exact reviewed traffic-close script SHA-256>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT='<FILL exact reviewed read-only closure-check script path>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256='<FILL exact reviewed closure-check script SHA-256>'
PROD_TRAFFIC_CONTROL_REVIEW_RECORD='<FILL exact peer-review/approval record for both scripts>'
PARKED_CONTAINER='new-api-production-pre-auth-session-20260722'
FAILED_CONTAINER='new-api-auth-session-failed-20260722'
CANDIDATE_IMAGE_ID='sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
: "${PROD_CONTAINER:?ABORT: production container name missing}"
: "${PROD_CONTAINER_CREATE_SCRIPT:?ABORT: create script path missing}"
[[ "$PROD_CONTAINER_CREATE_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: create script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_CONTAINER_CREATE_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: create script SHA-256 is not 64 lowercase hex' >&2; exit 1; }
test -f "$PROD_CONTAINER_CREATE_SCRIPT"
test "$(stat -c '%u' "$PROD_CONTAINER_CREATE_SCRIPT")" = "$PROD_CONTAINER_CREATE_SCRIPT_OWNER_UID"
test "$(stat -c '%a' "$PROD_CONTAINER_CREATE_SCRIPT")" = '700'
test "$(sha256sum "$PROD_CONTAINER_CREATE_SCRIPT" | awk '{print $1}')" = \
  "$PROD_CONTAINER_CREATE_SCRIPT_SHA256" || { echo 'ABORT: create script checksum mismatch' >&2; exit 1; }
bash -n "$PROD_CONTAINER_CREATE_SCRIPT"
grep -Eq '(^|[[:space:]])docker[[:space:]]+create([[:space:]]|$)' "$PROD_CONTAINER_CREATE_SCRIPT"
grep -Fxq '# CUTOVER_CONTRACT: docker-create-candidate-by-immutable-id-v1' "$PROD_CONTAINER_CREATE_SCRIPT"
grep -Fq "$CANDIDATE_IMAGE_ID" "$PROD_CONTAINER_CREATE_SCRIPT"
if grep -Eq '<FILL|TODO|REPLACE_ME' "$PROD_CONTAINER_CREATE_SCRIPT"; then
  echo 'ABORT: create script contains an unresolved placeholder' >&2
  exit 1
fi
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: traffic-close script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: traffic-close script SHA-256 is invalid' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: closure-check script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: closure-check script SHA-256 is invalid' >&2; exit 1; }
: "${PROD_TRAFFIC_CONTROL_REVIEW_RECORD:?ABORT: traffic-control peer-review record missing}"
[[ "$PROD_TRAFFIC_CONTROL_REVIEW_RECORD" != *'<FILL'* ]] || { echo 'ABORT: traffic-control review record unresolved' >&2; exit 1; }
for script in "$PROD_TRAFFIC_CLOSE_SCRIPT" "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"; do
  test -f "$script" && test ! -L "$script"
  test "$(stat -c '%a' "$script")" = '700'
  bash -n "$script"
  if grep -Eq '<FILL|TODO|REPLACE_ME' "$script"; then
    echo 'ABORT: traffic-control script contains an unresolved placeholder' >&2
    exit 1
  fi
done
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSE_SCRIPT")" = "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID"
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT")" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID"
test "$(sha256sum "$PROD_TRAFFIC_CLOSE_SCRIPT" | awk '{print $1}')" = "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256"
test "$(sha256sum "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT" | awk '{print $1}')" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256"
grep -Fxq '# CUTOVER_CONTRACT: close-approved-domain-routes-v1' "$PROD_TRAFFIC_CLOSE_SCRIPT"
grep -Fxq '# CUTOVER_CONTRACT: verify-approved-domain-routes-closed-v1' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
BASELINE_IMAGE_ID="$(docker image inspect --format '{{.Id}}' 'newapi:production-console-20260722-739cb2775')"
[[ "$BASELINE_IMAGE_ID" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo 'ABORT: baseline image ID is invalid' >&2; exit 1; }
test "$(docker image inspect --format '{{.Id}}' 'newapi:auth-session-candidate-20260722')" = "$CANDIDATE_IMAGE_ID"
docker container inspect "$PROD_CONTAINER" >/dev/null
test "$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")" = "$BASELINE_IMAGE_ID"
test "$(docker container inspect --format '{{.Config.Image}}' "$PROD_CONTAINER")" = \
  'newapi:production-console-20260722-739cb2775'
docker container inspect "$PARKED_CONTAINER" >/dev/null 2>&1 && { echo 'ABORT: parked container name already exists' >&2; exit 1; }
docker container inspect "$FAILED_CONTAINER" >/dev/null 2>&1 && { echo 'ABORT: failed container name already exists' >&2; exit 1; }

converge_forward_container_to_baseline() {
  trap - EXIT INT TERM
  set +e
  recovery_rc=0
  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  closed_rc=$?
  if [[ "$close_rc" -ne 0 || "$closed_rc" -ne 0 ]]; then recovery_rc=1; fi

  if docker container inspect "$PROD_CONTAINER" >/dev/null 2>&1; then
    prod_image="$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")"
    if [[ "$prod_image" = "$CANDIDATE_IMAGE_ID" ]]; then
      if [[ "$(docker container inspect --format '{{.State.Running}}' "$PROD_CONTAINER")" = 'true' ]]; then
        docker stop --time 30 "$PROD_CONTAINER"
        if [[ $? -ne 0 ]]; then recovery_rc=1; fi
      fi
      if ! docker container inspect "$FAILED_CONTAINER" >/dev/null 2>&1; then
        docker rename "$PROD_CONTAINER" "$FAILED_CONTAINER"
        if [[ $? -ne 0 ]]; then recovery_rc=1; fi
      else
        recovery_rc=1
      fi
    elif [[ "$prod_image" != "$BASELINE_IMAGE_ID" ]]; then
      recovery_rc=1
    fi
  fi

  if docker container inspect "$FAILED_CONTAINER" >/dev/null 2>&1 && \
    [[ "$(docker container inspect --format '{{.State.Running}}' "$FAILED_CONTAINER")" = 'true' ]]; then
    docker stop --time 30 "$FAILED_CONTAINER"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
  fi
  if ! docker container inspect "$PROD_CONTAINER" >/dev/null 2>&1; then
    if docker container inspect "$PARKED_CONTAINER" >/dev/null 2>&1 && \
      [[ "$(docker container inspect --format '{{.Image}}' "$PARKED_CONTAINER")" = "$BASELINE_IMAGE_ID" ]]; then
      docker rename "$PARKED_CONTAINER" "$PROD_CONTAINER"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    else
      recovery_rc=1
    fi
  fi
  if docker container inspect "$PROD_CONTAINER" >/dev/null 2>&1 && \
    [[ "$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")" = "$BASELINE_IMAGE_ID" ]]; then
    docker start "$PROD_CONTAINER"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    for attempt in $(seq 1 30); do
      if curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' | \
        jq -e '.success == true' >/dev/null; then
        break
      fi
      sleep 2
    done
    curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' | \
      jq -e '.success == true' >/dev/null
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
  else
    recovery_rc=1
  fi

  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  final_close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  final_closed_rc=$?
  if [[ "$final_close_rc" -ne 0 || "$final_closed_rc" -ne 0 ]]; then recovery_rc=1; fi
  if [[ "$recovery_rc" -ne 0 ]]; then
    echo 'CRITICAL: forward container recovery or traffic-closure proof failed; keep traffic closed and escalate' >&2
  else
    echo 'CRITICAL: forward container switch failed; baseline is healthy and traffic remains closed pending review' >&2
  fi
  exit 91
}
trap converge_forward_container_to_baseline EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker stop --time 30 "$PROD_CONTAINER"
docker rename "$PROD_CONTAINER" "$PARKED_CONTAINER"
"$PROD_CONTAINER_CREATE_SCRIPT"
docker container inspect "$PROD_CONTAINER" >/dev/null
test "$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")" = "$CANDIDATE_IMAGE_ID"
test "$(docker container inspect --format '{{.Config.Image}}' "$PROD_CONTAINER")" = \
  "$CANDIDATE_IMAGE_ID"
test "$(docker container inspect --format '{{.State.Status}}' "$PROD_CONTAINER")" = 'created'
docker start "$PROD_CONTAINER"
for attempt in $(seq 1 30); do
  if curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' | \
    jq -e '.success == true' >/dev/null; then
    break
  fi
  sleep 2
done
curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' | \
  jq -e '.success == true' >/dev/null
trap - EXIT INT TERM
```

Do not remove the parked old container. A create, start, or loopback health
failure automatically parks the failed candidate when possible, renames the old
container back to its exact production name, starts it, and verifies old health.
If automatic recovery itself fails, keep traffic closed and escalate.

### Observation Window

For at least 5 minutes and up to 15 minutes after traffic switch, record checks
at switch time, +1, +5, +10, and +15 minutes. Continue to the application matrix
only while every gate stays green:

```bash
set -Eeuo pipefail
set +x
umask 077
ACTIVE_CONTAINER='<FILL new-api-auth-session-candidate-20260722 for proxy switch, or exact PROD_CONTAINER for replacement>'
CANDIDATE_IMAGE_ID='sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
: "${ACTIVE_CONTAINER:?ABORT: active container name missing}"
date -u '+%Y-%m-%dT%H:%M:%SZ'
curl --fail --silent --show-error 'https://yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
curl --fail --silent --show-error 'https://global.yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
docker container inspect "$ACTIVE_CONTAINER" >/dev/null
test "$(docker container inspect --format '{{.Image}}' "$ACTIVE_CONTAINER")" = "$CANDIDATE_IMAGE_ID"
docker stats --no-stream "$ACTIVE_CONTAINER"
```

At each checkpoint, inspect dashboards for request rate, p50/p95 latency,
database connections/locks, Redis errors, login success by method, refresh status
codes, session mismatch/race recovery, and authenticated endpoint `401`, `409`,
`429`, and `5xx` rates. A brief login increase is expected because old dashboard
cookies are invalid. Persistent health, error, or latency regression is an abort
and rollback condition.

## Phase 8: Application and Relay Acceptance Matrix

Use dedicated low-privilege production test accounts. Obtain credentials, 2FA
material, email inbox access, and relay key from the approved secret store. Do
not place them in commands, screenshots, logs, or this record.

Complete in both supported domains where routing differs:

| Check | Exact acceptance condition |
| --- | --- |
| Old cookie behavior | A browser holding only a pre-cutover dashboard cookie is signed out and can reach login; no redirect loop. |
| Password login | Login succeeds and establishes a refresh cookie plus in-memory access token; no legacy cookie dependence. |
| Refresh | Let the access token expire or use the approved expiry test path; one refresh succeeds and the authenticated page remains stable. |
| 2FA | A 2FA-enabled account completes password plus OTP; invalid OTP is rejected without creating a session. |
| Profile | Profile loads and an approved reversible profile edit persists after refresh. |
| Wallet | Balance and wallet/top-up history match the pre-switch account snapshot; do not make a real purchase. |
| Logs | Usage/log pages load the expected recent records without cross-account data. |
| Registration | Registration policy, invitation/Turnstile behavior, and disabled/enabled state match production configuration. |
| Email | When SMTP is approved, send only to the dedicated test inbox and confirm verification delivery/link flow. If SMTP is not approved, this is a blocking unverified item, not a pass. |
| Session management | Current session is listed; create a second test session, revoke it, and verify it loses access; `revoke others` preserves only the current session. |
| Account switching | Sign out of account A, sign into account B in the same browser, and confirm profile, wallet, logs, queries, and cached UI contain no account A data. |
| Logout | Logout revokes the current server session, clears the refresh cookie, and protected routes require login. |
| Relay API key | Existing dedicated relay key remains valid; no dashboard token is accepted as a relay-key substitute. |

Relay `/v1/models` check, using a mode-0600 curl config generated by the approved
secret integration. The config supplies the authorization header without putting
the key in process arguments:

```bash
set -Eeuo pipefail
set +x
umask 077
RELAY_CURL_CONFIG='<FILL mode-0600 curl config containing the dedicated relay Authorization header>'
: "${RELAY_CURL_CONFIG:?ABORT: relay curl config missing}"
test -f "$RELAY_CURL_CONFIG"
test "$(stat -c '%a' "$RELAY_CURL_CONFIG")" = '600' || { echo 'ABORT: relay curl config permissions must be 600' >&2; exit 1; }
curl --config "$RELAY_CURL_CONFIG" --fail --silent --show-error \
  'https://yuaiapi.com/v1/models' | jq -e '.data | type == "array"' >/dev/null
```

Run a minimal-cost `/v1/responses` request only when upstream billing and the
dedicated test model are explicitly approved. Store its JSON body in a mode-0600
request file so prompts and credentials do not appear in shell history:

```bash
set -Eeuo pipefail
set +x
umask 077
RELAY_CURL_CONFIG='<FILL mode-0600 curl config containing the dedicated relay Authorization header>'
RESPONSES_REQUEST='<FILL mode-0600 approved minimal request JSON path>'
: "${RELAY_CURL_CONFIG:?ABORT: relay curl config missing}"
: "${RESPONSES_REQUEST:?ABORT: responses request path missing}"
test "$(stat -c '%a' "$RELAY_CURL_CONFIG")" = '600'
test "$(stat -c '%a' "$RESPONSES_REQUEST")" = '600'
curl --config "$RELAY_CURL_CONFIG" --fail --silent --show-error \
  --header 'Content-Type: application/json' \
  --data-binary "@$RESPONSES_REQUEST" \
  'https://yuaiapi.com/v1/responses' | jq -e '.id != null' >/dev/null
```

**Gate:** any failed acceptance item requires an explicit decision to rollback.
Do not label skipped live email or `/v1/responses` checks as passed.

## Phase 9: Rollback

Application rollback is the first response. The migration adds authentication
tables and columns that the baseline application generally ignores, so the old
application can normally run against the additive schema. Do not perform a
destructive database rollback merely to return application traffic to the old
image.

### Proxy-Switch Rollback

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_PROXY_CONFIG='<FILL exact active proxy config path>'
PROD_PROXY_CONFIG_OWNER_UID='<FILL expected numeric owner UID>'
PROD_PROXY_CONFIG_GROUP_GID='<FILL original numeric group GID>'
PROD_PROXY_CONFIG_MODE='<FILL original 3- or 4-digit octal mode>'
PROD_TRAFFIC_CLOSE_SCRIPT='<FILL exact reviewed traffic-close script path>'
PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSE_SCRIPT_SHA256='<FILL exact reviewed traffic-close script SHA-256>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT='<FILL exact reviewed read-only closure-check script path>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256='<FILL exact reviewed closure-check script SHA-256>'
PROD_TRAFFIC_CONTROL_REVIEW_RECORD='<FILL exact peer-review/approval record for both scripts>'
ROLLBACK_CONFIG="$PROD_PROXY_CONFIG.pre-auth-session-20260722"
ROLLBACK_SHA="$ROLLBACK_CONFIG.sha256"
ROLLBACK_STAT="$ROLLBACK_CONFIG.stat"
ROLLBACK_ACL="$ROLLBACK_CONFIG.acl"
ROLLBACK_XATTR="$ROLLBACK_CONFIG.xattr"
ROLLBACK_METADATA_SHA="$ROLLBACK_CONFIG.metadata.sha256"
: "${PROD_PROXY_CONFIG:?ABORT: active proxy config missing}"
[[ "$PROD_PROXY_CONFIG_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: proxy owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_PROXY_CONFIG_GROUP_GID" =~ ^[0-9]+$ ]] || { echo 'ABORT: proxy group GID must be numeric' >&2; exit 1; }
[[ "$PROD_PROXY_CONFIG_MODE" =~ ^[0-7]{3,4}$ ]] || { echo 'ABORT: proxy mode must be octal' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: traffic-close script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: traffic-close script SHA-256 is not 64 lowercase hex' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: closure-check script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: closure-check script SHA-256 is invalid' >&2; exit 1; }
: "${PROD_TRAFFIC_CONTROL_REVIEW_RECORD:?ABORT: traffic-control peer-review record missing}"
[[ "$PROD_TRAFFIC_CONTROL_REVIEW_RECORD" != *'<FILL'* ]] || { echo 'ABORT: traffic-control review record unresolved' >&2; exit 1; }
test -f "$PROD_PROXY_CONFIG" && test -f "$ROLLBACK_CONFIG" && test -f "$ROLLBACK_SHA"
test -f "$ROLLBACK_STAT" && test -f "$ROLLBACK_ACL" && test -f "$ROLLBACK_XATTR" && test -f "$ROLLBACK_METADATA_SHA"
test ! -L "$PROD_PROXY_CONFIG" && test ! -L "$ROLLBACK_CONFIG" || { echo 'ABORT: proxy config symlinks are unsupported' >&2; exit 1; }
test "$PROD_PROXY_CONFIG" = "$(realpath -e -- "$PROD_PROXY_CONFIG")" || { echo 'ABORT: active proxy path must be absolute and canonical' >&2; exit 1; }
test "$(stat -c '%F' "$PROD_PROXY_CONFIG")" = 'regular file'
test "$(stat -c '%F' "$ROLLBACK_CONFIG")" = 'regular file'
for script in "$PROD_TRAFFIC_CLOSE_SCRIPT" "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"; do
  test -f "$script" && test ! -L "$script"
  test "$(stat -c '%a' "$script")" = '700'
  bash -n "$script"
  if grep -Eq '<FILL|TODO|REPLACE_ME' "$script"; then
    echo 'ABORT: traffic-control script contains an unresolved placeholder' >&2
    exit 1
  fi
done
test "$(stat -c '%u' "$PROD_PROXY_CONFIG")" = "$PROD_PROXY_CONFIG_OWNER_UID"
test "$(stat -c '%g' "$PROD_PROXY_CONFIG")" = "$PROD_PROXY_CONFIG_GROUP_GID"
test "$(stat -c '%a' "$PROD_PROXY_CONFIG")" = "$PROD_PROXY_CONFIG_MODE"
test "$(stat -c '%u' "$ROLLBACK_CONFIG")" = "$PROD_PROXY_CONFIG_OWNER_UID"
test "$(stat -c '%a' "$ROLLBACK_CONFIG")" = '600'
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSE_SCRIPT")" = "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID"
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT")" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID"
test "$(sha256sum "$PROD_TRAFFIC_CLOSE_SCRIPT" | awk '{print $1}')" = \
  "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256" || { echo 'ABORT: traffic-close script checksum mismatch' >&2; exit 1; }
test "$(sha256sum "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT" | awk '{print $1}')" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256"
grep -Fxq '# CUTOVER_CONTRACT: close-approved-domain-routes-v1' "$PROD_TRAFFIC_CLOSE_SCRIPT"
grep -Fxq '# CUTOVER_CONTRACT: verify-approved-domain-routes-closed-v1' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
PROXY_DIR="$(dirname "$PROD_PROXY_CONFIG")"
command -v realpath >/dev/null
command -v getfacl >/dev/null
command -v getfattr >/dev/null
(
  cd "$PROXY_DIR"
  sha256sum --check "$(basename "$ROLLBACK_SHA")"
  sha256sum --check "$(basename "$ROLLBACK_METADATA_SHA")"
)
nginx -t -c "$ROLLBACK_CONFIG"

INSTALL_TMP="$(mktemp "$PROXY_DIR/.proxy-rollback-install.tmp.XXXXXX")"
CONVERGE_TMP=''
ACTIVE_STAT="$(cat -- "$ROLLBACK_STAT")"
test "$(stat -c '%F|%u|%g|%a|%C' "$PROD_PROXY_CONFIG")" = "$ACTIVE_STAT"
getfacl -cpn -- "$PROD_PROXY_CONFIG" | cmp --silent - "$ROLLBACK_ACL"
getfattr --absolute-names --dump --match=- -- "$PROD_PROXY_CONFIG" | sed '/^# file:/d' | cmp --silent - "$ROLLBACK_XATTR"
BASELINE_PROXY_SHA256="$(sha256sum "$ROLLBACK_CONFIG" | awk '{print $1}')"

converge_baseline_proxy_on_failure() {
  trap - EXIT INT TERM
  set +e
  recovery_rc=0
  rm -f -- "$INSTALL_TMP"
  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  closed_rc=$?
  if [[ "$close_rc" -ne 0 || "$closed_rc" -ne 0 ]]; then recovery_rc=1; fi
  if [[ -f "$PROD_PROXY_CONFIG" && ! -L "$PROD_PROXY_CONFIG" ]]; then
    active_sha="$(sha256sum "$PROD_PROXY_CONFIG" | awk '{print $1}')"
  else
    active_sha='missing-or-invalid'
    recovery_rc=1
  fi
  if [[ "$active_sha" != "$BASELINE_PROXY_SHA256" && -f "$PROD_PROXY_CONFIG" && ! -L "$PROD_PROXY_CONFIG" ]]; then
    CONVERGE_TMP="$(mktemp "$PROXY_DIR/.proxy-baseline-converge.tmp.XXXXXX")"
    cp --preserve=all "$PROD_PROXY_CONFIG" "$CONVERGE_TMP"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    cat -- "$ROLLBACK_CONFIG" >"$CONVERGE_TMP"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    if [[ "$(sha256sum "$CONVERGE_TMP" | awk '{print $1}')" = "$BASELINE_PROXY_SHA256" ]]; then
      mv -- "$CONVERGE_TMP" "$PROD_PROXY_CONFIG"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    else
      recovery_rc=1
    fi
  fi
  if [[ -f "$PROD_PROXY_CONFIG" && ! -L "$PROD_PROXY_CONFIG" && \
    "$(sha256sum "$PROD_PROXY_CONFIG" | awk '{print $1}')" = "$BASELINE_PROXY_SHA256" ]]; then
    if [[ "$(stat -c '%F|%u|%g|%a|%C' "$PROD_PROXY_CONFIG")" != "$ACTIVE_STAT" ]]; then recovery_rc=1; fi
    getfacl -cpn -- "$PROD_PROXY_CONFIG" | cmp --silent - "$ROLLBACK_ACL"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    getfattr --absolute-names --dump --match=- -- "$PROD_PROXY_CONFIG" | sed '/^# file:/d' | cmp --silent - "$ROLLBACK_XATTR"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    if [[ "$recovery_rc" -eq 0 ]]; then
      nginx -t -c "$PROD_PROXY_CONFIG"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    fi
    if [[ "$recovery_rc" -eq 0 ]]; then
      nginx -s reload -c "$PROD_PROXY_CONFIG"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    fi
  else
    recovery_rc=1
  fi
  rm -f -- "$INSTALL_TMP" "$CONVERGE_TMP"
  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  final_close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  final_closed_rc=$?
  if [[ "$final_close_rc" -ne 0 || "$final_closed_rc" -ne 0 ]]; then recovery_rc=1; fi
  if [[ "$recovery_rc" -ne 0 ]]; then
    echo 'CRITICAL: baseline proxy convergence or traffic closure failed; keep traffic closed and escalate' >&2
  else
    echo 'CRITICAL: rollback step failed; baseline config was reinstalled and traffic remains closed pending manual health confirmation' >&2
  fi
  exit 92
}
trap converge_baseline_proxy_on_failure EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cp --preserve=all "$PROD_PROXY_CONFIG" "$INSTALL_TMP"
cat -- "$ROLLBACK_CONFIG" >"$INSTALL_TMP"
test "$(sha256sum "$INSTALL_TMP" | awk '{print $1}')" = "$BASELINE_PROXY_SHA256"
test "$(stat -c '%F|%u|%g|%a|%C' "$INSTALL_TMP")" = "$ACTIVE_STAT"
getfacl -cpn -- "$INSTALL_TMP" | cmp --silent - "$ROLLBACK_ACL"
getfattr --absolute-names --dump --match=- -- "$INSTALL_TMP" | sed '/^# file:/d' | cmp --silent - "$ROLLBACK_XATTR"
mv -- "$INSTALL_TMP" "$PROD_PROXY_CONFIG"
nginx -t -c "$PROD_PROXY_CONFIG"
nginx -s reload -c "$PROD_PROXY_CONFIG"

curl --fail --silent --show-error 'https://yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
curl --fail --silent --show-error 'https://global.yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
docker stop --time 30 'new-api-auth-session-candidate-20260722'
trap - EXIT INT TERM
```

Confirm traffic reaches the unchanged old container/configuration and image
`newapi:production-console-20260722-739cb2775`. Keep the stopped candidate for
forensics until the incident owner approves removal.

### Container-Replacement Rollback

```bash
set -Eeuo pipefail
set +x
umask 077
PROD_CONTAINER='<FILL exact production application container name>'
PROD_TRAFFIC_CLOSE_SCRIPT='<FILL exact reviewed traffic-close script path>'
PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSE_SCRIPT_SHA256='<FILL exact reviewed traffic-close script SHA-256>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT='<FILL exact reviewed read-only closure-check script path>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID='<FILL expected numeric owner UID>'
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256='<FILL exact reviewed closure-check script SHA-256>'
PROD_TRAFFIC_CONTROL_REVIEW_RECORD='<FILL exact peer-review/approval record for both scripts>'
PARKED_CONTAINER='new-api-production-pre-auth-session-20260722'
FAILED_CONTAINER='new-api-auth-session-failed-20260722'
CANDIDATE_IMAGE_ID='sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee'
: "${PROD_CONTAINER:?ABORT: production container name missing}"
: "${PROD_TRAFFIC_CLOSE_SCRIPT:?ABORT: traffic-close script path missing}"
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: traffic-close script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: traffic-close script SHA-256 is not 64 lowercase hex' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID" =~ ^[0-9]+$ ]] || { echo 'ABORT: closure-check script owner UID must be numeric' >&2; exit 1; }
[[ "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256" =~ ^[0-9a-f]{64}$ ]] || { echo 'ABORT: closure-check script SHA-256 is invalid' >&2; exit 1; }
: "${PROD_TRAFFIC_CONTROL_REVIEW_RECORD:?ABORT: traffic-control peer-review record missing}"
[[ "$PROD_TRAFFIC_CONTROL_REVIEW_RECORD" != *'<FILL'* ]] || { echo 'ABORT: traffic-control review record unresolved' >&2; exit 1; }
for script in "$PROD_TRAFFIC_CLOSE_SCRIPT" "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"; do
  test -f "$script" && test ! -L "$script"
  test "$(stat -c '%a' "$script")" = '700'
  bash -n "$script"
  if grep -Eq '<FILL|TODO|REPLACE_ME' "$script"; then
    echo 'ABORT: traffic-control script contains an unresolved placeholder' >&2
    exit 1
  fi
done
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSE_SCRIPT")" = "$PROD_TRAFFIC_CLOSE_SCRIPT_OWNER_UID"
test "$(stat -c '%u' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT")" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_OWNER_UID"
test "$(sha256sum "$PROD_TRAFFIC_CLOSE_SCRIPT" | awk '{print $1}')" = \
  "$PROD_TRAFFIC_CLOSE_SCRIPT_SHA256" || { echo 'ABORT: traffic-close script checksum mismatch' >&2; exit 1; }
test "$(sha256sum "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT" | awk '{print $1}')" = "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256"
grep -Fxq '# CUTOVER_CONTRACT: close-approved-domain-routes-v1' "$PROD_TRAFFIC_CLOSE_SCRIPT"
grep -Fxq '# CUTOVER_CONTRACT: verify-approved-domain-routes-closed-v1' "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
BASELINE_IMAGE_ID="$(docker image inspect --format '{{.Id}}' 'newapi:production-console-20260722-739cb2775')"
[[ "$BASELINE_IMAGE_ID" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo 'ABORT: baseline image ID is invalid' >&2; exit 1; }
docker container inspect "$PROD_CONTAINER" >/dev/null
docker container inspect "$PARKED_CONTAINER" >/dev/null
docker container inspect "$FAILED_CONTAINER" >/dev/null 2>&1 && { echo 'ABORT: failed container name already exists' >&2; exit 1; }
test "$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")" = "$CANDIDATE_IMAGE_ID"
test "$(docker container inspect --format '{{.Config.Image}}' "$PROD_CONTAINER")" = \
  "$CANDIDATE_IMAGE_ID"
test "$(docker container inspect --format '{{.Image}}' "$PARKED_CONTAINER")" = "$BASELINE_IMAGE_ID"
test "$(docker container inspect --format '{{.Config.Image}}' "$PARKED_CONTAINER")" = \
  'newapi:production-console-20260722-739cb2775'

converge_baseline_container_on_failure() {
  trap - EXIT INT TERM
  set +e
  recovery_rc=0
  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  closed_rc=$?
  if [[ "$close_rc" -ne 0 || "$closed_rc" -ne 0 ]]; then recovery_rc=1; fi

  if docker container inspect "$PROD_CONTAINER" >/dev/null 2>&1; then
    prod_image="$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")"
    if [[ "$prod_image" = "$CANDIDATE_IMAGE_ID" ]]; then
      if [[ "$(docker container inspect --format '{{.State.Running}}' "$PROD_CONTAINER")" = 'true' ]]; then
        docker stop --time 30 "$PROD_CONTAINER"
        if [[ $? -ne 0 ]]; then recovery_rc=1; fi
      fi
      if ! docker container inspect "$FAILED_CONTAINER" >/dev/null 2>&1; then
        docker rename "$PROD_CONTAINER" "$FAILED_CONTAINER"
        if [[ $? -ne 0 ]]; then recovery_rc=1; fi
      else
        recovery_rc=1
      fi
    elif [[ "$prod_image" != "$BASELINE_IMAGE_ID" ]]; then
      recovery_rc=1
    fi
  fi

  if docker container inspect "$FAILED_CONTAINER" >/dev/null 2>&1 && \
    [[ "$(docker container inspect --format '{{.State.Running}}' "$FAILED_CONTAINER")" = 'true' ]]; then
    docker stop --time 30 "$FAILED_CONTAINER"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
  fi

  if ! docker container inspect "$PROD_CONTAINER" >/dev/null 2>&1; then
    if docker container inspect "$PARKED_CONTAINER" >/dev/null 2>&1 && \
      [[ "$(docker container inspect --format '{{.Image}}' "$PARKED_CONTAINER")" = "$BASELINE_IMAGE_ID" ]]; then
      docker rename "$PARKED_CONTAINER" "$PROD_CONTAINER"
      if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    else
      recovery_rc=1
    fi
  fi

  if docker container inspect "$PROD_CONTAINER" >/dev/null 2>&1 && \
    [[ "$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")" = "$BASELINE_IMAGE_ID" ]]; then
    docker start "$PROD_CONTAINER"
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
    for attempt in $(seq 1 30); do
      if curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' | \
        jq -e '.success == true' >/dev/null; then
        break
      fi
      sleep 2
    done
    curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' | \
      jq -e '.success == true' >/dev/null
    if [[ $? -ne 0 ]]; then recovery_rc=1; fi
  else
    recovery_rc=1
  fi

  "$PROD_TRAFFIC_CLOSE_SCRIPT"
  final_close_rc=$?
  "$PROD_TRAFFIC_CLOSED_CHECK_SCRIPT"
  final_closed_rc=$?
  if [[ "$final_close_rc" -ne 0 || "$final_closed_rc" -ne 0 ]]; then recovery_rc=1; fi
  if [[ "$recovery_rc" -ne 0 ]]; then
    echo 'CRITICAL: baseline container could not be made healthy; candidate remains stopped or parked, keep traffic closed and escalate' >&2
  else
    echo 'CRITICAL: rollback step failed; baseline container is healthy but traffic remains closed pending manual confirmation' >&2
  fi
  exit 93
}
trap converge_baseline_container_on_failure EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker stop --time 30 "$PROD_CONTAINER"
docker rename "$PROD_CONTAINER" "$FAILED_CONTAINER"
docker rename "$PARKED_CONTAINER" "$PROD_CONTAINER"
test "$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")" = "$BASELINE_IMAGE_ID"
docker start "$PROD_CONTAINER"

test "$(docker container inspect --format '{{.Image}}' "$PROD_CONTAINER")" = "$BASELINE_IMAGE_ID" || { echo 'ABORT: restored container immutable image mismatch' >&2; exit 1; }
test "$(docker container inspect --format '{{.Config.Image}}' "$PROD_CONTAINER")" = \
  'newapi:production-console-20260722-739cb2775' || { echo 'ABORT: restored container image mismatch' >&2; exit 1; }
curl --fail --silent --show-error 'http://127.0.0.1:3000/api/status' | jq -e '.success == true' >/dev/null
curl --fail --silent --show-error 'https://yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
curl --fail --silent --show-error 'https://global.yuaiapi.com/api/status' | jq -e '.success == true' >/dev/null
trap - EXIT INT TERM
```

This restores the exact old container, including its captured configuration,
instead of attempting to recreate it during an incident.

### Last Resort: Database Restore

Restore the pre-switch database only when a proven non-additive/incompatible
schema or data change must be reversed and application rollback alone cannot
recover service. Database restore discards post-backup writes and is therefore a
separate, destructive approval gate.

Before restoring:

1. Obtain explicit database-restore approval and name the accepted data-loss
   interval.
2. Stop public traffic and all application/background writers.
3. Stop the candidate and old application containers.
4. Re-run `sha256sum --check` against `PROD_DB_BACKUP_PATH` and compare the exact
   value with `PROD_DB_BACKUP_SHA256`.
5. Restore with the database owner's reviewed dialect-specific procedure into a
   new database first where feasible, then atomically repoint the baseline app.
6. Start only the baseline image and rerun health, login, relay, and integrity
   checks before reopening traffic.

Do not run ad hoc `DROP TABLE`, `ALTER TABLE`, `DELETE`, `git reset`, or volume
replacement commands. For MySQL use the verified SQL dump with the reviewed
`mysql` restore procedure; for PostgreSQL use `pg_restore --exit-on-error`; for
SQLite, with every writer stopped, restore the verified backup to a new file and
atomically replace/repoint the database path. Preserve the failed database for
forensics.

## Candidate Evidence and Remaining Gaps

Evidence recorded for local candidate image
`newapi:auth-session-candidate-20260722` at source provenance boundary
`0eabc2e884550b11536fffb4637bcdfe644101d1` and image ID
`sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee`:

- A no-cache build used a `23.92MB` Docker context for the checked-in default
  and classic frontend sources. The context was smaller than each excluded
  local artifact alone: `.local-preview/new-api-preview.exe` (about 143MB),
  `test-results/newapi-bf3da736-source.tar` (about 26MB), and
  `production-b5514ebe1.tar` (about 26MB). The `.dockerignore` also excludes
  `output/local-experiments/`, `output/imagegen/`, `web/experimental/`,
  `local-ui/`, and future `.candidate-auth-audit-*` directories.
- Fresh SQLite startup returned `/api/status` HTTP `200`; setup completed; a
  restart against the same database logged migration initialization and again
  returned HTTP `200` from `/api/status`.
- Dashboard login, authenticated self, refresh, and authenticated self again
  each returned HTTP `200`; the session SID remained stable across refresh.
  A restart against the same SQLite database again returned status and refresh
  HTTP `200` with the same SID.
- Personal-access-token email binding and WeChat binding each returned `401`
  before attempting their external binding paths.
- A disposable relay token returned HTTP `200` from `/v1/models` (the fresh
  database correctly exposed an empty model list).
- The final image payload and entrypoint were inspected. It contains no local
  preview, production archive, experiment, build, or frontend-source paths.
- The disposable candidate container was stopped and removed. Its ignored local
  audit directory remains only for local evidence and is not a deployment input.

Documented gaps that must not be represented as production validation:

- No sanitized production database was available to the local candidate.
- Redis was not covered locally.
- SMTP credentials and a real delivery path were not covered locally.
- Real upstream credentials and upstream requests were not covered locally.
- A real `/v1/responses` request was not exercised live.
- Email delivery was not exercised live.
- The image runs as root. This inherited hardening gap is non-blocking for this
  migration and remains follow-up work.

These gaps are why approved production backup, no-traffic candidate health,
database-specific migration checks, live email policy/delivery confirmation, and
minimal relay checks are explicit cutover gates.

## Cutover Record

Complete without secrets and attach only sanitized evidence:

```text
APPROVAL_RECORD=<reference>
PROD_HOST=<exact host>
PROD_CONTAINER=<exact old container>
EVIDENCE_DIR=<exact unique evidence directory>
RUNBOOK_COMMIT=<exact final reviewed commit; current HEAD must equal this value>
BASELINE_IMAGE_ID=<verified image ID>
CANDIDATE_IMAGE_ID=sha256:741e84e6d7af68cc3fecf2b7de23e7be6c6443136139399acb906288f89c44ee
CANDIDATE_ARCHIVE_SHA256=<mandatory after approved packaging>
PROD_DB_DIALECT=<mysql|postgresql|sqlite>
PROD_DB_BACKUP_PATH=<mandatory after approved backup>
PROD_DB_BACKUP_SHA256=<mandatory before migration/switch>
PROD_PROXY_CONFIG_OWNER_UID=<original numeric UID when proxy switching>
PROD_PROXY_CONFIG_GROUP_GID=<original numeric GID when proxy switching>
PROD_PROXY_CONFIG_MODE=<original octal mode when proxy switching>
PROD_TRAFFIC_CLOSE_SCRIPT_SHA256=<mandatory before rollback>
PROD_TRAFFIC_CLOSED_CHECK_SCRIPT_SHA256=<mandatory before any recovery/rollback>
PROD_TRAFFIC_CONTROL_REVIEW_RECORD=<mandatory peer-review/approval record>
BACKUP_VERIFIED_BY=<second operator>
FIRST_CANDIDATE_HEALTH_AT_UTC=<timestamp>
RESTART_HEALTH_AT_UTC=<timestamp>
TRAFFIC_SWITCH_AT_UTC=<timestamp>
CHECKS_AT_PLUS_1M=<pass|rollback plus sanitized evidence reference>
CHECKS_AT_PLUS_5M=<pass|rollback plus sanitized evidence reference>
CHECKS_AT_PLUS_10M=<pass|rollback plus sanitized evidence reference>
CHECKS_AT_PLUS_15M=<pass|rollback plus sanitized evidence reference>
APPLICATION_MATRIX=<pass|rollback plus sanitized evidence reference>
RELAY_MODELS=<pass|rollback plus sanitized evidence reference>
RELAY_RESPONSES=<pass|not-approved|rollback plus sanitized evidence reference>
EMAIL_DELIVERY=<pass|not-approved|rollback plus sanitized evidence reference>
FINAL_DECISION=<complete|rollback>
DECIDED_BY=<identity>
DECIDED_AT_UTC=<timestamp>
```

The cutover is complete only after the observation window and every approved
acceptance check pass. Otherwise restore old traffic immediately and retain all
rollback evidence.
