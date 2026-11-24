# 086-备案后HTTPS切换实施计划

| 项目 | 内容 |
| --- | --- |
| 编号 | 086 |
| 状态 | in-progress |
| 负责人 | AI |
| 创建日期 | 2026-11-23 |
| 最后更新 | 2026-11-24（v3：管理面公网可达，T3 已开工） |
| 对应设计 | [086-备案后HTTPS切换设计](../design/086-备案后HTTPS切换设计.md) |

## 1. 目标与范围

- 交付目标：`https://atlas.openapp.asia` / `https://quiz.openapp.asia` 公网可用；80 跳 443；
  管理面（后台 SPA 与 `/v1/admin/`）收进 tailnet；家长端 API 公网可用并有限流；备案号上页脚。
- 范围边界：`docker-compose.yml`（web 挂载）、`deploy/nginx/**`、`admin-web/src/App.tsx`（页脚）、
  `scripts/deploy.sh` 自检地址（如需）、`/srv/ops/state/exposure.json`、文档；**不改业务代码逻辑**，不动 `8081/8082`。
- 前置条件：
  1. 用户确认设计与本计划（状态改 `approved`）。
  2. ~~用户确认管理面暴露程度~~ → 已定 **方案 A（公网可达 + 强口令 + 限流）**（2026-11-24）。
  3. 用户在控制台把安全组 `TCP 80`、`TCP 443` 加回（`0.0.0.0/0`，人工，AI 不动）。
  4. 证书已到位：`/srv/nginx/certs/{atlas,quiz}.openapp.asia.{pem,key}`（已满足，2026-11-23）。
  5. 与 Atlas 通过 `/srv/ops/requests/` 对齐分工（Atlas 已发请求，QuizGo 回执即可）。

## 2. 任务拆分

| 序号 | 任务 | 依赖 | 产出物 | 验收标准 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T1 | 回执 Atlas 两条请求（改密、443） | — | `/srv/ops/requests/*.md` 的 `## 回复` | 两条 `status: done`，分工与证书归属写清 | done |
| T2 | 安全组加回 80/443 | 用户 | 控制台规则 | 公网 80/443 由超时变 RST；`status.sh` 自探变化 | done |
| T3 | compose 加只读证书挂载并部署（不动 conf） | — | `docker-compose.yml` | web 重建后 `docker inspect` 能看到 `/etc/nginx/certs` 挂载；线上行为无变化 | done |
| T4 | 三份 conf + `nginx.conf` 限流区 | T3 | `deploy/nginx/**` | `nginx -t` 通过（`deploy.sh` 失败自动回滚 `/srv/nginx.prev`） | done |
| T5 | `--with-nginx` 部署启用 80/443 | T2/T4 | 生效配置 | 本机 `--resolve` 全部通过（2026-11-24）；公网开放后复验 | done |
| T6 | 管理端页脚备案号 | T5 | `admin-web/src/App.tsx` + `dist` | 页脚可见 `粤ICP备2026058093号-2` 且可点击（后台公网可见，必需） | done |
| T7 | 写 `exposure.json` + 同步 `status.sh` 判据 | T5 | `/srv/ops/state/exposure.json` | 自探 5 行符合期望，无 ⚠️ | done |
| T8 | 全链路验收（含限流与管理面 403） | T5–T7 | 验收记录 | 第 4 节清单全绿 | done |
| T9 | 24 小时后开 HSTS | T8 | `deploy/nginx/conf.d/*` | 两站点响应头含 `Strict-Transport-Security` | 待执行（2026-11-25 之后） |
| T10 | 文档 / journal / 共享面登记收尾 | T8 | `deployment-todo`、ROADMAP、`state/quizgo.json` | 无漂移；journal 有 deploy/nginx 行；证书到期日入待办 | todo |
| T11 | 22 规则处置（独立事项） | 用户 VNC 验证 | 控制台 | 删后 `ssh quizgo-pub` 超时、`ssh quizgo` 正常 | todo |

## 3. 里程碑

| 里程碑 | 包含任务 | 完成标准 | 计划日期 | 实际日期 |
| --- | --- | --- | --- | --- |
| M1 前置就绪 | T1–T3 | 挂载就位、安全组已开、Atlas 已回执 | 2026-11-24 | 2026-11-24 |
| M2 公网 HTTPS | T4–T6 | 两站点 https 200，管理面可达且限流生效，备案号可见 | 2026-11-24 | 2026-11-24 |
| M3 观测与收尾 | T7–T10 | 自探全绿、HSTS 开启、文档闭环 | 2026-11-25 | 进行中（HSTS 待 24h 观察） |
| M4 公网 22 退役 | T11 | 仅 Tailscale 可 SSH | 待 VNC 验证 | |

## 4. 验证方式

