# 部署与运维手册

> 适用版本：0.1.0（第一期服务端）
> 对应设计：[001-项目总体设计方案](design/001-项目总体设计方案.md)

## 1. 组件

| 组件 | 说明 | 必需 |
| --- | --- | --- |
| `api` | HTTP 服务（家长端、电视端、管理端） | 是 |
| `worker` | 报告聚合定时任务 | 是（可与 api 合并部署） |
| PostgreSQL | 唯一事实来源，建议 16 | 是 |
| Redis | 验证码、限流、设备激活码 | 是 |
| Prometheus | 指标抓取与告警 | 未启用（2C2G 精简模式已移除） |
| 反向代理 | TLS 终止、限流、访问控制 | 生产必需 |

## 2. 环境变量

| 变量 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- |
| `APP_ENV` | 否 | `dev` | `dev` 输出文本日志；其他输出 JSON |
| `APP_VERSION` | 否 | `dev` | 透传到 `/v1/meta` |
| `HTTP_ADDR` | 否 | `:8080` | 监听地址 |
| `DATABASE_URL` | **是** | — | PostgreSQL DSN |
| `REDIS_URL` | 否 | `redis://localhost:6379/0` | Redis DSN |
| `DB_MAX_CONNS` | 否 | `10` | 连接池上限 |
| `LOG_LEVEL` | 否 | `info` | debug/info/warn/error |
| `SHUTDOWN_TIMEOUT` | 否 | `10s` | 优雅退出 |
| `JWT_SECRET` | 生产必填 | dev 默认 | 家长/管理端/授权令牌签名，≥32 字符 |
| `ACCESS_TOKEN_TTL` | 否 | `15m` | 家长访问令牌 |
| `REFRESH_TOKEN_TTL` | 否 | `720h` | 刷新令牌 |
| `DATA_ENCRYPTION_KEY` | 生产必填 | dev 默认 | 手机号 AES-256-GCM 密钥（base64 32 字节） |
| `PHONE_HASH_KEY` | 生产必填 | dev 默认 | 手机号 HMAC 密钥 |
| `OFFLINE_LEASE_KEY` | 生产必填 | dev 默认 | 离线租约 Ed25519 种子（base64 32 字节） |
| `SMS_MODE` | 否 | `mock` | 生产禁止 `mock`，需接入真实短信通道后设为 `disabled` 或扩展实现 |
| `PAYMENT_CALLBACK_SECRET` | 是 | dev 默认 | 支付回调 HMAC 密钥；生产必须显式配置，留空则禁用回调 |
| `REFUND_CALLBACK_SECRET` | 否 | 回退支付密钥 | 退款回调 HMAC 密钥；与支付密钥都为空则禁用退款回调 |
| `UPLOAD_DIR` | 否 | `./data/uploads` | 题目图片本地存储目录 |
| `UPLOAD_MAX_MB` | 否 | `5` | 单张题目图片大小上限（MB） |
| `SMS_MOCK_CODE` | 否 | `123456` | 仅 dev |
| `REPORT_INTERVAL` | 否 | `1h` | worker 聚合间隔 |
| `DEVICE_STALE_DAYS` | 否 | `30` | 设备多少天无活动后自动解绑；`0` 关闭 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 否 | 空 | OTLP/HTTP 追踪端点，留空关闭 |

> 生产启动会校验非 dev 环境的密钥与 `SMS_MODE`，缺失或仍为 mock 将拒绝启动。

## 3. 本地运行

```bash
cp .env.example .env
# 修改 DATABASE_URL / REDIS_URL
createdb openquizgo
make migrate-up
make run
```

初始化管理员：

```bash
go run ./cmd/admin create -username admin -password '********' -role super_admin -name 管理员
```

报告 worker：

```bash
make build
./bin/worker
```

## 4. Docker Compose

生产用根目录 `docker-compose.yml`（2C2G 精简模式），共 6 个服务：

