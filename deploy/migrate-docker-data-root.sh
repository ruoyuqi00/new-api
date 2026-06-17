#!/usr/bin/env bash
set -euo pipefail

TARGET="/www/docker"
EXECUTE="false"
YES="false"
ALLOW_SAME_DEVICE="false"

usage() {
  cat <<'USAGE'
Move Docker's data-root to a larger server disk.

Default mode is a dry-run diagnosis. It does not stop Docker or modify files.

Usage:
  sudo bash deploy/migrate-docker-data-root.sh [options]

Options:
  --target PATH          New Docker data-root. Default: /www/docker
  --execute              Stop Docker, copy data, update daemon.json, restart Docker
  --yes                  Do not prompt during --execute
  --allow-same-device    Allow target to be on the same device as /
  -h, --help             Show this help

Examples:
  sudo bash deploy/migrate-docker-data-root.sh
  sudo bash deploy/migrate-docker-data-root.sh --target /www/docker --execute
  sudo bash deploy/migrate-docker-data-root.sh --target /www/docker --execute --yes
USAGE
}

log() {
  printf '[docker-data-root] %s\n' "$*"
}

die() {
  printf '[docker-data-root] ERROR: %s\n' "$*" >&2
  exit 1
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --target)
      [ "$#" -ge 2 ] || die "--target requires a path"
      TARGET="$2"
      shift 2
      ;;
    --execute)
      EXECUTE="true"
      shift
      ;;
    --yes)
      YES="true"
      shift
      ;;
    --allow-same-device)
      ALLOW_SAME_DEVICE="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[ "$(id -u)" -eq 0 ] || die "run as root because Docker data-root and /etc/docker need root"
command -v docker >/dev/null 2>&1 || die "docker command not found"
command -v systemctl >/dev/null 2>&1 || die "systemctl command not found"
command -v rsync >/dev/null 2>&1 || die "rsync command not found"

OLD_ROOT="$(docker info --format '{{.DockerRootDir}}' 2>/dev/null || true)"
[ -n "$OLD_ROOT" ] || die "could not read current DockerRootDir"

OLD_ROOT_REAL="$(readlink -f "$OLD_ROOT")"
TARGET_REAL="$(readlink -m "$TARGET")"

[ "$OLD_ROOT_REAL" != "$TARGET_REAL" ] || die "Docker already uses target data-root: $TARGET_REAL"
case "$TARGET_REAL" in
  "$OLD_ROOT_REAL"/*)
    die "target must not be inside the current Docker data-root"
    ;;
esac

probe="$TARGET_REAL"
while [ ! -e "$probe" ] && [ "$probe" != "/" ]; do
  probe="$(dirname "$probe")"
done
[ -e "$probe" ] || die "could not find an existing parent for target: $TARGET_REAL"

ROOT_DEV="$(stat -c '%d' /)"
TARGET_DEV="$(stat -c '%d' "$probe")"
if [ "$ROOT_DEV" = "$TARGET_DEV" ] && [ "$ALLOW_SAME_DEVICE" != "true" ]; then
  die "target parent $probe is on the same device as /. Use another mounted disk or --allow-same-device."
fi

OLD_KB="$(du -sk "$OLD_ROOT_REAL" | awk '{print $1}')"
AVAIL_KB="$(df -Pk "$probe" | awk 'NR==2 {print $4}')"
NEEDED_KB=$((OLD_KB + OLD_KB / 5 + 1048576))

log "current DockerRootDir: $OLD_ROOT_REAL"
log "target DockerRootDir:  $TARGET_REAL"
log "target space probe:    $probe"
log "current Docker size:   $((OLD_KB / 1024)) MiB"
log "target free space:     $((AVAIL_KB / 1024)) MiB"

if [ "$AVAIL_KB" -lt "$NEEDED_KB" ]; then
  die "target does not have enough free space; require roughly current size + 20% + 1 GiB"
fi

log "Docker disk usage summary:"
docker system df || true

if [ "$EXECUTE" != "true" ]; then
  log "dry-run only. Re-run with --execute during a maintenance window to perform the move."
  log "this move stops Docker and affects every container, including Sub2API and NewAPI."
  exit 0
fi

if [ "$YES" != "true" ]; then
  printf 'This will stop Docker and all containers. Type MOVE to continue: '
  read -r answer
  [ "$answer" = "MOVE" ] || die "aborted"
fi

STAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="/opt/sub2api-backups/docker-data-root-${STAMP}"

log "creating backup directory: $BACKUP_DIR"
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

backup_file() {
  src="$1"
  [ -f "$src" ] || return 0
  dst="$BACKUP_DIR/$(printf '%s' "${src#/}" | tr '/' '_')"
  cp -a "$src" "$dst"
  chmod go-rwx "$dst" || true
}

backup_file /etc/docker/daemon.json
backup_file /opt/sub2api/docker-compose.yml
backup_file /opt/sub2api/.env
backup_file /opt/newapi/docker-compose.yml
backup_file /opt/newapi/.env

log "stopping Docker"
systemctl stop docker.socket 2>/dev/null || true
systemctl stop docker.service

log "copying Docker data with rsync"
mkdir -p "$TARGET_REAL"
rsync -aHAXS --numeric-ids "${OLD_ROOT_REAL}/" "${TARGET_REAL}/"

log "updating /etc/docker/daemon.json"
mkdir -p /etc/docker
if command -v python3 >/dev/null 2>&1; then
  python3 - "$TARGET_REAL" <<'PY'
import json
import os
import pathlib
import sys

target = sys.argv[1]
path = pathlib.Path("/etc/docker/daemon.json")
if path.exists() and path.stat().st_size > 0:
    data = json.loads(path.read_text())
else:
    data = {}
data["data-root"] = target
tmp = path.with_suffix(".json.tmp")
tmp.write_text(json.dumps(data, indent=2, sort_keys=True) + "\n")
os.replace(tmp, path)
PY
else
  [ ! -s /etc/docker/daemon.json ] || die "python3 is required to safely merge existing daemon.json"
  printf '{\n  "data-root": "%s"\n}\n' "$TARGET_REAL" > /etc/docker/daemon.json
fi

log "starting Docker"
systemctl start docker.service

NEW_ROOT="$(docker info --format '{{.DockerRootDir}}')"
[ "$(readlink -f "$NEW_ROOT")" = "$TARGET_REAL" ] || die "DockerRootDir verification failed: $NEW_ROOT"

log "DockerRootDir is now: $NEW_ROOT"

for stack_dir in /opt/sub2api /opt/newapi; do
  if [ -f "$stack_dir/docker-compose.yml" ] || [ -f "$stack_dir/compose.yml" ]; then
    log "compose status for $stack_dir"
    (cd "$stack_dir" && docker compose ps) || true
  fi
done

log "old Docker data remains at $OLD_ROOT_REAL for rollback. Do not delete it until the stack is stable."
log "backup files are in $BACKUP_DIR"
