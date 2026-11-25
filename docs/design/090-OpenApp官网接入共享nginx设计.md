# 090-OpenApp 官网接入共享 nginx 设计

| 项目 | 内容 |
| --- | --- |
| 编号 | 090 |
| 状态 | approved（按用户 2026-11-25 决策落地，事后补档） |
| 作者 | AI |
| 创建日期 | 2026-11-25 |
| 最后更新 | 2026-11-25 |
| 对应计划 | [090-OpenApp官网接入共享nginx实施计划](../plans/090-OpenApp官网接入共享nginx实施计划.md) |

## 1. 背景与目标

- OpenApp 官网（Astro 静态站，仓库 `../OpenApp`）需要一个公网入口：
  `https://www.openapp.asia` 为规范域名，裸域 `openapp.asia` 301 到 www。
- 服务器 `quizgo-ecs` 上只有一套共享 nginx（`quizgo-web` 容器），同时服务
  `atlas.openapp.asia` 与 `quiz.openapp.asia`；**配置源在本仓库** `deploy/nginx/`，
  其它项目的改动最终都要落到这里，否则下次 `--with-nginx` 发布会把配置删掉。
- 域名 `openapp.asia` / `www.openapp.asia` 的 A 记录已存在，安全组 80/443 已于 086 放开。
- 目标（可验证）：
  1. `https://www.openapp.asia/` 返回 200，展示占位页 + ICP 页脚；
  2. `http://www.openapp.asia/`、`https://openapp.asia/` 均 301 到 `https://www.openapp.asia/`；
  3. `atlas` / `quiz` 两站不受影响，未知 SNI 仍拒绝握手；
  4. 官网产物可由 OpenApp 仓库独立发布，不需要重建 QuizGo 后端镜像。

不在范围：DNS 变更、新增公网端口、安装包下载限速、官网正式内容、公安备案号页脚。

## 2. 需求与验收标准

| 编号 | 需求 | 验收标准 |
| --- | --- | --- |
| R1 | 官网 443 可访问 | `curl -I https://www.openapp.asia/` → 200 |
| R2 | 裸域统一到 www | `curl -I https://openapp.asia/`、`http://www.openapp.asia/` → 301，Location 为 `https://www.openapp.asia/…` |
| R3 | 裸 IP / 未知 Host 兜底 | `curl -I http://8.163.122.39/` → 301 到官网（见第 5 节决策 D1） |
| R4 | 证书正确 | `openssl x509` 显示 SAN 含 `openapp.asia` 与 `www.openapp.asia`，到期 2027-02-23 |
| R5 | 现有站点不回退 | `https://atlas.openapp.asia/`、`https://quiz.openapp.asia/v1/meta` → 200 |
| R6 | 配置可被下次发布保留 | `www.conf` 在仓库内，`deploy.sh --with-nginx` 会同步并在 state 里登记 md5 |
| R7 | 404 可控 | 不存在的路径返回 404，展示站点自己的 404 页 |

## 3. 方案对比与选型

| 方案 | 做法 | 优点 | 缺点 |
| --- | --- | --- | --- |
| A. 共享现有 nginx（选中） | 在 `conf.d/` 增加 `www.conf`，web 容器多挂 `/srv/www:ro` | 复用 TLS 终止、限流、证书与运维脚本；零新增进程 | 共享面耦合，改配置需与 QuizGo 发布节奏协调 |
| B. 独立 nginx 容器 | 为新站单开容器占 80/443 | 隔离 | 80/443 已被共享 nginx 占用；多一套证书/日志/发布流程 |
| C. 托管静态站（OSS/CDN） | 官网放对象存储 | 不占 ECS | 需备案接入与额外费用；当前阶段过重 |

选 **A**：官网是纯静态站，体量与 atlas 同级，共享 nginx 最省资源，也符合 `/srv/ops` 共享面约定。

## 4. 总体方案

```
DNS: openapp.asia / www.openapp.asia → 8.163.122.39
            │
      共享 nginx（quizgo-web 容器）
            ├─ 00-default.conf  80 default → 301 https://www.openapp.asia
            ├─ www.conf         80  → 301 https://www.openapp.asia
            │                   443 → root /srv/www/openapp（证书 openapp.asia.pem）
            ├─ atlas.conf       80/443 atlas.openapp.asia（原样）
            └─ quiz.conf        80/443 quiz.openapp.asia（原样）
```

- **证书**：DigiCert DV，`/srv/nginx/certs/openapp.asia.{pem,key}`，SAN 含裸域与 www，
  `notAfter 2027-02-22 23:59:59 GMT`（= 2027-02-23 07:59 CST）。90 天一次人工续期。
