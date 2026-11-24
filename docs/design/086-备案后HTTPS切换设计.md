# 086-备案通过后的 HTTPS 切换设计

| 项目 | 内容 |
| --- | --- |
| 编号 | 086 |
| 状态 | approved |
| 作者 | AI |
| 创建日期 | 2026-11-23 |
| 最后更新 | 2026-11-24（v4：HSTS 已开；www 子域确认不加 DNS，标注未启用） |
| 对应计划 | [086-备案后HTTPS切换实施计划](../plans/086-备案后HTTPS切换实施计划.md) |

## 0. 修订说明（v1 → v2）

v1 假设「服务器无证书、需自建 ACME」，选型 `acme.sh` + Let's Encrypt + 单张 4 SAN 证书。
实际情况已变：

- Atlas 已在阿里云下单 2 张免费 DV 证书（¥0，90 天），**已落地** `/srv/nginx/certs/`：
  - `atlas.openapp.asia.{pem,key}`，SAN 含 `www.atlas.openapp.asia`
  - `quiz.openapp.asia.{pem,key}`，SAN 含 `www.quiz.openapp.asia`
  - 签发者 DigiCert（Encryption Everywhere DV G2），`notAfter 2027-02-20 23:59:59 GMT`（= 2027-02-21 07:59 CST）
  - 目录 `700`、文件 `600`、属主 `ecs-user`，私钥与证书已核对匹配
- 因此 **v2 取消 acme.sh / webroot / 续期 timer**，改用已到位证书；代价是 90 天一次的人工换证，
  用「到期提醒 + `status.sh` 剩余天数」兜底，换证自动化另立 088 评估。
- 同时吸收 Atlas 请求 `2026-11-23-atlas-to-quizgo-enable-443-tls.md`（含其「修订」「修订 2」）的
  安全加固要求：`limit_req`、`/v1/admin/` 的 IP 限流。

### v3 变更（2026-11-24 用户决策）

- 用户选择「管理后台公网可达 + 强口令 + 限流」，**否决** Atlas 建议的「收进 tailnet」。
  因此 `/v1/admin/**` 与后台 SPA 均公网开放，防线改为三层：
  1. 12 位以上口令（087 已强制，禁弱口令与旧口令复用）；
  2. nginx `qz_auth` 按 IP 10r/m、burst 5 → 越界 429；
  3. Go 侧按用户名 5 次/15 分钟锁 15 分钟（已有）。
- 后台 SPA 对公网可见 ⇒ **页脚备案号从「可选」变为必需**（备案巡检会看）。

## 1. 背景与目标

- 现状：
  - ICP 备案 2026-11-23 通过，备案号 `粤ICP备2026058093号-2`（Atlas 页脚已上线）。
  - 安全组 `TCP 80/443` 已删除，公网对 80/443 超时（`status.sh` 自探 `exit 28`）。
  - 共享 nginx 只监听 `0.0.0.0:8081`（Atlas）/ `0.0.0.0:8082`（QuizGo 后台 + `/v1`），
    且 `allow 127.0.0.1 + 100.64.0.0/10; deny all`，公网不可达；证书目录未挂进容器。
  - 家长端 App 走 `/v1/`，公网不可用 → 只能在 tailnet 内联调。
- 目标（可验证）：
  1. `https://atlas.openapp.asia`、`https://quiz.openapp.asia` 公网可用，证书链完整。
  2. 80 全量 301 → 443；未知 Host / 裸 IP 不泄露默认站（444 或 TLS 握手拒绝）。
  3. 管理面公网可达，但受「强口令 + nginx IP 限流 + 应用层按用户名锁定」三层约束。
  4. 家长端 API 公网可用，且有 IP 维度限流兜底。
  5. 安全基线不回退：`server_tokens off`、TLS1.2+、`/metrics` 403、备份权限 600/700。
  6. 备案号在 Atlas 页脚（已有）与 QuizGo 后台页脚可见。
- 非目标：
  - 不引入 CDN / WAF / 负载均衡；不改业务逻辑与接口契约。
  - 不退役 `8081/8082`（保留 tailnet 排障通道与 `deploy.sh` 的 `BASE_URL`）。
  - 不做 IPv6、HTTP/3、OCSP stapling、双向 TLS、mTLS。
  - 不自动续期（90 天 DV 证书需控制台重新申请，见 088 待办）。
  - 首批不开 HSTS，观察 24 小时无混内容后再开（见 T9）。

## 2. 需求与验收标准