| 服务 | 作用 | 端口 | 内存上限 |
| --- | --- | --- | --- |
| `migrate` | 一次性执行 goose 迁移，成功后才放行 api/worker | 无 | 256m |
| `api` | HTTP 服务 | `127.0.0.1:8080` | 256m |
| `worker` | 报告聚合等后台任务 | 无 | 128m |
| `postgres` | 数据库 | 仅容器网络 | 512m |
| `redis` | 验证码 / 限流 / 激活码 | 仅容器网络 | 128m |
| `web` | nginx（Atlas 静态站 + 后台/API 反代） | host 网络 `8081` / `8082` | 64m |

```bash
docker compose up -d    # 先跑 migrate，失败则 api/worker 不启动
docker compose ps       # migrate 应显示 Exited (0)
```

- 管理后台与 API：`http://<host>:8082`（经 nginx，仅本机 + Tailscale 网段）
- 指标：`api` 的 `/metrics` 不对外；Prometheus 已移除，见第 7 节
- 发布用 `scripts/deploy.sh`，不要只重建 api（会跳过迁移）


## 5. 迁移

```bash
make migrate-up       # 应用全部迁移
make migrate-status   # 查看版本
make migrate-down     # 回滚一个（仅开发/预发）
```

- 迁移内嵌在 `migrate` 二进制，生产无需额外文件。
- 发布顺序：备份 → 迁移 → 启动新版本 → 验证（见下）。
- 探针分工：`/healthz` 是存活探针（进程在跑即 200），`/readyz` 是依赖就绪探针（DB/Redis 异常会失败）。
  发布自检看 **`/healthz` 与 `/v1/meta`**（能不能访问、版本对不对）；`/readyz` 用于排查依赖。
- 回滚：镜像回退 + 按需执行 down；数据迁移默认只向前。

## 6. 反向代理（Nginx 示例）

```nginx
server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate     /etc/ssl/api.crt;
    ssl_certificate_key /etc/ssl/api.key;

    location /metrics {
        allow 10.0.0.0/8;
        deny all;
        proxy_pass http://127.0.0.1:8080;
    }

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
        proxy_pass http://127.0.0.1:8080;
    }
}
```

- 只暴露 API 与必要端口；PostgreSQL/Redis 不对外。
- `/metrics` 仅允许监控网段。
- 管理端建议独立域名/VPN。

## 7. 监控与告警

**当前未启用 Prometheus**：2C2G 单机常驻不划算，已从 `docker-compose.yml` 移除。

- `deploy/prometheus/` 里的抓取与告警配置仅作扩容后的参考（目录内有 README 说明）。
- `api` 仍暴露 `/metrics`，但 nginx `location /metrics` 显式 `deny all`，只在容器网络/本机可达。
- 临时排查：`ssh quizgo 'cd /srv/quizgo && docker compose exec -T api wget -qO- 127.0.0.1:8080/metrics | head'`。
- 日常观测改用：`docker compose logs`、`/healthz`、`/readyz`、阿里云云监控（CPU/内存/磁盘）。
- 扩容后再抓这些指标：
  - `openquizgo_http_request_duration_seconds` p95
  - `openquizgo_http_requests_total{status=~"5.."}`
  - `openquizgo_http_in_flight_requests`
  - `openquizgo_quiz_answers_total{result}`
  - `openquizgo_watch_grants_issued_total`
- 日志为 JSON（非 dev），通过 `request_id`/`traceparent` 关联。

## 8. 备份与恢复

生产实现是 `scripts/backup-pg.sh`，由 systemd timer 调度（`deploy/systemd/quizgo-backup.*`）：

- 时间：每天 03:20，随机延迟最多 5 分钟，`Persistent=true`（关机错过会补跑）
- 产物：`/srv/backup/pg/openquizgo-YYYYmmdd-HHMMSS.sql.gz`
  （`pg_dump --clean --if-exists` 的纯 SQL，gzip 压缩，非 custom format）
