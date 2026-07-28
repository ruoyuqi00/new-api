# YuCore Production Cutover Runbook

## Scope And Authorization

This runbook replaces only the production `newapi` application image. It does
not change channels, groups, account pools, model mappings, prices, MySQL,
Redis, DNS, Cloudflare, or the experimental UI.

Do not run any production command in this document until the user explicitly
authorizes the production replacement. Preparation, cutover, and rollback must
be performed from the accepted worktree and accepted commit:

```text
Worktree: D:\yucore-local-production
Branch:   codex/local-production-brand-performance-20260725
```

The last read-only production audit found:

```text
Application container: newapi
Application upstream:  newapi:3000
Application host port: 127.0.0.1:3001
Application image:      newapi:veo-auto-resolution-20260724
Application image ID:   sha256:079c49d1452ca590ea8c3d90480352ad059c3d74055d9050a6c650db9a507026
Compose file:           /opt/newapi/docker-compose.yml
Caddy container:        yuapi-caddy
Caddy file:             /opt/edge/Caddyfile
Shared network:         sub2api_sub2api-network
```

Treat those values as an audit snapshot, not assumptions. If the current image
ID, container names, ports, network, or Caddy upstream differ at deployment
time, stop and repeat the read-only topology audit before changing anything.

## Required Acceptance Evidence

The accepted commit must already have fresh evidence for all of these gates:

- default frontend tests, typecheck, lint/format checks, and production build;
- classic frontend production build;
- desktop and mobile, light and dark, anonymous/user/admin visual checks;
- no second signal-field shader compilation after stable-home activation;
- affected Go package tests and `go build ./...`;
- account failover for `401`, `429`, retryable `5xx`, and transport errors;
- fallback from exhausted account pools to another eligible channel;
- stale channel/account affinity cleared before retry reselection;
- private groups hidden from unauthorized users and available to authorized
  downstream users;
- fixed-price, per-call image/video, and expression billing based on the public
  `OriginModelName`, not a cheaper mapped upstream name;
- ordinary user logs and task responses hiding `is_model_mapped` and
  `upstream_model_name`, while administrator diagnostics retain them;
- complete migrations on SQLite, MySQL 5.7, and PostgreSQL 9.6;
- a local Caddy forward-switch and rollback rehearsal with zero failed requests.

Do not substitute the presence of commits for these test results.

## Phase 1: Build The Immutable Candidate Locally

Run in PowerShell from the accepted worktree. Docker Desktop may be started for
this phase only after production replacement is authorized.

```powershell
Set-Location D:\yucore-local-production

$ExpectedBranch = 'codex/local-production-brand-performance-20260725'
$ActualBranch = (git branch --show-current).Trim()
if ($ActualBranch -ne $ExpectedBranch) { throw "Unexpected branch: $ActualBranch" }

git diff --check
if ($LASTEXITCODE -ne 0) { throw 'git diff --check failed' }

$TrackedChanges = git status --porcelain --untracked-files=no
if ($TrackedChanges) { throw "Tracked worktree changes exist: $TrackedChanges" }

$AcceptedCommit = (git rev-parse HEAD).Trim()
$Image = "newapi:yucore-$AcceptedCommit"
$ReleaseDir = Join-Path $env:TEMP "yucore-$AcceptedCommit"
if (Test-Path $ReleaseDir) { throw "Release directory already exists: $ReleaseDir" }
New-Item -ItemType Directory -Path $ReleaseDir | Out-Null

$ContextTar = Join-Path $ReleaseDir 'accepted-context.tar'
$BuildContext = Join-Path $ReleaseDir 'accepted-context'
New-Item -ItemType Directory -Force -Path $BuildContext | Out-Null
git archive --format=tar --output=$ContextTar $AcceptedCommit
if ($LASTEXITCODE -ne 0) { throw 'accepted commit archive failed' }
tar -xf $ContextTar -C $BuildContext
if ($LASTEXITCODE -ne 0) { throw 'accepted commit extraction failed' }

docker build --platform linux/amd64 --tag $Image $BuildContext
if ($LASTEXITCODE -ne 0) { throw 'candidate image build failed' }

$ImageId = (docker image inspect $Image --format '{{.Id}}').Trim()
$Artifact = Join-Path $ReleaseDir "newapi-yucore-$AcceptedCommit.tar"
docker save --output $Artifact $Image
if ($LASTEXITCODE -ne 0) { throw 'candidate image export failed' }

$ArtifactHash = (Get-FileHash -Algorithm SHA256 $Artifact).Hash.ToLowerInvariant()
Set-Content -NoNewline -Encoding ascii -Path "$Artifact.sha256" -Value "$ArtifactHash  $(Split-Path $Artifact -Leaf)"
Set-Content -NoNewline -Encoding ascii -Path "$Artifact.image-id" -Value $ImageId
Set-Content -NoNewline -Encoding ascii -Path "$Artifact.commit" -Value $AcceptedCommit
```

