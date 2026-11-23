# 086-备案通过后的 HTTPS 切换设计

| 项目 | 内容 |
| --- | --- |
| 编号 | 086 |
| 状态 | review |
| 作者 | AI |
| 创建日期 | 2026-11-23 |
| 最后更新 | 2026-11-23 |
| 对应计划 | [086-备案后HTTPS切换实施计划](../plans/086-备案后HTTPS切换实施计划.md) |

## 1. 背景与目标

- 现状与问题：
  - ICP 备案已通过（2026-11-23 用户告知），`openapp.asia` 及其 `www` / `atlas` / `quiz` 子域均已解析到 `8.163.122.39`。
  - 安全组 `TCP 80/443` 在备案前被删除，目前公网对 80/443 是超时（`status.sh` 自探 `exit 28`）。
  - 共享 nginx 只监听 `0.0.0.0:8081`（Atlas）/ `0.0.0.0:8082`（QuizGo 后台 + `/v1`），且 `allow 127.0.0.1 + 100.64.0.0/10; deny all`，公网不可达。
  - 全站无证书，`http://quizgo-ecs:8082` 只对 tailnet 可用；家长端/TV 无法在公网使用。
  - 页脚无备案号（ICP 合规要求）；公安联网备案未办（ICP 通过后 30 日内）。
- 本次目标（可验证）：
  1. `https://atlas.openapp.asia`、`https://quiz.openapp.asia` 公网可访问，证书有效、链完整。
  2. `http` 全量 301 到 `https`（ACME 校验路径除外），未知 Host 返回 444。
  3. 证书自动续期，续期失败可观测；`/v1/meta`、后台 SPA、`/healthz` 全部 200。
  4. 两个站点页脚展示 ICP 备案号，链接 `https://beian.miit.gov.cn`。
  5. 安全基线不回退：HSTS、TLS1.2+、`server_tokens off`、`/metrics` 仍 403、备份权限仍 600/700。
- 非目标（明确不做什么）：
  - 不引入 CDN / WAF / 负载均衡；不改业务代码逻辑与接口契约。
  - 不退役 `8081/8082`（保留作 tailnet 调试与 `deploy.sh` 的 `BASE_URL` 自检），退役另立事项。
  - 不做 IPv6、HTTP/3、OCSP stapling、双向 TLS。
  - 不在本次处理公安备案的页面展示细节（只预留位置与文案）。

## 2. 需求与验收标准

| 编号 | 需求 | 验收标准 | 优先级 |
| --- | --- | --- | --- |
| R1 | 公网 HTTPS 可用 | 两域名 `curl -I https://…` 返回 200，`openssl s_client` 验证链到 Let's Encrypt 根 | P0 |
| R2 | 80 跳 443 | `curl -I http://…` 返回 301 且 Location 为 https；`/.well-known/acme-challenge/` 不被跳转 | P0 |
| R3 | 未知 Host 不泄露 | `curl -H 'Host: x' http://8.163.122.39/` 返回 444；直连 IP 443 同样 444 | P0 |
| R4 | 证书自动续期 | 续期 timer `enabled+active`，`--cron` 空跑成功，续期后 nginx 自动 reload | P0 |
| R5 | 备案号展示 | Atlas 首页与后台登录页底部可见备案号，链接指向工信部 | P0 |
| R6 | 原有功能不回归 | `/v1/meta`、登录、SPA 深层路由刷新、`/assets` 长缓存、`/healthz` 均正常 | P0 |
| R7 | 安全基线 | HSTS 存在；TLS1.0/1.1 握手失败；`/metrics` 403；`server_tokens` 隐藏 | P1 |
| R8 | 可回滚 | `/srv/nginx.prev` 可在 1 分钟内恢复；证书故障时能退回 8081/8082 | P1 |

## 3. 方案对比与选型

### 3.1 ACME 客户端

| 方案 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- |
| A `acme.sh` | 单脚本无依赖（`dnf` 无 certbot）；webroot/standalone 都支持；自带续期 | 安装脚本来自网络，需校验 | ✅ 选用 |
| B `certbot` | 生态成熟 | ALinux 4 默认源无包，需 EPEL/pip，2C2G 上多一层依赖 | 否 |
| C 阿里云免费证书 | 控制台操作简单 | 3 个月手动续期，易忘；无法自动化 | 否 |

### 3.2 TLS 终止位置

共享 nginx 是唯一前端容器（`network_mode: host`），两个站点同源。**在共享 nginx 终止 TLS**，证书由 QuizGo 侧统一管理，Atlas 只消费文件——与 CONTRACT 2.1（`atlas.conf` 源在 OpenQuizGo 仓库）一致，避免两套证书与两套续期。

### 3.3 证书粒度

单张证书覆盖 4 个名字（`openapp.asia`、`www`、`atlas`、`quiz`）：一次签发、一次续期、一处 reload。缺点是一处失败全站受影响，但 4 个名字同属一个账号，风险等同。

## 4. 总体架构

