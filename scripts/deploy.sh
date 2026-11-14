#!/usr/bin/env bash
# OpenQuizGo 一键发布（对应设计/计划 078）
#
# 解决「提交即升版、服务器 .env 不自动同步」的版本真空期：
# 把「备份 → 打回滚点 → 同步 → nginx/units → 构建 → 写版本 → 迁移+重启 → 自检 → 登记」
# 固化成一条命令。构建成功后才写 .env，构建失败不会污染版本号。
#
# 跨项目协作（AGENTS.md §9）：开头读 /srv/ops 的 inbox，有 open 请求就 exit 2，
# 除非显式 --ack-open；收尾自动登记 /srv/ops/state/quizgo.json 与 journal。
#
# 用法：
#   scripts/deploy.sh                    # 标准发布
#   scripts/deploy.sh --with-admin-web   # 管理端前端有改动时，附带同步 admin-web/dist
#   scripts/deploy.sh --with-nginx       # deploy/nginx 有改动时，同步配置 + nginx -t + reload
#   scripts/deploy.sh --with-units       # deploy/systemd 有改动时，重装 unit 并重启 timer
#   scripts/deploy.sh --skip-backup      # 仅跳过备份（服务端代码/迁移没变时可用，有硬保护）
#   scripts/deploy.sh --skip-build       # 不重建镜像，只对齐版本号（服务端代码/迁移没变时可用，有硬保护）
#   scripts/deploy.sh --ack-open         # 跳过 inbox 闸门（已确认 open 请求与本次发布无关，或本次就是处理它）
#   scripts/deploy.sh --dry-run          # 只打印将执行的命令
#
# --skip-build / --skip-backup 的硬保护：以服务器上的 /srv/quizgo/.deployed-image
# （记录上次构建的 commit 与镜像 ID）为基线，若服务端代码或迁移有变化就直接拒绝，
# 避免出现「报新版本、实际跑旧代码」。
#
# 环境变量：SSH_HOST(默认 quizgo) REMOTE_DIR(默认 /srv/quizgo) BASE_URL(默认 http://127.0.0.1:8082)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SSH_HOST="${SSH_HOST:-quizgo}"
REMOTE_DIR="${REMOTE_DIR:-/srv/quizgo}"
BASE_URL="${BASE_URL:-http://127.0.0.1:8082}"
MARKER_NAME=".deployed-image"

WITH_ADMIN_WEB=0
WITH_NGINX=0
WITH_UNITS=0
SKIP_BACKUP=0
SKIP_BUILD=0
ACK_OPEN=0
DRY_RUN=0

for arg in "$@"; do
  case "$arg" in
    --with-admin-web) WITH_ADMIN_WEB=1 ;;
    --with-nginx)     WITH_NGINX=1 ;;
    --with-units)     WITH_UNITS=1 ;;
    --skip-backup)    SKIP_BACKUP=1 ;;
    --skip-build)     SKIP_BUILD=1 ;;
    --ack-open)       ACK_OPEN=1 ;;
    --dry-run)        DRY_RUN=1 ;;
    -h|--help)
      awk 'NR>1 && /^#/ { sub(/^# ?/, ""); print; next } NR>1 { exit }' "${BASH_SOURCE[0]}"
      exit 0 ;;
    *) echo "未知参数：${arg}（--help 查看用法）" >&2; exit 2 ;;
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

# ---------------------------------------------------------------------------
# 跨项目协作闸门（见 AGENTS.md §9）：发布改变线上，有发给 quizgo 的 open 请求时
# 必须先处理或显式确认无关。共享面不可达只告警，避免它变成发布的单点。
# ---------------------------------------------------------------------------
OPS_DIR="/srv/ops"
OPS_INBOX="$(ssh "$SSH_HOST" "$OPS_DIR/bin/inbox.sh quizgo" 2>/dev/null || true)"
if [ "$ACK_OPEN" = 1 ]; then
  echo "已按 --ack-open 跳过 inbox 闸门"
