# YuCore Production Cross-Server Migration Runbook

## Scope, Safety Boundary, And Authorization

This runbook moves the accepted production application, MySQL, Redis, runtime
files, Caddy state, and origin traffic from the current server to one new
server. It does not change channels, accounts, groups, mappings, prices,
Cloudflare policy, unrelated DNS records, or the experimental UI.

Every command in this document is an execution-time instruction. The runbook
is inert by default: do not run even a read-only production command until the
user supplies the new-server coordinates and explicitly authorizes production
execution. Do not enable shell tracing. Stop if any assertion fails; never
replace an expected value merely to pass a guard.

The source container names are exact and must not be discovered by image:

```text
Application: newapi
MySQL:       newapi-mysql
Redis:       newapi-redis
Caddy:       yuapi-caddy
```

The source address below is the approved inventory value. The new address,
access, accepted commit, and authorization are supplied interactively and are
not committed:

```bash
set -euo pipefail
set +x
umask 077

export OLD_HOST='156.239.252.210'
export OLD_SSH_PORT='46276'
export OLD_USER='root'
: "${NEW_HOST:?set NEW_HOST}"
: "${NEW_SSH_PORT:?set NEW_SSH_PORT}"
: "${NEW_USER:?set NEW_USER}"
: "${NEW_IPV6_POLICY:?set enabled or disabled from reviewed target plan}"
case "$NEW_IPV6_POLICY" in
  enabled) : "${NEW_IPV6_HOST:?set reviewed new origin IPv6 on CONTROL}" ;;
  disabled) ;;
  *) exit 1 ;;
esac
: "${ACCEPTED_COMMIT:?set ACCEPTED_COMMIT}"
export ACCEPTED_IMAGE="newapi:yucore-$ACCEPTED_COMMIT"
export MAINTENANCE_CONFIRMATION="${MAINTENANCE_CONFIRMATION:-}"
export PRODUCTION_AUTHORIZATION="${PRODUCTION_AUTHORIZATION:-}"

test "$(git rev-parse HEAD)" = "$ACCEPTED_COMMIT"
test "$(git rev-parse "$ACCEPTED_COMMIT^{commit}")" = "$ACCEPTED_COMMIT"
export AUTH_GUARD_DIR="$(mktemp -d)"
trap 'find "$AUTH_GUARD_DIR" -xdev -depth -delete' EXIT
git show "$ACCEPTED_COMMIT:scripts/production/yucore_migration_guard.py" \
  >"$AUTH_GUARD_DIR/yucore_migration_guard.py"
test -s "$AUTH_GUARD_DIR/yucore_migration_guard.py"
sha256sum "$AUTH_GUARD_DIR/yucore_migration_guard.py" \
  >"$AUTH_GUARD_DIR/yucore_migration_guard.py.sha256"
sha256sum --check "$AUTH_GUARD_DIR/yucore_migration_guard.py.sha256"
test "$PRODUCTION_AUTHORIZATION" = 'USER-AUTHORIZED-PRODUCTION-EXECUTION'
python3 "$AUTH_GUARD_DIR/yucore_migration_guard.py" confirm \
  --new-host "$NEW_HOST" \
  --confirmation "$MAINTENANCE_CONFIRMATION"
```

The normal value of `MAINTENANCE_CONFIRMATION` is empty, so the gate fails
closed. The user must explicitly authorize execution and enter the exact
`MIGRATE-YUCORE-PRODUCTION` confirmation at the start of the approved window.
Passing the gate authorizes only this runbook, not cleanup, pruning, unrelated
changes, or a later retry after a failed guard.

Use three named shells throughout the runbook:

- `CONTROL`: a trusted operator host containing the accepted Git worktree;
- `OLD`: an SSH shell on the old production server;
- `NEW`: an SSH shell on the new server.

Open production shells only after the authorization gate:

```bash
ssh -A -p "$OLD_SSH_PORT" "$OLD_USER@$OLD_HOST"
ssh -A -p "$NEW_SSH_PORT" "$NEW_USER@$NEW_HOST"
```

Agent forwarding holds the temporary new-server SSH credential on `CONTROL`.
Do not copy a private key to either server. Plain SSH does not propagate shell
variables. Immediately after login, run this public, credential-free context
bootstrap independently in both `OLD` and `NEW`. It prompts for every shared
remote value when absent and exports only after all values validate:

```bash
set -euo pipefail
set +x
umask 077

prompt_public_value() {
  variable_name="$1"
  prompt_text="$2"
  eval "current_value=\${$variable_name-}"
  if test -z "$current_value"; then
    read -rp "$prompt_text" current_value
  fi
  test -n "$current_value"
  printf -v "$variable_name" '%s' "$current_value"
  export "$variable_name"
}

prompt_public_value NEW_HOST 'New origin IPv4: '
prompt_public_value NEW_SSH_PORT 'New SSH port: '
prompt_public_value NEW_USER 'New SSH user: '
prompt_public_value NEW_IPV6_POLICY 'New IPv6 policy (enabled/disabled): '
prompt_public_value ACCEPTED_COMMIT 'Accepted 40-character commit: '
prompt_public_value MIGRATION_ID 'Reviewed UTC migration ID: '
prompt_public_value APPROVED_OLD_REDIS_MOUNT_SOURCE 'Approved old Redis mount source: '
prompt_public_value APPROVED_NEW_REDIS_MOUNT_SOURCE 'Approved new Redis bind source: '

case "$NEW_IPV6_POLICY" in
  enabled)
    prompt_public_value NEW_IPV6_HOST 'Reviewed new origin IPv6: '
    ;;
  disabled)
    unset NEW_IPV6_HOST
    ;;
  *) exit 1 ;;
esac
printf '%s' "$NEW_HOST" | python3 -c \
  'import ipaddress,sys; value=sys.stdin.read(); assert ipaddress.ip_address(value).version == 4'
if test "$NEW_IPV6_POLICY" = enabled; then
  printf '%s' "$NEW_IPV6_HOST" | python3 -c \
    'import ipaddress,sys; value=sys.stdin.read(); assert ipaddress.ip_address(value).version == 6'
fi
case "$NEW_SSH_PORT" in ''|*[!0-9]*) exit 1 ;; esac
test "$NEW_SSH_PORT" -ge 1
test "$NEW_SSH_PORT" -le 65535
printf '%s\n' "$ACCEPTED_COMMIT" | grep -Eq '^[0-9a-f]{40}$'
printf '%s\n' "$MIGRATION_ID" | grep -Eq '^[0-9]{8}T[0-9]{6}Z$'
case "$APPROVED_OLD_REDIS_MOUNT_SOURCE" in /*) ;; *) exit 1 ;; esac
case "$APPROVED_NEW_REDIS_MOUNT_SOURCE" in /*) ;; *) exit 1 ;; esac

export EXPECTED_CAPTURE_STATE_SHA256='c16b675f2341ffcebf715d835d5a1a01c09b1d5c6ed569d0753773736d044abf'
export EXPECTED_COMPARE_STATE_SHA256='37bd4a13b8991bfb0dfef7382036f44c881c1b1b59fc0a47ab3cc4fb90e1d3b6'
export EXPECTED_EXPORT_MYSQL_SHA256='eecf0bc1d7b97f2e2a0ae9aa1320a13b61ff1dd11954249c8896d3562fdc9d23'
export EXPECTED_RESTORE_MYSQL_SHA256='6608997582169b43546a9a68c2b343172178124c6003cf05ba3634f8095b476a'
export HELPER_CONTEXT_BOOTSTRAPPED='accepted-literal-hashes-v1'

verify_migration_helper() {
  helper_path="$1"
  expected_sha256="$2"
  test "$HELPER_CONTEXT_BOOTSTRAPPED" = 'accepted-literal-hashes-v1'
  test -f "$helper_path"
  test "$(sha256sum "$helper_path" | cut -d' ' -f1)" = "$expected_sha256"
}
verify_migration_helpers() {
  test "$HELPER_CONTEXT_BOOTSTRAPPED" = 'accepted-literal-hashes-v1'
  verify_migration_helper /opt/newapi/releases/capture-state-manifest \
    "$EXPECTED_CAPTURE_STATE_SHA256"
  verify_migration_helper /opt/newapi/releases/compare-state-snapshots \
    "$EXPECTED_COMPARE_STATE_SHA256"
  verify_migration_helper /opt/newapi/releases/export-mysql-snapshot \
    "$EXPECTED_EXPORT_MYSQL_SHA256"
  verify_migration_helper /opt/newapi/releases/restore-mysql-snapshot \
    "$EXPECTED_RESTORE_MYSQL_SHA256"
}
helper_count=0
for helper_path in \
  /opt/newapi/releases/capture-state-manifest \
  /opt/newapi/releases/compare-state-snapshots \
  /opt/newapi/releases/export-mysql-snapshot \
  /opt/newapi/releases/restore-mysql-snapshot; do
  if test -e "$helper_path"; then helper_count=$((helper_count + 1)); fi
done
case "$helper_count" in
  0) ;;
  4) verify_migration_helpers ;;
  *) exit 1 ;;
esac
```

Do not continue in a remote shell that has not completed this block. Re-run it
after reconnecting; no later section may rely on a variable from a prior SSH
process.

Use this execution order: build the immutable archive locally; after production
authorization, transfer the archive and reviewed guard; run both complete
read-only preflights; transfer deployment state; pre-stage and accept privately;
then enter the maintenance mutation gate. A section may not be skipped because
a similar command passed in an earlier window.

## Execution Variables

Set these public paths and names in both remote shells. They contain no
credentials. Before opening either remote shell, a human must review the old
audited Compose/state capture and the new rendered Compose plan, then type the
two approved absolute Redis sources into `APPROVED_OLD_REDIS_MOUNT_SOURCE` and
`APPROVED_NEW_REDIS_MOUNT_SOURCE`. Never populate either variable with
`docker inspect`, command substitution, or a value generated by this runbook.
The commands below record and hash the human-approved literals before target
containers or data paths are created:

```bash
set -euo pipefail
set +x
umask 077

export OLD_HOST='156.239.252.210'
export OLD_SSH_PORT='46276'
export OLD_USER='root'
: "${NEW_HOST:?set NEW_HOST}"
: "${NEW_SSH_PORT:?set NEW_SSH_PORT}"
: "${NEW_USER:?set NEW_USER}"
: "${NEW_IPV6_POLICY:?set NEW_IPV6_POLICY in this remote shell}"
case "$NEW_IPV6_POLICY" in
  enabled) : "${NEW_IPV6_HOST:?set NEW_IPV6_HOST in this remote shell}" ;;
  disabled) ;;
  *) exit 1 ;;
esac
: "${APPROVED_OLD_REDIS_MOUNT_SOURCE:?enter the separately reviewed old Redis mount source}"
: "${APPROVED_NEW_REDIS_MOUNT_SOURCE:?enter the separately reviewed new Redis bind source}"
export APP_CONTAINER='newapi'
export MYSQL_CONTAINER='newapi-mysql'
export REDIS_CONTAINER='newapi-redis'
export CADDY_CONTAINER='yuapi-caddy'
export MAINTENANCE_CONTAINER='yucore-migration-maintenance'
export COMPOSE_FILE='/opt/newapi/docker-compose.yml'
export ENV_FILE='/opt/newapi/.env'
export EDGE_COMPOSE='/opt/edge/docker-compose.yml'
export CADDY_FILE='/opt/edge/Caddyfile'
export RELEASE_ROOT='/opt/newapi/releases'
export MIGRATION_ROOT='/opt/newapi/migration'
export APP_DATA='/opt/newapi/data'
export APP_SERVICE='newapi'
export MYSQL_SERVICE='mysql'
export REDIS_SERVICE='redis'
export ACCEPTED_COMMIT="${ACCEPTED_COMMIT:?set ACCEPTED_COMMIT}"
export ACCEPTED_IMAGE="newapi:yucore-$ACCEPTED_COMMIT"
: "${MIGRATION_ID:?set the one CONTROL-generated UTC migration ID}"
export RUN_DIR="$MIGRATION_ROOT/$MIGRATION_ID"
export BRIDGE_BOUNDARY_MARKER="$RUN_DIR/evidence/new-write-authority-boundary.json"
export BRIDGE_BOUNDARY_MARKER_SHA256="$BRIDGE_BOUNDARY_MARKER.sha256"

require_remote_context() {
  : "${NEW_HOST:?missing NEW_HOST in current remote shell}"
  : "${NEW_SSH_PORT:?missing NEW_SSH_PORT in current remote shell}"
  : "${NEW_USER:?missing NEW_USER in current remote shell}"
  : "${NEW_IPV6_POLICY:?missing NEW_IPV6_POLICY in current remote shell}"
  : "${ACCEPTED_COMMIT:?missing ACCEPTED_COMMIT in current remote shell}"
  : "${MIGRATION_ID:?missing MIGRATION_ID in current remote shell}"
  : "${APPROVED_OLD_REDIS_MOUNT_SOURCE:?missing approved old Redis source}"
  : "${APPROVED_NEW_REDIS_MOUNT_SOURCE:?missing approved new Redis source}"
  case "$NEW_IPV6_POLICY" in
    enabled) : "${NEW_IPV6_HOST:?missing NEW_IPV6_HOST in current remote shell}" ;;
    disabled) ;;
    *) return 1 ;;
  esac
}
require_remote_context
```

If the Compose service labels in the old-host preflight do not equal the three
service values above, stop and record the reviewed labels before opening a new
execution window. Container names remain the exact four names listed above.
Generate `MIGRATION_ID` once on `CONTROL` with
`date -u +%Y%m%dT%H%M%SZ`, then enter that exact non-secret value in both
remote shells.

## Old Server Read-Only Preflight

Run this complete block in `OLD`. It changes no production state:

```bash
set -euo pipefail
set +x

for command_name in bash docker curl jq python3 tar gzip sha256sum rsync ssh scp openssl \
  lscpu lsblk findmnt ss timedatectl systemctl; do
  command -v "$command_name" >/dev/null
done

uname -a
cat /etc/os-release
lscpu
free -h
lsblk -e7 -o NAME,MODEL,SERIAL,SIZE,TYPE,FSTYPE,MOUNTPOINTS
findmnt -R /opt
df -hT / /opt
cat /proc/mdstat
command -v mdadm >/dev/null && mdadm --detail --scan
command -v nvme >/dev/null && nvme list
timedatectl status
timedatectl show -p NTPSynchronized --value | grep -Fx true
systemctl is-system-running | grep -E '^(running|degraded)$'
uptime

docker version
docker compose version
docker info --format '{{json .}}' | jq '{Architecture,OSType,ServerVersion,Driver,NCPU,MemTotal,DockerRootDir}'
test "$(docker info --format '{{.Architecture}}')" = 'x86_64'

for container_name in newapi newapi-mysql newapi-redis yuapi-caddy; do
  test "$(docker inspect --format '{{.State.Status}}' "$container_name")" = running
  docker inspect "$container_name" --format \
    'name={{.Name}} image={{.Config.Image}} id={{.Image}} restarts={{.RestartCount}}'
  docker inspect "$container_name" --format '{{json .Mounts}}' | \
    jq '[.[] | {Type,Source,Destination,RW}]'
  docker inspect "$container_name" --format '{{json .NetworkSettings.Networks}}' | \
    jq 'keys'
done

install -d -m 0700 "$RUN_DIR/evidence"
case "$APPROVED_OLD_REDIS_MOUNT_SOURCE" in
  /|/opt|/opt/newapi|/opt/newapi/data|/var/lib/docker|/var/lib/docker/volumes) exit 1 ;;
esac
printf '%s\t%s\n' '/data' "$APPROVED_OLD_REDIS_MOUNT_SOURCE" \
  >"$RUN_DIR/evidence/old-redis-mount.tsv"
sha256sum "$RUN_DIR/evidence/old-redis-mount.tsv" \
  >"$RUN_DIR/evidence/old-redis-mount.tsv.sha256"
chmod 0600 "$RUN_DIR/evidence/old-redis-mount.tsv" \
  "$RUN_DIR/evidence/old-redis-mount.tsv.sha256"

docker compose -f "$COMPOSE_FILE" config --format json \
  >"$RUN_DIR/evidence/old-compose-reviewed.json"
mapfile -t OLD_REDIS_PLAN < <(jq -er '
  [.services.redis.volumes[] | select(.target == "/data")] |
  if length == 1 then .[0] | [.type,.source] | @tsv
  else error("expected one planned Redis /data mount") end' \
  "$RUN_DIR/evidence/old-compose-reviewed.json")
IFS=$'\t' read -r OLD_REDIS_PLAN_TYPE OLD_REDIS_PLAN_SOURCE \
  <<<"${OLD_REDIS_PLAN[0]}"
test "$(docker inspect newapi-redis --format '{{json .Mounts}}' | jq -r \
  '[.[] | select(.Destination == "/data")] | length')" = 1
export CURRENT_OLD_REDIS_MOUNT_SOURCE="$(docker inspect newapi-redis \
  --format '{{json .Mounts}}' | jq -er \
  '[.[] | select(.Destination == "/data")][0].Source')"
test "$CURRENT_OLD_REDIS_MOUNT_SOURCE" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
if test "$OLD_REDIS_PLAN_TYPE" = bind; then
  test "$(realpath -m "$OLD_REDIS_PLAN_SOURCE")" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
elif test "$OLD_REDIS_PLAN_TYPE" = volume; then
  OLD_REDIS_PLAN_VOLUME="$(jq -er --arg source "$OLD_REDIS_PLAN_SOURCE" \
    '.volumes[$source].name // $source' "$RUN_DIR/evidence/old-compose-reviewed.json")"
  test "$(docker inspect newapi-redis --format '{{json .Mounts}}' | jq -er \
    '[.[] | select(.Destination == "/data")][0].Name')" = "$OLD_REDIS_PLAN_VOLUME"
  test "$(docker volume inspect "$OLD_REDIS_PLAN_VOLUME" --format '{{.Mountpoint}}')" = \
    "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
else
  exit 1
fi
printf '%s\n' "$APPROVED_OLD_REDIS_MOUNT_SOURCE" \
  >"$RUN_DIR/evidence/old-redis-planned-source"
sha256sum "$RUN_DIR/evidence/old-redis-planned-source" \
  >"$RUN_DIR/evidence/old-redis-planned-source.sha256"
sha256sum "$RUN_DIR/evidence/old-compose-reviewed.json" \
  >"$RUN_DIR/evidence/old-compose-reviewed.json.sha256"

test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.service"}}' newapi)" = newapi
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.service"}}' newapi-mysql)" = mysql
test "$(docker inspect --format '{{index .Config.Labels "com.docker.compose.service"}}' newapi-redis)" = redis
docker inspect newapi --format '{{range .Config.Env}}{{println .}}{{end}}' | \
  sed 's/=.*//' | LC_ALL=C sort -u

test -f /opt/newapi/docker-compose.yml
test -f /opt/newapi/.env
test -f /opt/edge/docker-compose.yml
test -f /opt/edge/Caddyfile
docker compose -f /opt/newapi/docker-compose.yml config --quiet
docker compose -f /opt/edge/docker-compose.yml config --quiet
python3 /opt/newapi/releases/yucore_migration_guard.py runtime-preflight \
  --container newapi \
  --compose-file /opt/newapi/docker-compose.yml \
  --service newapi

docker exec newapi-mysql sh -lc '
  set -eu
  export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
  mysql -uroot -NBe "SELECT VERSION(), @@version_comment, @@binlog_format, @@gtid_mode;"
  mysql -uroot -NBe "
    SELECT table_schema, COUNT(*),
           ROUND(SUM(data_length + index_length) / 1024 / 1024, 2)
    FROM information_schema.tables
    WHERE table_schema = DATABASE()
    GROUP BY table_schema;" "$MYSQL_DATABASE"
  mysql -uroot -NBe "
    SELECT table_name, table_rows,
           ROUND((data_length + index_length) / 1024 / 1024, 2)
    FROM information_schema.tables
    WHERE table_schema = DATABASE()
    ORDER BY data_length + index_length DESC;" "$MYSQL_DATABASE"
'

docker exec newapi-redis sh -lc '
  set -eu
  if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
  redis-cli --no-auth-warning INFO server | grep -E "^(redis_version|arch_bits):"
  redis-cli --no-auth-warning INFO persistence | grep -E \
    "^(loading|rdb_bgsave_in_progress|rdb_last_bgsave_status|aof_enabled|aof_rewrite_in_progress|aof_last_bgrewrite_status):"
  redis-cli --no-auth-warning DBSIZE
'

du -sh /opt/newapi/data /opt/newapi /opt/edge
find /opt/newapi/data -xdev -type f | wc -l
systemctl list-timers --all --no-pager | grep -Ei 'newapi|backup'
systemctl list-unit-files --no-pager | grep -Ei 'newapi.*backup|backup.*newapi'
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile

ss -lntup
command -v nft >/dev/null && nft list ruleset
command -v ufw >/dev/null && ufw status verbose
getent ahostsv4 yuaiapi.com
getent ahostsv4 api.yuaiapi.com
getent ahostsv4 global.yuaiapi.com
getent ahostsv4 vip.yuaiapi.com

for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error --connect-timeout 5 --max-time 20 \
    "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

If `/opt/newapi/releases/yucore_migration_guard.py` is absent, stop. Transfer
the reviewed guard only after authorization, then restart this preflight from
the first command. Never replace the exact container names with an image scan.

## New Server Read-Only Preflight

The provider firewall must deny public access to 80 and 443 during staging and
allow SSH only from the operator address. Later, temporarily allow 80 and 443
from the old server address for private acceptance and stale-origin bridging.
Run in `NEW` before copying any secret:

```bash
set -euo pipefail
set +x

