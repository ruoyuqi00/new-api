#!/usr/bin/env bash
set -Eeuo pipefail

# Production backup for the AI gateway stack.
# This script intentionally avoids printing secrets. Backup artifacts can still
# contain secrets and account payloads, so keep BACKUP_ROOT mode 700.

BACKUP_ROOT="${BACKUP_ROOT:-/www/backups/ai-stack}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
LOCK_FILE="${LOCK_FILE:-/var/lock/ai-stack-backup.lock}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${BACKUP_ROOT}/${TIMESTAMP}"

log() {
  printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

die() {
  log "ERROR: $*"
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

container_running() {
  docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null | grep -qx true
}

guard_backup_root() {
  case "$BACKUP_ROOT" in
    ""|"/"|"/opt"|"/var"|"/www"|"/tmp"|"/root")
      die "unsafe BACKUP_ROOT: ${BACKUP_ROOT}"
      ;;
  esac
}

dump_newapi_mysql() {
  local out="${RUN_DIR}/newapi-mysql.sql.gz"
  container_running newapi-mysql || die "newapi-mysql is not running"
  log "dumping NewAPI MySQL"
  docker exec newapi-mysql sh -c '
    if [ -n "${MYSQL_USER:-}" ] && [ -n "${MYSQL_PASSWORD:-}" ] && [ -n "${MYSQL_DATABASE:-}" ]; then
      MYSQL_PWD="$MYSQL_PASSWORD" exec mysqldump --single-transaction --quick --routines --triggers --events --hex-blob --no-tablespaces \
        -u"$MYSQL_USER" "$MYSQL_DATABASE"
    fi
    MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump --single-transaction --quick --routines --triggers --events --hex-blob --no-tablespaces \
      -uroot --all-databases
  ' | gzip -c > "$out"
  gzip -t "$out"
}

dump_optional_mysql() {
  local container="$1"
  local name="$2"
  local out="${RUN_DIR}/${name}.sql.gz"
  if ! container_running "$container"; then
    log "skipping ${name}: container is not running"
    return 0
  fi

  log "dumping optional MySQL: ${name}"
  if docker exec "$container" sh -c '
    if [ -n "${MYSQL_USER:-}" ] && [ -n "${MYSQL_PASSWORD:-}" ] && [ -n "${MYSQL_DATABASE:-}" ]; then
      MYSQL_PWD="$MYSQL_PASSWORD" exec mysqldump --single-transaction --quick --routines --triggers --events --hex-blob --no-tablespaces \
        -u"$MYSQL_USER" "$MYSQL_DATABASE"
    fi
    if [ -n "${MYSQL_ROOT_PASSWORD:-}" ]; then
      MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump --single-transaction --quick --routines --triggers --events --hex-blob --no-tablespaces \
        -uroot --all-databases
    fi
    exit 64
  ' | gzip -c > "$out"; then
    gzip -t "$out"
  else
    rm -f "$out"
    log "warning: optional MySQL dump failed for ${name}"
  fi
}

dump_sub2api_postgres() {
  local out="${RUN_DIR}/sub2api-postgres.dump"
  container_running sub2api-postgres || die "sub2api-postgres is not running"
  log "dumping Sub2API Postgres"
  docker exec sub2api-postgres sh -c '
    exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges
  ' > "$out"
  docker exec -i sub2api-postgres pg_restore -l >/dev/null < "$out"
}

archive_configs_and_appdata() {
  local out="${RUN_DIR}/configs-and-appdata.tar.gz"
  local paths=()
  local candidates=(
    /opt/newapi/docker-compose.yml
    /opt/newapi/.env
    /opt/newapi/data
    /opt/newapi/redis_data
    /opt/sub2api/docker-compose.yml
    /opt/sub2api/Caddyfile
    /opt/sub2api/caddy
    /opt/sub2api/.env
    /opt/sub2api/data
    /opt/sub2api/redis_data
    /opt/sub2api/kiro-rs/config
    /opt/sub2api/kiro-gateway/creds
    /opt/sub2api/kiro-gateway/state
    /opt/sub2api/windsurf-api/data
    /opt/sub2api/windsurf-api/opt/windsurf/data
    /opt/unified-ai-gateway/docker-compose.yml
    /opt/unified-ai-gateway/.env
    /opt/unified-ai-gateway/data
    /opt/unified-ai-gateway/uploads
  )

  for path in "${candidates[@]}"; do
    if [ -e "$path" ]; then
      paths+=("${path#/}")
    fi
  done

  if [ "${#paths[@]}" -eq 0 ]; then
    log "warning: no config/appdata paths found"
    return 0
  fi

  log "archiving compose files, env files, and app data"
  local tar_status=0
  tar -C / --warning=no-file-changed --ignore-failed-read -czf "$out" "${paths[@]}" || tar_status=$?
  if [ "$tar_status" -gt 1 ]; then
    die "config/appdata archive failed with tar status ${tar_status}"
  fi
  gzip -t "$out"
}

write_manifest() {
  log "writing manifest"
  {
    printf 'timestamp_utc=%s\n' "$TIMESTAMP"
    printf 'host=%s\n' "$(hostname)"
    printf 'backup_root=%s\n' "$BACKUP_ROOT"
    printf 'retention_days=%s\n' "$RETENTION_DAYS"
    printf '\n[containers]\n'
    docker ps --format '{{.Names}} {{.Image}} {{.Status}}' | sort
    printf '\n[disk]\n'
    df -h / /www 2>/dev/null || df -h
  } > "${RUN_DIR}/manifest.txt"

  (
    cd "$RUN_DIR"
    find . -maxdepth 1 -type f ! -name SHA256SUMS -printf '%P\n' | sort | xargs -r sha256sum
  ) > "${RUN_DIR}/SHA256SUMS"
}

prune_old_backups() {
  log "pruning backups older than ${RETENTION_DAYS} days"
  find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name '20*T*Z' -mtime +"$RETENTION_DAYS" -print0 |
    while IFS= read -r -d '' old_dir; do
      case "$old_dir" in
        "$BACKUP_ROOT"/20*T*Z) rm -rf -- "$old_dir" ;;
        *) log "warning: refused to prune unexpected path ${old_dir}" ;;
      esac
    done
}

main() {
  require_cmd docker
  require_cmd gzip
  require_cmd tar
  require_cmd sha256sum
  require_cmd flock
  guard_backup_root

  mkdir -p "$(dirname "$LOCK_FILE")"
  exec 9>"$LOCK_FILE"
  flock -n 9 || die "another backup is already running"

  umask 077
  mkdir -p "$RUN_DIR"
  chmod 700 "$BACKUP_ROOT" "$RUN_DIR"

  dump_newapi_mysql
  dump_sub2api_postgres
  dump_optional_mysql uag-mysql unified-ai-gateway-mysql
  archive_configs_and_appdata
  write_manifest
  ln -sfn "$RUN_DIR" "${BACKUP_ROOT}/latest"
  prune_old_backups

  log "backup complete: ${RUN_DIR}"
}

main "$@"