Transfer the four release files to `/opt/newapi/releases/`. Infrastructure
coordinates are supplied interactively and are never committed:

```powershell
$ProdHost = Read-Host 'Production host'
$ProdPort = Read-Host 'Production SSH port'
$ProdUser = Read-Host 'Production SSH user'

ssh -p $ProdPort "${ProdUser}@${ProdHost}" 'install -d -m 0700 /opt/newapi/releases'
if ($LASTEXITCODE -ne 0) { throw 'release directory preparation failed' }

scp -P $ProdPort $Artifact "$Artifact.sha256" "$Artifact.image-id" "$Artifact.commit" "${ProdUser}@${ProdHost}:/opt/newapi/releases/"
if ($LASTEXITCODE -ne 0) { throw 'release transfer failed' }
```

This preparation does not change live traffic. Keep the local image and release
files until the production observation window is complete.

## Phase 2: Production Preflight And Backups

Open one SSH shell. Do not enable shell tracing (`set -x`) because the live
container environment contains secrets.

```bash
set -euo pipefail

export APP_CONTAINER=newapi
export CADDY_CONTAINER=yuapi-caddy
export APP_NETWORK=sub2api_sub2api-network
export COMPOSE_FILE=/opt/newapi/docker-compose.yml
export CADDY_FILE=/opt/edge/Caddyfile
export CANDIDATE_CONTAINER=newapi-candidate
export CANDIDATE_PORT=3002
export AUDITED_IMAGE_ID=sha256:079c49d1452ca590ea8c3d90480352ad059c3d74055d9050a6c650db9a507026

command -v docker
command -v curl
command -v jq
command -v perl

test "$(docker inspect --format '{{.State.Status}}' "$APP_CONTAINER")" = running
test "$(docker inspect --format '{{.State.Health.Status}}' "$APP_CONTAINER")" = healthy
test "$(docker inspect --format '{{.RestartCount}}' "$APP_CONTAINER")" = 0
test "$(docker image inspect --format '{{.Id}}' "$(docker inspect --format '{{.Config.Image}}' "$APP_CONTAINER")")" = "$AUDITED_IMAGE_ID"
docker network inspect "$APP_NETWORK" >/dev/null
docker exec "$CADDY_CONTAINER" caddy validate --config /etc/caddy/Caddyfile
docker exec "$CADDY_CONTAINER" sh -c 'command -v wget >/dev/null'

export LIVE_IMAGE_REF="$(docker inspect --format '{{.Config.Image}}' "$APP_CONTAINER")"
export LIVE_IMAGE_ID="$(docker image inspect --format '{{.Id}}' "$LIVE_IMAGE_REF")"
export STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
export BACKUP_DIR="/opt/newapi/backups/$STAMP"
install -d -m 0700 "$BACKUP_DIR"
cp -a "$COMPOSE_FILE" "$BACKUP_DIR/docker-compose.yml"
cp -a "$CADDY_FILE" "$BACKUP_DIR/Caddyfile"
printf '%s\n' "$LIVE_IMAGE_REF" >"$BACKUP_DIR/live-image-ref"
printf '%s\n' "$LIVE_IMAGE_ID" >"$BACKUP_DIR/live-image-id"
docker inspect "$APP_CONTAINER" --format '{{json .State}}' >"$BACKUP_DIR/live-container-state.json"

export RELEASE_TAR="$(ls -1t /opt/newapi/releases/newapi-yucore-*.tar | head -n 1)"
test -f "$RELEASE_TAR.sha256"
test -f "$RELEASE_TAR.image-id"
test -f "$RELEASE_TAR.commit"
cd "$(dirname "$RELEASE_TAR")"
sha256sum --check "$(basename "$RELEASE_TAR").sha256"
export ACCEPTED_COMMIT="$(cat "$RELEASE_TAR.commit")"
export NEW_IMAGE="newapi:yucore-$ACCEPTED_COMMIT"
docker load --input "$RELEASE_TAR"
test "$(docker image inspect --format '{{.Id}}' "$NEW_IMAGE")" = "$(cat "$RELEASE_TAR.image-id")"
```

