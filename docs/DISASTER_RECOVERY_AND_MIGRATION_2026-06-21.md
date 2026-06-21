# Disaster Recovery And Server Migration Runbook - 2026-06-21

This note records the production recovery and migration plan for the current
NewAPI + Sub2API stack. It is intentionally operational and conservative:
database backups first, then config/appdata archives, then image/compose
rollback.

## Current Answer

Security hardening is partially complete, but disaster recovery was not complete
before this note.

Already in place:

- NewAPI user-facing request concurrency guard, default per-user in-flight
  request limit `5`.
- Sub2API downstream risk guard for obvious reverse-engineering, credential
  theft, account-automation, and bypass prompts.
- Key-level auto-disable behavior for repeated downstream risk hits.
- Docker and containerd storage moved away from the small root disk to `/www`.
- Prebuilt-image deployment policy for NewAPI to avoid production build storms.

Still required for a production launch:

- automatic daily backups;
- routine restore drills;
- off-server copy of backups;
- WAF/CDN rate limits and origin-bypass protection;
- monitoring/alerting for database, disk, error rate, and gateway latency.

## Production Persistence Map

Current server: `154.219.122.197`

NewAPI:

- App compose/config: `/opt/newapi/docker-compose.yml`, `/opt/newapi/.env`
- MySQL data: `/opt/newapi/mysql_data`
- Redis data: `/opt/newapi/redis_data`
- App data: `/opt/newapi/data`

Sub2API:

- App compose/config: `/opt/sub2api/docker-compose.yml`, `/opt/sub2api/.env`
- Postgres data: `/opt/sub2api/postgres_data`
- Redis data: `/opt/sub2api/redis_data`
- App data: `/opt/sub2api/data`
- Kiro config/account state: `/opt/sub2api/kiro-rs/config`,
  `/opt/sub2api/kiro-gateway/creds`, `/opt/sub2api/kiro-gateway/state`
- Windsurf adapter data: `/opt/sub2api/windsurf-api/data`,
  `/opt/sub2api/windsurf-api/opt/windsurf/data`

Image site / UAG, when used:

- Compose/config: `/opt/unified-ai-gateway/docker-compose.yml`,
  `/opt/unified-ai-gateway/.env`
- App data/uploads: `/opt/unified-ai-gateway/data`,
  `/opt/unified-ai-gateway/uploads`
- Database container, when present: `uag-mysql`

Backup artifacts contain secrets and account payloads. They must not be printed,
committed, pasted into chat, or placed in a public bucket.

## Backup Script

Tracked script:

```text
deploy/production-backup.sh
```

Default server destination:

```text
/www/backups/ai-stack/YYYYMMDDTHHMMSSZ/
```

It creates:

- `newapi-mysql.sql.gz`
- `sub2api-postgres.dump`
- `unified-ai-gateway-mysql.sql.gz`, when `uag-mysql` exists and can be dumped
- `configs-and-appdata.tar.gz`
- `manifest.txt`
- `SHA256SUMS`

The script uses mode `700` backup directories and does not echo database
passwords, API keys, refresh tokens, or account payloads. The backup files
themselves are sensitive.

Recommended production schedule:

```bash
install -m 700 deploy/production-backup.sh /usr/local/sbin/production-backup.sh
install -m 644 deploy/systemd/ai-stack-backup.service /etc/systemd/system/ai-stack-backup.service
install -m 644 deploy/systemd/ai-stack-backup.timer /etc/systemd/system/ai-stack-backup.timer
systemctl daemon-reload
systemctl enable --now ai-stack-backup.timer
systemctl list-timers ai-stack-backup.timer --no-pager
```

This runs around 03:17 server time with a small randomized delay and keeps local
daily backups for 14 days. This server did not have `crontab` installed on
2026-06-21, so production uses a systemd timer instead of cron.

Production status on 2026-06-21:

- backup script installed at `/usr/local/sbin/production-backup.sh`;
- systemd timer `ai-stack-backup.timer` enabled and active;
- first verified backup stored under `/www/backups/ai-stack`;
- `SHA256SUMS` verified for the latest backup;
- NewAPI, Sub2API, and public status endpoints remained healthy after the
  backup run.

## Recovery From Server Overload

If the server is hit hard but disk data is still intact:

1. Keep DNS pointed at Cloudflare/CDN and enable stricter WAF/rate-limit rules.
2. SSH in through the provider console if normal SSH is unreliable.
3. Check pressure:

   ```bash
   uptime
   free -h
   df -h / /www
   docker ps
   docker stats --no-stream
   ```

4. Stop only non-critical or abusive edge traffic first. Avoid deleting data.
5. Verify core health:

   ```bash
   curl -fsS https://dtrljm.com/api/status
   curl -fsS https://api.dtrljm.com/api/status
   curl -fsS https://api.vyywcw.cn/health
   ```

6. If the app container is unhealthy but databases are healthy, recreate only
   the app container:

   ```bash
   cd /opt/newapi && docker compose up -d --no-deps newapi
   cd /opt/sub2api && docker compose up -d --no-deps sub2api
   ```

7. If database corruption or data loss is suspected, stop and use the restore
   flow below. Do not keep writing traffic into a possibly damaged database.

## Restore On Same Server

Use this only after choosing a specific backup directory.

NewAPI MySQL restore outline:

```bash
BACKUP=/www/backups/ai-stack/<timestamp>
cd /opt/newapi
docker compose stop newapi
zcat "$BACKUP/newapi-mysql.sql.gz" | docker exec -i newapi-mysql sh -c '
  exec mysql -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE"
'
docker compose up -d --no-deps newapi
```

Sub2API Postgres restore outline:

```bash
BACKUP=/www/backups/ai-stack/<timestamp>
cd /opt/sub2api
docker compose stop sub2api
docker exec sub2api-postgres sh -c '
  dropdb -U "$POSTGRES_USER" "$POSTGRES_DB" &&
  createdb -U "$POSTGRES_USER" "$POSTGRES_DB"
'
docker exec -i sub2api-postgres pg_restore -U sub2api -d sub2api --clean --if-exists --no-owner --no-privileges < "$BACKUP/sub2api-postgres.dump"
docker compose up -d --no-deps sub2api
```

Config/appdata restore outline:

```bash
BACKUP=/www/backups/ai-stack/<timestamp>
tar -C / -xzf "$BACKUP/configs-and-appdata.tar.gz"
```

After restore, run health checks and a controlled test key call before reopening
traffic.

## Migration To A New Server

Preferred migration when user volume grows:

1. Prepare the new server with Docker, Compose, firewall, Cloudflare/CDN origin
   rules, and the same `/opt/newapi`, `/opt/sub2api`, `/opt/unified-ai-gateway`
   layout.
2. Load the known-good prebuilt images. Do not build large frontend images on
   the production server.
3. Copy the latest backup directory to the new server over SSH/rsync.
4. Restore config/appdata archive.
5. Start databases only.
6. Restore NewAPI MySQL and Sub2API Postgres.
7. Start app containers.
8. Verify internal health and admin login.
9. Temporarily reduce DNS TTL, then switch Cloudflare origin to the new server.
10. Keep the old server read-only for at least 24-72 hours as rollback.

Lowest-downtime migration:

1. Take a normal backup while the old server remains online.
2. Restore it to the new server.
3. Put the old server into maintenance or block new writes briefly.
4. Take a final backup.
5. Restore the final delta/final dump to the new server.
6. Switch Cloudflare origin.

For the current scale, dump-and-restore is simpler and safer than live database
replication. Add managed MySQL/Postgres or streaming replication only when the
stack needs near-zero downtime.

## What Must Be Added Before Large Expansion

- Off-server backups to S3/R2/Backblaze or another private object store.
- A monthly restore drill into a temporary server or isolated compose project.
- Monitoring alerts for:
  - root and `/www` disk usage;
  - MySQL/Postgres health;
  - container restarts;
  - NewAPI 5xx/error rate;
  - Sub2API 5xx/error rate;
  - p95/p99 latency and first-byte time.
- CDN/WAF rules for request rate, bot patterns, registration abuse, and origin
  bypass.
- Documented rollback image tags and compose backups for every deploy.

## Current Deployment Policy

For future NewAPI/Sub2API changes:

1. build off-server;
2. `docker save` or push a private image tag;
3. load/pull on the production server;
4. back up compose and run the backup script;
5. recreate only the target app container with `--no-deps`;
6. verify health;
7. keep the previous image and compose file for rollback.
