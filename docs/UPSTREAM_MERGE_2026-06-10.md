# Sub2API upstream merge and deploy record - 2026-06-10

## Scope

- Merged `upstream/main` from `Wei-Shaw/sub2api` through `0acf00c4`.
- Local merge commit: `70248bc8`.
- Local Docker build fix commit: `147d5515`.
- Deployed image: `sub2api-provider-adapters:upstream-merge-20260610-147d5515`.
- Compose backup on server: `/opt/sub2api/docker-compose.yml.bak-upstream-merge-20260610-147d5515`.

## Useful upstream changes included

- OpenAI `/responses` transport errors now participate in failover and temporary unscheduling.
- OpenAI failover path has a model body replacement fix.
- Gateway error-frame handling avoids double writes in stream/error paths.
- Non-streaming responses now return `Content-Type: application/json`.
- Bedrock beta/header filtering was tightened.
- Account group scheduler indexes were added.
- Proxy expiry/fallback behavior was improved.
- Admin user API key list can filter by group.
- Claude Fable 5 and OpenCode adaptive thinking configuration were added upstream.
- Idempotency keys now handle UTF-8 truncation more safely.
- Gateway debug log loop was optimized.
- New admin compliance acknowledgement gate was added.

## Local compatibility fix

Upstream added frontend raw imports for `docs/legal/admin-compliance.*.md`.
The local Docker build initially failed because the frontend build stage only copied `frontend/`, while `.dockerignore` excluded `docs/`.

Fixed in `147d5515`:

- `Dockerfile` copies `docs/legal/` into `/app/docs/legal/` before `pnpm run build`.
- `deploy/Dockerfile` applies the same copy rule.
- `.dockerignore` still excludes most docs, but allows `docs/legal/*.md`.

## Validation

Local validation:

- `go test ./internal/service ./internal/handler ./internal/server ./internal/pkg/antigravity ./internal/pkg/claude`
- `go test ./...` from `backend/`
- `corepack pnpm test:run src/components/keys/__tests__/UseKeyModal.spec.ts src/api/__tests__/client.spec.ts`
- `corepack pnpm build` from `frontend/`

Server validation:

- Docker image build succeeded for `sub2api-provider-adapters:upstream-merge-20260610-147d5515`.
- `docker compose ps sub2api` showed the new image as `Up` and `healthy`.
- Internal health check returned `{"status":"ok"}` from `http://127.0.0.1:8080/health`.
- Public health check returned HTTP 200 and `{"status":"ok"}` from `https://api.vyywcw.cn/health`.
- Recent server logs were scanned for `panic`, `fatal`, and `error`; no matches were found after deploy.

## Operational notes

- Only the `sub2api` service was recreated. Database, NewAPI, and other services were not restarted.
- Admin users may see the new upstream compliance acknowledgement dialog after login.
- The previous image was `sub2api-provider-adapters:upstream-merge-20260607-c75c6b1a`.

## Rollback

If rollback is needed, restore the old image tag in `/opt/sub2api/docker-compose.yml` or restore the compose backup above, then run:

```bash
cd /opt/sub2api
docker compose up -d --no-deps --force-recreate sub2api
```