Stop if the audited image guard fails. Do not silently replace
`AUDITED_IMAGE_ID` with a new value.

Create a transaction-consistent MySQL backup without exposing its password. The
official MySQL container already owns `MYSQL_ROOT_PASSWORD` and
`MYSQL_DATABASE`; neither value is printed or copied to the host environment:

```bash
export MYSQL_CONTAINER="$(docker ps --format '{{.Names}} {{.Image}}' | awk '$2 ~ /^mysql:/ {print $1; exit}')"
test -n "$MYSQL_CONTAINER"
docker exec "$MYSQL_CONTAINER" sh -lc 'test -n "$MYSQL_ROOT_PASSWORD" && test -n "$MYSQL_DATABASE"'
docker exec "$MYSQL_CONTAINER" sh -lc 'export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"; exec mysqldump --single-transaction --quick --routines --triggers --events -uroot "$MYSQL_DATABASE"' \
  | gzip -1 >"$BACKUP_DIR/mysql.sql.gz"
gzip --test "$BACKUP_DIR/mysql.sql.gz"
test -s "$BACKUP_DIR/mysql.sql.gz"
```

If production uses a separate log database, back it up with the same method.
The candidate runs as a slave and therefore does not migrate the database. The
final master may apply only the already reviewed additive migrations. A missing
backup or a backup configuration that has not previously passed a restore drill
blocks cutover.

## Phase 3: Start The Isolated Candidate

Copy the live container environment without printing it. The file lives under
`/run`, is mode `0600`, and is deleted at cleanup. Explicit `-e` values after
`--env-file` override all recurring master work:

```bash
test ! -e /run/newapi-candidate.env
install -m 0600 /dev/null /run/newapi-candidate.env
docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$APP_CONTAINER" >/run/newapi-candidate.env

test -z "$(docker ps -aq --filter "name=^/${CANDIDATE_CONTAINER}$")"
docker run -d \
  --name "$CANDIDATE_CONTAINER" \
  --restart no \
  --network "$APP_NETWORK" \
  --env-file /run/newapi-candidate.env \
  -e NODE_TYPE=slave \
  -e NODE_NAME=newapi-candidate \
  -e BATCH_UPDATE_ENABLED=false \
  -e UPDATE_TASK=false \
  -e CHANNEL_UPDATE_FREQUENCY= \
  -p "127.0.0.1:${CANDIDATE_PORT}:3000" \
  --volumes-from "$APP_CONTAINER" \
  --health-cmd 'wget -q -O /dev/null http://localhost:3000/api/status' \
  --health-interval 10s \
  --health-timeout 5s \
  --health-retries 6 \
  --health-start-period 15s \
  "$NEW_IMAGE" --log-dir /app/logs

for attempt in $(seq 1 30); do
  if curl --fail --silent "http://127.0.0.1:${CANDIDATE_PORT}/api/status" | jq -e '.success == true' >/dev/null; then
    break
  fi
  sleep 2
done

test "$(docker inspect --format '{{.State.Status}}' "$CANDIDATE_CONTAINER")" = running
test "$(docker inspect --format '{{.State.Health.Status}}' "$CANDIDATE_CONTAINER")" = healthy
test "$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$CANDIDATE_CONTAINER" | grep '^NODE_TYPE=' | cut -d= -f2-)" = slave
test "$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$CANDIDATE_CONTAINER" | grep '^BATCH_UPDATE_ENABLED=' | cut -d= -f2-)" = false
docker exec "$CADDY_CONTAINER" wget -q -O /dev/null http://newapi-candidate:3000/api/status
```