- 回归：`make test`（业务代码未改，作基线）；`nginx -t`；`docker compose ps` 全 healthy。
- 手工验收（逐条打勾，公网侧由 Mac 执行，管理面侧在 tailnet 内执行）：
  1. `curl -I https://atlas.openapp.asia/` → `200`。
  2. `curl -I http://atlas.openapp.asia/` → `301` → https。
  3. `curl -sI -H 'Host: x' http://8.163.122.39/` → `301` 到 `https://atlas.openapp.asia`。
  4. `curl -k -I https://8.163.122.39/` → 握手失败（`ssl_reject_handshake`）。
  5. `curl -s https://quiz.openapp.asia/v1/meta` → 版本号与 `VERSION` 一致。
  6. `curl -o /dev/null -w '%{http_code}' -X POST https://quiz.openapp.asia/v1/admin/auth/login -d '{}'` → `401/422`（可达，不是 403）。
  7. `curl -o /dev/null -w '%{http_code}' https://quiz.openapp.asia/` → `200`（公网，后台公开但鉴权在 API）。
  8. `curl -s -o /dev/null -w '%{http_code}' -X POST https://quiz.openapp.asia/v1/auth/login -d '{}'` → `401/422`（可达）。
  9. 连打 8 次 `/v1/admin/auth/login`（公网）→ 出现 `429`（nginx `qz_auth` 生效）。
  10. `openssl s_client -connect quiz.openapp.asia:443 -servername quiz.openapp.asia -verify_return_error` → 链到 DigiCert 根。
  11. `openssl s_client -tls1_1 …` → 握手失败。
  12. SPA 深层路由刷新 `https://quiz.openapp.asia/questions` → 200；页脚备案号可见。
  13. `/assets/<hash>.js` → `Cache-Control: immutable`；`/metrics` → 403。
  14. 页脚备案号可见、链接 `https://beian.miit.gov.cn`。
  15. `ssh quizgo '/srv/ops/bin/status.sh'` → 共享面无漂移、公网自探全符合 `exposure.json`。
  16. `8081/8082` 从 tailnet 仍可访问（兜底通道未断）。
- 客户端：家长端 / TV 以 `--dart-define=API_BASE_URL=https://quiz.openapp.asia/` 构建并登录一次（随下次客户端发版验证）。

## 5. 回滚方案

| 场景 | 动作 | 预计耗时 |
| --- | --- | --- |
| 443 配置有误 | `deploy.sh` 的 `nginx -t` 失败自动回滚 `/srv/nginx.prev`；或手工 `rsync -a --delete /srv/nginx.prev/ /srv/nginx/` + reload | < 1 分钟 |
| 证书不可用/过期 | 保持 80 兜底配置 + `8081/8082`；公网回到不可用（与备案前一致） | < 2 分钟 |
| 限流误伤 | 去掉对应 `limit_req` 行并 `--with-nginx` | < 3 分钟 |
| 公网撞库告警 | 临时把 `location /v1/admin/` 加 `allow 127.0.0.1; allow 100.64.0.0/10; deny all;` 收回 tailnet | < 3 分钟 |
| 安全组设置错误 | 控制台删 80/443，回到当前状态 | < 1 分钟 |

- 数据影响：无迁移、无表结构变化，不需要回库。
- 证书私钥：仅存在 `/srv/nginx/certs`（700/600）；不进仓库、不进备份包、不写进对话归档。

## 6. 决策记录

| 时间 | 决策 | 理由 |
| --- | --- | --- |
| 2026-11-23 | v1 选 `acme.sh` + webroot，单证书 4 SAN | ALinux 4 无 certbot 包；单证书减少续期点 |
| 2026-11-23 | **v2 取消 acme.sh，改用 Atlas 已采购的 2 张 DigiCert DV 证书** | 证书已到位（¥0、SAN 含 www、私钥不出机器）；避免为 90 天引入额外链路。代价是手动换证，用 `status.sh` 剩余天数 + 待办兜底 |
| 2026-11-24 | **管理面改回公网可达（方案 A）**，靠 12 位口令 + `qz_auth` 10r/m + 应用层锁定 | 用户 2026-11-24 明确要求任意设备可登录；收回 tailnet 的代价（必须装 Tailscale）他不能接受。已把「必要时一行改回 tailnet」写进回滚 |
| 2026-11-23 | 保留 `8081/8082` | `deploy.sh` 自检与排障仍依赖；退役另立事项 |
| 2026-11-23 | 裸域 `openapp.asia`/`www` 80 兜底 301 到 `https://atlas.openapp.asia` | apex 无证书；备案巡检直接访问裸域名，需给出可读页面 |
| 2026-11-23 | HSTS 延后 24 小时 | 避免配置错时把自己锁死；先观察无混内容 |
| 2026-11-23 | 分两次部署（先挂载、后配置） | 新配置引用 `/etc/nginx/certs`，挂载不存在则 `nginx -t` 必失败 |