for command_name in bash docker curl jq python3 tar gzip sha256sum rsync ssh scp openssl \
  lscpu lsblk findmnt ss timedatectl systemctl; do
  command -v "$command_name" >/dev/null
done
command -v nvme >/dev/null
command -v smartctl >/dev/null
command -v mdadm >/dev/null
command -v ufw >/dev/null

uname -a
cat /etc/os-release
lscpu
free -h
lsblk -e7 -o NAME,MODEL,SERIAL,SIZE,TYPE,FSTYPE,MOUNTPOINTS
nvme list
for device in /dev/nvme*n1; do
  test -b "$device" || continue
  nvme smart-log "$device"
  smartctl -x "$device"
done
cat /proc/mdstat
mdadm --detail --scan
findmnt -R /opt
df -hT / /opt
timedatectl status
timedatectl show -p NTPSynchronized --value | grep -Fx true
systemctl is-system-running | grep -E '^(running|degraded)$'

docker version
docker compose version
docker info --format '{{json .}}' | jq '{Architecture,OSType,ServerVersion,Driver,NCPU,MemTotal,DockerRootDir}'
test "$(docker info --format '{{.Architecture}}')" = 'x86_64'

for reserved_name in newapi newapi-mysql newapi-redis yuapi-caddy \
  yucore-migration-maintenance; do
  test -z "$(docker ps -aq --filter "name=^/${reserved_name}$")"
done
test -z "$(ss -H -lnt '( sport = :80 or sport = :443 or sport = :3001 )')"

ss -lntup
command -v nft >/dev/null && nft list ruleset
command -v ufw >/dev/null && ufw status verbose
ip -brief address
ip route
getent ahostsv4 yuaiapi.com
getent ahostsv4 api.yuaiapi.com
getent ahostsv4 global.yuaiapi.com
getent ahostsv4 vip.yuaiapi.com
```

Compare the CPU model, socket/core topology, memory, NVMe model/serial/health,
mirroring mode, filesystem mounts, network routes, time synchronization, and
firewall output with the purchased specification. Any mismatch blocks secret
transfer and migration.

## Immutable Accepted Image Archive

Run in `CONTROL` from a clean accepted worktree. This builds only from the
accepted commit and explicitly targets `linux/amd64`:

```bash
set -euo pipefail
set +x
umask 077

: "${ACCEPTED_COMMIT:?set ACCEPTED_COMMIT}"
export ACCEPTED_IMAGE="newapi:yucore-$ACCEPTED_COMMIT"
test "$(git rev-parse HEAD)" = "$ACCEPTED_COMMIT"
test "$(git rev-parse "$ACCEPTED_COMMIT^{commit}")" = "$ACCEPTED_COMMIT"
test -z "$(git status --porcelain --untracked-files=no)"
git diff --check

export RELEASE_DIR="$(mktemp -d)"
export CONTEXT_TAR="$RELEASE_DIR/accepted-context.tar"
export CONTEXT_DIR="$RELEASE_DIR/context"
install -d -m 0700 "$CONTEXT_DIR"
git archive --format=tar --output="$CONTEXT_TAR" "$ACCEPTED_COMMIT"
tar -xf "$CONTEXT_TAR" -C "$CONTEXT_DIR"
export GUARD_ARTIFACT="$RELEASE_DIR/yucore_migration_guard.py"
git show "$ACCEPTED_COMMIT:scripts/production/yucore_migration_guard.py" \
  >"$GUARD_ARTIFACT"
test -s "$GUARD_ARTIFACT"
(cd "$RELEASE_DIR" && sha256sum yucore_migration_guard.py \
  >yucore_migration_guard.py.sha256)

docker build --pull=false --platform linux/amd64 \
  --label "org.opencontainers.image.revision=$ACCEPTED_COMMIT" \
  --tag "$ACCEPTED_IMAGE" "$CONTEXT_DIR"
test "$(docker image inspect "$ACCEPTED_IMAGE" --format '{{.Architecture}}/{{.Os}}')" = 'amd64/linux'
test "$(docker image inspect "$ACCEPTED_IMAGE" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')" = "$ACCEPTED_COMMIT"

export IMAGE_ARCHIVE="$RELEASE_DIR/newapi-yucore-$ACCEPTED_COMMIT-linux-amd64.tar"
docker save --output "$IMAGE_ARCHIVE" "$ACCEPTED_IMAGE"
docker image inspect "$ACCEPTED_IMAGE" --format '{{.Id}}' >"$IMAGE_ARCHIVE.image-id"
docker image inspect "$ACCEPTED_IMAGE" --format '{{json .RepoDigests}}' >"$IMAGE_ARCHIVE.repo-digests.json"
printf '%s\n' "$ACCEPTED_COMMIT" >"$IMAGE_ARCHIVE.commit"
(cd "$RELEASE_DIR" && sha256sum "$(basename "$IMAGE_ARCHIVE")" >"$(basename "$IMAGE_ARCHIVE").sha256")
(cd "$RELEASE_DIR" && sha256sum --check "$(basename "$IMAGE_ARCHIVE").sha256")
```

Transfer the archive, metadata, and reviewed guard to both servers. The target
release directory is mode `0700`; no floating tag or remote build is allowed:

```bash
for destination in \
  "$OLD_USER@$OLD_HOST:$OLD_SSH_PORT" \
  "$NEW_USER@$NEW_HOST:$NEW_SSH_PORT"; do
  host_and_user="${destination%:*}"
  port="${destination##*:}"
  ssh -p "$port" "$host_and_user" 'install -d -m 0700 /opt/newapi/releases'
  scp -q -P "$port" \
    "$IMAGE_ARCHIVE" \
    "$IMAGE_ARCHIVE.sha256" \
    "$IMAGE_ARCHIVE.image-id" \
    "$IMAGE_ARCHIVE.repo-digests.json" \
    "$IMAGE_ARCHIVE.commit" \
    "$GUARD_ARTIFACT" \
    "$GUARD_ARTIFACT.sha256" \
    "$host_and_user:/opt/newapi/releases/"
done
```

On both `OLD` and `NEW`, validate the archive before loading it:

```bash
set -euo pipefail
cd /opt/newapi/releases
export IMAGE_ARCHIVE="newapi-yucore-${ACCEPTED_COMMIT}-linux-amd64.tar"
sha256sum --check "$IMAGE_ARCHIVE.sha256"
sha256sum --check yucore_migration_guard.py.sha256
test "$(cat "$IMAGE_ARCHIVE.commit")" = "$ACCEPTED_COMMIT"
docker load --input "$IMAGE_ARCHIVE"
test "$(docker image inspect "$ACCEPTED_IMAGE" --format '{{.Id}}')" = \
  "$(cat "$IMAGE_ARCHIVE.image-id")"
test "$(docker image inspect "$ACCEPTED_IMAGE" --format '{{.Architecture}}/{{.Os}}')" = \
  'amd64/linux'
```

Keep the archive and metadata unchanged through the full retention period.

## Secret-Safe Deployment Transfer And Runtime Guard

In `OLD`, snapshot deployment files without printing their contents. The
runtime environment is written only to a mode-`0600` archive staging tree:

```bash
set -euo pipefail
set +x
umask 077
export RUN_DIR="/opt/newapi/migration/$MIGRATION_ID"
install -d -m 0700 "$RUN_DIR/deployment" "$RUN_DIR/evidence"

cp -a /opt/newapi/docker-compose.yml "$RUN_DIR/deployment/"
cp -a /opt/newapi/.env "$RUN_DIR/deployment/newapi.env"
cp -a /opt/edge "$RUN_DIR/deployment/edge"
docker inspect newapi --format '{{range .Config.Env}}{{println .}}{{end}}' \
  >"$RUN_DIR/deployment/runtime.env"
chmod 0600 "$RUN_DIR/deployment/newapi.env" "$RUN_DIR/deployment/runtime.env"

docker inspect newapi >"$RUN_DIR/evidence/old-newapi-inspect.json"
docker inspect newapi-mysql >"$RUN_DIR/evidence/old-mysql-inspect.json"
docker inspect newapi-redis >"$RUN_DIR/evidence/old-redis-inspect.json"
docker inspect yuapi-caddy >"$RUN_DIR/evidence/old-caddy-inspect.json"
docker compose -f /opt/newapi/docker-compose.yml config --hash='*' \
  >"$RUN_DIR/evidence/compose-service-hashes.txt"

tar -C "$RUN_DIR/deployment" -czf "$RUN_DIR/deployment.tar.gz" .
sha256sum "$RUN_DIR/deployment.tar.gz" >"$RUN_DIR/deployment.tar.gz.sha256"
```

From `OLD`, use the forwarded temporary SSH credential to transfer directly to
`NEW`; the archive bytes do not pass through a terminal pipeline:

```bash
: "${NEW_HOST:?set NEW_HOST}"
: "${NEW_SSH_PORT:?set NEW_SSH_PORT}"
: "${NEW_USER:?set NEW_USER}"
ssh -p "$NEW_SSH_PORT" "$NEW_USER@$NEW_HOST" \
  "install -d -m 0700 '$RUN_DIR'"
scp -q -P "$NEW_SSH_PORT" \
  "$RUN_DIR/deployment.tar.gz" \
  "$RUN_DIR/deployment.tar.gz.sha256" \
  "$NEW_USER@$NEW_HOST:$RUN_DIR/"
```

In `NEW`, validate before extracting, preserve ownership, and ensure all secret
files remain unreadable to other users:

```bash
set -euo pipefail
set +x
umask 077
cd "$RUN_DIR"
sha256sum --check deployment.tar.gz.sha256
install -d -m 0700 /opt/newapi /opt/edge
install -d -m 0700 "$RUN_DIR/deployment"
tar -xzf deployment.tar.gz -C "$RUN_DIR/deployment"
install -m 0600 "$RUN_DIR/deployment/docker-compose.yml" /opt/newapi/docker-compose.yml
install -m 0600 "$RUN_DIR/deployment/newapi.env" /opt/newapi/.env
rsync -aHAX --delete "$RUN_DIR/deployment/edge/" /opt/edge/
chmod 0600 /opt/newapi/.env

docker compose -f /opt/newapi/docker-compose.yml config --quiet
docker compose -f /opt/edge/docker-compose.yml config --quiet
```

No command prints an environment value. The migration guard prints only
container/service names or drift labels.

## State Manifest Command

The four inline helpers below are part of the accepted runbook, not mutable
operator input. These literal SHA-256 values are computed from the exact UTF-8
heredoc bodies in the accepted commit, including the final newline. `CONTROL`
must already have proved `HEAD == ACCEPTED_COMMIT`. Set the same literals in
both remote shells before installing any helper:

```bash
export EXPECTED_CAPTURE_STATE_SHA256='c16b675f2341ffcebf715d835d5a1a01c09b1d5c6ed569d0753773736d044abf'
export EXPECTED_COMPARE_STATE_SHA256='37bd4a13b8991bfb0dfef7382036f44c881c1b1b59fc0a47ab3cc4fb90e1d3b6'
export EXPECTED_EXPORT_MYSQL_SHA256='eecf0bc1d7b97f2e2a0ae9aa1320a13b61ff1dd11954249c8896d3562fdc9d23'
export EXPECTED_RESTORE_MYSQL_SHA256='6608997582169b43546a9a68c2b343172178124c6003cf05ba3634f8095b476a'
export HELPER_CONTEXT_BOOTSTRAPPED='accepted-literal-hashes-v1'

verify_migration_helper() {
  helper_path="$1"
  expected_sha256="$2"
  test "$HELPER_CONTEXT_BOOTSTRAPPED" = 'accepted-literal-hashes-v1'
  test -f "$helper_path"
  test "$(sha256sum "$helper_path" | cut -d' ' -f1)" = "$expected_sha256"
}

verify_migration_helpers() {
  test "$HELPER_CONTEXT_BOOTSTRAPPED" = 'accepted-literal-hashes-v1'
  verify_migration_helper /opt/newapi/releases/capture-state-manifest \
    "$EXPECTED_CAPTURE_STATE_SHA256"
  verify_migration_helper /opt/newapi/releases/compare-state-snapshots \
    "$EXPECTED_COMPARE_STATE_SHA256"
  verify_migration_helper /opt/newapi/releases/export-mysql-snapshot \
    "$EXPECTED_EXPORT_MYSQL_SHA256"
  verify_migration_helper /opt/newapi/releases/restore-mysql-snapshot \
    "$EXPECTED_RESTORE_MYSQL_SHA256"
}
```

Install this reviewed manifest command on both servers. It prints counts,
maximum IDs, schema hashes, content hashes, option-value hashes, Redis counts,
and file hashes, never secret values. `MYSQL_PWD` exists only inside the MySQL
container process:

Redis key identity is reduced inside Redis Lua to cryptographic digests before
leaving the container. The manifest contains only total count, per-key
cryptographic digests, and Redis types. Live TTL and aging
buckets are excluded. Persistent digests/types compare exactly; expiring keys
carry absolute `PEXPIRETIME` timestamps for the expiry-aware comparator below.
The command never writes or emits a raw key or value. Lua strings are
binary-safe, so embedded NUL and newline bytes in keys are hashed without
passing through shell text parsing.

```bash
install -m 0700 /dev/stdin /opt/newapi/releases/capture-state-manifest <<'MANIFEST'
#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077

output="${1:?output path required}"
tmp="$(mktemp -d)"
trap 'find "$tmp" -xdev -depth -delete' EXIT
mysql_container="${MYSQL_CONTAINER_NAME:-newapi-mysql}"
redis_container="${REDIS_CONTAINER_NAME:-newapi-redis}"

