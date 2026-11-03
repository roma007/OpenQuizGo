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
| Prometheus | 指标抓取与告警 | 推荐 |
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
| `SMS_MOCK_CODE` | 否 | `123456` | 仅 dev |
| `REPORT_INTERVAL` | 否 | `1h` | worker 聚合间隔 |
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

```bash
docker compose up --build
```

包含：PostgreSQL、Redis、API（启动前自动迁移）、worker、Prometheus（:9090）。

- API：http://localhost:8080
- Prometheus：http://localhost:9090
- 指标：http://localhost:8080/metrics

## 5. 迁移

```bash
make migrate-up       # 应用全部迁移
make migrate-status   # 查看版本
make migrate-down     # 回滚一个（仅开发/预发）
```

- 迁移内嵌在 `migrate` 二进制，生产无需额外文件。
- 发布顺序：备份 → 迁移 → 启动新版本 → 验证 `/readyz`。
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

- 抓取配置：[`deploy/prometheus/prometheus.yml`](../deploy/prometheus/prometheus.yml)
- 告警规则：[`deploy/prometheus/alerts.yml`](../deploy/prometheus/alerts.yml)
- 关键指标：
  - `openquizgo_http_request_duration_seconds` p95
  - `openquizgo_http_requests_total{status=~"5.."}`
  - `openquizgo_http_in_flight_requests`
  - `openquizgo_quiz_answers_total{result}`
  - `openquizgo_watch_grants_issued_total`
- 日志为 JSON（非 dev），通过 `request_id`/`traceparent` 关联。

## 8. 备份与恢复

```bash
# 每日全量
pg_dump --format=custom --file=/backup/openquizgo-$(date +%F).dump "$DATABASE_URL"

# 恢复
pg_restore --clean --if-exists --dbname="$DATABASE_URL" /backup/openquizgo-YYYY-MM-DD.dump
```

- 建议开启 WAL 归档，RPO ≤15 分钟。
- 对象存储（题干图/音）单独做跨区复制。
- 每季度演练一次恢复。

## 9. 扩容与高可用

- API 无状态，可水平扩容；会话/限流状态在 Redis，授权判断在 PostgreSQL。
- 授权签发使用孩子级 advisory lock，多实例安全。
- worker 可多实例运行（upsert 幂等），建议单实例或加分布式锁以省资源。
- PostgreSQL 主从 + 连接池；Redis 主从/哨兵。
- 容量基线：1 万家庭 / 5 万设备 / 峰值 1000 QPS（设计假设）。

## 10. 发布与回滚

1. 预发执行迁移与全量集成测试。
2. 灰度 5% 设备，观察 5xx、p95、授权拒绝率。
3. 全量发布。
4. 异常回滚镜像；数据库不做破坏性 down，必要时用前向修复迁移。

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
./gradlew :app:assembleRelease -PapiBaseUrl=https://<生产域名>/     # 正式包
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