elif [ -z "$OPS_INBOX" ]; then
  echo "提示：读不到 $OPS_DIR/bin/inbox.sh（共享面未就绪？），本次跳过 inbox 闸门" >&2
elif printf '%s\n' "$OPS_INBOX" | grep -q '⬜'; then
  echo "有发给 quizgo 的未处理请求，发布已中止：" >&2
  printf '%s\n' "$OPS_INBOX" | sed 's/^/  /' >&2
  echo >&2
  echo "处理完把请求 status 改成 done 并追加 ## 回复；" >&2
  echo "确认与本次发布无关，可加 --ack-open 放行。" >&2
  exit 2
elif printf '%s\n' "$OPS_INBOX" | grep -q '没有发给 quizgo 的未处理请求'; then
  echo "inbox：没有发给 quizgo 的未处理请求"
else
  echo "提示：inbox.sh 输出无法识别，本次放行" >&2
fi

# ---------------------------------------------------------------------------
# 发布前保护：--skip-build / --skip-backup 只允许用于「不改变二进制与表结构」的情况
# ---------------------------------------------------------------------------
# 服务端代码路径：这些路径有变化就必须重建镜像
CODE_PATHS=(api internal cmd migrations go.mod go.sum Dockerfile docker-compose.yml)

# 基线是只读查询，dry-run 也要读，否则跳过类参数的护栏在 dry-run 下形同虚设
DEPLOYED_SHA="$(ssh "$SSH_HOST" "cat $REMOTE_DIR/$MARKER_NAME 2>/dev/null | head -n1" 2>/dev/null || true)"

SKIP_FLAGS_USED=""
if [ "$SKIP_BUILD" = 1 ] || [ "$SKIP_BACKUP" = 1 ]; then SKIP_FLAGS_USED=1; fi

# 脏工作区无法确定镜像对应哪份代码，跳过类参数一律拒绝
if [ "$SKIP_FLAGS_USED" = 1 ] && [ -n "$(git status --porcelain)" ]; then
  echo "工作区有未提交改动，不能用 --skip-build / --skip-backup" >&2
  exit 2
fi

BASELINE=""
HAVE_MARKER=0
if [ -n "$DEPLOYED_SHA" ]; then
  case "$DEPLOYED_SHA" in
    *+dirty)
      echo "警告：上次发布来自脏工作区（${DEPLOYED_SHA}），该基线不可信" >&2
      if [ "$SKIP_FLAGS_USED" = 1 ]; then
        echo "请先做一次干净的完整发布，再用 --skip-build / --skip-backup" >&2
        exit 2
      fi
      DEPLOYED_SHA=""
      ;;
    *)
      HAVE_MARKER=1
      ;;
  esac
fi

if [ "$HAVE_MARKER" = 1 ]; then
  if git rev-parse --verify -q "${DEPLOYED_SHA}^{commit}" >/dev/null 2>&1; then
    BASELINE="$DEPLOYED_SHA"
  else
    # 记录存在但本地没有这个 commit（换机器 / 未 fetch）：拒绝跳过类参数，
    # 不能退回 HEAD~1，否则只比较最近一次提交，可能放行旧镜像
    echo "无法解析服务器记录的发布基线 ${DEPLOYED_SHA}（本机没有该 commit）" >&2
    if [ "$SKIP_FLAGS_USED" = 1 ]; then
      echo "拒绝使用 --skip-build / --skip-backup：请先 git fetch 到该 commit，或做一次完整发布" >&2
      exit 2
    fi
  fi
elif [ "$SKIP_FLAGS_USED" = 1 ]; then
  # 只有「从未发布过」（没有基线文件）才允许退回 HEAD~1
  if git rev-parse --verify -q 'HEAD~1^{commit}' >/dev/null 2>&1; then
    BASELINE='HEAD~1'
    echo "提示：服务器没有发布基线（首次发布），按 HEAD~1 校验跳过类参数"
  else
    echo "无法确定发布基线，拒绝使用 --skip-build / --skip-backup" >&2
    exit 2
  fi
