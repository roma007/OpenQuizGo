#!/usr/bin/env bash
# OpenQuizGo 一键发布（对应设计/计划 078）
#
# 解决「提交即升版、服务器 .env 不自动同步」的版本真空期：
# 把「备份 → 打回滚点 → 同步 → 版本对齐 → 构建 → 迁移+重启 → 自检」固化成一条命令。
#
# 用法：
#   scripts/deploy.sh                    # 标准发布
#   scripts/deploy.sh --with-admin-web   # 管理端前端有改动时，附带同步 admin-web/dist
#   scripts/deploy.sh --with-units       # deploy/systemd 有改动时，重装 unit 并重启 timer
#   scripts/deploy.sh --skip-backup      # 仅跳过备份（无迁移的纯前端改动才可用）
#   scripts/deploy.sh --dry-run          # 只打印将执行的命令
#
# 环境变量：SSH_HOST(默认 quizgo) REMOTE_DIR(默认 /srv/quizgo) BASE_URL(默认 http://127.0.0.1:8082)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SSH_HOST="${SSH_HOST:-quizgo}"
REMOTE_DIR="${REMOTE_DIR:-/srv/quizgo}"
BASE_URL="${BASE_URL:-http://127.0.0.1:8082}"

WITH_ADMIN_WEB=0
WITH_UNITS=0
SKIP_BACKUP=0
DRY_RUN=0

for arg in "$@"; do
  case "$arg" in
    --with-admin-web) WITH_ADMIN_WEB=1 ;;
    --with-units)     WITH_UNITS=1 ;;
    --skip-backup)    SKIP_BACKUP=1 ;;
    --dry-run)        DRY_RUN=1 ;;
    -h|--help)        sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "未知参数：$arg（--help 查看用法）" >&2; exit 2 ;;
  esac
done

VERSION="$(tr -d '[:space:]' < VERSION)"
[ -n "$VERSION" ] || { echo "VERSION 文件为空" >&2; exit 1; }

run() {
  if [ "$DRY_RUN" = 1 ]; then
    printf '[dry-run] %s\n' "$*"
  else
    printf '\n==> %s\n' "$*"
    "$@"
  fi
}

# ssh 命令用字符串形式，方便在远端组合管道/&&
run_ssh() {
  if [ "$DRY_RUN" = 1 ]; then
    printf '[dry-run] ssh %s %q\n' "$SSH_HOST" "$1"
  else
    printf '\n==> ssh %s: %s\n' "$SSH_HOST" "$1"
    ssh "$SSH_HOST" "$1"
  fi
}

echo "发布版本：$VERSION   目标：$SSH_HOST:$REMOTE_DIR"

if [ -n "$(git status --porcelain)" ]; then
  echo "提示：工作区有未提交改动（发布仍会继续，建议先提交再发布）"
fi

# 1. 打回滚点：旧镜像留作 openquizgo:rollback
run_ssh 'docker tag openquizgo:local openquizgo:rollback 2>/dev/null || true'

# 2. 备份数据库：迁移是单向的，带迁移的发布必须先备份
if [ "$SKIP_BACKUP" = 1 ]; then
  echo "跳过备份（--skip-backup）"
else
  run_ssh 'sudo systemctl start quizgo-backup.service'
fi

# 3. 同步源码：不带 --delete，避免删掉服务器上的产物；.env 永远不传
RSYNC_ARGS=(-az --exclude '.git' --exclude '.env' --exclude 'node_modules'
            --exclude 'admin-web/dist' --exclude 'data')
run rsync "${RSYNC_ARGS[@]}" ./ "$SSH_HOST:$REMOTE_DIR/"

if [ "$WITH_ADMIN_WEB" = 1 ]; then
  run rsync -az --delete admin-web/dist/ "$SSH_HOST:$REMOTE_DIR/admin-web/dist/"
fi

# 4. 版本写入 .env：容器只读 env_file，不同步就永远是旧版本
run_ssh "sudo sed -i 's|^APP_VERSION=.*|APP_VERSION=$VERSION|' $REMOTE_DIR/.env && sudo chown ecs-user:ecs-user $REMOTE_DIR/.env && sudo chmod 600 $REMOTE_DIR/.env && sudo grep -E '^APP_VERSION=' $REMOTE_DIR/.env"

# 5. 可选：重装 systemd unit（deploy/systemd 有改动时）
if [ "$WITH_UNITS" = 1 ]; then
  run_ssh "sudo install -m 644 $REMOTE_DIR/deploy/systemd/quizgo-backup.service $REMOTE_DIR/deploy/systemd/quizgo-backup.timer $REMOTE_DIR/deploy/systemd/tailscale-aliyun-fix.service $REMOTE_DIR/deploy/systemd/tailscale-aliyun-fix.timer /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable quizgo-backup.timer tailscale-aliyun-fix.timer && sudo systemctl restart quizgo-backup.timer tailscale-aliyun-fix.timer"
fi

# 6. 构建并重启：必须用 up -d（而不是只重建 api/worker），
#    这样镜像变更会重建 migrate，迁移成功后才起 api/worker
run_ssh "cd $REMOTE_DIR && docker compose build api && docker compose up -d"

# 7. 自检：版本、状态、日志
if [ "$DRY_RUN" = 1 ]; then
  echo '[dry-run] 跳过自检'
  exit 0
fi

echo
echo "==> 等待 API 就绪"
meta=''
for _ in $(seq 1 30); do
  meta="$(ssh "$SSH_HOST" "curl -sf $BASE_URL/v1/meta" 2>/dev/null || true)"
  [ -n "$meta" ] && break
  sleep 2
done

echo "==> /v1/meta: ${meta:-<无响应>}"
case "$meta" in
  *"\"version\":\"$VERSION\""*) ;;
  *)
    echo "版本未对齐：期望 $VERSION" >&2
    echo "回滚：ssh $SSH_HOST 'docker tag openquizgo:rollback openquizgo:local && cd $REMOTE_DIR && docker compose up -d --no-build'" >&2
    exit 1
    ;;
esac

ssh "$SSH_HOST" "cd $REMOTE_DIR && docker compose ps --format 'table {{.Service}}\t{{.Status}}'"
ssh "$SSH_HOST" "cd $REMOTE_DIR && docker compose logs --tail=80 api 2>&1 | grep -iE 'error|panic' || echo '日志无 error/panic'"
ssh "$SSH_HOST" "free -m | awk 'NR==2 {print \"可用内存：\" \$7 \" MB\"}'; df -h / | awk 'NR==2 {print \"根分区：\" \$5 \" 已用\"}'"

echo
echo "发布完成：$VERSION"