## 7. 实际进展（2026-11-24）

| 任务 | 结果 |
| --- | --- |
| T1 | 已回执：改密请求置 `done`；443 请求留 `open`（等安全组 + 方案确认），并写明「采纳你的配置，但证书用已到位的、不用 ACME」 |
| T3 | ✅ `docker-compose.yml` 加 `/srv/nginx/certs:/etc/nginx/certs:ro`；容器能看到 4 个证书文件；`nginx -t` 通过 |
| T4 | ✅ 三份 conf + `nginx.conf` 限流区；用 `nginx:1.27-alpine` 在临时目录试装通过（`ssl_reject_handshake` 不需要证书，已实测） |
| T5 | ✅ 已启用（`1.0.51`）：`0.0.0.0:80/443` 在监听，本机 `--resolve` 全绿（见第 4 节前 13 项） |
| T6 | ✅ 后台页脚（登录页 + 内页）展示 `粤ICP备2026058093号-2`，链 `beian.miit.gov.cn`，`1.0.52` 部署 |
| T2 | ✅ 用户 2026-11-24 08:1x 放开安全组；公网 80/443 由 `exit 28` 变可访问，8080-8082 仍 blocked |
| T7 | ✅ 已写 `/srv/ops/state/exposure.json`（80/443 allowed、8080-8082 blocked），`status.sh` 5/5 ✅ |
| T8 | ✅ 公网侧全绿（见第 8 节） |
| T9–T10 | 待办：24h 后开 HSTS；ROADMAP/README 与 085 文档同批提交 |

### 踩到的坑（已修，记在 `scripts/deploy.sh` 注释里）

`/srv/nginx/nginx.conf` 是**单文件 bind 挂载**，`rsync` 默认「写临时文件 + rename」会换 inode，
容器仍指向旧 inode → 出现「`conf.d` 已更新、`nginx.conf` 未更新」的半更新状态。
第一次 `--with-nginx` 就撞上了：`limit_req` 引用的 `qz_auth` 在容器里不存在，
报 `zero size shared memory zone "qz_auth"`，`deploy.sh` 按预期自动回滚，线上没受影响。

修正：
1. 同步改用 `rsync --inplace`（保留 inode），回滚路径同样加 `--inplace`；
2. 同步后**断言宿主与容器内 `nginx.conf` 的 md5 一致**，不一致立即中止并说明原因；
3. 已经错过一次的文件需要 `docker compose up -d --force-recreate web` 重新绑定挂载，之后 `--inplace` 就稳了（本次已做）。

## 8. 公网验收结果（2026-11-24 08:2x，安全组放开后）

从开发机走真实公网执行，全部通过：

| 项 | 结果 |
| --- | --- |
| `https://atlas.openapp.asia/` | 200，页脚 `粤ICP备2026058093号-2`，gzip 生效 |
| `http://atlas/`、`http://quiz/`、`http://openapp.asia/`、`http://8.163.122.39/` | 均 301，Location 正确 |
| `Host: evil.example`（80） | 301 → `https://atlas.openapp.asia` |
| `https://8.163.122.39/` | 握手被拒，无证书下发 |
| `https://quiz.openapp.asia/v1/meta` | 200，`version=1.0.52` |
| `/`、`/questions`、`/healthz` | 200 / 200 / 200 |
| `/metrics` | 403 |
| `POST /v1/admin/auth/login`（错口令） | 401（可达，鉴权在应用层） |
| `POST /v1/auth/login`（空参数） | 422 |
| 静态资源 / `sw.js` | `immutable` / `no-cache` |
| 证书链（不加 `-k`） | 200，`CN=quiz.openapp.asia`，`notAfter 2027-02-20` |
| TLS1.1 | alert 70 拒绝（服务器侧 OpenSSL 3 权威验证；macOS 客户端输出格式不同，早期误报过一次） |
| 8080/8081/8082 公网 | 全部超时（安全组未放行） |
| 限流 | 连打 8 次 → `401×5 + 429×3` |
| `status.sh` | 公网自探 5/5 ✅、证书剩 88 天、共享面 md5 无漂移 |

### 已知小缺口：`www.atlas` / `www.quiz` 无 DNS 记录

证书 SAN 与 `server_name` 都写入了 www，但阿里云 DNS 只有 `www.openapp.asia`，没有两个子域的 www 记录，
所以 `https://www.atlas.openapp.asia` / `https://www.quiz.openapp.asia` 解析失败（`000`）。
已告知 Atlas：要么加两条 A 记录，要么在文档里标注「SAN 含 www、DNS 未启用」。主域名不受影响。