If health does not become green, inspect only bounded logs, remove the candidate,
delete `/run/newapi-candidate.env`, and stop. Live traffic is still on the old
container at this point.

```bash
docker logs --tail 200 "$CANDIDATE_CONTAINER"
docker rm -f "$CANDIDATE_CONTAINER"
rm -f /run/newapi-candidate.env
```

## Phase 4: Candidate Acceptance Probes

Use disposable, least-privilege production validation credentials. Read secrets
without echo and never place them in shell history, files, or the runbook:

```bash
read -rsp 'Disposable downstream API key: ' RELAY_KEY; printf '\n'
read -rsp 'Authorized private-group dashboard PAT: ' USER_PAT; printf '\n'
read -rsp 'Disposable administrator dashboard PAT: ' ADMIN_PAT; printf '\n'
read -rp 'Private group name: ' PRIVATE_GROUP
read -rp 'Known public mapped model: ' MAPPED_PUBLIC_MODEL
read -rp 'Minimal ordinary chat model: ' CHAT_MODEL
```

### Status, UI, Authentication, And Private Group

```bash
curl --fail --silent "http://127.0.0.1:${CANDIDATE_PORT}/api/status" | jq -e '.success == true and (.data.system_name | length > 0)'
curl --fail --silent "http://127.0.0.1:${CANDIDATE_PORT}/" | grep -q '<html'

curl --fail --silent \
  -H "Authorization: Bearer $USER_PAT" \
  "http://127.0.0.1:${CANDIDATE_PORT}/api/user/models?group=$(printf '%s' "$PRIVATE_GROUP" | jq -sRr @uri)" \
  | jq -e '.success == true and (.data | length > 0)'

curl --fail --silent \
  -H "Authorization: Bearer $RELAY_KEY" \
  "http://127.0.0.1:${CANDIDATE_PORT}/v1/models" \
  | jq -e --arg model "$MAPPED_PUBLIC_MODEL" '.data | any(.id == $model)'
```

In an incognito request without `USER_PAT`, `/api/pricing` must not expose the
private group. Secure-cookie browser checks are performed immediately after the
atomic Caddy switch; direct alternate-port browser login is not authoritative
because it does not use the production HTTPS origin.

### Ordinary And Streaming Relay

```bash
CHAT_BODY="$(jq -nc --arg model "$CHAT_MODEL" '{model:$model,messages:[{role:"user",content:"Reply only OK"}],max_tokens:8,stream:false}')"
CHAT_RESPONSE="$(curl --fail --silent \
  -H "Authorization: Bearer $RELAY_KEY" \
  -H 'Content-Type: application/json' \
  --data "$CHAT_BODY" \
  "http://127.0.0.1:${CANDIDATE_PORT}/v1/chat/completions")"
printf '%s' "$CHAT_RESPONSE" | jq -e '.choices | length > 0'

STREAM_BODY="$(jq -nc --arg model "$CHAT_MODEL" '{model:$model,messages:[{role:"user",content:"Reply only OK"}],max_tokens:8,stream:true}')"
curl --fail --no-buffer --silent \
  -H "Authorization: Bearer $RELAY_KEY" \
  -H 'Content-Type: application/json' \
  --data "$STREAM_BODY" \
  "http://127.0.0.1:${CANDIDATE_PORT}/v1/chat/completions" \
  | grep -q 'data:'
```

