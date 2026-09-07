# Independent Sub2API Pool Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mirror official Sub2API main into ruoyuqi00/sub2api-provider-adapters:sub2api, deploy a fresh isolated account-pool service at sub2.yuaiapi.com, and connect it to YuAPI only over a private Docker network.

**Architecture:** The source mirror remains an unrelated Git history and contains no deployment secrets. The server runs a new Compose project with fresh PostgreSQL, Redis, and application volumes, a loopback-only administration listener on 127.0.0.1:18473, and the existing external Docker data plane `sub2api_sub2api-network` for YuAPI-to-Sub2API traffic. Existing YuAPI account-pool code and production channels remain intact until the new pool contains accounts and passes grey-release tests.

**Tech Stack:** Git/GitHub, Go, Docker Buildx, Docker Compose, PostgreSQL 18, Redis 8, Caddy/Cloudflare, and the existing YuAPI/NewAPI OpenAI relay adapter.

**Execution note (2026-09-07):** Production inventory found that the corrected
server already uses `sub2api_sub2api-network` as its stable YuAPI/edge data
plane. The new application therefore joined that existing network with alias
`sub2api-internal`; PostgreSQL and Redis remain isolated. Because the deployed
Sub2API group is OpenAI-compatible, YuAPI uses its existing OpenAI channel
adapter. No YuAPI source, image, container, database service, or Redis service
was rebuilt or restarted. The detailed implemented state is recorded in
`docs/YUAPI_SUB2API_INDEPENDENT_POOL_DEPLOYMENT_2026-09-07.md`.

**Spec:** docs/superpowers/specs/2026-09-06-sub2api-independent-deployment-design.md

## Global Constraints

- Official source is https://github.com/Wei-Shaw/sub2api.git, branch main; observed baseline ab99d56e9626e6cd731592dae8553c9758a0efa2.
- User repository is https://github.com/ruoyuqi00/sub2api-provider-adapters.git; target branch is exactly sub2api.
- Never merge Sub2API history into YuAPI/NewAPI main, and never force-push the target branch.
- Do not restore, delete, or reuse /opt/sub2api, its old containers, old volumes, old configuration, or old backups.
- New directory is /opt/yuapi-sub2api-v2; Compose project is yuapi-sub2api-v2.
- New app listener is 127.0.0.1:18473:8080; PostgreSQL and Redis expose no host ports.
- New containers are yuapi-sub2api-v2-app, yuapi-sub2api-v2-postgres, and yuapi-sub2api-v2-redis.
- New private network is yuapi-sub2api-v2-internal; the shared external network is the pre-existing sub2api_sub2api-network; app alias is sub2api-internal.
- Public administration is https://sub2.yuaiapi.com; unknown paths and all model gateway paths remain denied by a positive edge allowlist.
- Root-only server .env has mode 0600; secrets never enter Git or logs.
- Registration, password recovery, OAuth sign-up, and payment are disabled; the administrator enables TOTP after first login.
- The YuAPI OpenAI channel is created disabled with no enabled abilities until accounts and grey-release tests pass.
- Do not claim success until the design spec verification matrix has been run and recorded.

---

### Task 1: Mirror Official Sub2API Into the User-Owned sub2api Branch

**Files:**
- Create: remote branch sub2api in ruoyuqi00/sub2api-provider-adapters
- Test: Git remote object IDs and ancestry checks

**Interfaces:**
- Consumes: official https://github.com/Wei-Shaw/sub2api.git main
- Produces: the exact official main commit at ruoyuqi00/sub2api-provider-adapters:sub2api

- [ ] Step 1: Fetch both remotes into a disposable bare mirror

~~~powershell
$mirror = Join-Path $env:TEMP 'sub2api-branch-mirror-20260906.git'
if (Test-Path -LiteralPath $mirror) { Remove-Item -Recurse -Force -LiteralPath $mirror }
git clone --bare https://github.com/Wei-Shaw/sub2api.git $mirror
git -C $mirror remote add ruoyu https://github.com/ruoyuqi00/sub2api-provider-adapters.git
$official = git -C $mirror rev-parse refs/heads/main
$existing = git -C $mirror ls-remote ruoyu refs/heads/sub2api
~~~

