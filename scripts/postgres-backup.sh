#!/usr/bin/env sh
set -eu

: "${DATABASE_URL:?DATABASE_URL is required}"

BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="${BACKUP_DIR%/}/myankafe-finance-${STAMP}.dump"

umask 077
mkdir -p "$BACKUP_DIR"

echo "Creating PostgreSQL logical backup: $DEST"
pg_dump   --dbname="$DATABASE_URL"   --format=custom   --compress=9   --no-owner   --no-acl   --file="$DEST"

echo "Verifying backup archive"
pg_restore --list "$DEST" >/dev/null

case "$RETENTION_DAYS" in
  ''|*[!0-9]*) echo "RETENTION_DAYS must be a non-negative integer" >&2; exit 2 ;;
esac

if [ "$RETENTION_DAYS" -gt 0 ]; then
  find "$BACKUP_DIR" -type f -name 'myankafe-finance-*.dump' -mtime "+$RETENTION_DAYS" -print -delete
fi

echo "Backup complete: $DEST"