fi

changed_since_baseline() {
  [ -n "$BASELINE" ] || return 0
  git diff --name-only "$BASELINE" -- "$@" 2>/dev/null || true
}

if [ "$SKIP_BUILD" = 1 ]; then
  changed="$(changed_since_baseline "${CODE_PATHS[@]}")"
  if [ -n "$changed" ]; then
    echo "--skip-build 不能用于服务端代码/迁移变更，检测到：" >&2
    echo "$changed" | sed 's/^/  /' >&2
    exit 2
  fi
fi

if [ "$SKIP_BACKUP" = 1 ]; then
  changed="$(changed_since_baseline migrations)"
  if [ -n "$changed" ]; then
    echo "--skip-backup 不能用于带迁移的发布，检测到：" >&2
    echo "$changed" | sed 's/^/  /' >&2
    exit 2
  fi
fi

# nginx / systemd 不在镜像里，漏传参数就会「发布成功但配置没生效」，所以改成硬性要求
if [ "$WITH_NGINX" = 0 ]; then
  changed="$(changed_since_baseline deploy/nginx)"
  if [ -n "$changed" ]; then
    echo "deploy/nginx 有变化，必须加 --with-nginx（否则服务器 nginx 仍是旧配置）：" >&2
    echo "$changed" | sed 's/^/  /' >&2
    exit 2
  fi
fi

if [ "$WITH_UNITS" = 0 ]; then
  changed="$(changed_since_baseline deploy/systemd)"
  if [ -n "$changed" ]; then
    echo "deploy/systemd 有变化，必须加 --with-units（否则 unit 仍是旧的）：" >&2
    echo "$changed" | sed 's/^/  /' >&2
    exit 2
  fi
fi

# 1. 打回滚点：旧镜像留作 openquizgo:rollback
run_ssh 'docker tag openquizgo:local openquizgo:rollback 2>/dev/null || true'

# 2. 备份数据库：迁移是单向的，带迁移的发布必须先备份
if [ "$SKIP_BACKUP" = 1 ]; then
  echo "跳过备份（--skip-backup，已确认无迁移变更）"
else
  run_ssh 'sudo systemctl start quizgo-backup.service'
fi

# 3. 同步源码：不带 --delete，避免删掉服务器上的产物；.env 永远不传
#    只同步服务端构建/运行需要的部分：parent(Flutter)、tv、android、admin-web 源码
#    都是 GB 级构建产物，传过去既没用又会把 40G 盘塞满（实测曾把发布卡住 15 分钟）
RSYNC_ARGS=(-az
            --exclude '.git' --exclude '.env' --exclude 'node_modules'
            --exclude 'parent' --exclude 'tv' --exclude 'android'
            --exclude 'admin-web' --exclude 'bin' --exclude 'dist'
            --exclude 'docs/conversations'
            --exclude 'data' --exclude '.opencode' --exclude "$MARKER_NAME")
run rsync "${RSYNC_ARGS[@]}" ./ "$SSH_HOST:$REMOTE_DIR/"

if [ "$WITH_ADMIN_WEB" = 1 ]; then
  run rsync -az --delete admin-web/dist/ "$SSH_HOST:$REMOTE_DIR/admin-web/dist/"
fi