### Mapped-Model Privacy And Billing

Call the known mapped public model with the smallest valid request. The client
model field must remain public and no routing metadata may appear:

```bash
MAPPED_BODY="$(jq -nc --arg model "$MAPPED_PUBLIC_MODEL" '{model:$model,messages:[{role:"user",content:"Reply only OK"}],max_tokens:8,stream:false}')"
MAPPED_RESPONSE="$(curl --fail --silent \
  -H "Authorization: Bearer $RELAY_KEY" \
  -H 'Content-Type: application/json' \
  --data "$MAPPED_BODY" \
  "http://127.0.0.1:${CANDIDATE_PORT}/v1/chat/completions")"
printf '%s' "$MAPPED_RESPONSE" | jq -e --arg model "$MAPPED_PUBLIC_MODEL" '.model == $model'
printf '%s' "$MAPPED_RESPONSE" | jq -e '[.. | objects | has("upstream_model_name") or has("is_model_mapped")] | any | not'

curl --fail --silent \
  -H "Authorization: Bearer $USER_PAT" \
  "http://127.0.0.1:${CANDIDATE_PORT}/api/log/self?p=0&page_size=20" \
  | jq -e '[.. | objects | has("upstream_model_name") or has("is_model_mapped")] | any | not'

curl --fail --silent \
  -H "Authorization: Bearer $USER_PAT" \
  "http://127.0.0.1:${CANDIDATE_PORT}/api/task/self?p=0&page_size=20" \
  | jq -e '[.. | objects | has("upstream_model_name") or has("is_model_mapped")] | any | not'
```

In the administrator log detail, confirm that the same request retains its
upstream routing name for diagnostics while the charged model and price use the
public requested model. Check one fixed-price model, one cheapest per-call
image/video model, and one expression-priced model. The pre-consume and final
settlement deltas must each occur once; a failed async task must refund once.

For the paid image/video probe, use the smallest supported resolution, duration,
and sample count documented for the configured upstream. Run it through
`https://vip.yuaiapi.com` after the candidate receives traffic. Do not use a
public CF route as the long-running validation endpoint.

## Phase 5: Atomically Move Caddy To The Candidate

Build and validate a candidate Caddyfile before replacing the mounted file. The
expected occurrence count prevents a partial host switch:

```bash
export CADDY_NEXT="$BACKUP_DIR/Caddyfile.to-candidate"
export FROM_UPSTREAM='newapi:3000'
export TO_UPSTREAM='newapi-candidate:3000'
export UPSTREAM_COUNT="$(grep -Fc "$FROM_UPSTREAM" "$CADDY_FILE")"
test "$UPSTREAM_COUNT" -ge 2

cp -a "$CADDY_FILE" "$CADDY_NEXT"
FROM_UPSTREAM="$FROM_UPSTREAM" TO_UPSTREAM="$TO_UPSTREAM" perl -0pi -e 's/\Q$ENV{FROM_UPSTREAM}\E/$ENV{TO_UPSTREAM}/g' "$CADDY_NEXT"
test "$(grep -Fc "$TO_UPSTREAM" "$CADDY_NEXT")" = "$UPSTREAM_COUNT"
test "$(grep -Fc "$FROM_UPSTREAM" "$CADDY_NEXT")" = 0

docker cp "$CADDY_NEXT" "$CADDY_CONTAINER:/tmp/Caddyfile.next"
docker exec "$CADDY_CONTAINER" caddy validate --config /tmp/Caddyfile.next
cp "$CADDY_NEXT" "$CADDY_FILE"
docker exec "$CADDY_CONTAINER" caddy validate --config /etc/caddy/Caddyfile
docker exec "$CADDY_CONTAINER" caddy reload --config /etc/caddy/Caddyfile

curl --fail --silent https://api.yuaiapi.com/api/status | jq -e '.success == true'
curl --fail --silent https://global.yuaiapi.com/api/status | jq -e '.success == true'
curl --fail --silent https://vip.yuaiapi.com/api/status | jq -e '.success == true'
```