- [ ] Step 2: Create only when the branch is absent, otherwise require a fast-forward

~~~powershell
if ($existing) {
  $existingSha = ($existing -split '\s+')[0]
  git -C $mirror fetch ruoyu refs/heads/sub2api:refs/remotes/ruoyu/sub2api
  git -C $mirror merge-base --is-ancestor refs/remotes/ruoyu/sub2api refs/heads/main
  if ($LASTEXITCODE -ne 0) {
    throw "Remote sub2api at $existingSha is not an ancestor of official main"
  }
  git -C $mirror push ruoyu refs/heads/main:refs/heads/sub2api
} else {
  git -C $mirror push ruoyu refs/heads/main:refs/heads/sub2api
}
~~~

Expected: no force-push; a divergent existing branch stops the task.

- [ ] Step 3: Record the immutable source SHA

~~~powershell
git -C $mirror ls-remote ruoyu refs/heads/sub2api
git -C $mirror show --no-patch --format='%H %cI %s' refs/heads/main
~~~

### Task 2: Verify SSH Access and Inventory the Production Server

**Files:**
- Read: server inventory at 199.231.85.194
- Read: exact edge-proxy configuration path returned by Docker/systemd inventory
- Read: /opt/newapi and /opt/sub2api Compose metadata

**Interfaces:**
- Consumes: the configured production SSH identity
- Produces: collision-free inventory plus EDGE_CONFIG, EDGE_CONTAINER, and YUAPI_CONTAINER values for later tasks

- [ ] Step 1: Verify SSH before any mutation

~~~powershell
$key = Join-Path $env:USERPROFILE '.ssh\yuaet_codex_ed25519'
ssh -vvv -i $key -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 root@199.231.85.194 true
~~~

Expected: exit code 0. If the server closes before an SSH banner, stop and request the correct SSH user/key, firewall allowlist, or console access.

- [ ] Step 2: Run a read-only inventory

~~~bash
set -eu
hostname
docker ps -a --format '{{.Names}}|{{.Image}}|{{.Status}}|{{.Ports}}'
docker network ls
docker volume ls
ss -ltnp
for name in newapi sub2api sub2api-postgres sub2api-redis sub2api-caddy; do
  docker inspect "$name" --format '{{.Name}}|{{.Config.Image}}|{{.Config.Labels}}|{{json .NetworkSettings.Networks}}' 2>/dev/null || true
done
find /opt/newapi /opt/sub2api -maxdepth 2 -type f -printf '%p\n' 2>/dev/null | sort
systemctl is-active caddy 2>/dev/null || true
systemctl cat caddy 2>/dev/null || true
~~~

Record the exact edge-proxy path and current YuAPI container. Do not alter the old stack.

- [ ] Step 3: Check all proposed resource collisions

~~~bash
test ! -e /opt/yuapi-sub2api-v2
! docker ps -a --format '{{.Names}}' | grep -Fxq yuapi-sub2api-v2-app
! docker ps -a --format '{{.Names}}' | grep -Fxq yuapi-sub2api-v2-postgres
! docker ps -a --format '{{.Names}}' | grep -Fxq yuapi-sub2api-v2-redis
! docker network ls --format '{{.Name}}' | grep -Fxq yuapi-sub2api-v2-internal
! ss -ltn '( sport = :18473 )' | grep -q 18473
~~~

Expected: every assertion succeeds. Any collision is a stop condition.

- [ ] Step 4: Audit the existing shared data plane before reusing it

~~~bash
docker network inspect sub2api_sub2api-network --format '{{.Name}}|{{json .Labels}}|{{json .Containers}}'
~~~

Confirm that `sub2api_sub2api-network` is the existing YuAPI/edge data plane. Do not remove, rename, or recreate it. Stop if it is absent or is not attached to the approved production resources.

### Task 3: Clone and Build the Pinned Sub2API Image

**Files:**
- Create on server: /opt/yuapi-sub2api-v2/src
- Create on server: local image yuapi-sub2api:OFFICIAL_SHA

**Interfaces:**
- Consumes: OFFICIAL_SHA from Task 1 and the collision-free inventory from Task 2
- Produces: a locally built immutable image tagged with the official commit

- [ ] Step 1: Clone the target branch at the recorded SHA