# 5a. 可选：同步 nginx 配置并热加载（deploy/nginx 有改动时）
#     容器挂的是 /srv/nginx/*，仓库里的 deploy/nginx/* 不会自动生效，必须显式同步
if [ "$WITH_NGINX" = 1 ]; then
  # 只同步 nginx.conf 与 conf.d/；仓库里若新增 snippets/ 之类，这里必须同步扩展，
  # 否则 include 指向不存在的文件（有护栏不至于挂站，但配置不会生效）
  extras="$(cd deploy/nginx && ls -1 | grep -vEx 'nginx\.conf|conf\.d' | grep -v '^\.' || true)"
  if [ -n "$extras" ]; then
    echo "警告：deploy/nginx 下有未纳管条目：$(echo "$extras" | tr '\n' ' ')" >&2
    echo "       deploy.sh 只同步 nginx.conf 与 conf.d/，请扩展同步逻辑" >&2
  fi

  # 漂移检查（仅告警，必须在同步前做）：/srv/nginx 是独立副本不是软链，
  # 有人手工改过的话先暴露出来，再被仓库版本覆盖
  local_sum="$(cd deploy/nginx && python3 - <<'PY'
import glob, hashlib
for f in ['nginx.conf'] + sorted(glob.glob('conf.d/*.conf')):
    print(hashlib.md5(open(f, 'rb').read()).hexdigest() + '  ' + f)