- **站点根**：`/srv/www/openapp`，容器只读挂载 `/srv/www`。
- **ACME 例外**：80 段保留 `/.well-known/acme-challenge/` → `/srv/www/acme`，
  避免将来换证书时被 301 吃掉验证文件。
- **缓存**：`/_astro/` 一年 immutable、`/images/` 七天、HTML 不缓存。
- **安全头**：`X-Content-Type-Options`、`Referrer-Policy`、CSP；HSTS 暂缓（D2）。

## 5. 决策记录

| 编号 | 决策 | 理由 |
| --- | --- | --- |
| D1 | 80 `default_server`（裸 IP / 未知 Host）从 atlas 改跳官网 | 官网是品牌首页，比事件年表更适合做兜底入口；备案巡检访问裸 IP 也能看到主体信息与备案号。与 086 计划里「未知 Host 跳 atlas」的描述不同，以本设计为准 |
| D2 | 官网暂不开 HSTS | 首次上线用占位页，先观察一次证书续期再评估；避免证书异常时无法回退 http（见 `www.conf` 注释） |
| D3 | 暂不做 `/downloads/` 限速与 `/brand/`、`/media/` 缓存规则 | 当前只有占位页，无安装包；等正式站上线再按 OpenApp 请求文档补齐，避免先写死无效配置 |
| D4 | `www.conf` 不声明新的 `limit_req_zone` | `nginx.conf` 已有全局 gzip/限流；`conf.d` 里重复声明同名 zone 会报错 |
| D5 | 官网产物由 OpenApp 仓库 `rsync` 到 `/srv/www/openapp` | 产物与源码同仓，避免把 Astro 构建塞进 QuizGo 发布流程 |

## 6. 配置与数据模型

无数据库变更。配置面：

| 文件 | 变更 |
| --- | --- |
| `deploy/nginx/conf.d/www.conf` | 新增：80 跳转 + 443 静态站 + 裸域 301 |
| `deploy/nginx/conf.d/00-default.conf` | 80 `default_server` 移除 `openapp.asia`/`www`，兜底改跳官网 |
| `docker-compose.yml` | web 服务增加 `/srv/www:/srv/www:ro` |
| `scripts/deploy.sh` | `state/quizgo.json` 的 `nginxMd5` 由 3 项扩到 4 项（含 `www.conf`） |
| `AGENTS.md` 9.3 | 同步「四个文件都要记」 |

## 7. 非功能要求

- **可用性**：改配置只 reload，不重启 nginx；`nginx -t` 失败不 reload。
- **隔离**：`/srv/www` 只读挂载，官网进程无法写宿主机；官网 404 不回落到 atlas。
- **性能**：静态文件直出，无 upstream；总量与 atlas 同一量级，2C2G 足够。
- **安全**：TLS 1.2+、`server_tokens off`（全局）、未知 SNI `ssl_reject_handshake`；
  CSP 限制脚本为同源。
- **合规**：占位页页脚含 `粤ICP备2026058093号-2`；公安备案号出号后由 OpenApp 仓库补。

## 8. 风险与对策

| 风险 | 影响 | 对策 |
| --- | --- | --- |
| `00-default.conf` 与 `www.conf` 不同步 | nginx 报 conflicting server name，官网 80 段被忽略，仍跳 atlas | 两个文件必须同一次提交/同步；reload 前 `nginx -t` |
| 下次 `deploy.sh --with-nginx` 覆盖服务器配置 | 官网消失 | 配置源已入本仓库；state 快照已扩到 4 项 |
| 证书 90 天到期 | HSTS 下官网直接打不开 | `status.sh` 证书剩余天数；到期前 30 天续期（同 086 T12 节奏） |
| 公安备案审核期改动 80/443 | 审核风险 | 本次不新增端口、不改 DNS，只增加 `server_name`；审核期内不再动官网块 |
| 官网产物误删 | 404 | `/srv/www/openapp` 由 OpenApp 发布脚本 `rsync --delete` 维护，保留上一版备份 |

## 9. 未决问题

1. `status.sh` 增加官网三项探测（200 / 裸域 301 / 证书剩余天数）——服务器侧改动，待与 Atlas 协调后做。
2. 安装包下载的 `/downloads/` 限速（每 IP 2 连接、1500k）待正式站有下载入口时补。
3. HSTS 是否开启，等下一次证书续期成功后评估。
4. 公安备案号页脚由 OpenApp 仓库在出号后补，不在本仓库改。
5. 官网正式内容上线时间由 OpenApp 项目排期决定，与本次基础设施解耦。