| 编号 | 需求 | 验收标准 | 优先级 |
| --- | --- | --- | --- |
| R1 | 公网 HTTPS | 两域名 `curl -I https://…` → 200；`openssl s_client -verify_return_error` 链到 DigiCert 根 | P0 |
| R2 | 80 跳 443 | `curl -I http://…` → 301，Location 为 https；`openapp.asia`/`www` → `https://atlas.openapp.asia` | P0 |
| R3 | 未知 Host 不泄露 | `curl -H 'Host: x' http://8.163.122.39/` → 301（兜底跳 Atlas）；`curl -k https://8.163.122.39/` 握手失败（`ssl_reject_handshake`） | P0 |
| R4 | 管理面可登录但抗爆破 | 公网 `POST /v1/admin/auth/login` 错口令 → 401；同一 IP 连打 8 次 → 429；后台页面公网 200 | P0 |
| R5 | 家长端 API | 公网 `/v1/meta` 200；`/v1/auth/login` 可达（错口令 401/422） | P0 |
| R6 | 限流 | 连打 8 次 `/v1/admin/auth/login` 出现 429；`/v1/auth/` 超 10r/m 出现 429 | P1 |
| R7 | 备案号 | QuizGo 后台页脚可见 `粤ICP备2026058093号-2`，链接 `https://beian.miit.gov.cn` | P1 |
| R8 | 不回归 | `/v1/meta`、登录、SPA 深层路由刷新、`/assets` 长缓存、`/healthz`、8081/8082 均正常 | P0 |
| R9 | 可回滚 | `/srv/nginx.prev` 1 分钟内恢复；证书故障可退回 8081/8082 | P1 |
| R10 | 证书可观测 | `status.sh` 显示剩余天数；到期前 30 天登记待办 | P1 |

## 3. 方案对比与选型

### 3.1 证书来源（v2 变更点）

| 方案 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- |
| A 用 Atlas 已采购的 DV 证书 | 已到位、¥0、SAN 含 www、私钥不出服务器 | **90 天手动换证**，易忘 | ✅ 选用（v2） |
| B `acme.sh` + Let's Encrypt | 全自动续期 | 需自行申请/校验/续期，且与已到位证书重复 | 否（留作 088 候选） |
| C 付费 1 年 OV 证书 | 一年一换 | 花钱；本期无预算 | 否 |

选型理由：证书已经买好放在机器上，再引入 ACME 只会多一条链路和一份待办；90 天的风险用
「`status.sh` 剩余天数 + ROADMAP 待办」可控。到期前（2027-01 下旬）再决定是否切 acme.sh 自动化。

### 3.2 TLS 终止位置

仍在共享 nginx（唯一前端容器，`network_mode: host`）终止。证书放 `/srv/nginx/certs`，
以**只读**方式挂进 `quizgo-web-1`；`atlas.conf` / `quiz.conf` 的源文件都在本仓库，由 `--with-nginx` 同步。
证书目录**不得**放在 `conf.d/` 下——`--with-nginx` 用 `rsync --delete`，会把它删掉。

### 3.3 管理面暴露程度（v3 已定：方案 A）

| 方案 | 做法 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- | --- |
| A 公网可达 + 强口令 + 限流 | 后台与 `/v1/admin/` 公网开放，`qz_auth` 限流 | 任意设备都能登录，无客户端依赖 | 后台是撞库目标 | ✅ 用户 2026-11-24 选定 |
| B 收进 tailnet | 后台与 `/v1/admin/` 仅 tailnet | 公网连登录请求都进不来 | 管理员设备必须装 Tailscale | 用户否决 |

配套硬性条件（缺一不可）：
- 口令 ≥12 字符且不得与旧口令/用户名相同（087 实现，`422` 拒绝）；
- `qz_auth` 10r/m + burst 5（同一 IP 撞库在 nginx 层就被挡）；
- Go 侧 `rl:admin:login:<username>` 5 次/15 分钟（已有，改密用独立键）；
- 后台页脚展示备案号（公网可见，备案巡检会看）；
- 观察 access log 中的 401/429 突增，异常时再考虑加 Cloudflare 或收回 tailnet。

## 4. 总体架构

- 证书与挂载：
  - 宿主：`/srv/nginx/certs/`（`700`，文件 `600`，属主 `ecs-user`）——由 Atlas 维护，QuizGo 只读消费。
  - 容器：`- /srv/nginx/certs:/etc/nginx/certs:ro`（compose `web.volumes` 新增一行）。
  - 私钥仅存在于宿主该目录，**不进仓库、不进备份包**；`/srv/ops/state` 不记录证书内容。
- nginx 配置（仓库 `deploy/nginx/`）：
  - `nginx.conf`：`http` 块新增两个 `limit_req_zone`（`qz_auth` 10r/m、`qz_api` 600r/m）与 `limit_req_status 429`。
  - `conf.d/00-default.conf`（新增）：80 `default_server` → 301 `https://atlas.openapp.asia`；443 `default_server` + `ssl_reject_handshake on`。
  - `conf.d/atlas.conf`：80 → 301；443 静态站，`server_name atlas.openapp.asia www.atlas.openapp.asia`；`8081` 原样保留。
  - `conf.d/quiz.conf`：80 → 301；443 后台 + `/v1/`，`server_name quiz.openapp.asia www.quiz.openapp.asia`；
    `location /v1/admin/`（`qz_auth` 限流，公网可达）、`location /v1/auth/`（`qz_auth`）、
    `location /v1/`（`qz_api`）、`location /`（公网 SPA + 页脚备案号）；`8082` 原样保留。
  - `conf.d/apex` 不需要：apex/www 无证书，统一走 `00-default.conf` 的 80 兜底跳 Atlas。