docker exec "$mysql_container" sh -lc '
  set -eu
  set +x
  export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
  database="$MYSQL_DATABASE"
  tables="$(mysql -uroot -NBe "
    SELECT table_name FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_type = '\''BASE TABLE'\''
    ORDER BY table_name" "$database")"
  printf "%s\n" "$tables" | while IFS= read -r table; do
    test -n "$table" || continue
    count="$(mysql -uroot -NBe "SELECT COUNT(*) FROM \`$table\`" "$database")"
    has_id="$(mysql -uroot -NBe "SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE() AND table_name = '\''$table'\''
      AND column_name = '\''id'\''" "$database")"
    case "$has_id" in
      1) max_id="$(mysql -uroot -NBe "SELECT COALESCE(MAX(id),0) FROM \`$table\`" "$database")" ;;
      "") max_id="" ;;
      *) exit 1 ;;
    esac
    schema_file="$(mktemp)"
    mysql -uroot -NBe "
      SELECT CONCAT_WS('\\x1f', ordinal_position, column_name, column_type,
        is_nullable, COALESCE(column_default, '\''<NULL>'\''), extra,
        character_set_name, collation_name)
      FROM information_schema.columns
      WHERE table_schema = DATABASE() AND table_name = '\''$table'\''
      ORDER BY ordinal_position" "$database" >"$schema_file"
    schema_sha="$(sha256sum "$schema_file" | cut -d" " -f1)"
    rm -f "$schema_file"
    table_dump="$(mktemp)"
    mysqldump --default-character-set=utf8mb4 -uroot \
      --single-transaction --quick --skip-lock-tables --set-gtid-purged=OFF \
      --compact --skip-comments --no-create-info --order-by-primary \
      "$database" "$table" >"$table_dump"
    test -f "$table_dump"
    content_sha="$(sha256sum "$table_dump" | cut -d" " -f1)"
    rm -f "$table_dump"
    printf "%s\t%s\t%s\t%s\t%s\n" "$table" "$count" "$max_id" "$schema_sha" "$content_sha"
  done
' | jq -Rsc '
  split("\n") | map(select(length > 0) | split("\t") |
    {name:.[0],count:.[1],max_id:(if .[2] == "" then null else .[2] end),
     schema_sha256:.[3],content_sha256:.[4]})' >"$tmp/mysql-tables.json"

docker exec "$mysql_container" sh -lc '
  set -eu
  set +x
  export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
  if mysql -uroot -NBe "SHOW TABLES LIKE '\''options'\''" "$MYSQL_DATABASE" | grep -qx options; then
    mysql -uroot -NBe "SELECT \`key\`, SHA2(CAST(value AS BINARY),256)
      FROM options ORDER BY \`key\`" "$MYSQL_DATABASE"
  fi
' | jq -Rsc 'split("\n") | map(select(length > 0) | split("\t") | {(.[0]): .[1]}) | add // {}' \
  >"$tmp/options.json"

redis_dbsize="$(docker exec "$redis_container" sh -lc '
  set -eu
  set +x
  if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
  redis-cli --no-auth-warning --raw DBSIZE
')"
docker exec "$redis_container" sh -lc '
  set -eu
  set +x
  if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
  redis-cli --no-auth-warning --raw INFO persistence | tr -d "\r" | \
    grep -E "^(aof_enabled|rdb_last_bgsave_status|aof_last_bgrewrite_status):"
' >"$tmp/redis-persistence.txt"
cat >"$tmp/redis-manifest.lua" <<'LUA'
local cursor = "0"
local persistent = {}
local expiring = {}
local now_reply = redis.call("TIME")
local observed_at_ms = tonumber(now_reply[1]) * 1000 + math.floor(tonumber(now_reply[2]) / 1000)
repeat
  local page = redis.call("SCAN", cursor, "COUNT", 1000)
  cursor = page[1]
  for _, key in ipairs(page[2]) do
    local kind = redis.call("TYPE", key).ok
    local key_hash = redis.sha1hex(key)
    local absolute_expiry = redis.call("PEXPIRETIME", key)
    if absolute_expiry == -1 then
      persistent[#persistent + 1] = {key_sha1 = key_hash, type = kind}
    elseif absolute_expiry >= 0 then
      expiring[#expiring + 1] = {
        key_sha1 = key_hash, type = kind, expire_at_ms = absolute_expiry
      }
    end
  end
until cursor == "0"
table.sort(persistent, function(a, b) return a.key_sha1 < b.key_sha1 end)
table.sort(expiring, function(a, b) return a.key_sha1 < b.key_sha1 end)
return cjson.encode({
  observed_at_ms = observed_at_ms,
  persistent = persistent,
  expiring = expiring
})
LUA
docker cp "$tmp/redis-manifest.lua" "$redis_container:/tmp/yucore-redis-manifest.lua"
redis_key_document="$(docker exec "$redis_container" sh -lc '
  set -eu
  set +x
  if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
  redis-cli --no-auth-warning --raw --eval /tmp/yucore-redis-manifest.lua
  rm -f /tmp/yucore-redis-manifest.lua
')"
printf '%s' "$redis_key_document" | jq -e '
  (if .persistent == {} then .persistent = []
   elif (.persistent | type) == "array" then .
   else error("persistent Redis category must be an array or exact empty object") end) |
  (if .expiring == {} then .expiring = []
   elif (.expiring | type) == "array" then .
   else error("expiring Redis category must be an array or exact empty object") end) |
  if ((.observed_at_ms | type == "number") and
      (.persistent | type == "array") and (.expiring | type == "array") and
      ([.persistent[].key_sha1,.expiring[].key_sha1] | length == (unique | length)))
  then . else error("invalid Redis digest document") end' \
  >"$tmp/redis-keys.json"
test "$(jq '[.persistent,.expiring] | map(length) | add' "$tmp/redis-keys.json")" = \
  "$redis_dbsize"
jq -n --arg dbsize "$redis_dbsize" \
  --rawfile persistence "$tmp/redis-persistence.txt" \
  --slurpfile keys "$tmp/redis-keys.json" \
  '{dbsize:$dbsize,persistence:$persistence,
    observed_at_ms:$keys[0].observed_at_ms,
    persistent:$keys[0].persistent,expiring:$keys[0].expiring}' \
  >"$tmp/redis.json"

file_manifest() {
  root="$1"
  if test -d "$root"; then
    find "$root" -xdev -type f -print0 | LC_ALL=C sort -z | \
      xargs -0 -r sha256sum | sha256sum | cut -d' ' -f1
  else
    printf 'absent'
  fi
}

jq -n \
  --arg accepted_commit "${ACCEPTED_COMMIT:?set ACCEPTED_COMMIT}" \
  --arg accepted_image_id "$(docker image inspect newapi:yucore-"$ACCEPTED_COMMIT" --format '{{.Id}}')" \
  --slurpfile tables "$tmp/mysql-tables.json" \
  --slurpfile options "$tmp/options.json" \
  --slurpfile redis "$tmp/redis.json" \
  --arg app_data_sha256 "$(file_manifest /opt/newapi/data)" \
  --arg edge_sha256 "$(file_manifest /opt/edge)" \
  '{accepted_commit:$accepted_commit,accepted_image_id:$accepted_image_id,
    mysql_tables:$tables[0],option_value_sha256:$options[0],redis:$redis[0],
    files:{app_data_sha256:$app_data_sha256,edge_sha256:$edge_sha256}}' >"$output"
chmod 0600 "$output"
MANIFEST
verify_migration_helper /opt/newapi/releases/capture-state-manifest \
  "$EXPECTED_CAPTURE_STATE_SHA256"
```

The table content hashes can take several minutes on the `logs` table. That is
expected. Do not weaken the manifest or omit large tables during the final or
reverse-migration comparisons.

Install this expiry-aware state comparator on both servers. It keeps every
non-Redis section exact, requires the persistent Redis digest/type map to match
exactly, rejects target extras and type drift, and permits a source expiring
key to be absent only when its absolute expiry is no later than the target
observation time plus the explicit tolerance. Present expiring keys retain the
same absolute expiry within that tolerance, and DB sizes reconcile exactly
after permitted expirations:

```bash
install -m 0700 /dev/stdin /opt/newapi/releases/compare-state-snapshots <<'COMPARE_STATE'
#!/usr/bin/env python3
import argparse
import json
import sys


def fail(message):
    raise RuntimeError(message)


def keyed(entries, expiring):
    if not isinstance(entries, list):
        fail("Redis digest entries must be arrays")
    result = {}
    required = {"key_sha1", "type", "expire_at_ms"} if expiring else {"key_sha1", "type"}
    for entry in entries:
        if not isinstance(entry, dict) or set(entry) != required:
            fail("invalid Redis digest entry")
        digest = entry["key_sha1"]
        if not isinstance(digest, str) or len(digest) != 40 or digest in result:
            fail("invalid or duplicate Redis digest")
        if not isinstance(entry["type"], str) or not entry["type"]:
            fail("invalid Redis type")
        if expiring and type(entry["expire_at_ms"]) is not int:
            fail("invalid absolute Redis expiry")
        result[digest] = entry
    return result


parser = argparse.ArgumentParser()
parser.add_argument("--source", required=True)
parser.add_argument("--target", required=True)
parser.add_argument("--tolerance-ms", type=int, default=2000)
args = parser.parse_args()
if args.tolerance_ms < 0 or args.tolerance_ms > 5000:
    fail("Redis expiry tolerance must be between 0 and 5000 ms")

with open(args.source, "r", encoding="utf-8") as handle:
    source = json.load(handle)
with open(args.target, "r", encoding="utf-8") as handle:
    target = json.load(handle)
if not isinstance(source, dict) or not isinstance(target, dict):
    fail("state snapshots must be JSON objects")

source_exact = {key: value for key, value in source.items() if key != "redis"}
target_exact = {key: value for key, value in target.items() if key != "redis"}
if source_exact != target_exact:
    fail("exact MySQL, option, image, commit, or file evidence drift")

source_redis = source.get("redis")
target_redis = target.get("redis")
if not isinstance(source_redis, dict) or not isinstance(target_redis, dict):
    fail("Redis evidence is missing")
if source_redis.get("persistence") != target_redis.get("persistence"):
    fail("Redis persistence configuration drift")
source_persistent = keyed(source_redis.get("persistent"), False)
target_persistent = keyed(target_redis.get("persistent"), False)
if source_persistent != target_persistent:
    fail("persistent Redis digest/type drift")
source_expiring = keyed(source_redis.get("expiring"), True)
target_expiring = keyed(target_redis.get("expiring"), True)
target_observed_at = target_redis.get("observed_at_ms")
if type(target_observed_at) is not int:
    fail("target Redis observation time is invalid")

extras = set(target_expiring) - set(source_expiring)
if extras:
    fail("target contains extra expiring Redis digests")
missing = set(source_expiring) - set(target_expiring)
for digest in missing:
    if source_expiring[digest]["expire_at_ms"] > target_observed_at + args.tolerance_ms:
        fail("unexpired source Redis digest is missing from target")
for digest, target_entry in target_expiring.items():
    source_entry = source_expiring[digest]
    if source_entry["type"] != target_entry["type"]:
        fail("expiring Redis type drift")
    if abs(source_entry["expire_at_ms"] - target_entry["expire_at_ms"]) > args.tolerance_ms:
        fail("absolute Redis expiry drift")

source_total = len(source_persistent) + len(source_expiring)
target_total = len(target_persistent) + len(target_expiring)
if int(source_redis.get("dbsize", -1)) != source_total:
    fail("source Redis DB size does not reconcile")
if int(target_redis.get("dbsize", -1)) != target_total:
    fail("target Redis DB size does not reconcile")
if target_total != source_total - len(missing):
    fail("Redis count delta does not equal permitted expirations")

print(json.dumps({"ok": True, "expired_keys": len(missing)}, separators=(",", ":")))
COMPARE_STATE
verify_migration_helper /opt/newapi/releases/compare-state-snapshots \
  "$EXPECTED_COMPARE_STATE_SHA256"
```

## Binary-Safe MySQL Snapshot Commands

Install these reviewed commands on both servers. Export writes an uncompressed
dump inside `newapi-mysql`, checks the `mysqldump` exit directly, records size
and SHA-256, compresses in a separate checked command, and only then uses
`docker cp`. Restore verifies both compressed and uncompressed evidence before
feeding a file to `mysql`; no dump byte crosses a shell pipeline:

```bash
install -m 0700 /dev/stdin /opt/newapi/releases/export-mysql-snapshot <<'EXPORT_MYSQL'
#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077
snapshot="${1:?snapshot name required}"
output_dir="${2:?output directory required}"
case "$snapshot" in *[!a-z0-9-]*) exit 1 ;; esac
install -d -m 0700 "$output_dir"

docker exec newapi-mysql sh -lc '
  set -eu
  set +x
  umask 077
  snapshot="$1"
  sql="/tmp/yucore-$snapshot.sql"
  gzip_file="$sql.gz"
  export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
  mysqldump --default-character-set=utf8mb4 -uroot \
    --single-transaction --quick --skip-lock-tables --routines --triggers \
    --events --hex-blob --set-gtid-purged=OFF --add-drop-database \
    --databases "$MYSQL_DATABASE" >"$sql"
  test -s "$sql"
  set -- $(sha256sum "$sql")
  printf "%s\n" "$1" >"$sql.sha256-value"
  wc -c <"$sql" >"$sql.bytes"
  gzip -1 -c "$sql" >"$gzip_file"
  gzip -t "$gzip_file"
  set -- $(sha256sum "$gzip_file")
  printf "%s\n" "$1" >"$gzip_file.sha256-value"
' sh "$snapshot"

base="yucore-$snapshot.sql"
for suffix in .gz .sha256-value .bytes .gz.sha256-value; do
  docker cp "newapi-mysql:/tmp/$base$suffix" "$output_dir/$base$suffix"
done
test "$(sha256sum "$output_dir/$base.gz" | cut -d' ' -f1)" = \
  "$(cat "$output_dir/$base.gz.sha256-value")"
docker exec newapi-mysql sh -lc '
  set -eu
  base="$1"
  rm -f "/tmp/$base" "/tmp/$base.gz" "/tmp/$base.sha256-value" \
    "/tmp/$base.bytes" "/tmp/$base.gz.sha256-value"
' sh "$base"
EXPORT_MYSQL
verify_migration_helper /opt/newapi/releases/export-mysql-snapshot \
  "$EXPECTED_EXPORT_MYSQL_SHA256"

install -m 0700 /dev/stdin /opt/newapi/releases/restore-mysql-snapshot <<'RESTORE_MYSQL'
#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077
snapshot="${1:?snapshot name required}"
input_dir="${2:?input directory required}"
mysql_container="${3:-newapi-mysql}"
case "$snapshot" in *[!a-z0-9-]*) exit 1 ;; esac
case "$mysql_container" in
  newapi-mysql|yucore-migration-*-mysql-verifier) ;;
  *) exit 1 ;;
esac
base="yucore-$snapshot.sql"
for suffix in .gz .sha256-value .bytes .gz.sha256-value; do
  test -s "$input_dir/$base$suffix"
done
test "$(sha256sum "$input_dir/$base.gz" | cut -d' ' -f1)" = \
  "$(cat "$input_dir/$base.gz.sha256-value")"
for suffix in .gz .sha256-value .bytes .gz.sha256-value; do
  docker cp "$input_dir/$base$suffix" "$mysql_container:/tmp/$base$suffix"
done

docker exec "$mysql_container" sh -lc '
  set -eu
  set +x
  base="$1"
  gzip -t "/tmp/$base.gz"
  gzip -dc "/tmp/$base.gz" >"/tmp/$base"
  test -s "/tmp/$base"
  set -- $(sha256sum "/tmp/$base")
  test "$1" = "$(cat "/tmp/$base.sha256-value")"
  test "$(wc -c <"/tmp/$base")" = "$(cat "/tmp/$base.bytes")"
  export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
  mysql --default-character-set=utf8mb4 -uroot <"/tmp/$base"
  test "$(mysql -uroot -NBe "SELECT 1" "$MYSQL_DATABASE")" = 1
  rm -f "/tmp/$base" "/tmp/$base.gz" "/tmp/$base.sha256-value" \
    "/tmp/$base.bytes" "/tmp/$base.gz.sha256-value"
' sh "$base"
RESTORE_MYSQL
verify_migration_helper /opt/newapi/releases/restore-mysql-snapshot \
  "$EXPECTED_RESTORE_MYSQL_SHA256"
verify_migration_helpers
```

On both `OLD` and `NEW`, record expected and actual helper hashes without
including helper contents or secrets:

```bash
install -d -m 0700 "$RUN_DIR/evidence"
verify_migration_helpers
{
  printf 'capture-state-manifest\t%s\t%s\n' "$EXPECTED_CAPTURE_STATE_SHA256" \
    "$(sha256sum /opt/newapi/releases/capture-state-manifest | cut -d' ' -f1)"
  printf 'compare-state-snapshots\t%s\t%s\n' "$EXPECTED_COMPARE_STATE_SHA256" \
    "$(sha256sum /opt/newapi/releases/compare-state-snapshots | cut -d' ' -f1)"
  printf 'export-mysql-snapshot\t%s\t%s\n' "$EXPECTED_EXPORT_MYSQL_SHA256" \
    "$(sha256sum /opt/newapi/releases/export-mysql-snapshot | cut -d' ' -f1)"
  printf 'restore-mysql-snapshot\t%s\t%s\n' "$EXPECTED_RESTORE_MYSQL_SHA256" \
    "$(sha256sum /opt/newapi/releases/restore-mysql-snapshot | cut -d' ' -f1)"
} >"$RUN_DIR/evidence/migration-helper-sha256.tsv"
(cd "$RUN_DIR/evidence" && sha256sum migration-helper-sha256.tsv \
  >migration-helper-sha256.tsv.sha256)
```

On `CONTROL`, anchor the evidence to the accepted commit, then pull and verify
both remote reports:

```bash
test "$(git rev-parse HEAD)" = "$ACCEPTED_COMMIT"
export ACCEPTED_RUNBOOK_COPY="$RELEASE_DIR/accepted-production-cross-server-migration.md"
git show "$ACCEPTED_COMMIT:docs/superpowers/runbooks/2026-07-28-production-cross-server-migration.md" \
  >"$ACCEPTED_RUNBOOK_COPY"
sha256sum "$ACCEPTED_RUNBOOK_COPY" >"$ACCEPTED_RUNBOOK_COPY.sha256"
sha256sum --check "$ACCEPTED_RUNBOOK_COPY.sha256"
export CONTROL_HELPER_EVIDENCE="$RELEASE_DIR/helper-evidence/$MIGRATION_ID"
export REMOTE_HELPER_EVIDENCE="/opt/newapi/migration/$MIGRATION_ID/evidence"
install -d -m 0700 "$CONTROL_HELPER_EVIDENCE"
export CANONICAL_HELPER_MANIFEST="$CONTROL_HELPER_EVIDENCE/canonical-helper-sha256.tsv"
python3 - "$ACCEPTED_RUNBOOK_COPY" "$CANONICAL_HELPER_MANIFEST" <<'CANONICAL_HELPERS'
import hashlib
import pathlib
import re
import sys

runbook = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").replace("\r\n", "\n")
output = pathlib.Path(sys.argv[2])
specs = (
    ("capture-state-manifest", "MANIFEST", "EXPECTED_CAPTURE_STATE_SHA256"),
    ("compare-state-snapshots", "COMPARE_STATE", "EXPECTED_COMPARE_STATE_SHA256"),
    ("export-mysql-snapshot", "EXPORT_MYSQL", "EXPECTED_EXPORT_MYSQL_SHA256"),
    ("restore-mysql-snapshot", "RESTORE_MYSQL", "EXPECTED_RESTORE_MYSQL_SHA256"),
)
rows = []
for helper_name, terminator, variable in specs:
    body_match = re.search(
        rf"<<'{terminator}'\n(.*?)\n{terminator}(?:\n|$)", runbook, re.DOTALL
    )
    if body_match is None:
        raise SystemExit(f"missing helper body: {helper_name}")
    canonical_hash = hashlib.sha256(
        (body_match.group(1) + "\n").encode("utf-8")
    ).hexdigest()
    literals = re.findall(
        rf"^export {variable}='([^']*)'$", runbook, re.MULTILINE
    )
    if (
        len(literals) < 2
        or any(re.fullmatch(r"[0-9a-f]{64}", value) is None for value in literals)
        or len(set(literals)) != 1
    ):
        raise SystemExit(f"missing, invalid, or inconsistent literals: {variable}")
    if literals[0] != canonical_hash:
        raise SystemExit(f"helper body/literal mismatch: {helper_name}")
    rows.append(f"{helper_name}\t{canonical_hash}\n")
output.write_text("".join(rows), encoding="utf-8", newline="\n")
CANONICAL_HELPERS
(cd "$CONTROL_HELPER_EVIDENCE" && sha256sum canonical-helper-sha256.tsv \
  >canonical-helper-sha256.tsv.sha256 && \
  sha256sum --check canonical-helper-sha256.tsv.sha256)

for source_spec in \
  "old:$OLD_USER@$OLD_HOST:$OLD_SSH_PORT" \
  "new:$NEW_USER@$NEW_HOST:$NEW_SSH_PORT"; do
  label="${source_spec%%:*}"
  remainder="${source_spec#*:}"
  host_and_user="${remainder%:*}"
  port="${remainder##*:}"
  destination="$CONTROL_HELPER_EVIDENCE/$label"
  install -d -m 0700 "$destination"
  scp -q -P "$port" \
    "$host_and_user:$REMOTE_HELPER_EVIDENCE/migration-helper-sha256.tsv" \
    "$host_and_user:$REMOTE_HELPER_EVIDENCE/migration-helper-sha256.tsv.sha256" \
    "$destination/"
  (cd "$destination" && sha256sum --check migration-helper-sha256.tsv.sha256)
done

export HELPER_COMPARISON_EVIDENCE="$CONTROL_HELPER_EVIDENCE/helper-comparison.tsv"
python3 - "$CANONICAL_HELPER_MANIFEST" \
  "$CONTROL_HELPER_EVIDENCE/old/migration-helper-sha256.tsv" \
  "$CONTROL_HELPER_EVIDENCE/new/migration-helper-sha256.tsv" \
  "$HELPER_COMPARISON_EVIDENCE" <<'COMPARE_HELPER_REPORTS'
import pathlib
import sys

expected_names = {
    "capture-state-manifest",
    "compare-state-snapshots",
    "export-mysql-snapshot",
    "restore-mysql-snapshot",
}


def parse(path, fields):
    rows = {}
    for line in pathlib.Path(path).read_text(encoding="utf-8").splitlines():
        parts = line.split("\t")
        if len(parts) != fields or parts[0] in rows:
            raise SystemExit(f"malformed or duplicate helper row: {path}")
        rows[parts[0]] = parts[1:]
    if set(rows) != expected_names:
        raise SystemExit(f"missing or extra helper rows: {path}")
    return rows


canonical = parse(sys.argv[1], 2)
comparison = []
for label, path in (("old", sys.argv[2]), ("new", sys.argv[3])):
    report = parse(path, 3)
    for name in sorted(expected_names):
        canonical_hash = canonical[name][0]
        remote_expected, remote_actual = report[name]
        if not (canonical_hash == remote_expected == remote_actual):
            raise SystemExit(f"canonical/remote helper mismatch: {label}:{name}")
        comparison.append(
            f"{label}\t{name}\t{canonical_hash}\t{remote_expected}\t{remote_actual}\n"
        )
pathlib.Path(sys.argv[4]).write_text(
    "".join(comparison), encoding="utf-8", newline="\n"
)
COMPARE_HELPER_REPORTS
(cd "$CONTROL_HELPER_EVIDENCE" && sha256sum helper-comparison.tsv \
  >helper-comparison.tsv.sha256 && sha256sum --check helper-comparison.tsv.sha256)
```

## Pre-Stage Mutable State

Run in `OLD` after the user-authorized preparation begins. Create a recent
logical snapshot inside the MySQL container, then copy the binary archive out;
dump bytes never pass through a PowerShell or terminal text pipeline:

The accepted image archive carries compiled static brand assets. Externally
stored logos, uploads, and historical application logs are covered by the
exact `/opt/newapi/data` sync. The complete `/opt/edge` sync covers Caddy
configuration, certificate storage, and any edge-served static files.

```bash
set -euo pipefail
set +x
umask 077
install -d -m 0700 "$RUN_DIR/prestage" "$RUN_DIR/evidence"

verify_migration_helpers
/opt/newapi/releases/export-mysql-snapshot prestage "$RUN_DIR/prestage"

docker exec newapi-redis sh -lc '
  set -eu
  set +x
  if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
  redis-cli --no-auth-warning SAVE >/dev/null
'
export REDIS_DATA_SOURCE="$(docker inspect newapi-redis --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}')"
sha256sum --check "$RUN_DIR/evidence/old-redis-mount.tsv.sha256"
IFS=$'\t' read -r REDIS_DESTINATION RECORDED_OLD_REDIS_MOUNT_SOURCE \
  <"$RUN_DIR/evidence/old-redis-mount.tsv"
test "$REDIS_DESTINATION" = '/data'
test "$RECORDED_OLD_REDIS_MOUNT_SOURCE" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
test "$REDIS_DATA_SOURCE" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
tar -C "$REDIS_DATA_SOURCE" -czf "$RUN_DIR/prestage/redis-data.tar.gz" .

rsync -aHAX --numeric-ids /opt/newapi/data/ "$RUN_DIR/prestage/app-data/"
rsync -aHAX --numeric-ids /opt/edge/ "$RUN_DIR/prestage/edge/"
if compgen -G '/etc/systemd/system/*newapi*backup*' >/dev/null; then
  install -d -m 0700 "$RUN_DIR/prestage/systemd"
  cp -a /etc/systemd/system/*newapi*backup* "$RUN_DIR/prestage/systemd/"
fi
if find /opt/newapi -maxdepth 1 -type f -iname '*backup*.sh' -print -quit | grep -q .; then
  install -d -m 0700 "$RUN_DIR/prestage/backup-tools"
  find /opt/newapi -maxdepth 1 -type f -iname '*backup*.sh' \
    -exec cp -a -t "$RUN_DIR/prestage/backup-tools" -- {} +
fi

(cd "$RUN_DIR/prestage" && find . -type f -print0 | LC_ALL=C sort -z | \
  xargs -0 sha256sum >"$RUN_DIR/evidence/prestage-files.sha256")
tar -C "$RUN_DIR/prestage" -czf "$RUN_DIR/prestage.tar.gz" .
sha256sum "$RUN_DIR/prestage.tar.gz" >"$RUN_DIR/prestage.tar.gz.sha256"
jq -n \
  --arg mysql_sql_sha256 "$(cat "$RUN_DIR/prestage/yucore-prestage.sql.sha256-value")" \
  --arg mysql_sql_bytes "$(cat "$RUN_DIR/prestage/yucore-prestage.sql.bytes")" \
  --arg mysql_gzip_sha256 "$(cat "$RUN_DIR/prestage/yucore-prestage.sql.gz.sha256-value")" \
  --arg redis_archive_sha256 "$(sha256sum "$RUN_DIR/prestage/redis-data.tar.gz" | cut -d' ' -f1)" \
  --arg files_manifest_sha256 "$(sha256sum "$RUN_DIR/evidence/prestage-files.sha256" | cut -d' ' -f1)" \
  '{mysql:{sql_sha256:$mysql_sql_sha256,sql_bytes:$mysql_sql_bytes,gzip_sha256:$mysql_gzip_sha256},
    redis:{archive_sha256:$redis_archive_sha256},files:{manifest_sha256:$files_manifest_sha256}}' \
  >"$RUN_DIR/evidence/source-prestage-artifacts.json"
```

Transfer directly from `OLD` to `NEW` and verify in `NEW`:

```bash
# OLD
ssh -p "$NEW_SSH_PORT" "$NEW_USER@$NEW_HOST" "install -d -m 0700 '$RUN_DIR'"
scp -q -P "$NEW_SSH_PORT" \
  "$RUN_DIR/prestage.tar.gz" "$RUN_DIR/prestage.tar.gz.sha256" \
  "$RUN_DIR/evidence/prestage-files.sha256" \
  "$RUN_DIR/evidence/source-prestage-artifacts.json" \
  "$NEW_USER@$NEW_HOST:$RUN_DIR/"

# NEW
cd "$RUN_DIR"
sha256sum --check prestage.tar.gz.sha256
install -d -m 0700 "$RUN_DIR/prestage" "$RUN_DIR/evidence"
mv "$RUN_DIR/prestage-files.sha256" "$RUN_DIR/evidence/prestage-files.sha256"
mv "$RUN_DIR/source-prestage-artifacts.json" \
  "$RUN_DIR/evidence/source-prestage-artifacts.json"
tar -xzf prestage.tar.gz -C "$RUN_DIR/prestage"
(cd "$RUN_DIR/prestage" && sha256sum --check "$RUN_DIR/evidence/prestage-files.sha256")
jq -n \
  --arg mysql_sql_sha256 "$(cat "$RUN_DIR/prestage/yucore-prestage.sql.sha256-value")" \
  --arg mysql_sql_bytes "$(cat "$RUN_DIR/prestage/yucore-prestage.sql.bytes")" \
  --arg mysql_gzip_sha256 "$(cat "$RUN_DIR/prestage/yucore-prestage.sql.gz.sha256-value")" \
  --arg redis_archive_sha256 "$(sha256sum "$RUN_DIR/prestage/redis-data.tar.gz" | cut -d' ' -f1)" \
  --arg files_manifest_sha256 "$(sha256sum "$RUN_DIR/evidence/prestage-files.sha256" | cut -d' ' -f1)" \
  '{mysql:{sql_sha256:$mysql_sql_sha256,sql_bytes:$mysql_sql_bytes,gzip_sha256:$mysql_gzip_sha256},
    redis:{archive_sha256:$redis_archive_sha256},files:{manifest_sha256:$files_manifest_sha256}}' \
  >"$RUN_DIR/evidence/target-prestage-artifacts.json"
python3 /opt/newapi/releases/yucore_migration_guard.py compare-manifests \
  --source "$RUN_DIR/evidence/source-prestage-artifacts.json" \
  --target "$RUN_DIR/evidence/target-prestage-artifacts.json"
rsync -aHAX --numeric-ids "$RUN_DIR/prestage/app-data/" /opt/newapi/data/
rsync -aHAX --numeric-ids "$RUN_DIR/prestage/edge/" /opt/edge/
if test -d "$RUN_DIR/prestage/systemd"; then
  cp -a "$RUN_DIR/prestage/systemd/"* /etc/systemd/system/
  systemctl daemon-reload
  for unit_file in "$RUN_DIR/prestage/systemd/"*; do
    systemctl enable "$(basename "$unit_file")"
  done
fi
if test -d "$RUN_DIR/prestage/backup-tools"; then
  cp -a "$RUN_DIR/prestage/backup-tools/." /opt/newapi/
fi
```

## Restore The Pre-Stage Snapshot On The New Server

In `NEW`, create an image-only Compose override, start only MySQL and Redis,
restore both snapshots, then start the application as a non-recurring slave.
Provider firewall rules still block public traffic. Old production remains
writable throughout: the source evidence is the immutable dump/hash, not a
later live-state query. The same dump is restored independently into target
MySQL and a labeled verifier MySQL on `NEW`; their manifests must match before
the verifier is removed and private acceptance begins:

```bash
set -euo pipefail
set +x
umask 077

cat >"$RUN_DIR/image-only.yml" <<EOF
services:
  newapi:
    image: $ACCEPTED_IMAGE
EOF

cat >"$RUN_DIR/candidate.yml" <<EOF
services:
  newapi:
    image: $ACCEPTED_IMAGE
    environment:
      NODE_TYPE: slave
      NODE_NAME: migration-private-candidate
      BATCH_UPDATE_ENABLED: "false"
      UPDATE_TASK: "false"
      CHANNEL_UPDATE_FREQUENCY: ""
EOF
chmod 0600 "$RUN_DIR/image-only.yml" "$RUN_DIR/candidate.yml"

docker compose -f /opt/newapi/docker-compose.yml -f "$RUN_DIR/image-only.yml" \
  config --quiet
case "$APPROVED_NEW_REDIS_MOUNT_SOURCE" in
  /|/opt|/opt/newapi|/opt/newapi/data|/var/lib/docker|/var/lib/docker/volumes) exit 1 ;;
esac
printf '%s\t%s\n' '/data' "$APPROVED_NEW_REDIS_MOUNT_SOURCE" \
  >"$RUN_DIR/evidence/new-redis-mount.tsv"
sha256sum "$RUN_DIR/evidence/new-redis-mount.tsv" \
  >"$RUN_DIR/evidence/new-redis-mount.tsv.sha256"
chmod 0600 "$RUN_DIR/evidence/new-redis-mount.tsv" \
  "$RUN_DIR/evidence/new-redis-mount.tsv.sha256"
docker compose -f /opt/newapi/docker-compose.yml -f "$RUN_DIR/image-only.yml" \
  config --format json >"$RUN_DIR/evidence/new-compose-reviewed.json"
mapfile -t NEW_REDIS_PLAN < <(jq -er '
  [.services.redis.volumes[] | select(.target == "/data")] |
  if length == 1 then .[0] | [.type,.source] | @tsv
  else error("expected one planned Redis /data mount") end' \
  "$RUN_DIR/evidence/new-compose-reviewed.json")
IFS=$'\t' read -r NEW_REDIS_PLAN_TYPE NEW_REDIS_PLAN_SOURCE \
  <<<"${NEW_REDIS_PLAN[0]}"
test "$NEW_REDIS_PLAN_TYPE" = bind
test "$(realpath -m "$NEW_REDIS_PLAN_SOURCE")" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
printf '%s\n' "$APPROVED_NEW_REDIS_MOUNT_SOURCE" \
  >"$RUN_DIR/evidence/new-redis-planned-source"
sha256sum "$RUN_DIR/evidence/new-redis-planned-source" \
  >"$RUN_DIR/evidence/new-redis-planned-source.sha256"
sha256sum "$RUN_DIR/evidence/new-compose-reviewed.json" \
  >"$RUN_DIR/evidence/new-compose-reviewed.json.sha256"
test -z "$(docker ps -aq --filter 'name=^/newapi-redis$')"
docker compose -f /opt/newapi/docker-compose.yml -f "$RUN_DIR/image-only.yml" \
  up -d mysql redis

for attempt in $(seq 1 60); do
  mysql_status="$(docker inspect newapi-mysql --format '{{.State.Health.Status}}')"
  redis_status="$(docker inspect newapi-redis --format '{{.State.Status}}')"
  test "$mysql_status" = healthy && test "$redis_status" = running && break
  sleep 1
done
test "$(docker inspect newapi-mysql --format '{{.State.Health.Status}}')" = healthy
test "$(docker inspect newapi-redis --format '{{.State.Status}}')" = running

test "$(docker inspect newapi-redis --format '{{json .Mounts}}' | jq -r \
  '[.[] | select(.Destination == "/data")] | length')" = 1
export CURRENT_NEW_REDIS_MOUNT_SOURCE="$(docker inspect newapi-redis \
  --format '{{json .Mounts}}' | jq -er \
  '[.[] | select(.Destination == "/data")][0].Source')"
test "$CURRENT_NEW_REDIS_MOUNT_SOURCE" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"

verify_migration_helpers
/opt/newapi/releases/restore-mysql-snapshot prestage "$RUN_DIR/prestage"

docker stop --time 5 newapi-redis
export REDIS_DATA_TARGET="$(docker inspect newapi-redis --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}')"
sha256sum --check "$RUN_DIR/evidence/new-redis-mount.tsv.sha256"
sha256sum --check "$RUN_DIR/evidence/new-redis-planned-source.sha256"
IFS=$'\t' read -r REDIS_DESTINATION RECORDED_NEW_REDIS_MOUNT_SOURCE \
  <"$RUN_DIR/evidence/new-redis-mount.tsv"
test "$REDIS_DESTINATION" = '/data'
test "$RECORDED_NEW_REDIS_MOUNT_SOURCE" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
test "$REDIS_DATA_TARGET" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
test "$(cat "$RUN_DIR/evidence/new-redis-planned-source")" = \
  "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
test "$(docker inspect newapi-redis --format '{{json .Mounts}}' | jq -r \
  '[.[] | select(.Destination == "/data")] | length')" = 1
find "$REDIS_DATA_TARGET" -xdev -depth -mindepth 1 -delete
test -z "$(find "$REDIS_DATA_TARGET" -xdev -mindepth 1 -print -quit)"
tar -xzf "$RUN_DIR/prestage/redis-data.tar.gz" -C "$REDIS_DATA_TARGET"
docker start newapi-redis

export VERIFIER_MYSQL_CONTAINER="yucore-migration-${MIGRATION_ID}-mysql-verifier"
export VERIFIER_MYSQL_VOLUME="yucore-migration-${MIGRATION_ID}-mysql-verifier-data"
export VERIFIER_REDIS_CONTAINER="yucore-migration-${MIGRATION_ID}-redis-verifier"
export VERIFIER_REDIS_VOLUME="yucore-migration-${MIGRATION_ID}-redis-verifier-data"
export VERIFIER_MYSQL_ENV_FILE="$RUN_DIR/mysql-verifier.env"
export VERIFIER_REDIS_ENV_FILE="$RUN_DIR/redis-verifier.env"

cleanup_verifiers() {
  cleanup_status=0
  for verifier_spec in \
    "$VERIFIER_MYSQL_CONTAINER:mysql-verifier" \
    "$VERIFIER_REDIS_CONTAINER:redis-verifier"; do
    verifier_name="${verifier_spec%%:*}"
    verifier_role="${verifier_spec##*:}"
    case "$verifier_name" in
      "yucore-migration-${MIGRATION_ID}-${verifier_role%-verifier}-verifier") ;;
      *) cleanup_status=1; continue ;;
    esac
    if docker container inspect "$verifier_name" >/dev/null 2>&1; then
      if test "$(docker inspect "$verifier_name" --format \
          '{{index .Config.Labels "yucore.migration.id"}}')" = "$MIGRATION_ID" && \
         test "$(docker inspect "$verifier_name" --format \
          '{{index .Config.Labels "yucore.migration.role"}}')" = "$verifier_role"; then
        docker rm --force "$verifier_name" >/dev/null || cleanup_status=1
      else
        cleanup_status=1
      fi
    fi
  done
  for volume_spec in \
    "$VERIFIER_MYSQL_VOLUME:mysql-verifier" \
    "$VERIFIER_REDIS_VOLUME:redis-verifier"; do
    volume_name="${volume_spec%%:*}"
    volume_role="${volume_spec##*:}"
    case "$volume_name" in
      "yucore-migration-${MIGRATION_ID}-${volume_role%-verifier}-verifier-data") ;;
      *) cleanup_status=1; continue ;;
    esac
    if docker volume inspect "$volume_name" >/dev/null 2>&1; then
      if test "$(docker volume inspect "$volume_name" --format \
          '{{index .Labels "yucore.migration.id"}}')" = "$MIGRATION_ID" && \
         test "$(docker volume inspect "$volume_name" --format \
          '{{index .Labels "yucore.migration.role"}}')" = "$volume_role"; then
        docker volume rm "$volume_name" >/dev/null || cleanup_status=1
      else
        cleanup_status=1
      fi
    fi
  done
  for secret_file in "$VERIFIER_MYSQL_ENV_FILE" "$VERIFIER_REDIS_ENV_FILE"; do
    case "$secret_file" in
      "$RUN_DIR/mysql-verifier.env"|"$RUN_DIR/redis-verifier.env")
        rm -f -- "$secret_file" || cleanup_status=1 ;;
      *) cleanup_status=1 ;;
    esac
  done
  unset VERIFIER_MYSQL_PASSWORD
  return "$cleanup_status"
}
trap cleanup_verifiers EXIT INT TERM

test -z "$(docker ps -aq --filter "name=^/${VERIFIER_MYSQL_CONTAINER}$")"
test -z "$(docker ps -aq --filter "name=^/${VERIFIER_REDIS_CONTAINER}$")"
test -z "$(docker volume ls -q --filter "name=^${VERIFIER_MYSQL_VOLUME}$")"
test -z "$(docker volume ls -q --filter "name=^${VERIFIER_REDIS_VOLUME}$")"
export VERIFIER_MYSQL_IMAGE_ID="$(docker inspect newapi-mysql --format '{{.Image}}')"
export VERIFIER_REDIS_IMAGE_ID="$(docker inspect newapi-redis --format '{{.Image}}')"
printf '%s\n' "$VERIFIER_MYSQL_IMAGE_ID" "$VERIFIER_REDIS_IMAGE_ID" | \
  grep -E '^sha256:[0-9a-f]{64}$' | wc -l | grep -qx 2
test "$(docker image inspect "$VERIFIER_MYSQL_IMAGE_ID" --format '{{.Id}}')" = \
  "$VERIFIER_MYSQL_IMAGE_ID"
test "$(docker image inspect "$VERIFIER_REDIS_IMAGE_ID" --format '{{.Id}}')" = \
  "$VERIFIER_REDIS_IMAGE_ID"
printf 'mysql\t%s\nredis\t%s\n' \
  "$VERIFIER_MYSQL_IMAGE_ID" "$VERIFIER_REDIS_IMAGE_ID" \
  >"$RUN_DIR/evidence/verifier-image-ids.tsv"
sha256sum "$RUN_DIR/evidence/verifier-image-ids.tsv" \
  >"$RUN_DIR/evidence/verifier-image-ids.tsv.sha256"
sha256sum --check "$RUN_DIR/evidence/verifier-image-ids.tsv.sha256"
export VERIFIER_MYSQL_DATABASE="$(docker exec newapi-mysql sh -lc \
  'test -n "$MYSQL_DATABASE"; printf "%s" "$MYSQL_DATABASE"')"
export VERIFIER_MYSQL_PASSWORD="$(openssl rand -hex 32)"
printf 'MYSQL_ROOT_PASSWORD=%s\nMYSQL_DATABASE=%s\n' \
  "$VERIFIER_MYSQL_PASSWORD" "$VERIFIER_MYSQL_DATABASE" >"$VERIFIER_MYSQL_ENV_FILE"
docker inspect newapi-redis --format '{{range .Config.Env}}{{println .}}{{end}}' \
  >"$VERIFIER_REDIS_ENV_FILE"
chmod 0600 "$VERIFIER_MYSQL_ENV_FILE" "$VERIFIER_REDIS_ENV_FILE"
test "$(docker inspect newapi-redis --format '{{json .Mounts}}' | jq -r \
  '[.[] | select(.Destination != "/data")] | length')" = 0
mapfile -d '' -t VERIFIER_REDIS_CMD < <(docker inspect newapi-redis \
  --format '{{json .Config.Cmd}}' | jq -jr '.[]? | ., "\u0000"')
docker volume create \
  --label "yucore.migration.id=$MIGRATION_ID" \
  --label 'yucore.migration.role=mysql-verifier' \
  "$VERIFIER_MYSQL_VOLUME" >/dev/null
docker volume create \
  --label "yucore.migration.id=$MIGRATION_ID" \
  --label 'yucore.migration.role=redis-verifier' \
  "$VERIFIER_REDIS_VOLUME" >/dev/null
export VERIFIER_REDIS_DATA="$(docker volume inspect "$VERIFIER_REDIS_VOLUME" \
  --format '{{.Mountpoint}}')"
test -z "$(find "$VERIFIER_REDIS_DATA" -xdev -mindepth 1 -print -quit)"
tar -xzf "$RUN_DIR/prestage/redis-data.tar.gz" -C "$VERIFIER_REDIS_DATA"
docker run -d --name "$VERIFIER_MYSQL_CONTAINER" --restart no --network none \
  --label "yucore.migration.id=$MIGRATION_ID" \
  --label 'yucore.migration.role=mysql-verifier' \
  --env-file "$VERIFIER_MYSQL_ENV_FILE" \
  --mount "type=volume,source=$VERIFIER_MYSQL_VOLUME,target=/var/lib/mysql" \
  "$VERIFIER_MYSQL_IMAGE_ID"
docker run -d --name "$VERIFIER_REDIS_CONTAINER" --restart no --network none \
  --label "yucore.migration.id=$MIGRATION_ID" \
  --label 'yucore.migration.role=redis-verifier' \
  --env-file "$VERIFIER_REDIS_ENV_FILE" \
  --mount "type=volume,source=$VERIFIER_REDIS_VOLUME,target=/data" \
  "$VERIFIER_REDIS_IMAGE_ID" "${VERIFIER_REDIS_CMD[@]}"
for attempt in $(seq 1 90); do
  if docker exec "$VERIFIER_MYSQL_CONTAINER" sh -lc '
    export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"
    mysqladmin -uroot --silent ping
  '; then break; fi
  sleep 1
done
test "$(docker inspect "$VERIFIER_MYSQL_CONTAINER" --format '{{.State.Running}}')" = true
for attempt in $(seq 1 60); do
  if docker exec "$VERIFIER_REDIS_CONTAINER" sh -lc '
    if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
    redis-cli --no-auth-warning PING
  ' | grep -qx PONG; then break; fi
  sleep 1
done
test "$(docker inspect "$VERIFIER_REDIS_CONTAINER" --format '{{.State.Running}}')" = true
verify_migration_helpers
/opt/newapi/releases/restore-mysql-snapshot \
  prestage "$RUN_DIR/prestage" "$VERIFIER_MYSQL_CONTAINER"

verify_migration_helpers
MYSQL_CONTAINER_NAME="$VERIFIER_MYSQL_CONTAINER" \
  REDIS_CONTAINER_NAME="$VERIFIER_REDIS_CONTAINER" ACCEPTED_COMMIT="$ACCEPTED_COMMIT" \
  /opt/newapi/releases/capture-state-manifest \
  "$RUN_DIR/evidence/source_snapshot.json"
verify_migration_helpers
ACCEPTED_COMMIT="$ACCEPTED_COMMIT" \
  /opt/newapi/releases/capture-state-manifest \
  "$RUN_DIR/evidence/target_snapshot.json"
verify_migration_helpers
python3 /opt/newapi/releases/compare-state-snapshots \
  --source "$RUN_DIR/evidence/source_snapshot.json" \
  --target "$RUN_DIR/evidence/target_snapshot.json" --tolerance-ms 2000

cleanup_verifiers
test -z "$(docker ps -aq --filter "name=^/${VERIFIER_MYSQL_CONTAINER}$")"
test -z "$(docker ps -aq --filter "name=^/${VERIFIER_REDIS_CONTAINER}$")"
test -z "$(docker volume ls -q --filter "name=^${VERIFIER_MYSQL_VOLUME}$")"
test -z "$(docker volume ls -q --filter "name=^${VERIFIER_REDIS_VOLUME}$")"
test ! -e "$VERIFIER_MYSQL_ENV_FILE"
test ! -e "$VERIFIER_REDIS_ENV_FILE"
trap - EXIT INT TERM

docker compose -f /opt/newapi/docker-compose.yml -f "$RUN_DIR/candidate.yml" \
  up -d newapi
for attempt in $(seq 1 60); do
  test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy && break
  sleep 1
done
test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy
test "$(docker inspect newapi --format '{{.RestartCount}}')" = 0
test "$(docker inspect newapi --format '{{.Image}}')" = \
  "$(cat "/opt/newapi/releases/newapi-yucore-${ACCEPTED_COMMIT}-linux-amd64.tar.image-id")"

docker compose -f /opt/newapi/docker-compose.yml -f "$RUN_DIR/candidate.yml" \
  config >"$RUN_DIR/candidate.rendered.yml"
chmod 0600 "$RUN_DIR/candidate.rendered.yml"
python3 /opt/newapi/releases/yucore_migration_guard.py runtime-preflight \
  --container newapi --compose-file "$RUN_DIR/candidate.rendered.yml" --service newapi
```

Start the new Caddy only after the new provider firewall allows 80/443 from the
old server IP and still denies the public Internet:

```bash
docker compose -f /opt/edge/docker-compose.yml config --quiet
docker compose -f /opt/edge/docker-compose.yml up -d
test "$(docker inspect yuapi-caddy --format '{{.State.Status}}')" = running
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
```

## Private Acceptance Before Maintenance

Run from `OLD`, where the new firewall permits access. Each probe pins the real
hostname to the new origin without changing DNS:

```bash
set -euo pipefail
set +x
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error --connect-timeout 5 --max-time 30 \
    --resolve "$hostname:443:$NEW_HOST" \
    "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

Read disposable validation credentials without echo. Do not save them in a
file or shell history:

```bash
read -rsp 'Disposable downstream API key: ' RELAY_KEY; printf '\n'
read -rsp 'Authorized user session or PAT: ' USER_PAT; printf '\n'
read -rsp 'Disposable administrator PAT: ' ADMIN_PAT; printf '\n'
read -rp 'Known private group: ' PRIVATE_GROUP
read -rp 'Public mapped model: ' MAPPED_PUBLIC_MODEL
read -rp 'Small ordinary chat model: ' CHAT_MODEL
read -rp 'Small image model: ' IMAGE_MODEL
read -rp 'Small video model: ' VIDEO_MODEL
```

Complete and record every private check before maintenance:

1. Anonymous desktop/mobile, both themes, login, registration, pricing, and
   public model discovery render through `curl --resolve` or a local hosts-file
   override.
2. Existing user login, profile, wallet, usage logs, tasks, public groups, and
   authorized private-group discovery work; an anonymous user cannot discover
   the private group.
3. Administrator users, channels, account pools, provider accounts, mappings,
   billing settings, backup status, and system settings render without a new
   `500` response.
4. Ordinary and streaming chat complete through the resolved API hostname.
5. The client-visible mapped model remains the public name. Ordinary user
   responses, logs, and tasks contain neither upstream routing fields nor
   mapped-model flags; administrator diagnostics retain the routing evidence.
6. Fixed-price, per-call image/video, and expression prices use the public
   requested model. Pre-consume, final settlement, and refund each occur once.
7. The smallest accepted image and video probes complete through the resolved
   VIP hostname using disposable funds.
8. SMTP registration, Turnstile hostname acceptance, and supported OAuth
   callbacks work for the unchanged production hostnames.
9. MySQL table counts, maximum IDs, option hashes, Redis key counts, and file
   manifests match the pre-stage source snapshot.

Use exact API bodies built with `jq`; do not interpolate JSON by hand:

```bash
CHAT_BODY="$(jq -nc --arg model "$CHAT_MODEL" \
  '{model:$model,messages:[{role:"user",content:"Reply only OK"}],max_tokens:8,stream:false}')"
curl --fail --silent --show-error \
  --resolve "api.yuaiapi.com:443:$NEW_HOST" \
  -H "Authorization: Bearer $RELAY_KEY" -H 'Content-Type: application/json' \
  --data "$CHAT_BODY" https://api.yuaiapi.com/v1/chat/completions | \
  jq -e '.choices | length > 0'

STREAM_BODY="$(jq -nc --arg model "$CHAT_MODEL" \
  '{model:$model,messages:[{role:"user",content:"Reply only OK"}],max_tokens:8,stream:true}')"
curl --fail --silent --show-error --no-buffer \
  --resolve "api.yuaiapi.com:443:$NEW_HOST" \
  -H "Authorization: Bearer $RELAY_KEY" -H 'Content-Type: application/json' \
  --data "$STREAM_BODY" https://api.yuaiapi.com/v1/chat/completions | grep -q 'data:'
```

The accepted image remains a slave during pre-stage acceptance. Do not enable
scheduled master work or public ingress.

## Maintenance Mutation Gate

Immediately before maintenance, re-enter the user authorization and exact
confirmation in `OLD`. Also verify direct SSH from old to new and all immutable
image metadata:

```bash
set -euo pipefail
set +x
read -rp 'Production execution authorization: ' PRODUCTION_AUTHORIZATION
read -rp 'Maintenance confirmation: ' MAINTENANCE_CONFIRMATION
test "$PRODUCTION_AUTHORIZATION" = 'USER-AUTHORIZED-PRODUCTION-EXECUTION'
(cd /opt/newapi/releases && sha256sum --check yucore_migration_guard.py.sha256)
python3 /opt/newapi/releases/yucore_migration_guard.py confirm \
  --new-host "$NEW_HOST" --confirmation "$MAINTENANCE_CONFIRMATION"
ssh -p "$NEW_SSH_PORT" "$NEW_USER@$NEW_HOST" true
(cd /opt/newapi/releases && \
  sha256sum --check "newapi-yucore-${ACCEPTED_COMMIT}-linux-amd64.tar.sha256")
test "$(docker image inspect "$ACCEPTED_IMAGE" --format '{{.Id}}')" = \
  "$(cat "/opt/newapi/releases/newapi-yucore-${ACCEPTED_COMMIT}-linux-amd64.tar.image-id")"
export OLD_IMAGE_REF="$(docker inspect newapi --format '{{.Config.Image}}')"
export OLD_IMAGE_ID="$(docker inspect newapi --format '{{.Image}}')"
printf '%s\n' "$OLD_IMAGE_REF" >"$RUN_DIR/evidence/old-image-ref"
printf '%s\n' "$OLD_IMAGE_ID" >"$RUN_DIR/evidence/old-image-id"
```

Record the user, UTC timestamp, accepted commit, old image ID, and new image ID
in the protected evidence directory. Do not record credentials.

## Enter Maintenance On The Old Origin

In `OLD`, start a dedicated Nginx maintenance upstream on the existing
application network. It returns `503` and `Retry-After: 180`:

```bash
set -euo pipefail
set +x
umask 077
export APP_NETWORK="$(docker inspect newapi --format '{{range $name, $value := .NetworkSettings.Networks}}{{println $name}}{{end}}' | head -n1)"
test -n "$APP_NETWORK"
test -z "$(docker ps -aq --filter 'name=^/yucore-migration-maintenance$')"

install -d -m 0700 "$RUN_DIR/maintenance"
cat >"$RUN_DIR/maintenance/default.conf" <<'NGINX'
server {
    listen 8080;
    server_name _;
    location / {
        add_header Retry-After 180 always;
        return 503;
    }
}
NGINX

docker run -d --name yucore-migration-maintenance --restart no \
  --network "$APP_NETWORK" \
  -v "$RUN_DIR/maintenance/default.conf:/etc/nginx/conf.d/default.conf:ro" \
  nginx:1.27-alpine
for attempt in $(seq 1 30); do
  docker exec yuapi-caddy wget -q -S -O /dev/null \
    http://yucore-migration-maintenance:8080/ 2>&1 | grep -q '503' && break
  sleep 1
done

cp -a /opt/edge/Caddyfile "$RUN_DIR/evidence/Caddyfile.before-maintenance"
export CADDY_NEXT="$RUN_DIR/maintenance/Caddyfile"
export FROM_UPSTREAM='newapi:3000'
export TO_UPSTREAM='yucore-migration-maintenance:8080'
export UPSTREAM_COUNT="$(grep -Fc "$FROM_UPSTREAM" /opt/edge/Caddyfile)"
test "$UPSTREAM_COUNT" -ge 1
cp -a /opt/edge/Caddyfile "$CADDY_NEXT"
FROM_UPSTREAM="$FROM_UPSTREAM" TO_UPSTREAM="$TO_UPSTREAM" \
  perl -0pi -e 's/\Q$ENV{FROM_UPSTREAM}\E/$ENV{TO_UPSTREAM}/g' "$CADDY_NEXT"
test "$(grep -Fc "$TO_UPSTREAM" "$CADDY_NEXT")" = "$UPSTREAM_COUNT"
test "$(grep -Fc "$FROM_UPSTREAM" "$CADDY_NEXT")" = 0
docker cp "$CADDY_NEXT" yuapi-caddy:/tmp/Caddyfile.next
docker exec yuapi-caddy caddy validate --config /tmp/Caddyfile.next
cp "$CADDY_NEXT" /opt/edge/Caddyfile
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec yuapi-caddy caddy reload --config /etc/caddy/Caddyfile

for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  status="$(curl --silent --output /dev/null --write-out '%{http_code}' "https://$hostname/")"
  test "$status" = 503
done
```

After Caddy serves maintenance, give the old application exactly five seconds
to exit and let Docker force termination if required:

```bash
docker stop --time 5 newapi
test "$(docker inspect newapi --format '{{.State.Running}}')" = false
```

MySQL, Redis, Caddy, and maintenance Nginx stay running for final capture.

## Capture And Transfer The Final Old-Authoritative State

Run in `OLD`. With the application already stopped, complete MySQL export,
force Redis `SAVE`, and only then capture source evidence while Redis is still
queryable but has no writer. Stop Redis immediately afterward and archive the
same persisted root. This orders the source manifest consistently with the RDB
artifact before any target master starts:

```bash
set -euo pipefail
set +x
umask 077
install -d -m 0700 "$RUN_DIR/final" "$RUN_DIR/evidence"

verify_migration_helpers
/opt/newapi/releases/export-mysql-snapshot final "$RUN_DIR/final"
cp "$RUN_DIR/final/yucore-final.sql.bytes" \
  "$RUN_DIR/evidence/mysql-uncompressed-bytes"
rsync -aHAX --numeric-ids --delete /opt/newapi/data/ "$RUN_DIR/final/app-data/"
rsync -aHAX --numeric-ids --delete /opt/edge/ "$RUN_DIR/final/edge/"

docker exec newapi-redis sh -lc '
  set -eu
  set +x
  if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
  redis-cli --no-auth-warning SAVE >/dev/null
