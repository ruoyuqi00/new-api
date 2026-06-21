# Public Access And Recharge Runbook - 2026-06-21

This note records the current public access split for NewAPI/Sub2API and the
NewAPI recharge entry behavior. Do not put API keys, passwords, refresh tokens,
or full account payloads in this file.

## Public API Endpoint

The user-facing API endpoint intended for normal API clients, including
mainland China clients, is:

```text
https://api.dtrljm.com/v1
```

This hostname is routed by production Caddy to the NewAPI container and was
verified on 2026-06-21 through `https://api.dtrljm.com/api/status`.

Give users this endpoint as the OpenAI-compatible Base URL. Users should not
call Sub2API directly unless an internal/debug workflow explicitly requires it.

## Web Site And Admin Access Split

NewAPI is the public control plane. Sub2API is the private scheduler/supply
plane.

The intended access policy is hostname-based:

| Hostname | Role | Mainland normal access |
| --- | --- | --- |
| `api.dtrljm.com` | User API calls | Keep open |
| `dtrljm.com` | NewAPI user web site | Block/challenge before rollout |
| `www.dtrljm.com` | NewAPI user web site alias | Block/challenge before rollout |
| `admin.dtrljm.com` | NewAPI admin/login entry | Block/challenge before rollout |
| `newapi.dtrljm.com` | NewAPI compatibility/admin entry | Block/challenge before rollout |
| `newapi.vyywcw.cn` | Old NewAPI compatibility entry | Keep only while migrating |
| `api.vyywcw.cn` | Old Sub2API public/API entry | Internal/legacy only |

Do not create one broad country block for the entire `dtrljm.com` zone. That
would also block `api.dtrljm.com` and break mainland API clients.

Recommended Cloudflare rule shape:

```text
if ip.geoip.country eq "CN"
and http.host in {
  "dtrljm.com"
  "www.dtrljm.com"
  "admin.dtrljm.com"
  "newapi.dtrljm.com"
}
then Managed Challenge or Block
```

Keep `api.dtrljm.com` out of that rule. Hong Kong and Taiwan should not be
blocked. Macau can be decided separately if needed.

## Current Caddy Routing And Mainland Web Block

Production Caddy lives on the server at:

```text
/opt/sub2api/Caddyfile
```

As of 2026-06-21, Caddy routes these hostnames to NewAPI:

```text
newapi.vyywcw.cn
dtrljm.com
www.dtrljm.com
api.dtrljm.com
admin.dtrljm.com
newapi.dtrljm.com
```

Caddy now also enforces a source-side safety rule for requests that arrive with
Cloudflare's country header:

```caddy
@mainlandWeb {
    host dtrljm.com www.dtrljm.com admin.dtrljm.com newapi.dtrljm.com
    header CF-IPCountry CN
}
handle @mainlandWeb {
    respond "Web access is restricted in this region. API clients should use https://api.dtrljm.com/v1." 403
}
```

The sanitized reusable snippet is kept at:

```text
deploy/caddy-snippets/newapi-mainland-web-block.caddy
```

Backup before the production Caddy change:

```text
/opt/sub2api/Caddyfile.bak-cn-web-block-20260621101317
```

Verification on the server with local SNI/Host override:

- `CF-IPCountry: CN` + `https://dtrljm.com/` returned HTTP 403.
- `CF-IPCountry: CN` + `https://api.dtrljm.com/api/status` returned HTTP 200.
- `CF-IPCountry: HK` + `https://dtrljm.com/` returned HTTP 200.

Cloudflare WAF/Rules are still recommended as the first edge control, because
the Caddy rule only acts after traffic reaches the origin. Public client-side
tests cannot reliably spoof `CF-IPCountry`, because Cloudflare owns/overwrites
that header.

## Recharge Entry

NewAPI already has the external recharge link configured:

```text
TopUpLink=https://pay.ldxp.cn/shop/yuqi
```

Default upstream NewAPI shows this link only as a small redemption-code helper
link in the wallet/recharge page, so it is easy for users to miss.

A NewAPI frontend patch was prepared to make the configured `TopUpLink` show as
a prominent recharge card at the top of the wallet add-funds panel:

```text
patches/newapi/rc10-recharge-entry.patch
```

Patch intent:

- show `充值入口` / `Recharge Portal` when `TopUpLink` exists;
- add a primary `打开充值卡网` / `Open Recharge Store` button;
- open the configured top-up store in a new tab;
- keep the existing redemption-code helper link unchanged.

The patch is based on upstream NewAPI `v1.0.0-rc.10`, matching the production
NewAPI version observed on 2026-06-21.

## Patch Verification

Local verification performed in:

```text
D:\wflogin\_github_research\new-api
```

Commands:

```bash
git switch -C yuapi-rc10-topup-entry v1.0.0-rc.10
git apply D:/wflogin/sub2api-private/patches/newapi/rc10-recharge-entry.patch
cd web/default
npm install --no-package-lock
npm run build
```

Result:

- `npm run build` succeeded.
- `npm run typecheck` still fails on an existing upstream rc.10
  `usage-logs-mobile-card.tsx` generic typing issue, unrelated to the recharge
  patch.
- `npx eslint ...` failed in local dependency resolution with an ESLint
  `brace_expansion` error, unrelated to the recharge patch.
- A full Docker image build was attempted locally but timed out; do not treat
  that as a production deploy.

## Deployment Note

The patch is not automatically live just because it is committed here. To make
the NewAPI recharge-card UI live, build a custom NewAPI image from
`v1.0.0-rc.10` with the patch applied, change `/opt/newapi/docker-compose.yml`
to that custom image tag, and recreate only the `newapi` container.

Before deployment:

1. confirm NewAPI is healthy;
2. back up `/opt/newapi/docker-compose.yml`;
3. keep `newapi-mysql` and `newapi-redis` running;
4. recreate only `newapi`;
5. verify `https://dtrljm.com/`, `https://api.dtrljm.com/api/status`, and the
   logged-in wallet page.

Avoid restarting NewAPI while users are actively using it unless the short
frontend refresh is acceptable.

2026-06-21 production attempt:

- The patch was applied in `/www/build/new-api-rc10-topup-entry`.
- A full server-side Docker build was attempted, but it drove the production
  host load very high and the SSH session dropped before the image was created.
- The running NewAPI image was not changed; `newapi`, `newapi-mysql`,
  `newapi-redis`, and `sub2api-caddy` remained healthy after recovery.
- `https://api.dtrljm.com/api/status` and `https://api.vyywcw.cn/health`
  returned HTTP 200 after recovery.
- The failed build cache was pruned, returning `/www` to about 26% used.

Do not run a full NewAPI Docker build on the production host during active
traffic. Preferred follow-up paths:

1. build the patched image on a separate machine, `docker save` it, upload it,
   then `docker load` on the server;
2. or schedule a low-traffic maintenance window and temporarily stop unrelated
   build-heavy services before building;
3. or keep the patch as a reproducible delta until the next planned NewAPI
   upgrade.