- 保留 7 天；同目录另有 `.env` 快照 `env-*.bak`（600，**含密钥**，不参与异地备份）
- 校验：文件 ≥4KB 且 ≥10 张 `CREATE TABLE`，否则判失败
- 磁盘保护：可用空间 <1GB 时直接跳过，避免写满
- 已做恢复演练：43 表 / 1 管理员 / goose 迁移记录全部恢复成功

```bash
# 手动备份一次
ssh quizgo 'sudo systemctl start quizgo-backup.service'
ssh quizgo 'ls -lh /srv/backup/pg/'

# 恢复（先停 api/worker，避免写入干扰）
ssh quizgo 'cd /srv/quizgo && docker compose stop api worker'
ssh quizgo 'cd /srv/quizgo && gunzip -c /srv/backup/pg/openquizgo-XXXX.sql.gz | docker compose exec -T postgres psql -U openquizgo -d openquizgo'
ssh quizgo 'cd /srv/quizgo && docker compose start api worker'
```

- **没有 WAL 归档**，RPO = 最后一个备份点（最长约 24 小时）；要更低 RPO 再上 WAL/流复制。
- 异地备份（OSS）是 P3 待办：只传 `*.sql.gz`，`env-*.bak` 必须排除或先加密。
- 每季度演练一次恢复。


## 9. 扩容与高可用

- API 无状态，可水平扩容；会话/限流状态在 Redis，授权判断在 PostgreSQL。
- 授权签发使用孩子级 advisory lock，多实例安全。
- worker 可多实例运行（upsert 幂等），建议单实例或加分布式锁以省资源。
- PostgreSQL 主从 + 连接池；Redis 主从/哨兵。
- 容量基线：1 万家庭 / 5 万设备 / 峰值 1000 QPS（设计假设）。

## 10. 发布与回滚

> **可执行流程见仓库根目录 `AGENTS.md` 第八节。**
> `AGENTS.md` 会被 AI 代理自动加载，本文件不会；两者冲突时以 `AGENTS.md` 为准。
> 本节只保留取舍与背景，供人阅读和排查时参考。

### 10.1 发布顺序（固定，不可调整）

```
备份 → 迁移 → 启动新版本 → 验证 /healthz 与 /v1/meta → 观察日志与资源
```

- 迁移失败（非 0 退出）时，`api` / `worker` 不会启动，这是 compose 的 `service_completed_successfully` 保护。
- 验证不通过立即回滚镜像；不要在现场改配置试错。

### 10.2 为什么迁移不可回滚

goose 迁移是单向的，`down` 仅用于开发环境。生产回滚只回代码、不回库：

- 迁移约定**只加不删**（加表、加列、加索引），旧代码面对新结构仍可运行。
- 删除列/改类型这类破坏性变更，必须拆成「先加新字段 → 双写 → 切读 → 再删旧字段」多次发布。
- 真要回库只能恢复备份，代价是全量数据回退，因此**带迁移的发布必须先备份**。

### 10.3 回滚语义

| 回滚对象 | 手段 | 影响 |
| --- | --- | --- |
| 应用代码 | `docker tag openquizgo:rollback openquizgo:local` + `up -d --no-build` | 秒级，不停库 |
| 管理端前端 | 重新 rsync 上一版 `dist` | 秒级 |
| Atlas 静态站 | `rsync -a --delete /srv/atlas.prev/ /srv/atlas/` | 秒级，无损 |
| 数据库 | 用 `/srv/backup/pg/` 恢复 | 分钟级，丢失备份点之后的数据 |

### 10.4 单机部署的现实约束

- 服务器 2C2G + 4G swap，不适合在机器上跑 Atlas 构建，也不适合并行构建多个镜像。
- 发布窗口内不要同时做备份、构建、压测，避免触发 OOM。
- 单实例谈不上灰度：**发布即全量**，所以「先备份 + 快速回滚」比「灰度」更现实。
- 镜像每次构建约 63MB，定期 `docker builder prune -f` 控制磁盘。