'
verify_migration_helpers
ACCEPTED_COMMIT="$ACCEPTED_COMMIT" \
  /opt/newapi/releases/capture-state-manifest "$RUN_DIR/evidence/source-final.json"
docker stop --time 5 newapi-redis
export REDIS_DATA_SOURCE="$(docker inspect newapi-redis --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}')"
sha256sum --check "$RUN_DIR/evidence/old-redis-mount.tsv.sha256"
IFS=$'\t' read -r REDIS_DESTINATION RECORDED_OLD_REDIS_MOUNT_SOURCE \
  <"$RUN_DIR/evidence/old-redis-mount.tsv"
test "$REDIS_DESTINATION" = '/data'
test "$RECORDED_OLD_REDIS_MOUNT_SOURCE" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
test "$REDIS_DATA_SOURCE" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
tar -C "$REDIS_DATA_SOURCE" -czf "$RUN_DIR/final/redis-data.tar.gz" .
(cd "$RUN_DIR/final" && find . -type f -print0 | LC_ALL=C sort -z | \
  xargs -0 sha256sum >"$RUN_DIR/evidence/final-files.sha256")
tar -C "$RUN_DIR/final" -czf "$RUN_DIR/final.tar.gz" .
sha256sum "$RUN_DIR/final.tar.gz" >"$RUN_DIR/final.tar.gz.sha256"
```

Transfer directly to `NEW` over SSH, then verify there before extraction:

```bash
# OLD
ssh -p "$NEW_SSH_PORT" "$NEW_USER@$NEW_HOST" "install -d -m 0700 '$RUN_DIR'"
scp -q -P "$NEW_SSH_PORT" \
  "$RUN_DIR/final.tar.gz" "$RUN_DIR/final.tar.gz.sha256" \
  "$RUN_DIR/evidence/source-final.json" "$RUN_DIR/evidence/final-files.sha256" \
  "$NEW_USER@$NEW_HOST:$RUN_DIR/"

# NEW
cd "$RUN_DIR"
sha256sum --check final.tar.gz.sha256
install -d -m 0700 "$RUN_DIR/final" "$RUN_DIR/evidence"
tar -xzf final.tar.gz -C "$RUN_DIR/final"
mv "$RUN_DIR/source-final.json" "$RUN_DIR/evidence/source-final.json"
mv "$RUN_DIR/final-files.sha256" "$RUN_DIR/evidence/final-files.sha256"
(cd "$RUN_DIR/final" && sha256sum --check "$RUN_DIR/evidence/final-files.sha256")
```

The old MySQL remains running but frozen. The old Redis and application remain
stopped. The old Caddy continues returning maintenance.

## Final Restore And Master Start On The New Server

Run in `NEW`. Stop the private candidate before replacing the database and
Redis state:

```bash
set -euo pipefail
set +x
umask 077

docker stop --time 5 newapi
docker stop --time 5 yuapi-caddy
verify_migration_helpers
/opt/newapi/releases/restore-mysql-snapshot final "$RUN_DIR/final"

docker stop --time 5 newapi-redis
export REDIS_DATA_TARGET="$(docker inspect newapi-redis --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}')"
sha256sum --check "$RUN_DIR/evidence/new-redis-mount.tsv.sha256"
sha256sum --check "$RUN_DIR/evidence/new-redis-planned-source.sha256"
IFS=$'\t' read -r REDIS_DESTINATION RECORDED_NEW_REDIS_MOUNT_SOURCE \
  <"$RUN_DIR/evidence/new-redis-mount.tsv"
test "$REDIS_DESTINATION" = '/data'
test "$RECORDED_NEW_REDIS_MOUNT_SOURCE" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
test "$REDIS_DATA_TARGET" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
test "$(cat "$RUN_DIR/evidence/new-redis-planned-source")" = \
  "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
test "$(docker inspect newapi-redis --format '{{json .Mounts}}' | jq -r \
  '[.[] | select(.Destination == "/data")] | length')" = 1
find "$REDIS_DATA_TARGET" -xdev -depth -mindepth 1 -delete
test -z "$(find "$REDIS_DATA_TARGET" -xdev -mindepth 1 -print -quit)"
tar -xzf "$RUN_DIR/final/redis-data.tar.gz" -C "$REDIS_DATA_TARGET"
docker start newapi-redis

rsync -aHAX --numeric-ids --delete "$RUN_DIR/final/app-data/" /opt/newapi/data/
rsync -aHAX --numeric-ids --delete "$RUN_DIR/final/edge/" /opt/edge/

for attempt in $(seq 1 60); do
  test "$(docker inspect newapi-mysql --format '{{.State.Health.Status}}')" = healthy && \
    test "$(docker inspect newapi-redis --format '{{.State.Status}}')" = running && break
  sleep 1
done
test "$(docker inspect newapi-mysql --format '{{.State.Health.Status}}')" = healthy
test "$(docker inspect newapi-redis --format '{{.State.Status}}')" = running

verify_migration_helpers
ACCEPTED_COMMIT="$ACCEPTED_COMMIT" \
  /opt/newapi/releases/capture-state-manifest "$RUN_DIR/evidence/target-final-before-start.json"
verify_migration_helpers
python3 /opt/newapi/releases/compare-state-snapshots \
  --source "$RUN_DIR/evidence/source-final.json" \
  --target "$RUN_DIR/evidence/target-final-before-start.json" \
  --tolerance-ms 2000

cp "$RUN_DIR/deployment/edge/Caddyfile" /opt/edge/Caddyfile
docker start yuapi-caddy
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec yuapi-caddy caddy reload --config /etc/caddy/Caddyfile
```

The manifest comparison is a hard gate. Only after it passes, recreate the
application from the base Compose file plus the accepted image override. The
slave override is deliberately absent, so this is the single final master:

```bash
docker compose -f /opt/newapi/docker-compose.yml -f "$RUN_DIR/image-only.yml" \
  config >"$RUN_DIR/final-master.rendered.yml"
chmod 0600 "$RUN_DIR/final-master.rendered.yml"
docker compose -f /opt/newapi/docker-compose.yml -f "$RUN_DIR/image-only.yml" \
  up -d --no-deps --force-recreate newapi

for attempt in $(seq 1 90); do
  test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy && break
  sleep 1
done
test "$(docker inspect newapi --format '{{.State.Status}}')" = running
test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy
test "$(docker inspect newapi --format '{{.RestartCount}}')" = 0
test "$(docker inspect newapi --format '{{.Image}}')" = \
  "$(cat "/opt/newapi/releases/newapi-yucore-${ACCEPTED_COMMIT}-linux-amd64.tar.image-id")"
python3 /opt/newapi/releases/yucore_migration_guard.py runtime-preflight \
  --container newapi --compose-file "$RUN_DIR/final-master.rendered.yml" --service newapi
if test -d "$RUN_DIR/prestage/systemd"; then
  for timer_file in "$RUN_DIR/prestage/systemd/"*.timer; do
    test -f "$timer_file" || continue
    systemctl start "$(basename "$timer_file")"
  done
fi
```

Repeat the complete private anonymous/user/admin, mapped-model privacy,
ordinary relay, streaming relay, SMTP, Turnstile, OAuth, and smallest paid
image/video acceptance set through `curl --resolve`. Public ingress remains
blocked until every check passes. A restart, migration error, manifest drift,
unexpected charge/refund, private-group leak, or new `500` invokes the
pre-traffic rollback.

## Bridge The Old Origin To The New Origin

Before changing Cloudflare, replace old maintenance routing with a four-host
TLS bridge. This catches stale Cloudflare origin routing and cached VIP DNS.
Run in `OLD` after the new final master passes all private gates:

```bash
set -euo pipefail
set +x
umask 077
: "${NEW_HOST:?set NEW_HOST}"

cat >"$RUN_DIR/maintenance/Caddyfile.bridge-new" <<EOF
yuaiapi.com {
    reverse_proxy https://$NEW_HOST:443 {
        header_up Host yuaiapi.com
        transport http { tls_server_name yuaiapi.com }
    }
}
api.yuaiapi.com {
    reverse_proxy https://$NEW_HOST:443 {
        header_up Host api.yuaiapi.com
        transport http { tls_server_name api.yuaiapi.com }
    }
}
global.yuaiapi.com {
    reverse_proxy https://$NEW_HOST:443 {
        header_up Host global.yuaiapi.com
        transport http { tls_server_name global.yuaiapi.com }
    }
}
vip.yuaiapi.com {
    reverse_proxy https://$NEW_HOST:443 {
        header_up Host vip.yuaiapi.com
        transport http { tls_server_name vip.yuaiapi.com }
    }
}
EOF
docker cp "$RUN_DIR/maintenance/Caddyfile.bridge-new" yuapi-caddy:/tmp/Caddyfile.next
docker exec yuapi-caddy caddy validate --config /tmp/Caddyfile.next
test ! -e "$BRIDGE_BOUNDARY_MARKER"
test ! -e "$BRIDGE_BOUNDARY_MARKER_SHA256"
jq -n \
  --arg migration_id "$MIGRATION_ID" \
  --arg accepted_commit "$ACCEPTED_COMMIT" \
  --arg old_origin "$OLD_HOST" \
  --arg new_origin "$NEW_HOST" \
  --arg created_at "$(date -u --iso-8601=seconds)" \
  '{migration_id:$migration_id,accepted_commit:$accepted_commit,
    old_origin:$old_origin,new_origin:$new_origin,
    authority:"new",boundary:"before-old-caddy-bridge",created_at:$created_at}' \
  >"$BRIDGE_BOUNDARY_MARKER"
(cd "$(dirname "$BRIDGE_BOUNDARY_MARKER")" && \
  sha256sum "$(basename "$BRIDGE_BOUNDARY_MARKER")" \
  >"$(basename "$BRIDGE_BOUNDARY_MARKER_SHA256")")
(cd "$(dirname "$BRIDGE_BOUNDARY_MARKER")" && \
  sha256sum --check "$(basename "$BRIDGE_BOUNDARY_MARKER_SHA256")")
cp "$RUN_DIR/maintenance/Caddyfile.bridge-new" /opt/edge/Caddyfile
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec yuapi-caddy caddy reload --config /etc/caddy/Caddyfile

for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

Immediately copy the already-created boundary evidence to protected `CONTROL`
storage and verify the basename checksum there. The copy does not define the
boundary; the hashed `OLD` marker created immediately before Caddy activation
does:

```bash
# CONTROL
export CONTROL_BOUNDARY_DIR="$RELEASE_DIR/boundaries/$MIGRATION_ID"
export BRIDGE_BOUNDARY_BASENAME='new-write-authority-boundary.json'
export BRIDGE_BOUNDARY_SHA_BASENAME="$BRIDGE_BOUNDARY_BASENAME.sha256"
export REMOTE_BRIDGE_BOUNDARY_MARKER="/opt/newapi/migration/$MIGRATION_ID/evidence/$BRIDGE_BOUNDARY_BASENAME"
export REMOTE_BRIDGE_BOUNDARY_MARKER_SHA256="$REMOTE_BRIDGE_BOUNDARY_MARKER.sha256"
install -d -m 0700 "$CONTROL_BOUNDARY_DIR"
scp -q -P "$OLD_SSH_PORT" \
  "$OLD_USER@$OLD_HOST:$REMOTE_BRIDGE_BOUNDARY_MARKER" \
  "$OLD_USER@$OLD_HOST:$REMOTE_BRIDGE_BOUNDARY_MARKER_SHA256" \
  "$CONTROL_BOUNDARY_DIR/"
(cd "$CONTROL_BOUNDARY_DIR" && \
  sha256sum --check "$BRIDGE_BOUNDARY_SHA_BASENAME")
jq -e --arg migration_id "$MIGRATION_ID" --arg commit "$ACCEPTED_COMMIT" '
  .migration_id == $migration_id and .accepted_commit == $commit and
  .authority == "new" and .boundary == "before-old-caddy-bridge"' \
  "$CONTROL_BOUNDARY_DIR/$BRIDGE_BOUNDARY_BASENAME" >/dev/null
```

Do not stop old Caddy or remove maintenance Nginx. There is never an
active-active database period: every request reaching the old IP now writes to
the single new master. The successful old-bridge reload is the authoritative
write boundary: from that instant onward, use the post-traffic reverse data
migration for every rollback, even before firewall or DNS changes.

## Transition The Provider Firewall To Public Ingress

UFW cannot open a port that the hosting-provider firewall still blocks. Before
the host-firewall commands below, deliberately change the provider firewall
from staging access (TCP 80/443 from the exact old-origin IPv4 only) to public
TCP 80/443. Because the DNS-only VIP shares the origin, public means
`0.0.0.0/0` and, when the reviewed IPv6 policy is enabled, `::/0`. SSH remains
restricted to the operator source; ports 3000, 3001, 3306, and 6379 remain
unpublished. Do not change DNS in this step.

Use the provider console or its separately reviewed provider-specific API
workflow to make exactly that transition. Export the provider's applied-state
result as normalized JSON on `CONTROL` with fields `change_id`, `status`,
`ipv6_policy`, `http_https.ports`, `http_https.sources`, and
`publicly_denied_ports`. A second operator reviews that export out of band and
supplies its SHA-256. These commands fail closed unless the boundary marker and
the exact provider result both validate:

```bash
# CONTROL
: "${PROVIDER_FIREWALL_PUBLIC_EVIDENCE:?set provider applied-state JSON path}"
: "${APPROVED_PROVIDER_FIREWALL_PUBLIC_SHA256:?set second-operator approved SHA-256}"
: "${NEW_IPV6_POLICY:?set enabled or disabled}"
case "$NEW_IPV6_POLICY" in enabled|disabled) ;; *) exit 1 ;; esac
(cd "$CONTROL_BOUNDARY_DIR" && \
  sha256sum --check "$BRIDGE_BOUNDARY_SHA_BASENAME")
test "$(sha256sum "$PROVIDER_FIREWALL_PUBLIC_EVIDENCE" | cut -d' ' -f1)" = \
  "$APPROVED_PROVIDER_FIREWALL_PUBLIC_SHA256"
jq -e --arg old_source "$OLD_HOST/32" --arg ipv6 "$NEW_IPV6_POLICY" '
  .status == "applied" and (.change_id | type == "string" and length > 0) and
  .ipv6_policy == $ipv6 and .http_https.ports == [80,443] and
  (.publicly_denied_ports | sort) == [3000,3001,3306,6379] and
  (if $ipv6 == "enabled" then
     (.http_https.sources | sort) == ["0.0.0.0/0","::/0"]
   else
     .http_https.sources == ["0.0.0.0/0"]
   end) and
  .staging_before.ports == [80,443] and
  .staging_before.sources == [$old_source]' \
  "$PROVIDER_FIREWALL_PUBLIC_EVIDENCE" >/dev/null
read -rp 'Provider firewall transition confirmation: ' PROVIDER_FIREWALL_CONFIRMATION
test "$PROVIDER_FIREWALL_CONFIRMATION" = \
  'PROVIDER-FIREWALL-PUBLIC-80-443-APPLIED-AND-REVIEWED'
install -m 0600 "$PROVIDER_FIREWALL_PUBLIC_EVIDENCE" \
  "$CONTROL_BOUNDARY_DIR/provider-firewall-public.json"
printf '%s  %s\n' "$APPROVED_PROVIDER_FIREWALL_PUBLIC_SHA256" \
  'provider-firewall-public.json' \
  >"$CONTROL_BOUNDARY_DIR/provider-firewall-public.json.sha256"
(cd "$CONTROL_BOUNDARY_DIR" && \
  sha256sum --check provider-firewall-public.json.sha256)
```

Only after this provider evidence passes may `NEW` UFW be opened. The external
direct-origin probe after the UFW change proves both firewall layers admit the
new origin before any public DNS update.

## Deliberately Open And Verify New-Origin Ingress

The old bridge must be live before this section. The proxied hostnames and the
DNS-only VIP share one origin IPv4 address, so a source-IP-only firewall cannot
restrict that address to Cloudflare while also accepting direct VIP clients.
The reviewed rule set therefore opens only TCP 80/443 to all clients, keeps SSH
under the separately reviewed operator rule, and publishes no application,
MySQL, or Redis port. Do not substitute a broader service rule.

Run in `NEW`, record the exact before/after rules, and fail if UFW or the
expected container/listener topology differs:

```bash
set -euo pipefail
set +x
umask 077
declare -F require_remote_context >/dev/null
require_remote_context
test "$(ufw status | sed -n '1s/^Status: //p')" = active
ufw status numbered >"$RUN_DIR/evidence/ufw-before-public-ingress.txt"
sha256sum "$RUN_DIR/evidence/ufw-before-public-ingress.txt" \
  >"$RUN_DIR/evidence/ufw-before-public-ingress.txt.sha256"

test "$(docker inspect yuapi-caddy --format '{{.State.Status}}')" = running
test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy
test "$(docker inspect newapi --format '{{.RestartCount}}')" = 0
if test "$NEW_IPV6_POLICY" = enabled; then
  grep -Eq '^IPV6=yes$' /etc/default/ufw
else
  grep -Eq '^IPV6=no$' /etc/default/ufw
fi
test -z "$(ss -H -lnt | awk '
  $4 ~ /:(3000|3001|3306|6379)$/ &&
  $4 !~ /^(127\.0\.0\.1|\[::1\]):/ {print; exit}')"

ufw allow 80/tcp comment 'yucore-migration-public-http'
ufw allow 443/tcp comment 'yucore-migration-public-https-and-direct-vip'
ufw status numbered >"$RUN_DIR/evidence/ufw-after-public-ingress.txt"
grep -Fq '80/tcp' "$RUN_DIR/evidence/ufw-after-public-ingress.txt"
grep -Fq '443/tcp' "$RUN_DIR/evidence/ufw-after-public-ingress.txt"
grep -Fq 'yucore-migration-public-http' "$RUN_DIR/evidence/ufw-after-public-ingress.txt"
grep -Fq 'yucore-migration-public-https-and-direct-vip' \
  "$RUN_DIR/evidence/ufw-after-public-ingress.txt"
if test "$NEW_IPV6_POLICY" = enabled; then
  grep -Eiq '80/tcp.*\(v6\)' "$RUN_DIR/evidence/ufw-after-public-ingress.txt"
  grep -Eiq '443/tcp.*\(v6\)' "$RUN_DIR/evidence/ufw-after-public-ingress.txt"
fi
sha256sum "$RUN_DIR/evidence/ufw-after-public-ingress.txt" \
  >"$RUN_DIR/evidence/ufw-after-public-ingress.txt.sha256"
date -u --iso-8601=seconds >"$RUN_DIR/evidence/new-ingress-opened-at"
```

