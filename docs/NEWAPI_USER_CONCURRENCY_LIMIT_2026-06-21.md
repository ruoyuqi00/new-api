# NewAPI User Concurrency Limit - 2026-06-21

This note records the NewAPI-side user concurrency guard prepared for the
public rollout. Do not put API keys, passwords, refresh tokens, or full account
payloads in this file.

## Intent

NewAPI remains the user-facing control plane:

- users, API keys, billing, visible groups, and public API access live in
  NewAPI;
- Sub2API remains the internal scheduling and supply/account-pool layer;
- normal users should call `https://api.dtrljm.com/v1`, not Sub2API directly.

To prevent one downstream user from occupying too much upstream capacity, the
NewAPI relay path now has a small user concurrency overlay:

- default per-user in-flight model requests: `5`;
- can be enabled/disabled from system settings;
- can be globally changed later;
- can be overridden per group with a JSON object;
- separate from existing request-rate limits, so RPM/window throttling remains
  independent.

This answers the operational requirement: new users start at concurrency `5`,
and the operator can tune it later without changing code.

## Patch Location

The NewAPI working tree is not the authoritative main repository for this
stack. The reproducible overlay patch is stored in the Sub2API maintenance
repository:

```text
patches/newapi/user-concurrency-limit-20260621.patch
```

Apply it to the NewAPI checkout with:

```bash
cd /path/to/new-api
git apply /path/to/sub2api-private/patches/newapi/user-concurrency-limit-20260621.patch
```

## What The Patch Changes

Backend:

- adds `middleware.UserConcurrencyLimit()`;
- installs it after `TokenAuth()` and before existing model request rate limits
  on OpenAI-compatible `/v1` relay routes;
- installs it on Gemini `/v1beta` relay routes;
- adds option keys:
  - `UserConcurrencyLimitEnabled`
  - `UserConcurrencyLimit`
  - `UserConcurrencyLimitGroup`
- validates per-group concurrency JSON.

Frontend:

- adds controls in NewAPI system settings -> security/rate limiting:
  - enable user concurrency limit;
  - default user concurrency;
  - group-based concurrency overrides.

Tests:

- adds `middleware/user-concurrency-limit_test.go`;
- verifies that the second simultaneous request for the same user receives
  HTTP `429`, and that the slot is released after the first request completes.

## Runtime Settings

Default behavior after the patch is applied:

```text
UserConcurrencyLimitEnabled=true
UserConcurrencyLimit=5
UserConcurrencyLimitGroup={}
```

Example group override:

```json
{
  "gpt-team": 5,
  "gpt-plus": 8,
  "gpt-pro": 12
}
```

`0` means unlimited for that scope. Keep public/default users at `5` unless
there is enough Sub2API upstream capacity to raise it.

## Verification

Local backend verification on 2026-06-21 used the portable Go toolchain:

```powershell
$env:GOROOT='D:\wflogin\.tools\go1.26.4\go'
$env:PATH="$env:GOROOT\bin;$env:PATH"
go test -count=1 ./middleware -run TestUserConcurrencyLimit
go test -count=1 ./middleware ./setting ./router
go test -count=1 ./controller -run '^$'
```

Result:

- `./middleware` targeted concurrency test passed;
- `./middleware ./setting ./router` passed;
- `./controller -run '^$'` compiled successfully.

Broader `./controller` test execution hit an existing local SQLite fixture issue
(`sql: database is closed`) in unrelated model-list tests, so it was not used as
the gating signal for this overlay.

Frontend type/build verification was not completed in this pass because the
local NewAPI frontend uses catalog-style dependencies and no lockfile was
available in the workspace. Verify during the next NewAPI image build.

## Deployment Guidance

Do not run a full NewAPI Docker build on the production server while it is under
active traffic. A previous server-side build attempt pushed the host load above
100 and made SSH/HTTPS health checks intermittently time out.

Preferred deployment path:

1. apply the patch to a clean NewAPI checkout;
2. run backend tests and frontend build off the production server;
3. build a custom NewAPI image off-server;
4. `docker save` and upload the image to the production server;
5. `docker load` on the server;
6. back up `/opt/newapi/docker-compose.yml`;
7. switch only the `newapi` service image tag;
8. recreate only the NewAPI application container;
9. verify:
   - `https://api.dtrljm.com/api/status`;
   - `https://dtrljm.com/`;
   - NewAPI system settings show the concurrency controls;
   - a controlled same-user concurrent request test returns one success and one
     HTTP `429` when the limit is `1`.

If a production-server build is unavoidable, schedule a low-traffic maintenance
window and confirm host load is normal before starting.

## Deployment Result

The overlay was deployed on 2026-06-21 with the prebuilt-image flow:

1. built locally on Windows Docker Desktop:
   `newapi:user-concurrency-20260621-81d4a3f6`;
2. saved to a tar archive locally;
3. uploaded the tar archive to the server;
4. loaded it with `docker load` on the server;
5. backed up `/opt/newapi/docker-compose.yml`;
6. changed only the `newapi` service image to:
   `newapi:user-concurrency-20260621-81d4a3f6`;
7. recreated only the NewAPI application container:
   `docker compose up -d --no-deps newapi`.

Database and Redis containers were not recreated.

Production status after deployment:

- `newapi` container image:
  `newapi:user-concurrency-20260621-81d4a3f6`;
- container state: `running healthy`;
- `newapi-mysql` and `newapi-redis`: still `healthy`;
- `https://api.dtrljm.com/api/status`: HTTP `200`;
- `https://dtrljm.com/api/status`: HTTP `200`;
- `https://api.vyywcw.cn/health`: HTTP `200`;
- host load after deployment was normal.

Backup file:

```text
/opt/newapi/docker-compose.yml.bak-user-concurrency-20260621-153134
```

Rollback path:

```bash
cd /opt/newapi
cp docker-compose.yml.bak-user-concurrency-20260621-153134 docker-compose.yml
docker compose up -d --no-deps newapi
```

The old upstream image remained present on the server:

```text
calciumion/new-api:latest
```

Database inspection did not show persisted values for the three new option
keys, so production is currently using the code defaults:

```text
UserConcurrencyLimitEnabled=true
UserConcurrencyLimit=5
UserConcurrencyLimitGroup={}
```

When the admin settings page saves these options later, they will become
database-backed runtime values.

## Production Build Policy

Do not run full NewAPI Docker builds on the production server. The failed
server-side build attempt pushed load over 100, made SSH unreliable, and caused
public HTTPS health checks to time out. Future NewAPI and large frontend-backed
deployments must use the prebuilt-image flow:

```text
build off-server -> docker save or push -> docker load or pull on server ->
compose image tag switch -> recreate only the target app container
```

Sub2API builds should follow the same policy when the change requires a heavy
image build. Lightweight config changes and `docker compose up -d --no-deps`
against an already loaded image remain acceptable.
