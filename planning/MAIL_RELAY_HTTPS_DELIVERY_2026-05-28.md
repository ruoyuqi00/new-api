# HTTPS Mail Relay Delivery Plan

Date: 2026-05-28

## Why this exists

The server can reach HTTPS 443, but SMTP ports to common mail providers time out
at the network/provider layer. Host firewall output is open, so changing local
firewall rules will not fix it.

The practical deployment path is:

```text
Sub2API
  -> internal SMTP mail-relay:1025
  -> HTTPS API on port 443
  -> Resend or Cloudflare Email Service
```

## Provider choice

Short-term recommendation: **Resend**.

Reason:

- `vyywcw.cn` is not currently on Cloudflare DNS.
- Cloudflare Email Service REST API is now available, but Cloudflare docs say
  the domain must use Cloudflare DNS before Email Service can be onboarded.
- Resend only needs DNS verification records added at the current DNS provider.

Medium-term option: **Cloudflare Email Service**.

Reason:

- If `vyywcw.cn` is moved to Cloudflare DNS later, the relay can switch provider
  by changing environment variables. No Sub2API code or admin SMTP setting has
  to change.

## Official protocol references

Cloudflare Email Service REST API:

- Endpoint: `POST https://api.cloudflare.com/client/v4/accounts/{account_id}/email/sending/send`
- Auth: `Authorization: Bearer <API_TOKEN>`
- Requires Cloudflare DNS onboarding for the sender domain.
- Reference: https://developers.cloudflare.com/email-service/api/send-emails/rest-api/

Resend:

- Base URL: `https://api.resend.com`
- Send endpoint: `POST /emails`
- Auth: `Authorization: Bearer re_xxxxxxxxx`
- Reference: https://resend.com/docs/api-reference/emails/send-email

## Runtime configuration

Compose service:

```yaml
mail-relay:
  build:
    context: ../adapters/mail-relay
  image: sub2api-mail-relay:latest
  networks:
    - sub2api-network
```

No public port mapping is used. The SMTP listener is only reachable inside the
Docker network.

The repository ships this as an optional compose override:

```bash
docker compose -f docker-compose.yml -f docker-compose.mail-relay.yml up -d
```

For Resend:

```env
MAIL_RELAY_PROVIDER=resend
MAIL_RELAY_FROM=no-reply@vyywcw.cn
MAIL_RELAY_FROM_NAME=vyywcw
RESEND_API_KEY=<secret>
```

For Cloudflare:

```env
MAIL_RELAY_PROVIDER=cloudflare
MAIL_RELAY_FROM=no-reply@vyywcw.cn
MAIL_RELAY_FROM_NAME=vyywcw
CLOUDFLARE_ACCOUNT_ID=<account_id>
CLOUDFLARE_API_TOKEN=<token with Email Sending permission>
```

## Sub2API admin SMTP settings

After the relay provider API key is configured:

```text
SMTP Host: mail-relay
SMTP Port: 1025
SMTP Username: empty
SMTP Password: empty
From Email: no-reply@vyywcw.cn
From Name: vyywcw
Use TLS: false
```

The Sub2API backend now skips SMTP AUTH when both username and password are
empty. This is required for an internal plain SMTP relay.

## DNS checklist for Resend

1. Add `vyywcw.cn` in Resend Domains.
2. Add the DNS records shown by Resend at the current DNS provider:
   SPF/TXT, DKIM/TXT or CNAME, and optional DMARC.
3. Wait for Resend to mark the domain as verified.
4. Create a restricted API key for sending.
5. Put the key in server `.env` as `RESEND_API_KEY`.
6. Restart `mail-relay`.
7. Update Sub2API SMTP settings and send a test email.

## DNS checklist for Cloudflare later

1. Move `vyywcw.cn` authoritative DNS to Cloudflare.
2. In Cloudflare dashboard, open Email Sending and onboard the domain.
3. Let Cloudflare add bounce MX, SPF, DKIM, and DMARC records.
4. Create an API token with Email Sending permission.
5. Set `MAIL_RELAY_PROVIDER=cloudflare`,
   `CLOUDFLARE_ACCOUNT_ID`, and `CLOUDFLARE_API_TOKEN`.
6. Restart `mail-relay`; Sub2API SMTP settings stay the same.

## Validation commands

Relay health:

```bash
cd /opt/sub2api
docker compose ps mail-relay
docker compose logs --tail=100 mail-relay
docker compose exec -T sub2api wget -qO- http://mail-relay:8080/health
```

SMTP connectivity from Sub2API:

```bash
docker compose exec -T sub2api sh -lc 'nc -vz mail-relay 1025'
```

Public exposure check:

```bash
docker compose port mail-relay 1025 || true
docker compose port mail-relay 8080 || true
```

Expected: no host port is published.

## 2026-05-28 server deployment

Production now has the relay service installed and Sub2API upgraded:

```text
Sub2API image: sub2api-provider-adapters:6611e027
mail-relay image: sub2api-mail-relay:5e5d524a
mail-relay provider: dry-run
public health: https://api.vyywcw.cn/health -> 200
```

Validation completed on the server:

- `POST /api/v1/admin/settings/test-smtp` with `mail-relay:1025`, empty
  username/password, TLS disabled -> HTTP 200.
- `POST /api/v1/admin/settings/send-test-email` with the same relay settings
  -> HTTP 200.
- `mail-relay` logs showed `dry-run accepted mail`.
- `docker compose port mail-relay 1025` did not expose a public host port.

Important: production is intentionally still using `MAIL_RELAY_PROVIDER=dry-run`
until a Resend or Cloudflare API key is added. Do not switch Sub2API's saved
SMTP settings to `mail-relay` for real users until a real provider key is in
server `.env`.

Current saved Sub2API SMTP settings remain the previous QQ SMTP values, because
the upstream server blocks SMTP outbound and no HTTPS sender API key has been
provided yet.

## Activation after API key is ready

For Resend:

```bash
cd /opt/sub2api
cp .env .env.bak-mail-relay-$(date +%Y%m%d-%H%M%S)

sed -i 's/^MAIL_RELAY_PROVIDER=.*/MAIL_RELAY_PROVIDER=resend/' .env
sed -i 's/^MAIL_RELAY_FROM=.*/MAIL_RELAY_FROM=no-reply@vyywcw.cn/' .env
sed -i 's/^MAIL_RELAY_FROM_NAME=.*/MAIL_RELAY_FROM_NAME=vyywcw/' .env
# edit RESEND_API_KEY manually; never paste it into logs
nano .env

docker compose up -d mail-relay
docker compose logs --tail=80 mail-relay
```

Then set Sub2API admin SMTP settings:

```text
SMTP Host: mail-relay
SMTP Port: 1025
SMTP Username: empty
SMTP Password: empty
From Email: no-reply@vyywcw.cn
From Name: vyywcw
Use TLS: false
```

Run the Sub2API "test SMTP" and "send test email" buttons after saving.

## 2026-05-28 Resend activation attempt

The Resend API key was installed on the server and `mail-relay` was switched
from `dry-run` to `resend`:

```text
MAIL_RELAY_PROVIDER=resend
MAIL_RELAY_FROM=no-reply@vyywcw.cn
MAIL_RELAY_FROM_NAME=vyywcw
RESEND_API_KEY=<stored in /opt/sub2api/.env>
```

Relay health passed:

```text
{"ok": true, "provider": "resend", ...}
```

Sub2API admin `test-smtp` passed with:

```text
SMTP Host: mail-relay
SMTP Port: 1025
SMTP Username: empty
SMTP Password: empty
Use TLS: false
```

Real send failed because Resend rejected the sender domain:

```text
HTTP 403: The vyywcw.cn domain is not verified.
```

Public DNS check showed these likely mistakes:

```text
send.vyywcw.cn TXT              -> p=MIGf...  (looks like DKIM public key)
resend._domainkey.vyywcw.cn TXT -> p=MIGf...  (DKIM-looking value)
_dmarc.vyywcw.cn TXT            -> p=MIGf...  (wrong for DMARC)
```

Only the DKIM record should look like `p=MIGf...`. The SPF TXT record should
start with `v=spf1 ...`, and the DMARC TXT record should start with
`v=DMARC1; ...`. In Resend, reopen:

```text
Domains -> vyywcw.cn -> DNS Records
```

Then copy each row exactly into the current DNS provider:

```text
Resend Type  -> DNS record type
Resend Name  -> DNS host/name
Resend Value -> DNS value
Priority     -> MX priority only
```

After fixing the DNS records, click `Verify DNS Records` in Resend and rerun the
Sub2API "send test email" check.
