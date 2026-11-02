# OpenQuizGo

面向家庭的儿童电视使用管理服务：孩子在电视上答题，按**加权分**解锁第三方视频 App 的使用时长；家长在手机端配置规则、查看报告。服务端是唯一裁决者，电视端只负责执行与上报。

> 项目规约见 [`AGENTS.md`](AGENTS.md)：所有对话留档、所有开发先设计后实施。
> 设计基线：[`docs/design/001-项目总体设计方案.md`](docs/design/001-项目总体设计方案.md)
> 实施计划：[`docs/plans/001-项目总体实施计划.md`](docs/plans/001-项目总体实施计划.md)

## 当前进度

- ✅ T1 工程骨架
- ✅ T2 数据模型与迁移（26 张业务表 + 标签种子）
- ✅ T3 身份/家庭/设备（登录、孩子、激活、PIN、邀请）
- ✅ T4 题库与治理 API（CRUD、检索、状态机、导入、RBAC、审计）
- ✅ T5 组卷 v0（权重配额、趣味上限、去重、降级兜底）
- ✅ T6 答题会话与判分（一次性令牌、服务端判分、防重放）
- 🚧 T0 设备管控 PoC（APK 已产出，待实机验证）
- ⏳ T7 加权规则引擎

## 技术栈

- 服务端：Go 1.25（模块化单体）、PostgreSQL、Redis
- 迁移：goose（SQL 内嵌，单二进制）
- 电视端：Android TV（Kotlin + Compose for TV）
- 家长端：Flutter

## 目录结构

```
cmd/api           HTTP 服务入口
cmd/migrate       数据库迁移入口
cmd/admin         管理端账号 CLI
internal/config   环境变量配置
internal/httpapi  路由、中间件、统一响应
internal/platform 数据库 / Redis / 日志适配
migrations        SQL 迁移（goose）
android           T0 设备管控 PoC（Java，见 android/README.md）
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
| `JWT_SECRET` | dev 默认值 | 访问令牌签名密钥，生产必须替换且 ≥32 字符 |
| `ACCESS_TOKEN_TTL` | `15m` | 访问令牌有效期 |
| `REFRESH_TOKEN_TTL` | `720h` | 刷新令牌有效期 |
| `DATA_ENCRYPTION_KEY` | dev 默认值 | 手机号字段加密密钥（base64 32 字节） |
| `PHONE_HASH_KEY` | dev 默认值 | 手机号查询哈希密钥 |
| `SMS_MODE` | `mock` | `mock` 固定验证码；`disabled` 关闭 |
| `SMS_MOCK_CODE` | `123456` | mock 模式验证码 |

## 已实现接口（T3）

```text
POST /v1/auth/sms/code          发送登录验证码（mock 写入日志）
POST /v1/auth/login             手机号 + 验证码登录（首登自动建家庭）
POST /v1/auth/refresh           轮换刷新令牌
POST /v1/auth/logout            吊销刷新令牌

GET  /v1/family/me              当前账号
GET  /v1/family/children        孩子列表
POST /v1/family/children        新增孩子（自动创建默认规则）
GET  /v1/family/children/{id}   孩子详情
PUT  /v1/family/children/{id}   更新孩子

POST /v1/family/pin             设置/修改家长 PIN
POST /v1/family/pin/verify      校验 PIN

POST /v1/family/members/invite  生成家庭邀请码
POST /v1/family/members/join    使用邀请码加入
GET  /v1/family/members         家庭成员

POST /v1/device/activate/code   电视端申请激活码
POST /v1/family/devices/activate 家长确认绑定
POST /v1/device/token           电视端兑换设备令牌
GET  /v1/device/me              设备信息（设备令牌鉴权）
GET  /v1/family/devices         设备列表
DELETE /v1/family/devices/{id}  解绑（令牌立即失效）
```

## 管理端（T4）

```text
POST /v1/admin/auth/login            管理员登录
GET  /v1/admin/me                    当前管理员与权限

GET  /v1/admin/questions             四维检索 + 关键词 + 分页
POST /v1/admin/questions             创建题目（draft）
GET  /v1/admin/questions/{id}        题目详情
PUT  /v1/admin/questions/{id}        编辑（仅 draft/offline，版本 +1）
DELETE /v1/admin/questions/{id}      删除（仅 draft）
POST /v1/admin/questions/import      批量导入（JSON 或 CSV）
POST /v1/admin/questions/{id}/submit-review
POST /v1/admin/questions/{id}/review 审核（approve/reject）
POST /v1/admin/questions/{id}/publish
POST /v1/admin/questions/{id}/offline
POST /v1/admin/questions/{id}/resubmit

GET  /v1/admin/audit                 审计日志
POST /v1/admin/quiz/preview          组卷 dry-run（查看配额、题目、警告）
```

## 电视端答题（T6，设备令牌）

```text
POST /v1/quiz/sessions                    创建会话（返回题目 + 一次性题令牌，不含答案）
GET  /v1/quiz/sessions/{id}               断线重连，查看进度与未答题
POST /v1/quiz/sessions/{id}/answers       提交答案（token + answer，服务端判分）
POST /v1/quiz/sessions/{id}/finish        结算（加权分 ≥ 门槛 → passed）
```

- 会话有效期 10 分钟；同一设备同时只能有一个进行中的会话。
- 每题令牌一次性，重放返回 409；答案与解析不下发，判分后才返回。
- 服务端计时 < 300ms 标记 `suspicious`。

状态机：`draft → review → trial → online → offline`，`review` 可驳回回 `draft`，`offline` 可重新提交。

创建管理员（需先迁移）：

```bash
go run ./cmd/admin create -username admin -password 'admin12345' -role super_admin -name 管理员
go run ./cmd/admin list
```

角色：`super_admin`（全部）、`editor`（读写+导入）、`reviewer`（查看+审核）、`ops`（查看+上下线+审计）。

本地冒烟：`make dev` 后请求验证码，服务端日志会打印 `123456`。

## 守护端能力边界（重要）

观看内容为第三方 App，播放器不可控：

- 拦截依赖「使用情况访问 + 悬浮窗 + 无障碍」（标准模式）或 Device Owner（增强模式）。
- 答题暂停/恢复按 MediaSession → 音频焦点 → 媒体按键组合尝试，属**尽力而为**。
- HDMI、投屏、手机热点不在管控范围。

以上能力须经第 0 期 PoC 在目标机型上验证，详见设计 4.6–4.8。