~~~bash
install -d -m 0755 /opt/yuapi-sub2api-v2
git clone --branch sub2api --single-branch https://github.com/ruoyuqi00/sub2api-provider-adapters.git /opt/yuapi-sub2api-v2/src
cd /opt/yuapi-sub2api-v2/src
git fetch --depth=1 origin sub2api
git checkout --detach "$OFFICIAL_SHA"
test "$(git rev-parse HEAD)" = "$OFFICIAL_SHA"
printf '%s\n' "$OFFICIAL_SHA" >/opt/yuapi-sub2api-v2/source-sha
~~~

- [ ] Step 2: Build with the exact source SHA

~~~bash
cd /opt/yuapi-sub2api-v2/src
OFFICIAL_SHA=$(git rev-parse HEAD)
docker buildx build --load --tag "yuapi-sub2api:$OFFICIAL_SHA" --build-arg COMMIT="$OFFICIAL_SHA" .
docker image inspect "yuapi-sub2api:$OFFICIAL_SHA" --format '{{index .Config.Labels "org.opencontainers.image.source"}}|{{.Id}}'
~~~

Expected: build succeeds and the image ID is recorded. If Buildx cannot load the image, stop before Compose creation.

### Task 4: Create the Fresh Isolated Compose Stack

**Files:**
- Create on server: /opt/yuapi-sub2api-v2/docker-compose.yml
- Create on server: /opt/yuapi-sub2api-v2/.env with mode 0600
- Create on server: /opt/yuapi-sub2api-v2/credentials.txt with mode 0600, temporary

**Interfaces:**
- Consumes: pinned image from Task 3 and collision-free inventory from Task 2
- Produces: healthy application, PostgreSQL, and Redis containers with fresh volumes and private network

- [ ] Step 1: Generate secrets without echoing them

~~~bash
umask 077
OFFICIAL_SHA=$(cat /opt/yuapi-sub2api-v2/source-sha)
POSTGRES_PASSWORD=$(openssl rand -hex 32)
REDIS_PASSWORD=$(openssl rand -hex 32)
ADMIN_PASSWORD=$(openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | head -c 32)
JWT_SECRET=$(openssl rand -hex 32)
TOTP_ENCRYPTION_KEY=$(openssl rand -hex 32)
printf 'OFFICIAL_SHA=%s\nADMIN_EMAIL=admin@yuapi.internal\nPOSTGRES_PASSWORD=%s\nREDIS_PASSWORD=%s\nADMIN_PASSWORD=%s\nJWT_SECRET=%s\nTOTP_ENCRYPTION_KEY=%s\n' "$OFFICIAL_SHA" "$POSTGRES_PASSWORD" "$REDIS_PASSWORD" "$ADMIN_PASSWORD" "$JWT_SECRET" "$TOTP_ENCRYPTION_KEY" > /opt/yuapi-sub2api-v2/.env
chmod 0600 /opt/yuapi-sub2api-v2/.env
printf 'ADMIN_EMAIL=admin@yuapi.internal\nADMIN_PASSWORD=%s\n' "$ADMIN_PASSWORD" > /opt/yuapi-sub2api-v2/credentials.txt
chmod 0600 /opt/yuapi-sub2api-v2/credentials.txt
unset POSTGRES_PASSWORD REDIS_PASSWORD ADMIN_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY
~~~

- [ ] Step 2: Write the Compose file with explicit names and networks

Create the file with these exact service properties:

