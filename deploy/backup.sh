#!/usr/bin/env bash
set -euo pipefail

backup_root=${BACKUP_ROOT:-/opt/laoshirenvip-agent-shop/backups}
retention_days=${RETENTION_DAYS:-7}
stamp=$(date -u +%Y%m%dT%H%M%SZ)
target="$backup_root/$stamp"

umask 077
mkdir -p "$target"

docker exec deploy-postgres-1 pg_dump -U agent_shop -d agent_shop --format=custom >"$target/postgres.dump"
docker run --rm \
  -v deploy_uploads:/data:ro \
  -v "$target:/backup" \
  alpine:3.21 sh -c 'cd /data && tar -czf /backup/uploads.tar.gz .'

sha256sum "$target/postgres.dump" "$target/uploads.tar.gz" >"$target/SHA256SUMS"
find "$backup_root" -mindepth 1 -maxdepth 1 -type d -mtime "+$retention_days" -exec rm -rf -- {} +

printf 'backup=%s\n' "$target"
