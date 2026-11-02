# OpenQuizGo

面向家庭的儿童电视使用管理服务：孩子在电视上答题，按**加权分**解锁第三方视频 App 的使用时长；家长在手机端配置规则、查看报告。服务端是唯一裁决者，电视端只负责执行与上报。

> 项目规约见 [`AGENTS.md`](AGENTS.md)：所有对话留档、所有开发先设计后实施。
> 设计基线：[`docs/design/001-项目总体设计方案.md`](docs/design/001-项目总体设计方案.md)
> 实施计划：[`docs/plans/001-项目总体实施计划.md`](docs/plans/001-项目总体实施计划.md)

## 当前进度

- ✅ T1 工程骨架
- ✅ T2 数据模型与迁移（26 张业务表 + 标签种子）
- ⏳ T0 设备管控 PoC（需电视实机）
- ⏳ T3 身份/家庭/设备

## 技术栈

- 服务端：Go 1.25（模块化单体）、PostgreSQL、Redis
- 迁移：goose（SQL 内嵌，单二进制）
- 电视端：Android TV（Kotlin + Compose for TV）
- 家长端：Flutter

## 目录结构

```
cmd/api           HTTP 服务入口
cmd/migrate       数据库迁移入口
internal/config   环境变量配置
internal/httpapi  路由、中间件、统一响应
internal/platform 数据库 / Redis / 日志适配
migrations        SQL 迁移（goose）
docs              设计、计划、对话归档
```

## 本地启动

前置：Go 1.25+、PostgreSQL 14+、Redis 6+。

```bash
cp .env.example .env
# 按本机情况修改 DATABASE_URL / REDIS_URL

createdb openquizgo
make migrate-up
make run
```

验证：

```bash
curl localhost:8080/healthz
curl localhost:8080/readyz
curl localhost:8080/v1/meta
```

## 常用命令

```bash
make help           # 查看全部命令
make test           # 单元测试
make vet            # 静态检查
make build          # 编译到 bin/
make migrate-up     # 执行迁移
make migrate-down   # 回滚一个迁移
make dev            # 迁移并启动
```

## 容器启动

```bash
docker compose up --build
```

## 配置

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `dev` | `dev` 输出文本日志，其他输出 JSON |
| `APP_VERSION` | `dev` | 版本号，透传到 `/v1/meta` |
| `HTTP_ADDR` | `:8080` | 监听地址 |
| `DATABASE_URL` | 无（必填） | PostgreSQL DSN |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis DSN |
| `DB_MAX_CONNS` | `10` | 连接池上限 |
| `LOG_LEVEL` | `info` | debug / info / warn / error |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅退出超时 |

## 守护端能力边界（重要）

观看内容为第三方 App，播放器不可控：

- 拦截依赖「使用情况访问 + 悬浮窗 + 无障碍」（标准模式）或 Device Owner（增强模式）。
- 答题暂停/恢复按 MediaSession → 音频焦点 → 媒体按键组合尝试，属**尽力而为**。
- HDMI、投屏、手机热点不在管控范围。

以上能力须经第 0 期 PoC 在目标机型上验证，详见设计 4.6–4.8。
