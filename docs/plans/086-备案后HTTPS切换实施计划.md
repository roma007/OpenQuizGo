# 086-备案后HTTPS切换实施计划

| 项目 | 内容 |
| --- | --- |
| 编号 | 086 |
| 状态 | review |
| 负责人 | AI |
| 创建日期 | 2026-11-23 |
| 最后更新 | 2026-11-23 |
| 对应设计 | [086-备案后HTTPS切换设计](../design/086-备案后HTTPS切换设计.md) |

## 1. 目标与范围

- 交付目标：`https://atlas.openapp.asia` / `https://quiz.openapp.asia` 公网可用，80 跳 443，证书自动续期，备案号上页面。
- 范围边界：只改共享 nginx 配置、证书链路、web 容器挂载、systemd 续期单元、管理端页脚与文档；不改业务代码逻辑、不动 `8081/8082`。
- 前置条件：
  1. 用户确认设计与本计划（状态改 `approved`）。
  2. 用户提供 **ICP 备案号** 与证书通知邮箱。
  3. 用户在控制台把安全组 `TCP 80`、`TCP 443` 加回（`0.0.0.0/0`）。
  4. 与 Atlas 就「80/443 启用、证书路径、页脚备案号」通过 `/srv/ops/requests/` 达成一致。

## 2. 任务拆分

| 序号 | 任务 | 依赖 | 产出物 | 验收标准 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T1 | 安全组加回 80/443 | 用户 | 控制台规则 | 公网 80/443 由超时变 RST（`nc` 验证） | todo |
| T2 | 写 Atlas 请求并达成一致 | — | `/srv/ops/requests/…` | Atlas 回 `done`，确认证书/SAN/页脚分工 | todo |
| T3 | 安装 acme.sh、建目录与权限 | T1 | `/srv/certs`、`/srv/acme`、acme.sh | `acme.sh --version` 正常；目录权限 700/644/600 | todo |
| T4 | compose 加只读挂载 + 「仅 80」配置，部署 | T3 | `docker-compose.yml`、`deploy/nginx/conf.d/*` | `nginx -t` 通过；`/ .well-known/acme-challenge/x` 返回 404；80 上其它路径 301/444 | todo |
| T5 | 签发证书并安装 | T4 | `/srv/certs/fullchain.pem`、`privkey.pem` | 4 个 SAN 齐全；`openssl x509 -noout -dates` 有效 | todo |
| T6 | 启用 443 配置，`--with-nginx` 部署 | T5 | 最终 `conf.d/*` | 两域名 200；80 301；未知 Host 444；HSTS 存在 | todo |
| T7 | 管理端页脚加备案号并构建部署 | T6 | `admin-web/src/App.tsx`、`dist` | 登录页与内页底部可见备案号并可点击 | todo |
| T8 | 续期 timer 落地 | T5 | `scripts/renew-cert.sh`、`deploy/systemd/quizgo-cert-renew.*` | timer `enabled+active`；`--cron` 空跑成功 | todo |
| T9 | 全链路验收 | T6/T7/T8 | 验收记录 | 见第 4 节清单全绿 | todo |
| T10 | 文档、journal、共享面登记收尾 | T9 | `deployment-todo`、ROADMAP、`state/quizgo.json` | 无漂移；journal 有 deploy/note | todo |
| T11 | 22 规则处置（独立） | 用户 VNC 验证 | 控制台 | 删后 `ssh quizgo-pub` 超时、`ssh quizgo` 正常 | todo |

## 3. 里程碑

| 里程碑 | 包含任务 | 完成标准 | 计划日期 | 实际日期 |
| --- | --- | --- | --- | --- |
| M1 证书就绪 | T1–T5 | 证书文件存在且 SAN 正确 | 2026-11-24 | |
| M2 公网 HTTPS | T6–T7 | 两站点 https 200，备案号可见 | 2026-11-24 | |
| M3 自动化与收尾 | T8–T10 | 续期 timer 生效，文档/登记闭环 | 2026-11-25 | |
| M4 公网 22 退役 | T11 | 仅 Tailscale 可 SSH | 2026-11-26 | |

## 4. 验证方式

- 单元/集成：`make test`（业务代码未改，作为回归基线）；`nginx -t`。
- 手工验收（逐条打勾）：
  1. `curl -I https://atlas.openapp.asia/` → `200`，`Strict-Transport-Security` 存在。
  2. `curl -I https://quiz.openapp.asia/` → `200`，CSP/X-Frame-Options 仍在。
  3. `curl -s https://quiz.openapp.asia/v1/meta` → 版本号与 `VERSION` 一致。
  4. `curl -I http://quiz.openapp.asia/v1/meta` → `301` 到 https。
  5. `curl -sI -H 'Host: evil.example' http://8.163.122.39/` → `444`（或直接断开）。
  6. `openssl s_client -connect quiz.openapp.asia:443 -servername quiz.openapp.asia` → 链完整、`TLSv1.3`。
  7. `openssl s_client -tls1_1 …` → 握手失败。
  8. SPA：直接访问 `https://quiz.openapp.asia/families` 刷新 → 200（回退 index.html）。
  9. `curl -sI https://quiz.openapp.asia/assets/<hash>.js` → `Cache-Control: immutable`。
  10. `curl -o /dev/null -w '%{http_code}' https://quiz.openapp.asia/metrics` → `403`。
  11. 页脚备案号可见、链接正确（两个站点）。
  12. `systemctl list-timers | grep cert`；`renew-cert.sh --dry-run` 成功。
  13. `status.sh` 无漂移；自探规则已按新语义更新。
- 端到端：家长端/TV 以 `--dart-define=API_BASE_URL=https://quiz.openapp.asia/` 构建并登录一次。

## 5. 回滚方案

| 场景 | 动作 | 预计耗时 |
| --- | --- | --- |
| 443 配置有误 | `deploy.sh` 的 `nginx -t` 失败自动回滚 `/srv/nginx.prev`；或手工 `rsync -a --delete /srv/nginx.prev/ /srv/nginx/` + reload | < 1 分钟 |
| 证书不可用 | 保持 80 配置 + `8081/8082`，公网退回不可用（与备案前一致） | < 2 分钟 |
| 安全组设置错误 | 控制台删 80/443，回到当前状态 | < 1 分钟 |
| 备案号写错 | 只改前端文案并重新构建，不影响服务端 | < 5 分钟 |

- 数据影响：无迁移、无表结构变化，不需要回库。
- 证书私钥：`/srv/certs` 不进仓库；备份时与数据库分开加密存放。

## 6. 决策记录

| 时间 | 决策 | 理由 |
| --- | --- | --- |
| 2026-11-23 | 选 `acme.sh` + webroot，单证书 4 SAN | ALinux 4 无 certbot 包；单证书减少续期点 |
| 2026-11-23 | 保留 `8081/8082` | `deploy.sh` 自检与 Atlas 自动化仍依赖；退役另立事项 |
| 2026-11-23 | 首页裸域 `openapp.asia` 301 到 `atlas` | 备案主体是「开放应用记录」，静态站是唯一内容站 |