Before any DNS API call, verify both paths from `OLD`: first the old-origin
bridge, then the new origin directly. Repeat the direct new-origin probe from
an external `CONTROL` network:

```bash
# OLD
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error --resolve "$hostname:443:$OLD_HOST" \
    "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
  curl --fail --silent --show-error --resolve "$hostname:443:$NEW_HOST" \
    "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done

# CONTROL
(cd "$(dirname "$GUARD_ARTIFACT")" && \
  sha256sum --check "$(basename "$GUARD_ARTIFACT").sha256")
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error --resolve "$hostname:443:$NEW_HOST" \
    "https://$hostname/api/status" | python3 "$GUARD_ARTIFACT" validate-status
done
if test "$NEW_IPV6_POLICY" = enabled; then
  : "${NEW_IPV6_HOST:?set reviewed new origin IPv6}"
  for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
    curl --fail --silent --show-error --resolve "$hostname:443:[$NEW_IPV6_HOST]" \
      "https://$hostname/api/status" | python3 "$GUARD_ARTIFACT" validate-status
  done
fi
```

If ingress or either path fails, do not call Cloudflare. The old bridge has
already made new MySQL authoritative, so run the post-traffic reverse data
migration and restore old authority. Keep the new ingress for the new-to-old
rollback bridge, then remove exactly the named UFW allowances with the
post-traffic cleanup block after its two-TTL bridge interval. Pre-traffic
rollback is permitted only before the old bridge reload succeeds.

## Snapshot, Plan, Review, And Apply Cloudflare A Records

Use a newly issued temporary token with only Zone DNS Edit, Zone Settings Read,
and Rulesets Read on the one production zone. Run on `CONTROL` with tracing
disabled. Read the token silently and never write or print it:

```bash
set -euo pipefail
set +x
umask 077
: "${NEW_HOST:?set NEW_HOST}"
: "${CF_ZONE_ID:?set CF_ZONE_ID}"
(cd "$CONTROL_BOUNDARY_DIR" && \
  sha256sum --check "$BRIDGE_BOUNDARY_SHA_BASENAME" && \
  sha256sum --check provider-firewall-public.json.sha256)
jq -e '.authority == "new" and .boundary == "before-old-caddy-bridge"' \
  "$CONTROL_BOUNDARY_DIR/$BRIDGE_BOUNDARY_BASENAME" >/dev/null
export CF_SNAPSHOT_DIR="$(mktemp -d)"
chmod 0700 "$CF_SNAPSHOT_DIR"
export CF_CURL_CONFIG="$CF_SNAPSHOT_DIR/curl-auth.conf"
cleanup_cf_auth() {
  case "$CF_CURL_CONFIG" in "$CF_SNAPSHOT_DIR/curl-auth.conf") ;; *) return 1 ;; esac
  unset CF_API_TOKEN
  rm -f -- "$CF_CURL_CONFIG"
}
trap cleanup_cf_auth EXIT INT TERM
read -rsp 'Temporary least-privilege Cloudflare token: ' CF_API_TOKEN; printf '\n'
case "$CF_API_TOKEN" in ''|*[!A-Za-z0-9._-]*) exit 1 ;; esac
set +x
printf 'header = "Authorization: Bearer %s"\nheader = "Content-Type: application/json"\n' \
  "$CF_API_TOKEN" >"$CF_CURL_CONFIG"
chmod 0600 "$CF_CURL_CONFIG"
unset CF_API_TOKEN

cf_get() {
  endpoint="$1"
  output="$2"
  curl --config "$CF_CURL_CONFIG" --fail --silent --show-error \
    "https://api.cloudflare.com/client/v4$endpoint" >"$output"
  jq -e '.success == true' "$output" >/dev/null
}

cf_get "/zones/$CF_ZONE_ID/dns_records?per_page=500" \
  "$CF_SNAPSHOT_DIR/dns-before-response.json"
jq -e '.result' "$CF_SNAPSHOT_DIR/dns-before-response.json" \
  >"$CF_SNAPSHOT_DIR/dns-before.json"
cf_get "/zones/$CF_ZONE_ID/settings" "$CF_SNAPSHOT_DIR/settings-before.json"
cf_get "/zones/$CF_ZONE_ID/rulesets" "$CF_SNAPSHOT_DIR/rulesets-before.json"

(cd "$(dirname "$GUARD_ARTIFACT")" && \
  sha256sum --check "$(basename "$GUARD_ARTIFACT").sha256")
python3 "$GUARD_ARTIFACT" cloudflare-plan \
  --records "$CF_SNAPSHOT_DIR/dns-before.json" --new-ip "$NEW_HOST" \
  >"$CF_SNAPSHOT_DIR/dns-plan.json"

jq -e 'length == 4 and
  map(.name) == ["yuaiapi.com","api.yuaiapi.com","global.yuaiapi.com","vip.yuaiapi.com"] and
  map(.proxied) == [true,true,true,false]' "$CF_SNAPSHOT_DIR/dns-plan.json" >/dev/null
jq -r '.[] | [.name,.type,.content,.proxied,.ttl,.id] | @tsv' \
  "$CF_SNAPSHOT_DIR/dns-plan.json"
```

The operator must compare the four displayed records with the before snapshot.
Only `content` may change. Record ID, name, type, `proxied`, and TTL must remain
identical. In particular, apex/API/global stay proxied; VIP stays DNS-only with
its existing 300-second TTL. SSL/TLS mode, WAF, rulesets, cache, redirects,
transforms, Turnstile, OAuth, webhooks, and every unrelated record are
preserved because no command updates them.

Apply only after a second human review phrase:

```bash
read -rp 'Cloudflare apply confirmation: ' CF_APPLY_CONFIRMATION
test "$CF_APPLY_CONFIRMATION" = 'APPLY-FOUR-YUCORE-A-RECORDS'

jq -c '.[]' "$CF_SNAPSHOT_DIR/dns-plan.json" | while IFS= read -r record; do
  record_id="$(jq -r '.id' <<<"$record")"
  body="$(jq -c '{type,name,content,proxied,ttl}' <<<"$record")"
  response="$CF_SNAPSHOT_DIR/apply-$record_id.json"
  curl --config "$CF_CURL_CONFIG" --fail --silent --show-error -X PUT \
    --data "$body" \
    "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records/$record_id" \
    >"$response"
  jq -e '.success == true' "$response" >/dev/null
done

cf_get "/zones/$CF_ZONE_ID/dns_records?per_page=500" \
  "$CF_SNAPSHOT_DIR/dns-after-response.json"
jq -e '.result' "$CF_SNAPSHOT_DIR/dns-after-response.json" \
  >"$CF_SNAPSHOT_DIR/dns-after.json"
(cd "$(dirname "$GUARD_ARTIFACT")" && \
  sha256sum --check "$(basename "$GUARD_ARTIFACT").sha256")
python3 "$GUARD_ARTIFACT" cloudflare-plan \
  --records "$CF_SNAPSHOT_DIR/dns-after.json" --new-ip "$NEW_HOST" \
  >"$CF_SNAPSHOT_DIR/dns-after-plan.json"
cmp "$CF_SNAPSHOT_DIR/dns-plan.json" "$CF_SNAPSHOT_DIR/dns-after-plan.json"

jq -S 'map(if .type == "A" and (.name == "yuaiapi.com" or
  .name == "api.yuaiapi.com" or .name == "global.yuaiapi.com" or
  .name == "vip.yuaiapi.com") then del(.content,.modified_on)
  else . end) | sort_by(.id)' "$CF_SNAPSHOT_DIR/dns-before.json" \
  >"$CF_SNAPSHOT_DIR/dns-before-preserved.json"
jq -S 'map(if .type == "A" and (.name == "yuaiapi.com" or
  .name == "api.yuaiapi.com" or .name == "global.yuaiapi.com" or
  .name == "vip.yuaiapi.com") then del(.content,.modified_on)
  else . end) | sort_by(.id)' "$CF_SNAPSHOT_DIR/dns-after.json" \
  >"$CF_SNAPSHOT_DIR/dns-after-preserved.json"
cmp "$CF_SNAPSHOT_DIR/dns-before-preserved.json" "$CF_SNAPSHOT_DIR/dns-after-preserved.json"
cleanup_cf_auth
test ! -e "$CF_CURL_CONFIG"
trap - EXIT INT TERM
unset CF_APPLY_CONFIRMATION
```

Copy the mode-`0700` snapshot directory to protected retention storage. It must
contain no token. Do not revoke the temporary token or temporary SSH access
until the user separately authorizes delayed cleanup; remove the token from
the current process immediately with `unset`. Curl-config cleanup does not
revoke the Cloudflare token; revocation is a separate manual post-gate action.

## Public Traffic Gates

After the four updates, run at least one probe from outside both servers and
one from `OLD`. Confirm DNS and origin behavior:

```bash
# CONTROL
(cd "$(dirname "$GUARD_ARTIFACT")" && \
  sha256sum --check "$(basename "$GUARD_ARTIFACT").sha256")
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  getent ahostsv4 "$hostname"
  curl --fail --silent --show-error "https://$hostname/api/status" | \
    python3 "$GUARD_ARTIFACT" validate-status
done

# OLD
(cd /opt/newapi/releases && sha256sum --check yucore_migration_guard.py.sha256)
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  getent ahostsv4 "$hostname"
  curl --fail --silent --show-error "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

Repeat all private acceptance checks publicly. For VIP, test both the new IP
and the old bridge with explicit `curl --resolve` until at least two 300-second
TTL periods have elapsed. Verify registration email, direct VIP image/video,
ordinary and streaming relay, private groups, mapped-model privacy, account
pool failover, pricing, usage logs, tasks, and exactly-once settlement.

New-origin ingress was deliberately opened and verified before the DNS API
calls. Do not add or broaden a firewall rule here. Keep 80/443 on the old
origin available for the bridge.

## Rollback Before Any Public Write

Use this path if the target fails before the old bridge or Cloudflare can send
any request to the new master. In `NEW`, stop the target application. In `OLD`,
restore Redis and the old immutable application, then restore the original
Caddy file. The frozen old MySQL remains authoritative. This path independently
requires both the bridge-authority marker and public-ingress marker to be
absent; either marker routes to post-traffic reverse migration:

```bash
# OLD
if test -e "$BRIDGE_BOUNDARY_MARKER" || test -e "$BRIDGE_BOUNDARY_MARKER_SHA256"; then
  printf '%s\n' 'bridge authority marker exists; use post-traffic rollback' >&2
  exit 1
fi

# NEW
if ufw status numbered | grep -q 'yucore-migration-public-'; then
  printf '%s\n' 'public migration ingress exists; use post-traffic rollback' >&2
  exit 1
fi
new_running="$(docker inspect newapi --format '{{.State.Running}}')"
case "$new_running" in
  true) docker stop --time 5 newapi ;;
  false) ;;
  *) exit 1 ;;
esac
test "$(docker inspect newapi --format '{{.State.Running}}')" = false

# OLD
set -euo pipefail
export OLD_IMAGE_REF="$(cat "$RUN_DIR/evidence/old-image-ref")"
export OLD_IMAGE_ID="$(cat "$RUN_DIR/evidence/old-image-id")"
test "$(docker image inspect "$OLD_IMAGE_REF" --format '{{.Id}}')" = "$OLD_IMAGE_ID"
docker start newapi-redis
docker compose -f /opt/newapi/docker-compose.yml up -d --no-deps newapi
for attempt in $(seq 1 60); do
  test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy && break
  sleep 1
done
test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy
test "$(docker inspect newapi --format '{{.RestartCount}}')" = 0
test "$(docker inspect newapi --format '{{.Image}}')" = "$OLD_IMAGE_ID"
cp "$RUN_DIR/evidence/Caddyfile.before-maintenance" /opt/edge/Caddyfile
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec yuapi-caddy caddy reload --config /etc/caddy/Caddyfile
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

Cloudflare is unchanged in this path. Keep the new server and all evidence for
diagnosis; do not delete or retry without a new execution authorization.

## Rollback After Public Traffic: Reverse Data Migration

Once any public request may write to new MySQL, DNS-only rollback is forbidden.
The new server is authoritative. Re-enter maintenance on both origins, wait
five seconds, capture the new state, restore it to the old server, verify exact
manifests, start the old immutable image, bridge stale new-origin traffic back
to old, and only then revert the same four Cloudflare records.

First run the maintenance Nginx/Caddy switch from the earlier section on
`NEW`, replacing `newapi:3000` with `yucore-migration-maintenance:8080`. Keep
the old origin in maintenance while preparing reverse transfer. Then:

```bash
# OLD: the bridge marker makes reverse migration mandatory
test -f "$BRIDGE_BOUNDARY_MARKER"
test -f "$BRIDGE_BOUNDARY_MARKER_SHA256"
(cd "$(dirname "$BRIDGE_BOUNDARY_MARKER")" && \
  sha256sum --check "$(basename "$BRIDGE_BOUNDARY_MARKER_SHA256")")
jq -e '.authority == "new" and .boundary == "before-old-caddy-bridge"' \
  "$BRIDGE_BOUNDARY_MARKER" >/dev/null

# NEW: freeze the authoritative application
docker stop --time 5 newapi
test "$(docker inspect newapi --format '{{.State.Running}}')" = false

: "${REVERSE_ID:?set one immutable UTC reverse migration ID on NEW}"
case "$REVERSE_ID" in *[!0-9TZ]*) exit 1 ;; esac
export REVERSE_DIR="/opt/newapi/migration/reverse-$REVERSE_ID"
test ! -e "$REVERSE_DIR"
install -d -m 0700 "$REVERSE_DIR/data" "$REVERSE_DIR/evidence"
printf '%s\n' "$REVERSE_ID" >"$REVERSE_DIR/evidence/reverse-id"
sha256sum "$REVERSE_DIR/evidence/reverse-id" \
  >"$REVERSE_DIR/evidence/reverse-id.sha256"
verify_migration_helpers
/opt/newapi/releases/export-mysql-snapshot reverse "$REVERSE_DIR/data"
rsync -aHAX --numeric-ids --delete /opt/newapi/data/ "$REVERSE_DIR/data/app-data/"
rsync -aHAX --numeric-ids --delete /opt/edge/ "$REVERSE_DIR/data/edge/"

docker exec newapi-redis sh -lc '
  set -eu
  set +x
  if test -n "${REDIS_PASSWORD:-}"; then export REDISCLI_AUTH="$REDIS_PASSWORD"; fi
  redis-cli --no-auth-warning SAVE >/dev/null
'
verify_migration_helpers
ACCEPTED_COMMIT="$ACCEPTED_COMMIT" \
  /opt/newapi/releases/capture-state-manifest "$REVERSE_DIR/evidence/new-authoritative.json"
docker stop --time 5 newapi-redis
export REDIS_DATA_NEW="$(docker inspect newapi-redis --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}')"
sha256sum --check "$RUN_DIR/evidence/new-redis-mount.tsv.sha256"
IFS=$'\t' read -r REDIS_DESTINATION RECORDED_NEW_REDIS_MOUNT_SOURCE \
  <"$RUN_DIR/evidence/new-redis-mount.tsv"
test "$REDIS_DESTINATION" = '/data'
test "$RECORDED_NEW_REDIS_MOUNT_SOURCE" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
test "$REDIS_DATA_NEW" = "$APPROVED_NEW_REDIS_MOUNT_SOURCE"
tar -C "$REDIS_DATA_NEW" -czf "$REVERSE_DIR/data/redis-data.tar.gz" .
tar -C "$REVERSE_DIR/data" -czf "$REVERSE_DIR/reverse.tar.gz" .
sha256sum "$REVERSE_DIR/reverse.tar.gz" >"$REVERSE_DIR/reverse.tar.gz.sha256"
```

Transfer from `NEW` to `OLD` over temporary forwarded SSH, verify the hash, and
restore while old Caddy remains in maintenance:

```bash
# NEW
ssh -p "$OLD_SSH_PORT" "$OLD_USER@$OLD_HOST" "install -d -m 0700 '$REVERSE_DIR'"
scp -q -P "$OLD_SSH_PORT" \
  "$REVERSE_DIR/reverse.tar.gz" "$REVERSE_DIR/reverse.tar.gz.sha256" \
  "$REVERSE_DIR/evidence/new-authoritative.json" \
  "$REVERSE_DIR/evidence/reverse-id" "$REVERSE_DIR/evidence/reverse-id.sha256" \
  "$OLD_USER@$OLD_HOST:$REVERSE_DIR/"

# OLD
: "${REVERSE_ID:?set the reviewed NEW reverse migration ID independently on OLD}"
case "$REVERSE_ID" in *[!0-9TZ]*) exit 1 ;; esac
export REVERSE_DIR="/opt/newapi/migration/reverse-$REVERSE_ID"
cd "$REVERSE_DIR"
sha256sum --check reverse.tar.gz.sha256
install -d -m 0700 "$REVERSE_DIR/data" "$REVERSE_DIR/evidence"
mv "$REVERSE_DIR/reverse-id" "$REVERSE_DIR/evidence/reverse-id"
mv "$REVERSE_DIR/reverse-id.sha256" "$REVERSE_DIR/evidence/reverse-id.sha256"
sha256sum --check "$REVERSE_DIR/evidence/reverse-id.sha256"
test "$(cat "$REVERSE_DIR/evidence/reverse-id")" = "$REVERSE_ID"
tar -xzf reverse.tar.gz -C "$REVERSE_DIR/data"
mv "$REVERSE_DIR/new-authoritative.json" "$REVERSE_DIR/evidence/new-authoritative.json"

verify_migration_helpers
/opt/newapi/releases/restore-mysql-snapshot reverse "$REVERSE_DIR/data"

redis_running="$(docker inspect newapi-redis --format '{{.State.Running}}')"
case "$redis_running" in
  true) docker stop --time 5 newapi-redis ;;
  false) ;;
  *) exit 1 ;;
esac
test "$(docker inspect newapi-redis --format '{{.State.Running}}')" = false
export REDIS_DATA_OLD="$(docker inspect newapi-redis --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}')"
sha256sum --check "$RUN_DIR/evidence/old-redis-mount.tsv.sha256"
sha256sum --check "$RUN_DIR/evidence/old-redis-planned-source.sha256"
IFS=$'\t' read -r REDIS_DESTINATION RECORDED_OLD_REDIS_MOUNT_SOURCE \
  <"$RUN_DIR/evidence/old-redis-mount.tsv"
test "$REDIS_DESTINATION" = '/data'
test "$RECORDED_OLD_REDIS_MOUNT_SOURCE" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
test "$REDIS_DATA_OLD" = "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
test "$(cat "$RUN_DIR/evidence/old-redis-planned-source")" = \
  "$APPROVED_OLD_REDIS_MOUNT_SOURCE"
test "$(docker inspect newapi-redis --format '{{json .Mounts}}' | jq -r \
  '[.[] | select(.Destination == "/data")] | length')" = 1
find "$REDIS_DATA_OLD" -xdev -depth -mindepth 1 -delete
test -z "$(find "$REDIS_DATA_OLD" -xdev -mindepth 1 -print -quit)"
tar -xzf "$REVERSE_DIR/data/redis-data.tar.gz" -C "$REDIS_DATA_OLD"
docker start newapi-redis
rsync -aHAX --numeric-ids --delete "$REVERSE_DIR/data/app-data/" /opt/newapi/data/
docker stop --time 5 yuapi-caddy
rsync -aHAX --numeric-ids --delete "$REVERSE_DIR/data/edge/" /opt/edge/

verify_migration_helpers
ACCEPTED_COMMIT="$ACCEPTED_COMMIT" \
  /opt/newapi/releases/capture-state-manifest "$REVERSE_DIR/evidence/old-restored.json"
verify_migration_helpers
python3 /opt/newapi/releases/compare-state-snapshots \
  --source "$REVERSE_DIR/evidence/new-authoritative.json" \
  --target "$REVERSE_DIR/evidence/old-restored.json" --tolerance-ms 2000
docker start yuapi-caddy
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
```