Run the ordinary and streaming probes again through the public API host. Run the
minimal paid image/video probe through the VIP host. If any probe fails, use the
immediate rollback block below before investigating.

In a normal production-origin browser, verify one existing user login and one
administrator login. Check profile, wallet, usage logs, private-group pricing,
users, channels, account pools, and system settings. Any new `500` triggers the
immediate Caddy rollback before investigation.

### Drain Relays From The Old Container

Caddy reloads are graceful, so connections accepted before the switch can
continue against the old container. Do not recreate that container until its
relay counter reaches zero on three consecutive samples. `/api/status/test`
reports the `StatsMiddleware` counter for relay routes; this admin request is
outside that counter and does not keep it above zero:

```bash
export OLD_DRAIN_STREAK=0
for attempt in $(seq 1 180); do
  OLD_ACTIVE="$(curl --fail --silent \
    -H "Authorization: Bearer $ADMIN_PAT" \
    http://127.0.0.1:3001/api/status/test \
    | jq -er '.http_stats.active_connections')"

  if test "$OLD_ACTIVE" = 0; then
    OLD_DRAIN_STREAK=$((OLD_DRAIN_STREAK + 1))
  else
    OLD_DRAIN_STREAK=0
  fi

  printf 'old relay connections=%s zero_streak=%s/3\n' "$OLD_ACTIVE" "$OLD_DRAIN_STREAK"
  if test "$OLD_DRAIN_STREAK" -ge 3; then
    break
  fi
  sleep 5
done
test "$OLD_DRAIN_STREAK" -ge 3
```

This allows up to 15 minutes for existing streams to finish. If it does not
drain, leave both containers running and stop before Phase 6. New traffic
continues on the candidate; no in-flight request is forcefully terminated.

## Phase 6: Recreate The Final Master Behind Candidate Traffic

Identify the Compose service without guessing. The expected service must own
the `newapi` container:

```bash
cd /opt/newapi
export APP_SERVICE="$(docker compose -f "$COMPOSE_FILE" config --services | grep -E '^new-?api$' | head -n 1)"
test -n "$APP_SERVICE"

cp -a "$COMPOSE_FILE" "$BACKUP_DIR/docker-compose.before-image-update.yml"
OLD_IMAGE_REF="$LIVE_IMAGE_REF" NEW_IMAGE="$NEW_IMAGE" perl -0pi -e 's/^(\s*image:\s*)\Q$ENV{OLD_IMAGE_REF}\E\s*$/${1}$ENV{NEW_IMAGE}/m' "$COMPOSE_FILE"
grep -F "image: $NEW_IMAGE" "$COMPOSE_FILE"

docker compose -f "$COMPOSE_FILE" config >/dev/null
docker compose -f "$COMPOSE_FILE" up -d --no-deps --force-recreate "$APP_SERVICE"

for attempt in $(seq 1 45); do
  if test "$(docker inspect --format '{{.State.Health.Status}}' "$APP_CONTAINER" 2>/dev/null || true)" = healthy; then
    break
  fi
  sleep 2
done

test "$(docker inspect --format '{{.State.Status}}' "$APP_CONTAINER")" = running
test "$(docker inspect --format '{{.State.Health.Status}}' "$APP_CONTAINER")" = healthy
test "$(docker inspect --format '{{.RestartCount}}' "$APP_CONTAINER")" = 0
test "$(docker inspect --format '{{.Config.Image}}' "$APP_CONTAINER")" = "$NEW_IMAGE"
curl --fail --silent http://127.0.0.1:3001/api/status | jq -e '.success == true'
docker exec "$CADDY_CONTAINER" wget -q -O /dev/null http://newapi:3000/api/status
```