- 目录与权限：
  - `/srv/certs/`：`fullchain.pem`(644)、`privkey.pem`(600, root)，挂载进容器 `ro` → `/etc/nginx/certs`。
  - `/srv/acme/`：HTTP-01 校验目录，挂载 `ro` → `/srv/acme`。
  - acme.sh 家目录在 `/home/ecs-user/.acme.sh`（仅续期用，不挂载）。
- nginx 结构（全部在仓库 `deploy/nginx/conf.d/`，由 `--with-nginx` 同步）：
  - `00-default.conf`：80/443 `default_server`，仅放行 ACME 路径，其余 `return 444`。
  - `atlas.conf`：80 跳转 + 443 静态站（公开，去掉 tailnet allowlist）。
  - `quiz.conf`：80 跳转 + 443 后台 SPA + `/v1` 反代 + `/healthz`（公开）。
  - `apex.conf`：`openapp.asia` / `www` 301 到 `https://atlas.openapp.asia`。
  - `8081/8082`：维持 `0.0.0.0` + allowlist 不变（`deploy.sh` 的 `BASE_URL` 仍用 `127.0.0.1:8082`）。
- 关键流程（首次签发）：
  1. 安全组开 80/443（人工）。
  2. 部署「仅 80」配置，验证 `/.well-known/acme-challenge/` 返回 404（目录存在、路由正确）。
  3. `acme.sh --issue --webroot /srv/acme -d …`（4 个 `-d`）。
  4. `--install-cert` 写入 `/srv/certs` 并 `docker exec quizgo-web-1 nginx -s reload`。
  5. 部署 443 配置；`nginx -t` 失败由 `deploy.sh` 自动回滚 `/srv/nginx.prev`。
- 续期：`quizgo-cert-renew.timer`（每日 04:10 + 随机延迟）→ `scripts/renew-cert.sh` → `acme.sh --cron` → 仅在证书变化时 reload。
- 目录结构影响：`docker-compose.yml` web 服务新增两个只读挂载；`deploy/systemd/` 新增 renew 单元；`scripts/` 新增 `renew-cert.sh`。

## 5. 数据模型

不涉及。仅新增两个主机目录，不写数据库、不新增表/字段。

## 6. 接口设计

无新增业务接口。对外行为变化：

- 新增 `https://quiz.openapp.asia/v1/**`（与 8082 同一后端，`X-Forwarded-Proto: https`）。
- 管理端 Cookie 的 `Secure` 由 `isSecureRequest`（识别 `X-Forwarded-Proto`）自动置位，**无需改代码**。
- 家长端/TV 改用 `API_BASE_URL=https://quiz.openapp.asia/`（构建参数，见 README）。

## 7. 非功能要求

- 性能：TLS 会话缓存 10MB；gzip 不变；2C2G 上 nginx 内存上限 64m 不变。
- 安全：TLS1.2/1.3、`ssl_session_tickets off`、HSTS 1 年（暂不含 `preload`）、未知 Host 444、管理端登录失败已有应用层限流（3 次）。
- 可观测：续期失败写 journal；`status.sh` 公网自探保持不变（此后 80/443 应为「放行」而非超时，需同步判断规则）。
- 合规：页脚备案号；公安备案 30 日内办理。

## 8. 风险与对策

| 风险 | 影响 | 对策 |
| --- | --- | --- |
| 80/443 放开后管理后台对公网暴露 | 暴力破解、扫描 | 应用层登录限流 + 强口令（已改）；CSP/HSTS；必要时后续加 nginx `limit_req` |
| 证书私钥丢失/泄露 | 全站 TLS 失效 | 私钥 600 root；不进仓库；`/srv/certs` 加入备份并加密 |
| ACME 速率限制（5 次/周/域名组） | 重复签发被锁 | 先 dry-run；失败只重试一次；不 `--force` 反复签发 |
| 首次签发与 nginx 配置顺序错位 | 443 无证书启动失败 | 分两次部署（先 80 后 443）；`nginx -t` 失败自动回滚 |
| 共享面改动影响 Atlas | 静态站 502/证书错 | 走 `--with-nginx` + 锁；改完请 Atlas 跑四条自检；`/srv/nginx.prev` 兜底 |
| `status.sh` 自探语义反转 | 误报「安全组被放开」 | 同批更新判断规则：80/443 从「应为超时」改为「应为 RST」，并保留 8080-8082 超时 |
| 备案号写错/漏链 | 备案复核不通过 | 以用户提供的完整备案号为准，页面加 `target=_blank` 链接；Atlas 侧由其自行修改 |

## 9. 未决问题

1. ICP 备案号完整文本（形如 `粤ICP备2026xxxxxx号-1`）——需用户提供。
2. 证书到期通知邮箱（Let's Encrypt 账户）——需用户提供或指定 `admin@openapp.asia`。
3. `www.openapp.asia` 是否保留并跳 Atlas（默认保留）。
4. `8081/8082` 退役时间（默认本次不动）。
5. 公安备案：ICP 通过后 30 日内，需用户提交（`beian.mps.gov.cn`）。