PY
)"
  remote_sum="$(ssh "$SSH_HOST" "cd /srv/nginx && md5sum nginx.conf conf.d/*.conf" 2>/dev/null | awk '{print $1"  "$2}')"
  if [ -n "$remote_sum" ] && [ "$local_sum" != "$remote_sum" ]; then
    echo "警告：/srv/nginx 与仓库 deploy/nginx 不一致（可能被手工改过），本次将用仓库版本覆盖：" >&2
    diff <(printf '%s\n' "$local_sum") <(printf '%s\n' "$remote_sum") | sed 's/^/  /' >&2 || true
  fi

  # 备份必须整体替换：cp -a 在目标已存在时会把源塞进目标目录，第二次就套娃，
  # 之后的「自动回滚」会恢复到更早一代。备份失败（磁盘满/权限）直接中止发布。
  run_ssh "sudo rm -rf /srv/nginx.prev && sudo cp -a /srv/nginx /srv/nginx.prev"
  run rsync -az deploy/nginx/nginx.conf "$SSH_HOST:/srv/nginx/nginx.conf"
  run rsync -az --delete deploy/nginx/conf.d/ "$SSH_HOST:/srv/nginx/conf.d/"

  # 先 nginx -t 再 reload；失败自动回滚——否则磁盘上留着坏配置，
  # 只要 web 容器因 OOM / 重启 / 下次 up -d 重建，两个站会一起起不来
  if ! run_ssh "cd $REMOTE_DIR && docker compose exec -T web nginx -t"; then
    echo "nginx 配置校验失败，回滚 /srv/nginx ← /srv/nginx.prev" >&2
    if run_ssh "sudo rsync -a --delete /srv/nginx.prev/ /srv/nginx/ && cd $REMOTE_DIR && docker compose exec -T web nginx -t"; then
      echo "已回滚，运行中的 nginx 未受影响；请修正 deploy/nginx 后重跑" >&2
    else
      echo "自动回滚也失败，请人工处理：/srv/nginx.prev 是上一代可用配置" >&2
    fi
    exit 1
  fi
  run_ssh "cd $REMOTE_DIR && docker compose exec -T web nginx -s reload"
  echo "nginx 配置已生效"
fi

# 5b. 可选：重装 systemd unit（deploy/systemd 有改动时）
if [ "$WITH_UNITS" = 1 ]; then
  run_ssh "sudo install -m 644 $REMOTE_DIR/deploy/systemd/quizgo-backup.service $REMOTE_DIR/deploy/systemd/quizgo-backup.timer $REMOTE_DIR/deploy/systemd/tailscale-aliyun-fix.service $REMOTE_DIR/deploy/systemd/tailscale-aliyun-fix.timer /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable quizgo-backup.timer tailscale-aliyun-fix.timer && sudo systemctl restart quizgo-backup.timer tailscale-aliyun-fix.timer"
fi

# 5c. 版本写入 .env：容器只读 env_file，不同步就永远是旧版本。
#     只在「构建成功之后」或「--skip-build 且即将重建 api」时调用：
#     构建失败（OOM / 拉依赖）时必须让 .env 停在上一版，否则之后任一
#     docker compose up -d 都会「报新版本、跑旧代码」。
write_app_version() {
  run_ssh "sudo sed -i 's|^APP_VERSION=.*|APP_VERSION=$VERSION|' $REMOTE_DIR/.env && sudo chown ecs-user:ecs-user $REMOTE_DIR/.env && sudo chmod 600 $REMOTE_DIR/.env && sudo grep -E '^APP_VERSION=' $REMOTE_DIR/.env"
}

# up -d 失败（migrate 报错 / 依赖起不来）时 .env 已是新版本、容器还是旧的，
# set -e 退出又会跳过版本自检，于是没人发现。写版本前记下旧值，失败时回写。
APP_VERSION_BEFORE=""
capture_app_version() {
  APP_VERSION_BEFORE="$(ssh "$SSH_HOST" "sudo grep -E '^APP_VERSION=' $REMOTE_DIR/.env | head -n1 | cut -d= -f2" 2>/dev/null || true)"
}

restore_app_version() {
  [ -n "$APP_VERSION_BEFORE" ] || return 0
  echo "回写 .env 版本：$VERSION → $APP_VERSION_BEFORE" >&2
  ssh "$SSH_HOST" "sudo sed -i 's|^APP_VERSION=.*|APP_VERSION=$APP_VERSION_BEFORE|' $REMOTE_DIR/.env && sudo chmod 600 $REMOTE_DIR/.env && sudo grep -E '^APP_VERSION=' $REMOTE_DIR/.env" \
    || echo "警告：回写 .env 失败，请手工恢复 APP_VERSION=$APP_VERSION_BEFORE" >&2
}

# 6. 构建并重启：必须用 up -d（而不是只重建 api/worker），
#    这样镜像变更会重建 migrate，迁移成功后才起 api/worker
if [ "$SKIP_BUILD" = 1 ]; then
  echo "跳过镜像构建（--skip-build，基线以来服务端代码无变化）"
  capture_app_version
  write_app_version
  if ! run_ssh "cd $REMOTE_DIR && docker compose up -d --force-recreate api worker"; then
    restore_app_version
    exit 1
  fi
else
  run_ssh "cd $REMOTE_DIR && docker compose build api"
  # 构建成功后才写版本；build 失败会因 set -e 立即退出，.env 保持上一版
  capture_app_version
  write_app_version
  if ! run_ssh "cd $REMOTE_DIR && docker compose up -d"; then
    restore_app_version
    exit 1
  fi
fi

# 7. 自检：版本号 + 镜像 ID + 状态 + 日志
if [ "$DRY_RUN" = 1 ]; then
  echo '[dry-run] 跳过自检与 /srv/ops 登记'
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

# 镜像 ID 核对：防止「版本号对了、容器还是旧镜像」
EXPECT_IMAGE="$(ssh "$SSH_HOST" "docker image inspect openquizgo:local --format '{{.Id}}'")"
RUN_IMAGE="$(ssh "$SSH_HOST" "cd $REMOTE_DIR && docker inspect --format '{{.Image}}' \$(docker compose ps -q api)")"
echo "==> 镜像：期望 $EXPECT_IMAGE / 运行 $RUN_IMAGE"
if [ "$EXPECT_IMAGE" != "$RUN_IMAGE" ]; then
  echo "镜像不一致：运行中的 api 不是 openquizgo:local 最新构建" >&2
  exit 1
fi

# 记录本次发布基线，供下次 --skip-build / --skip-backup 校验
SHA="$(git rev-parse HEAD)"
if [ -n "$(git status --porcelain)" ]; then
  SHA="$SHA+dirty"
fi
ssh "$SSH_HOST" "printf '%s\n%s\n' '$SHA' '$EXPECT_IMAGE' | sudo tee $REMOTE_DIR/$MARKER_NAME >/dev/null && sudo chmod 644 $REMOTE_DIR/$MARKER_NAME"
echo "==> 已记录发布基线：$SHA"

# 8. 跨项目协作登记：覆盖 state/quizgo.json + 追加 journal 一行。
#    失败只告警，不影响发布结果——发布已经自检通过，共享面写失败不应诱发回滚。
ops_register() {
  local now sha image md5_lines json log_month
  now="$(python3 -c 'import datetime; print(datetime.datetime.now().astimezone().isoformat(timespec="seconds"))')" || return 1
  sha="$(git rev-parse HEAD)" || return 1
  if [ -n "$(git status --porcelain)" ]; then
    sha="${sha}+dirty"
  fi
  image="$EXPECT_IMAGE"
  md5_lines="$(ssh "$SSH_HOST" "cd /srv/nginx && md5sum nginx.conf conf.d/atlas.conf conf.d/quiz.conf" 2>/dev/null)" || return 1
  [ "$(printf '%s\n' "$md5_lines" | grep -c '  ' || true)" -eq 3 ] || return 1
  json="$(printf '%s\n' "$md5_lines" | python3 -c '
import json, sys
md5 = {}
for line in sys.stdin:
    parts = line.split()
    if len(parts) == 2:
        md5[parts[1]] = parts[0]
keys = ["nginx.conf", "conf.d/atlas.conf", "conf.d/quiz.conf"]
missing = [k for k in keys if k not in md5]
if missing:
    raise SystemExit("缺少 nginx 配置：" + ", ".join(missing))
doc = {
    "project": "quizgo",
    "updatedAt": sys.argv[1],
    "updatedBy": "quizgo-ai",
    "release": {"version": sys.argv[2], "commit": sys.argv[3], "image": sys.argv[4]},
    "nginxMd5": {k: md5[k] for k in keys},
}
print(json.dumps(doc, ensure_ascii=False, indent=2))
' "$now" "$VERSION" "$sha" "$image")" || return 1
  printf '%s\n' "$json" | ssh "$SSH_HOST" "cat > /srv/ops/state/quizgo.json" || return 1
  log_month="${now:0:7}"
  ssh "$SSH_HOST" "printf '%s [quizgo] deploy | quizgo %s | image %s | ok\n' '$now' '$VERSION' '$image' >> /srv/ops/journal/${log_month}.log" || return 1
}

if ops_register; then
  echo "==> 已登记 /srv/ops/state/quizgo.json 与 journal"
else
  echo "警告：/srv/ops 登记失败（不影响发布结果），请手工补登 state/quizgo.json 与 journal" >&2
fi

# 8b. 共享面自检：status.sh 的漂移检测，出现 ⚠️ 只告警
ops_status="$(ssh "$SSH_HOST" "$OPS_DIR/bin/status.sh" 2>/dev/null || true)"
if printf '%s\n' "$ops_status" | grep -q '⚠️'; then
  echo "警告：/srv/ops/bin/status.sh 报告异常：" >&2
  printf '%s\n' "$ops_status" | grep '⚠️' | sed 's/^/  /' >&2
else
  echo "==> 共享面自检：无漂移"
fi

ssh "$SSH_HOST" "cd $REMOTE_DIR && docker compose ps --format 'table {{.Service}}\t{{.Status}}'"
ssh "$SSH_HOST" "cd $REMOTE_DIR && docker compose logs --tail=80 api 2>&1 | grep -iE 'error|panic' || echo '日志无 error/panic'"
ssh "$SSH_HOST" "free -m | awk 'NR==2 {print \"可用内存：\" \$7 \" MB\"}'; df -h / | awk 'NR==2 {print \"根分区：\" \$5 \" 已用\"}'"

echo
echo "发布完成：$VERSION"