The master startup is the only point that may apply reviewed additive
migrations. Caddy continues serving the healthy slave candidate during this
recreation. A migration error, unhealthy master, or restart blocks the final
traffic switch and triggers full rollback.

## Phase 7: Atomically Return Caddy To The Final Master

```bash
export CADDY_NEXT="$BACKUP_DIR/Caddyfile.to-final-master"
export FROM_UPSTREAM='newapi-candidate:3000'
export TO_UPSTREAM='newapi:3000'
export UPSTREAM_COUNT="$(grep -Fc "$FROM_UPSTREAM" "$CADDY_FILE")"
test "$UPSTREAM_COUNT" -ge 2

cp -a "$CADDY_FILE" "$CADDY_NEXT"
FROM_UPSTREAM="$FROM_UPSTREAM" TO_UPSTREAM="$TO_UPSTREAM" perl -0pi -e 's/\Q$ENV{FROM_UPSTREAM}\E/$ENV{TO_UPSTREAM}/g' "$CADDY_NEXT"
test "$(grep -Fc "$TO_UPSTREAM" "$CADDY_NEXT")" = "$UPSTREAM_COUNT"
test "$(grep -Fc "$FROM_UPSTREAM" "$CADDY_NEXT")" = 0

docker cp "$CADDY_NEXT" "$CADDY_CONTAINER:/tmp/Caddyfile.next"
docker exec "$CADDY_CONTAINER" caddy validate --config /tmp/Caddyfile.next
cp "$CADDY_NEXT" "$CADDY_FILE"
docker exec "$CADDY_CONTAINER" caddy validate --config /etc/caddy/Caddyfile
docker exec "$CADDY_CONTAINER" caddy reload --config /etc/caddy/Caddyfile

curl --fail --silent https://api.yuaiapi.com/api/status | jq -e '.success == true'
curl --fail --silent https://global.yuaiapi.com/api/status | jq -e '.success == true'
curl --fail --silent https://vip.yuaiapi.com/api/status | jq -e '.success == true'
```

Repeat login, private-group discovery, mapped-model privacy, ordinary relay,
streaming relay, and minimal VIP image/video checks against the final master.

## Observation And Rollback Triggers

Keep `newapi-candidate`, the old image, backups, and release artifact for at
least 30 minutes after the final switch. Sample at 1, 5, 15, and 30 minutes:

```bash
docker inspect --format 'status={{.State.Status}} health={{.State.Health.Status}} restarts={{.RestartCount}}' "$APP_CONTAINER"
docker stats --no-stream "$APP_CONTAINER" "$CANDIDATE_CONTAINER" "$CADDY_CONTAINER"
docker logs --since 5m "$APP_CONTAINER" 2>&1 | tail -n 300
docker logs --since 5m "$CADDY_CONTAINER" 2>&1 | tail -n 300
curl --fail --silent https://api.yuaiapi.com/api/status | jq -e '.success == true'
curl --fail --silent https://vip.yuaiapi.com/api/status | jq -e '.success == true'
```

Rollback immediately for any of these:

- application health is not green, any restart, panic, or migration error;
- sustained new `500`, `502`, `503`, `521`, or authentication failures;
- login, registration email, profile, wallet, or usage-log regressions;
- private groups disappear for authorized users or appear for unauthorized users;
- a mapped upstream name appears in an ordinary user's response, task, or log;
- pricing uses the mapped target instead of the public requested model;
- duplicate charge/refund, negative quota anomaly, or async settlement mismatch;
- retryable channel/account failures remain pinned to stale affinity;
- materially worse first-byte latency or streaming disconnects than the recorded
  baseline. `client_gone` alone is not a rollback signal unless it rises with
  server errors or reproducible server-side cancellation.

