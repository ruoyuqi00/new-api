# User 79 Session Issuance Exemption Hot Cutover

## Result

- Code commit: `b0f19ab47768f7d43661b0db79e4ec2818646b21`
- Image: `yuapi:production-20260823-session79-b0f19ab47`
- Active container: `newapi-session79-b0f19ab47`
- Previous container retained: `newapi-earth-cachefix-b6bfa6bbb`
- Configuration: `USER_SESSION_ISSUANCE_EXEMPT_USER_IDS=79`

The exemption skips only the rolling login-session issuance count for user 79. The active-session limit and all other authentication controls remain enabled. No historical sessions or database schema were changed.

## Cutover

Caddy was validated and gracefully reloaded to target `newapi-session79-b0f19ab47:3000`. The previous container was not stopped. The runtime and persistent Caddy backups are retained on the server under:

`/opt/newapi/backups/20260823T1215-session79/`

The backup includes the runtime Caddyfile that was active immediately before this cutover and the host-side Caddyfile copy.

## Verification

- `go test ./common ./model ./service ./controller -count=1` passed.
- `git diff --check` passed.
- Candidate `/api/status` responded successfully through the Caddy network.
- `https://api.yuaiapi.com/api/status` returned HTTP 200.
- `https://global.yuaiapi.com/sign-in` returned HTTP 200 and the approved branded asset reference.
- `https://vip.yuaiapi.com/api/status` returned HTTP 200.
- Candidate restart count: 0.
- Previous container restart count: 0.

## Rollback

Keep the current and previous containers available. To roll back, copy the saved runtime Caddyfile from the backup directory into the Caddy container's temporary path, validate it, and gracefully reload Caddy from that file. Restore the host-side Caddyfile from `Caddyfile.host-before` only after validation. Do not delete the candidate or previous image until the rollback decision is closed.