- 执行顺序（关键，不能颠倒）：
  1. 仓库：先只改 `docker-compose.yml`（加挂载）并部署 → web 带挂载重建，配置仍旧，线上无变化。
  2. 用户放开安全组 `TCP 80/443`（人工，AI 不动）。
  3. 仓库：改三份 conf + 后台页脚 → `deploy.sh --with-nginx --with-admin-web` → `nginx -t` 失败自动回滚 `/srv/nginx.prev`。
  4. 写 `/srv/ops/state/exposure.json`（期望表），`status.sh` 自探语义同步。
  5. 24 小时后开 HSTS（单独一次 `--with-nginx` + reload）。
- 客户端：家长端 / TV 的 `API_BASE_URL` 改为 `https://quiz.openapp.asia/`（构建参数，本次只更新文档，线上包随下次发版）。

## 5. 数据模型

不涉及。仅新增 1 个 compose 挂载、1 个 conf 文件、若干 conf 内容；不改表结构与 Redis 键。

## 6. 接口设计

无新增业务接口。对外行为变化：

- `https://quiz.openapp.asia/v1/**` 公网可达（`X-Forwarded-Proto: https`）；
- 管理端 Cookie 的 `Secure` 由 `isSecureRequest`（识别 `X-Forwarded-Proto`）自动置位，无需改代码；
- 公网访问 `/v1/admin/**` 与后台 `/` 返回 403（nginx，不落到 Go）。

## 7. 非功能要求

- 性能：TLS 会话缓存 `shared:*_tls:10m`、`ssl_session_tickets off`；nginx 内存上限 64m 不变。
- 安全：TLS1.2/1.3；`server_tokens off` 沿用；未知 SNI 握手拒绝；限流 429 可观测（access log 有 429）。
- 可观测：`status.sh` 新增「TLS 证书」段（已由 Atlas 加好，读 `/srv/nginx/certs`）；`exposure.json` 定义期望；
  journal 记录 `nginx` 类事件。
- 合规：备案号页脚；公安联网备案 30 日内（用户，约 2026-12-23 前）。

## 8. 风险与对策

| 风险 | 影响 | 对策 |
| --- | --- | --- |
| 90 天证书到期忘换 | 全站 TLS 失效，家长端不可用 | `status.sh` 剩余天数（<14 天 ⏳）；2027-01 下旬登记待办；到期前 30 天在 ROADMAP 挂条目 |
| 证书文件被 `--with-nginx` 的 `--delete` 删掉 | 443 全挂 | 证书放 `/srv/nginx/certs`（不在 `conf.d/` 内）；`deploy.sh` 同步范围只含 `nginx.conf` 与 `conf.d/` |
| 挂载未就位就上 443 配置 | `nginx -t` 失败 / 容器起不来 | 严格分两次部署：先挂载、后配置；`deploy.sh` 有 `nginx -t` 失败回滚 |
| 后台公网暴露被撞库 | 口令泄露、后台被登录 | 12 位口令 + `qz_auth` 10r/m + Go 侧 5 次锁定；观察 401/429 突增；必要时收回 tailnet（改一行 allow/deny） |
| 限流误伤家长端正常使用 | 401/429 增多、体验差 | `qz_api` 600r/m + burst 120（单设备远低于此）；先观察 access log 再收紧 |
| `status.sh` 自探语义反转 | 误报「端口被放开」 | 同批写 `exposure.json`，并同步脚本判断（80/443 期望 allowed，8080-8082 期望 blocked） |
| 共享面改动影响 Atlas | 静态站 502/证书错 | `flock /srv/ops/locks/nginx.lock`；`deploy.sh --with-nginx` 自带备份与回滚；改完请 Atlas 跑自检 |
| 备案号写错 | 备案复核不通过 | 用 `粤ICP备2026058093号-2`（Atlas 已上线并核对）；QuizGo 页脚复用同一串 |

## 9. 未决问题

1. ~~管理面暴露程度~~ → 2026-11-24 用户选定 **A（公网可达 + 强口令 + 限流）**。
2. 安全组 `TCP 80/443` 由用户加回（AI 不动安全组）——**当前仍未放开**（2026-11-24 自探 `exit 28`）。
3. ~~HSTS 是否在观察 24 小时后开启~~ → 用户 2026-11-24 决定**立即开启**（`max-age=31536000`，不含 `preload` / `includeSubDomains`）。
6. `www.atlas.openapp.asia` / `www.quiz.openapp.asia` 用户决定**不加 DNS**（只有 `www.openapp.asia` 有记录）。
   证书 SAN 与 `server_name` 保留 www（将来加解析即可用），当前 `https://www.<子域>` 解析失败属预期。
4. 证书到期前（2027-02-21）的换证方式：继续阿里云免费 DV（手动）还是切 `acme.sh` 自动化 → 088 评估。
5. 公安联网备案：用户须在 2026-12-23 前提交。