### 10.5 未来扩容后的灰度

多实例就绪后按以下顺序演进：预发跑迁移与集成测试 → 灰度 5% 设备观察 5xx / p95 / 授权拒绝率 →
全量发布。当前单机阶段可跳过，但不要因此省略备份与验证。

## 11. 安全

上线前逐项检查 [`docs/security-checklist.md`](security-checklist.md)，重点：

- 替换全部 dev 默认密钥。
- 关闭 `SMS_MODE=mock`。
- `/metrics` 与管理端网络隔离。
- 数据库与 Redis 不暴露公网。
- 开启审计日志与备份加密。

## 12. Android TV 守护端

### 12.1 构建

```bash
cd tv
./gradlew :app:assembleDebug -PapiBaseUrl=http://<测试地址>:18080/   # 本地联调
./gradlew :app:assembleRelease -PapiBaseUrl=https://<生产域名>/ \
  -PexpectedSignature=<APK 签名证书 SHA-256>                    # 正式包（建议配置签名 pin）
```

- 正式包必须使用 HTTPS（release 不允许明文 HTTP）；
- 生产签名放在仓库根目录 `keystore.properties`（已 gitignore）：
  `storeFile` / `storePassword` / `keyAlias` / `keyPassword`；
- 未提供签名配置时回退 debug 签名，仅限内部验收。

### 12.2 安装与升级

```bash
adb install -r dist/OpenQuizGo-TV-0.2.0.apk
adb shell settings put secure enabled_accessibility_services \
  com.openquizgo.quiz/com.openquizgo.guard.platform.GuardAccessibilityService
adb shell settings put secure accessibility_enabled 1
adb shell appops set com.openquizgo.quiz GET_USAGE_STATS allow
```

- 同签名升级保留激活令牌与本地授权；
- 换签名必须卸载，需重新激活；
- 创维机型安装/升级后无障碍可能失效，需重启电视并重设上述 settings。

### 12.3 本地联调

```bash
adb reverse tcp:18080 tcp:18080
./gradlew :app:assembleDebug -PapiBaseUrl=http://127.0.0.1:18080/
```

调试触发（仅 debug 构建）：

```bash
adb shell am broadcast -n com.openquizgo.quiz/com.openquizgo.guard.debug.DebugReceiver \
  -a com.openquizgo.guard.DEBUG_FORCE_QUIZ
adb shell am broadcast -n com.openquizgo.quiz/com.openquizgo.guard.debug.DebugReceiver \
  -a com.openquizgo.guard.DEBUG_FORCE_BLOCK
```

验收报告见 [`docs/acceptance/phase2.md`](acceptance/phase2.md)。

## 13. 题库治理后台（admin-web）

```bash
cd admin-web
npm install
npm run dev -- --host 127.0.0.1 --port 5175   # 代理 /v1 → 127.0.0.1:18080
npm run build                                  # 产物 dist/
```

- 管理员账号：`go run ./cmd/admin create -username admin -password '********' -role super_admin -name 管理员`
- 生产建议与 API 同源托管：`/admin/` 指向 `admin-web/dist`，`/v1/` 反向代理到 Go 服务；
- 同源部署可避免 CORS；管理端令牌由服务端写入 HttpOnly Cookie（`Path=/v1`，`SameSite=Strict`），前端不再使用 localStorage；
- Cookie 的 `Secure` 依赖反向代理传递 `X-Forwarded-Proto: https`（上面的 Nginx 示例已配置）；
- 管理端写请求需带 `X-Admin-Request: 1`（前端已内置）；生产建议在 Nginx 追加安全头：

```nginx
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'" always;
add_header X-Content-Type-Options nosniff always;
add_header Referrer-Policy strict-origin-when-cross-origin always;
```