The old immutable image ID captured before maintenance must still exist. Start
it and pass private gates before restoring traffic:

```bash
export OLD_IMAGE_REF="$(cat "$RUN_DIR/evidence/old-image-ref")"
export OLD_IMAGE_ID="$(cat "$RUN_DIR/evidence/old-image-id")"
test "$(docker image inspect "$OLD_IMAGE_REF" --format '{{.Id}}')" = "$OLD_IMAGE_ID"
docker compose -f /opt/newapi/docker-compose.yml up -d --no-deps --force-recreate newapi
for attempt in $(seq 1 90); do
  test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy && break
  sleep 1
done
test "$(docker inspect newapi --format '{{.State.Health.Status}}')" = healthy
test "$(docker inspect newapi --format '{{.RestartCount}}')" = 0
test "$(docker inspect newapi --format '{{.Image}}')" = "$OLD_IMAGE_ID"
```

Restore and validate the original old Caddy routing while Cloudflare still
reaches new-origin maintenance:

```bash
# OLD
cp "$RUN_DIR/evidence/Caddyfile.before-maintenance" /opt/edge/Caddyfile
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec yuapi-caddy caddy reload --config /etc/caddy/Caddyfile
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error --resolve "$hostname:443:$OLD_HOST" \
    "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

Before reverting Cloudflare, configure `NEW` Caddy as the exact mirror-image
bridge to the accepted old origin. This protects clients that still resolve the
new VIP address after rollback:

```bash
# NEW
set -euo pipefail
set +x
umask 077
: "${OLD_HOST:?set OLD_HOST}"
cat >"$REVERSE_DIR/Caddyfile.bridge-old" <<EOF
yuaiapi.com {
    reverse_proxy https://$OLD_HOST:443 {
        header_up Host yuaiapi.com
        transport http { tls_server_name yuaiapi.com }
    }
}
api.yuaiapi.com {
    reverse_proxy https://$OLD_HOST:443 {
        header_up Host api.yuaiapi.com
        transport http { tls_server_name api.yuaiapi.com }
    }
}
global.yuaiapi.com {
    reverse_proxy https://$OLD_HOST:443 {
        header_up Host global.yuaiapi.com
        transport http { tls_server_name global.yuaiapi.com }
    }
}
vip.yuaiapi.com {
    reverse_proxy https://$OLD_HOST:443 {
        header_up Host vip.yuaiapi.com
        transport http { tls_server_name vip.yuaiapi.com }
    }
}
EOF
docker cp "$REVERSE_DIR/Caddyfile.bridge-old" yuapi-caddy:/tmp/Caddyfile.next
docker exec yuapi-caddy caddy validate --config /tmp/Caddyfile.next
cp "$REVERSE_DIR/Caddyfile.bridge-old" /opt/edge/Caddyfile
docker exec yuapi-caddy caddy validate --config /etc/caddy/Caddyfile
docker exec yuapi-caddy caddy reload --config /etc/caddy/Caddyfile
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error --resolve "$hostname:443:$NEW_HOST" \
    "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

On `CONTROL`, read the temporary Cloudflare token silently. Select exactly the
four original A records from `dns-before.json`, assert unique IDs and preserved
flags/TTLs, display them for review, then PUT only those four objects:

```bash
set -euo pipefail
set +x
umask 077
export CF_ROLLBACK_AUTH_DIR="$(mktemp -d)"
chmod 0700 "$CF_ROLLBACK_AUTH_DIR"
export CF_ROLLBACK_CURL_CONFIG="$CF_ROLLBACK_AUTH_DIR/curl-auth.conf"
cleanup_cf_rollback_auth() {
  case "$CF_ROLLBACK_CURL_CONFIG" in
    "$CF_ROLLBACK_AUTH_DIR/curl-auth.conf") ;;
    *) return 1 ;;
  esac
  unset CF_API_TOKEN
  rm -f -- "$CF_ROLLBACK_CURL_CONFIG"
  rmdir "$CF_ROLLBACK_AUTH_DIR"
}
trap cleanup_cf_rollback_auth EXIT INT TERM
read -rsp 'Temporary least-privilege Cloudflare token: ' CF_API_TOKEN; printf '\n'
case "$CF_API_TOKEN" in ''|*[!A-Za-z0-9._-]*) exit 1 ;; esac
set +x
printf 'header = "Authorization: Bearer %s"\nheader = "Content-Type: application/json"\n' \
  "$CF_API_TOKEN" >"$CF_ROLLBACK_CURL_CONFIG"
chmod 0600 "$CF_ROLLBACK_CURL_CONFIG"
unset CF_API_TOKEN

jq -e '[.[] | select(.type == "A" and (.name == "yuaiapi.com" or
  .name == "api.yuaiapi.com" or .name == "global.yuaiapi.com" or
  .name == "vip.yuaiapi.com"))] as $records |
  ($records | length) == 4 and
  ($records | map(.name) | sort) ==
    ["api.yuaiapi.com","global.yuaiapi.com","vip.yuaiapi.com","yuaiapi.com"] and
  ($records | map(.id) | unique | length) == 4 and
  ($records | map(.proxied) | sort) == [false,true,true,true]' \
  "$CF_SNAPSHOT_DIR/dns-before.json" >/dev/null
jq -r '.[] | select(.type == "A" and (.name == "yuaiapi.com" or
  .name == "api.yuaiapi.com" or .name == "global.yuaiapi.com" or
  .name == "vip.yuaiapi.com")) | [.name,.content,.proxied,.ttl,.id] | @tsv' \
  "$CF_SNAPSHOT_DIR/dns-before.json"
read -rp 'Cloudflare rollback confirmation: ' CF_ROLLBACK_CONFIRMATION
test "$CF_ROLLBACK_CONFIRMATION" = 'REVERT-FOUR-YUCORE-A-RECORDS'

jq -c '.[] | select(.type == "A" and (.name == "yuaiapi.com" or
  .name == "api.yuaiapi.com" or .name == "global.yuaiapi.com" or
  .name == "vip.yuaiapi.com"))' "$CF_SNAPSHOT_DIR/dns-before.json" | \
while IFS= read -r record; do
  record_id="$(jq -r '.id' <<<"$record")"
  body="$(jq -c '{type,name,content,proxied,ttl}' <<<"$record")"
  response="$CF_SNAPSHOT_DIR/rollback-$record_id.json"
  curl --config "$CF_ROLLBACK_CURL_CONFIG" --fail --silent --show-error -X PUT \
    --data "$body" \
    "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records/$record_id" \
    >"$response"
  jq -e '.success == true' "$response" >/dev/null
done
cleanup_cf_rollback_auth
test ! -e "$CF_ROLLBACK_CURL_CONFIG"
test ! -e "$CF_ROLLBACK_AUTH_DIR"
trap - EXIT INT TERM
unset CF_ROLLBACK_CONFIRMATION
```

Keep the new bridge running for at least two prior VIP TTL periods. Keep the
new authoritative snapshot and restored-old manifest for forensic comparison.
Never merge independently writable databases or enable both applications as
masters.

After that bridge interval, reverse the provider firewall first, from public
80/443 back to staging 80/443 from `OLD_HOST/32` only, using provider-applied
evidence and a second operator. Then remove the two labeled UFW rules. Only
after both firewall layers are closed may `OLD` archive and remove the active
new-write authority marker:

```bash
# CONTROL: provider firewall public -> OLD_HOST-only
: "${PROVIDER_FIREWALL_ROLLBACK_EVIDENCE:?set provider rollback-state JSON path}"
: "${APPROVED_PROVIDER_FIREWALL_ROLLBACK_SHA256:?set approved rollback evidence SHA-256}"
test "$(sha256sum "$PROVIDER_FIREWALL_ROLLBACK_EVIDENCE" | cut -d' ' -f1)" = \
  "$APPROVED_PROVIDER_FIREWALL_ROLLBACK_SHA256"
jq -e --arg old_source "$OLD_HOST/32" '
  .status == "applied" and (.change_id | type == "string" and length > 0) and
  .http_https.ports == [80,443] and .http_https.sources == [$old_source] and
  (.publicly_denied_ports | sort) == [3000,3001,3306,6379]' \
  "$PROVIDER_FIREWALL_ROLLBACK_EVIDENCE" >/dev/null
read -rp 'Provider firewall rollback confirmation: ' PROVIDER_ROLLBACK_CONFIRMATION
test "$PROVIDER_ROLLBACK_CONFIRMATION" = \
  'PROVIDER-FIREWALL-RETURNED-TO-OLD-HOST-ONLY'
install -m 0600 "$PROVIDER_FIREWALL_ROLLBACK_EVIDENCE" \
  "$CONTROL_BOUNDARY_DIR/provider-firewall-rollback.json"
printf '%s  %s\n' "$APPROVED_PROVIDER_FIREWALL_ROLLBACK_SHA256" \
  'provider-firewall-rollback.json' \
  >"$CONTROL_BOUNDARY_DIR/provider-firewall-rollback.json.sha256"
(cd "$CONTROL_BOUNDARY_DIR" && \
  sha256sum --check provider-firewall-rollback.json.sha256)

# NEW: remove only labeled host-firewall rules
mapfile -t ingress_rule_numbers < <(ufw status numbered | sed -n \
  '/yucore-migration-public-/s/^\[ *\([0-9][0-9]*\)\].*/\1/p' | sort -rn)
test "${#ingress_rule_numbers[@]}" -ge 2
for rule_number in "${ingress_rule_numbers[@]}"; do
  ufw --force delete "$rule_number"
done
if ufw status numbered | grep -q 'yucore-migration-public-'; then exit 1; fi

# OLD: close the active authority marker only after both firewall gates
(cd "$(dirname "$BRIDGE_BOUNDARY_MARKER")" && \
  sha256sum --check "$(basename "$BRIDGE_BOUNDARY_MARKER_SHA256")")
install -d -m 0700 "$RUN_DIR/evidence/boundary-history"
cp "$BRIDGE_BOUNDARY_MARKER" "$BRIDGE_BOUNDARY_MARKER_SHA256" \
  "$RUN_DIR/evidence/boundary-history/"
(cd "$RUN_DIR/evidence/boundary-history" && \
  sha256sum --check "$(basename "$BRIDGE_BOUNDARY_MARKER_SHA256")")
jq -n --arg migration_id "$MIGRATION_ID" \
  --arg closed_at "$(date -u --iso-8601=seconds)" \
  '{migration_id:$migration_id,authority:"old",closed_at:$closed_at,
    reason:"post-traffic-rollback-firewalls-closed"}' \
  >"$RUN_DIR/evidence/new-write-authority-boundary.closed.json"
sha256sum "$RUN_DIR/evidence/new-write-authority-boundary.closed.json" \
  >"$RUN_DIR/evidence/new-write-authority-boundary.closed.json.sha256"
rm -f "$BRIDGE_BOUNDARY_MARKER" "$BRIDGE_BOUNDARY_MARKER_SHA256"
test ! -e "$BRIDGE_BOUNDARY_MARKER"
test ! -e "$BRIDGE_BOUNDARY_MARKER_SHA256"
```

## Observation And Rollback Triggers

Sample at 1, 5, 15, 30, and 60 minutes after public traffic reaches the new
origin. Repeat the same checks daily for seven days:

```bash
date -u --iso-8601=seconds
docker inspect newapi --format \
  'status={{.State.Status}} health={{.State.Health.Status}} restarts={{.RestartCount}} image={{.Image}}'
docker stats --no-stream newapi newapi-mysql newapi-redis yuapi-caddy
docker logs --since 5m newapi 2>&1 | tail -n 300
docker logs --since 5m yuapi-caddy 2>&1 | tail -n 300
if journalctl -k --since '-10 minutes' --no-pager | \
  grep -Ei 'oom|out of memory|killed process'; then
  printf '%s\n' 'kernel memory-pressure event detected'
fi
ss -s
df -hT / /opt
systemctl --failed --no-pager
systemctl list-timers --all --no-pager | grep -Ei 'newapi|backup'
for hostname in yuaiapi.com api.yuaiapi.com global.yuaiapi.com vip.yuaiapi.com; do
  curl --fail --silent --show-error "https://$hostname/api/status" | \
    python3 /opt/newapi/releases/yucore_migration_guard.py validate-status
done
```

At every sample, verify login, registration email, profile, wallet, usage logs,
private groups, mapped-model privacy, account-pool routing/failover, billing,
task settlement, first-byte latency, streaming disconnects, and HTTP
`5xx`/`521`. Roll back for health loss, any restart, panic, migration error,
sustained gateway errors, authentication regression, private-data exposure,
wrong public-model billing, duplicate settlement/refund, unexplained quota
change, or materially worse latency/stream reliability.

Verify the scheduled backup completes and passes an integrity check every day.
Do not treat the existence of a backup file as proof of a usable backup.

## Seven-Day Retention And Delayed Cleanup

Keep for at least seven full days:

- the old server, stopped old application/Redis state, old immutable image, and
  old Caddy bridge configuration;
- final forward and reverse MySQL/Redis archives, manifests, hashes, Compose
  snapshots, environment archive, file manifests, and Caddy snapshots;
- the accepted `linux/amd64` image archive and metadata on both servers and
  `CONTROL`;
- the complete Cloudflare before/plan/after snapshot and apply responses;
- temporary server access and the ability to issue or use the reviewed
  least-privilege Cloudflare rollback token.

Cleanup is a separate change window. Do not run it until the user confirms the
seven-day evidence is accepted and enters this independent phrase:

```bash
read -rp 'Delayed cleanup confirmation: ' CLEANUP_CONFIRMATION
test "$CLEANUP_CONFIRMATION" = 'DELETE-OLD-SERVER-AFTER-7-DAYS'
```

For a successful migration with no rollback, the new provider firewall remains
public because the DNS-only VIP is still served there; do not apply the
OLD_HOST-only rollback rules. Close the active authority marker only after the
seven-day evidence is accepted and copied to retention:

```bash
# OLD, successful-migration cleanup branch only
(cd "$(dirname "$BRIDGE_BOUNDARY_MARKER")" && \
  sha256sum --check "$(basename "$BRIDGE_BOUNDARY_MARKER_SHA256")")
install -d -m 0700 "$RUN_DIR/evidence/boundary-history"
cp "$BRIDGE_BOUNDARY_MARKER" "$BRIDGE_BOUNDARY_MARKER_SHA256" \
  "$RUN_DIR/evidence/boundary-history/"
(cd "$RUN_DIR/evidence/boundary-history" && \
  sha256sum --check "$(basename "$BRIDGE_BOUNDARY_MARKER_SHA256")")
jq -n --arg migration_id "$MIGRATION_ID" \
  --arg closed_at "$(date -u --iso-8601=seconds)" \
  '{migration_id:$migration_id,authority:"new",closed_at:$closed_at,
    reason:"successful-seven-day-retention-complete",
    provider_firewall:"public-80-443-retained"}' \
  >"$RUN_DIR/evidence/new-write-authority-boundary.closed.json"
sha256sum "$RUN_DIR/evidence/new-write-authority-boundary.closed.json" \
  >"$RUN_DIR/evidence/new-write-authority-boundary.closed.json.sha256"
rm -f "$BRIDGE_BOUNDARY_MARKER" "$BRIDGE_BOUNDARY_MARKER_SHA256"
test ! -e "$BRIDGE_BOUNDARY_MARKER"
test ! -e "$BRIDGE_BOUNDARY_MARKER_SHA256"
```

After confirmation and marker closure, archive evidence to the approved
retention store, revoke temporary SSH and Cloudflare access, stop the old/new
bridge, and release the
old server through the provider workflow. Remove only explicitly named
migration containers and directories after resolving and displaying their
exact paths. Do not run `docker system prune`, delete unrelated images or
volumes, delete backups, or remove the accepted worktree or experimental UI.

## Design Coverage And Final Record

The execution record must attach evidence for every row:

| Design requirement | Runbook evidence |
| --- | --- |
| Inert daytime boundary and explicit authorization | Scope gate and maintenance mutation gate outputs |
| New hardware and trust-boundary validation | Old/new read-only preflight captures |
| Immutable accepted candidate | Commit, image ID, platform, archive hash, and load checks |
| Secret-safe deployment | Mode-`0700` archive, mode-`0600` env files, runtime drift guard |
| Pre-staging and private acceptance | Initial MySQL/Redis/files restore and `curl --resolve` evidence |
| Short forced freeze | Maintenance Caddy/Nginx and five-second application stop |
| Final state transfer | Final dump/archive/delta hashes and source manifest |
| Exact target restoration | Before-start source/target manifest equality |
| Single final master | Health, zero restart, image ID, runtime drift, and private gates |
| Stale-origin protection | Old-to-new Caddy bridge before DNS updates |
| Preserved Cloudflare behavior | Before snapshot, exact four-record plan, review, after comparison |
| Pre-traffic rollback | Old frozen database restoration evidence |
| Post-traffic rollback | New-to-old database/Redis/file migration and manifest equality |
| Observation and retention | 1/5/15/30/60-minute samples and seven daily backup records |
| Delayed cleanup | Separate confirmation and named-resource removal record |

The final execution record must state whether production traffic moved, whether
rollback occurred, the authoritative database at close, the four final A
record contents/proxy flags/TTLs, and the earliest allowed cleanup timestamp.