~~~yaml
name: yuapi-sub2api-v2
services:
  app:
    image: yuapi-sub2api:${OFFICIAL_SHA}
    container_name: yuapi-sub2api-v2-app
    restart: unless-stopped
    security_opt: [no-new-privileges:true]
    ports: ['127.0.0.1:18473:8080']
    env_file: [.env]
    environment:
      AUTO_SETUP: 'true'
      SERVER_HOST: 0.0.0.0
      SERVER_PORT: '8080'
      SERVER_MODE: release
      DATABASE_HOST: postgres
      DATABASE_PORT: '5432'
      DATABASE_USER: sub2api
      DATABASE_PASSWORD: ${POSTGRES_PASSWORD}
      DATABASE_DBNAME: sub2api
      DATABASE_SSLMODE: disable
      REDIS_HOST: redis
      REDIS_PORT: '6379'
      REDIS_DB: '0'
      TZ: Asia/Shanghai
    volumes: [sub2api_data:/app/data]
    depends_on:
      postgres: {condition: service_healthy}
      redis: {condition: service_healthy}
    networks:
      internal: {}
      data_plane:
        aliases: [sub2api-internal]
    healthcheck:
      test: ['CMD', 'wget', '-q', '-T', '5', '-O', '/dev/null', 'http://localhost:8080/health']
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 120s
  postgres:
    image: postgres:18-alpine
    container_name: yuapi-sub2api-v2-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: sub2api
      POSTGRES_PASSWORD: $POSTGRES_PASSWORD
      POSTGRES_DB: sub2api
      PGDATA: /var/lib/postgresql/data
      TZ: Asia/Shanghai
    volumes: [postgres_data:/var/lib/postgresql/data]
    networks: [internal]
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U sub2api -d sub2api && psql -U sub2api -d sub2api -Atqc "SELECT 1" >/dev/null']
      interval: 10s
      timeout: 5s
      retries: 18
      start_period: 60s
  redis:
    image: redis:8-alpine
    container_name: yuapi-sub2api-v2-redis
    restart: unless-stopped
    command: ['sh', '-c', 'exec redis-server --appendonly yes --appendfsync everysec --requirepass "$$REDIS_PASSWORD"']
    environment:
      REDIS_PASSWORD: $REDIS_PASSWORD
      TZ: Asia/Shanghai
    volumes: [redis_data:/data]
    networks: [internal]
    healthcheck:
      test: ['CMD-SHELL', 'REDISCLI_AUTH="$$REDIS_PASSWORD" redis-cli ping | grep -q PONG']
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 5s
volumes:
  sub2api_data: {}
  postgres_data: {}
  redis_data: {}
networks:
  internal:
    name: yuapi-sub2api-v2-internal
  data_plane:
    name: sub2api_sub2api-network
    external: true
~~~

When writing the file, use a shell that expands the defined deployment variables; keep Compose runtime dollar signs escaped where needed. Do not copy the official Compose container_name sub2api or bind 0.0.0.0:8080.

- [ ] Step 3: Validate the existing external data plane and start the new stack

~~~bash
docker network inspect sub2api_sub2api-network >/dev/null
docker compose -f /opt/yuapi-sub2api-v2/docker-compose.yml --env-file /opt/yuapi-sub2api-v2/.env config >/opt/yuapi-sub2api-v2/compose.rendered.yml
docker compose -p yuapi-sub2api-v2 -f /opt/yuapi-sub2api-v2/docker-compose.yml --env-file /opt/yuapi-sub2api-v2/.env up -d
~~~

The containerized edge proxy and YuAPI already use `sub2api_sub2api-network`; do not change their network membership and never attach either to the private database network.

- [ ] Step 4: Verify fresh state and persistence

~~~bash
docker ps --format '{{.Names}}|{{.Image}}|{{.Status}}|{{.Ports}}' | grep '^yuapi-sub2api-v2-'
docker volume ls --format '{{.Name}}' | grep '^yuapi-sub2api-v2_'
docker inspect yuapi-sub2api-v2-app --format '{{json .NetworkSettings.Networks}}'
curl --fail --silent http://127.0.0.1:18473/health
~~~

Expected: health 200, loopback-only port, three new volumes, app on both networks, PostgreSQL/Redis only on the internal network.

### Task 5: Configure the Fixed Administrator and Disable Signup Features

**Files:**
- Modify server database settings through the authenticated admin settings API/UI
- Retain /opt/yuapi-sub2api-v2/credentials.txt until login and TOTP enrollment are confirmed

**Interfaces:**
- Consumes: initial admin credentials from Task 4
- Produces: one administrator, disabled registration/recovery/OAuth signup/payment settings, and a TOTP-ready account

- [ ] Step 1: Confirm initial login over loopback before exposing DNS

