#!/usr/bin/env bash
# OpenQuizGo PostgreSQL 每日备份
# 产物：/srv/backup/pg/openquizgo-YYYYmmdd-HHMMSS.sql.gz（保留 7 天）
# 恢复：gunzip -c 文件 | docker compose exec -T postgres psql -U openquizgo -d openquizgo
set -euo pipefail

# 备份里有手机号、密码哈希、家长与孩子档案：从创建那一刻就收紧权限，
# 不依赖调用方的 umask（历史上产物是 0644，已在下方对旧文件做收敛）
umask 077

BACKUP_DIR="${BACKUP_DIR:-/srv/backup/pg}"
RETAIN_DAYS="${RETAIN_DAYS:-7}"
COMPOSE_DIR="${COMPOSE_DIR:-/srv/quizgo}"
DB_USER="${DB_USER:-openquizgo}"
DB_NAME="${DB_NAME:-openquizgo}"

STAMP="$(date +%Y%m%d-%H%M%S)"
OUT="${BACKUP_DIR}/openquizgo-${STAMP}.sql.gz"

log() { printf '%s %s\n' "$(date -Is)" "$*"; }

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

# 磁盘保护：可用空间不足 1GB 时跳过，避免把盘写满
avail_kb=$(df -Pk "$BACKUP_DIR" | awk 'NR==2 {print $4}')
if [ "${avail_kb:-0}" -lt 1048576 ]; then
  log "skip: 磁盘不足 (可用 ${avail_kb}KB)"
  exit 1
fi

cd "$COMPOSE_DIR"

if ! docker compose exec -T postgres \
      pg_dump -U "$DB_USER" -d "$DB_NAME" --clean --if-exists --no-owner --no-privileges \
      | gzip -9 > "${OUT}.partial"; then
  log "failed: pg_dump 出错"
  rm -f "${OUT}.partial"
  exit 1
fi

mv "${OUT}.partial" "$OUT"
chmod 600 "$OUT"
gzip -t "$OUT"

size=$(stat -c %s "$OUT")
tables=$(gzip -dc "$OUT" | grep -c '^CREATE TABLE' || true)

# 内容校验：空库或半截文件不能当成"成功"
if [ "$size" -lt 4096 ] || [ "$tables" -lt 10 ]; then
  log "failed: 备份可疑 size=${size}B tables=${tables}"
  exit 1
fi

# .env 快照（含密钥，600），换机器时能直接恢复配置
cp -p "${COMPOSE_DIR}/.env" "${BACKUP_DIR}/env-${STAMP}.bak"
chmod 600 "${BACKUP_DIR}/env-${STAMP}.bak"

# 清理过期文件
# 顺带收敛历史产物权限：umask 只影响之后新建的文件，旧的 0644 在这里一并修掉
find "$BACKUP_DIR" -maxdepth 1 -type f \
  \( -name 'openquizgo-*.sql.gz' -o -name 'env-*.bak' \) -perm /077 \
  -exec chmod 600 {} + 2>/dev/null || true
find "$BACKUP_DIR" -type f -name 'openquizgo-*.sql.gz' -mtime +"$RETAIN_DAYS" -delete
find "$BACKUP_DIR" -type f -name 'env-*.bak' -mtime +"$RETAIN_DAYS" -delete

kept=$(find "$BACKUP_DIR" -name 'openquizgo-*.sql.gz' | wc -l)
log "ok file=${OUT} size=${size}B tables=${tables} kept=${kept}"