## Immediate Caddy Rollback

If traffic is currently on the candidate, restore the original Caddyfile and
reload. This does not restart the old application:

```bash
cp "$BACKUP_DIR/Caddyfile" "$CADDY_FILE"
docker exec "$CADDY_CONTAINER" caddy validate --config /etc/caddy/Caddyfile
docker exec "$CADDY_CONTAINER" caddy reload --config /etc/caddy/Caddyfile
curl --fail --silent https://api.yuaiapi.com/api/status | jq -e '.success == true'
curl --fail --silent https://vip.yuaiapi.com/api/status | jq -e '.success == true'
```

If the final master already replaced the old container, first move Caddy back
to the still-running candidate using the Phase 5 block. Then restore the old
Compose file and old immutable image while the candidate continues serving:

```bash
cp "$BACKUP_DIR/docker-compose.yml" "$COMPOSE_FILE"
test "$(docker image inspect --format '{{.Id}}' "$LIVE_IMAGE_REF")" = "$LIVE_IMAGE_ID"
docker compose -f "$COMPOSE_FILE" config >/dev/null
docker compose -f "$COMPOSE_FILE" up -d --no-deps --force-recreate "$APP_SERVICE"

for attempt in $(seq 1 45); do
  if test "$(docker inspect --format '{{.State.Health.Status}}' "$APP_CONTAINER" 2>/dev/null || true)" = healthy; then
    break
  fi
  sleep 2
done

test "$(docker inspect --format '{{.State.Health.Status}}' "$APP_CONTAINER")" = healthy
test "$(docker inspect --format '{{.Config.Image}}' "$APP_CONTAINER")" = "$LIVE_IMAGE_REF"
curl --fail --silent http://127.0.0.1:3001/api/status | jq -e '.success == true'

cp "$BACKUP_DIR/Caddyfile" "$CADDY_FILE"
docker exec "$CADDY_CONTAINER" caddy validate --config /etc/caddy/Caddyfile
docker exec "$CADDY_CONTAINER" caddy reload --config /etc/caddy/Caddyfile
curl --fail --silent https://api.yuaiapi.com/api/status | jq -e '.success == true'
curl --fail --silent https://vip.yuaiapi.com/api/status | jq -e '.success == true'
```

Do not roll back MySQL or Redis for an application rollback. The reviewed
migrations are additive and the old image must remain compatible with them.

## Successful Cleanup

Only after the full observation window and final user confirmation:

```bash
export CANDIDATE_DRAIN_STREAK=0
for attempt in $(seq 1 180); do
  CANDIDATE_ACTIVE="$(curl --fail --silent \
    -H "Authorization: Bearer $ADMIN_PAT" \
    "http://127.0.0.1:${CANDIDATE_PORT}/api/status/test" \
    | jq -er '.http_stats.active_connections')"

  if test "$CANDIDATE_ACTIVE" = 0; then
    CANDIDATE_DRAIN_STREAK=$((CANDIDATE_DRAIN_STREAK + 1))
  else
    CANDIDATE_DRAIN_STREAK=0
  fi

  printf 'candidate relay connections=%s zero_streak=%s/3\n' "$CANDIDATE_ACTIVE" "$CANDIDATE_DRAIN_STREAK"
  if test "$CANDIDATE_DRAIN_STREAK" -ge 3; then
    break
  fi
  sleep 5
done
test "$CANDIDATE_DRAIN_STREAK" -ge 3

docker rm -f "$CANDIDATE_CONTAINER"
rm -f /run/newapi-candidate.env
docker image inspect "$LIVE_IMAGE_REF" >/dev/null
docker exec "$CADDY_CONTAINER" rm -f /tmp/Caddyfile.next
```

Retain the old image and the timestamped backup directory until a later,
separately approved retention cleanup. Do not prune images, volumes, databases,
or the shared network as part of cutover.