~~~bash
admin_password=$(sed -n 's/^ADMIN_PASSWORD=//p' /opt/yuapi-sub2api-v2/credentials.txt)
printf '{"email":"admin@yuapi.internal","password":"%s"}\n' "$admin_password" >/tmp/sub2api-login.json
chmod 0600 /tmp/sub2api-login.json
curl --fail --silent -c /tmp/sub2api.cookies -H 'Content-Type: application/json' --data-binary @/tmp/sub2api-login.json http://127.0.0.1:18473/api/v1/auth/login
rm -f /tmp/sub2api-login.json
unset admin_password
~~~

Use the browser/SSH tunnel for the real login and never paste the password into shell history. A failed login stops the task.

- [ ] Step 2: Disable registration and payment through the admin settings contract

In the authenticated admin settings request, set registration_enabled=false, keep all OAuth provider credentials empty, and leave payment provider instances disabled/unconfigured. Verify the public settings response reports registration_enabled=false. Use the pinned source contract in backend/internal/handler/admin/setting_handler_update.go; do not write raw SQL against settings tables.

- [ ] Step 3: Enroll TOTP and rotate the temporary password

Use the admin profile TOTP setup flow, verify one generated code, then change the generated password to the user-provided password. Delete credentials.txt only after the user confirms successful login and 2FA.

- [ ] Step 4: Create the private YuAPI group and API key

In the Sub2API administrator UI, create one active group named yuapi-internal with platform openai, subscription type standard, and no public-facing payment or signup settings. Create an API key owned by the administrator, bind it to that group, and store the key in a root-only file under /opt/yuapi-sub2api-v2 with mode 0600. This key is an internal credential for the YuAPI channel; never put it in Git, Caddy, a public URL, or the redacted handoff. If later provider protocols need different Sub2API group platforms, create separate keys and channels rather than weakening this group boundary.

### Task 6: Configure HTTPS and the Positive Public Allowlist

**Files:**
- Modify: EDGE_CONFIG identified in Task 2
- Create if the proxy supports includes: /opt/yuapi-sub2api-v2/Caddyfile
- Backup: the path printed by the backup command in Step 1, under /opt/newapi/backups/caddy-before-sub2api-

**Interfaces:**
- Consumes: healthy loopback service from Task 4 and DNS sub2.yuaiapi.com
- Produces: Cloudflare-compatible HTTPS administration host with gateway paths denied

- [ ] Step 1: Back up and validate the current edge proxy

~~~bash
install -d -m 0700 /opt/newapi/backups
cp -a "$EDGE_CONFIG" "/opt/newapi/backups/caddy-before-sub2api-$(date +%Y%m%d%H%M%S)"
caddy validate --config "$EDGE_CONFIG"
~~~

- [ ] Step 2: Add the sub2.yuaiapi.com virtual host

Use the existing proxy syntax and include mechanism. Proxy only /, /home, /assets/*, /admin/*, /login*, /profile/*, /legal/*, /api/v1/auth/login, /api/v1/auth/login/2fa, /api/v1/auth/refresh, /api/v1/auth/logout, /api/v1/auth/me, /api/v1/user/*, /api/v1/settings/public, /api/v1/admin/*, /health, and /setup/status. The edge proxy is containerized and already shares `sub2api_sub2api-network`, so proxy to `sub2api-internal:8080` without changing its network membership. Add a final handler returning 404 for every other path, including registration, password-recovery, and OAuth sign-up endpoints. Do not proxy /v1, /v1beta, /responses, /models, /chat/completions, /embeddings, /images, /videos, /backend-api, /messages, /alpha, /antigravity, /tts, /stt, /custom-voices, /realtime, /web_search, or /x_search. Use the existing certificate automation or a Cloudflare Origin Certificate for this host, keep the key root-only, and set Cloudflare SSL/TLS mode to Full (strict).

Use this host block, replacing the upstream only when Task 2 proves the edge proxy is containerized:

~~~caddyfile
sub2.yuaiapi.com {
  @control path /api/v1/auth/login /api/v1/auth/login/2fa /api/v1/auth/refresh /api/v1/auth/logout /api/v1/auth/me /api/v1/user/* /api/v1/settings/public /api/v1/admin/* /health /setup/status
  handle @control {
    reverse_proxy sub2api-internal:8080
  }

  @ui {
    method GET HEAD
    path / /home /login /login/* /admin /admin/* /profile /profile/* /legal/* /assets/* /logo.svg /favicon.ico
  }
  handle @ui {
    reverse_proxy sub2api-internal:8080
  }

  respond 404
}
~~~

Do not attach Caddy to the private database network. If its single-file bind mount points to a replaced inode, recreate only the edge container after recording and restoring its existing network memberships; do not restart YuAPI.

- [ ] Step 3: Validate, reload, and verify Cloudflare TLS

~~~bash
caddy validate --config "$EDGE_CONFIG"
systemctl reload caddy 2>/dev/null || docker exec "$EDGE_CONTAINER" caddy reload --config "$EDGE_CONFIG"
curl --fail --silent --show-error --head https://sub2.yuaiapi.com/
curl --silent --show-error -o /dev/null -w '%{http_code}\n' https://sub2.yuaiapi.com/v1/models
curl --silent --show-error -o /dev/null -w '%{http_code}\n' https://sub2.yuaiapi.com/responses
~~~

Expected: root returns 200/3xx without 525; model gateway probes return 403/404.

### Task 7: Connect YuAPI Without Enabling Traffic

**Files:**
- Modify: one disabled channel through the existing YuAPI administration API
- Modify: no YuAPI source, image, Compose file, container, database service, or Redis service

**Interfaces:**
- Consumes: http://sub2api-internal:8080, the new Sub2API API key, and current YuAPI ruoyu/main
- Produces: a disabled standard OpenAI channel with no active abilities

- [ ] Step 1: Verify existing OpenAI compatibility and private DNS

~~~bash
docker exec "$YUAPI_CONTAINER" sh -c 'getent hosts sub2api-internal && wget -q -O- http://sub2api-internal:8080/health'
# Request /v1/models with the root-only internal key without printing the key.
~~~

The configured Sub2API group uses the OpenAI contract, including `/v1/models`,
chat completions, and Responses. The existing YuAPI OpenAI adapter is therefore
sufficient and no dedicated adapter or source deployment is required.

- [ ] Step 2: Keep the unused dedicated-adapter contingency out of production

Do not port commit `2d23cdf2915432632e37637198a72c752d642bcf`; its dedicated adapter is unnecessary for the OpenAI group and is bundled with unrelated billing and relay work. Future non-OpenAI groups must be implemented as separate protocol-specific groups, keys, and YuAPI channels.

- [ ] Step 3: Verify that no YuAPI source diff exists

~~~powershell
git diff --check
~~~

Expected: no adapter files or production build changes.

- [ ] Step 4: Keep the existing OpenAI channel label

Name the channel `Sub2API Internal Pool`; no new frontend label or locale key is needed.

- [ ] Step 5: Do not rebuild or restart YuAPI

Record the current YuAPI container ID, start time, and health before and after channel creation. They must remain unchanged.

- [ ] Step 6: Create the disabled channel and internal key

In the YuAPI admin panel, create a standard OpenAI channel named `Sub2API Internal Pool` with base URL `http://sub2api-internal:8080`, the new internal API key, manual-disabled status, and lower grey-release priority. Confirm every generated ability is disabled. Do not remove or disable existing YuAPI account-pool code or channels.

### Task 8: Verify Private Routing, Public Denials, and Existing YuAPI Health

**Files:**
- Create on server: /opt/yuapi-sub2api-v2/verification-record.txt
- Read: YuAPI and Sub2API container logs

**Interfaces:**
- Consumes: Tasks 4-7
- Produces: evidence that public administration works, public gateway is denied, private health works, and existing YuAPI traffic is unchanged

- [ ] Step 1: Check all listeners and network membership

~~~bash
ss -ltnp | grep -E '(:18473|:5432|:6379)' || true
docker inspect yuapi-sub2api-v2-app --format '{{json .NetworkSettings.Networks}}'
docker inspect yuapi-sub2api-v2-postgres --format '{{json .NetworkSettings.Networks}}'
docker inspect yuapi-sub2api-v2-redis --format '{{json .NetworkSettings.Networks}}'
~~~

Expected: only 127.0.0.1:18473 is exposed; database and Redis have no host listener.

- [ ] Step 2: Test the public boundary

~~~bash
for path in / /login /admin /api/v1/settings/public /api/v1/auth/register /api/v1/auth/forgot-password /api/v1/auth/oauth/github/start /v1/models /responses /chat/completions /backend-api/codex/responses /v1beta/models /images/generations /videos; do
  code=$(curl -ksS -o /dev/null -w '%{http_code}' "https://sub2.yuaiapi.com$path")
  printf '%s %s\n' "$code" "$path"
done
~~~

Expected: UI/control paths behave as designed; registration/recovery/OAuth signup and every gateway path return 403/404.

- [ ] Step 3: Test private DNS and health from the YuAPI container

~~~bash
docker exec "$YUAPI_CONTAINER" sh -c 'getent hosts sub2api-internal && wget -q -O- http://sub2api-internal:8080/health'
~~~

Expected: DNS resolves on the existing `sub2api_sub2api-network` data plane and health returns `{"status":"ok"}`. PostgreSQL and Redis remain absent from that shared network.

- [ ] Step 4: Confirm empty-pool behavior and existing YuAPI smoke traffic

Use the disabled channel and empty internal group to confirm no model request is selected. Run the existing YuAPI local status, pricing, unauthenticated model probes, and one known-good production smoke request; compare status, channel ID, and logs with the pre-deploy record. Do not enable the new channel.

- [ ] Step 5: Restart and verify persistence

~~~bash
docker compose -p yuapi-sub2api-v2 -f /opt/yuapi-sub2api-v2/docker-compose.yml --env-file /opt/yuapi-sub2api-v2/.env restart
docker compose -p yuapi-sub2api-v2 -f /opt/yuapi-sub2api-v2/docker-compose.yml --env-file /opt/yuapi-sub2api-v2/.env ps
curl --fail --silent http://127.0.0.1:18473/health
~~~

Expected: all three services become healthy, admin settings persist, and no old service restarts.

- [ ] Step 6: Record and validate the non-destructive rollback sequence

~~~bash
docker compose -p yuapi-sub2api-v2 -f /opt/yuapi-sub2api-v2/docker-compose.yml --env-file /opt/yuapi-sub2api-v2/.env config --quiet
docker network inspect sub2api_sub2api-network --format '{{json .Containers}}'
~~

Record these rollback actions without running them on a successful deployment: disable the new YuAPI channel; restore the Caddy backup captured in Task 6 and validate/reload Caddy; run docker compose stop for yuapi-sub2api-v2. Never disconnect or remove the pre-existing `sub2api_sub2api-network`, never run docker compose down -v, and never touch /opt/sub2api.

### Task 9: Document Operations and Commit the Implementation Record

**Files:**
- Create: docs/YUAPI_SUB2API_INDEPENDENT_POOL_DEPLOYMENT_2026-09-07.md
- Modify: no existing production source or historical status record until deployment actually succeeds

**Interfaces:**
- Consumes: verification record from Task 8 and recorded image/branch SHAs
- Produces: a redacted handoff with rollback commands and no credentials

- [ ] Step 1: Write the redacted deployment record

Include official SHA, user branch SHA, image tag, Compose project, container/network/volume names, host port, domain, edge-proxy config backup, verification results, known SSH/TLS issues, and exact rollback commands. Do not include API keys, passwords, OAuth tokens, private key material, or the contents of `.env`/`credentials.txt`.

- [ ] Step 2: Run documentation checks

~~~powershell
git diff --check
Select-String -Path docs/YUAPI_SUB2API_INDEPENDENT_POOL_DEPLOYMENT_2026-09-07.md -Pattern 'password=|api[_-]?key=|secret=|token=|TOTP_ENCRYPTION_KEY|POSTGRES_PASSWORD|REDIS_PASSWORD' -CaseSensitive:$false
~~~

Expected: the scan returns no credential values; remove any variable values before committing.

- [ ] Step 3: Commit only the approved YuAPI changes and redacted record

~~~powershell
git add -- docs/YUAPI_SUB2API_INDEPENDENT_POOL_DEPLOYMENT_2026-09-07.md
git add -- docs/superpowers/specs/2026-09-06-sub2api-independent-deployment-design.md docs/superpowers/plans/2026-09-06-sub2api-independent-deployment.md
git commit -m 'docs: record isolated sub2api account pool deployment'
~~~

Never stage .env, server Compose overlays, credential files, generated frontend artifacts, or unrelated pre-existing worktree changes.
