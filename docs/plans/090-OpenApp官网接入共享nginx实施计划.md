# 090-OpenApp 官网接入共享 nginx 实施计划

| 项目 | 内容 |
| --- | --- |
| 编号 | 090 |
| 状态 | in-progress（基础设施已上线，待正式站与 status 探测） |
| 负责人 | AI |
| 创建日期 | 2026-11-25 |
| 最后更新 | 2026-11-25 |
| 对应设计 | [090-OpenApp官网接入共享nginx设计](../design/090-OpenApp官网接入共享nginx设计.md) |
| 上游请求 | `../OpenApp/docs/plans/2026-11-25-官网基础设施变更请求.md`（C1–C6） |

## 1. 目标与范围

- 目标：`https://www.openapp.asia` 提供官网静态站（当前占位页），裸域 301 到 www；
  atlas / quiz 两站与共享 nginx 的安全基线不变。
- 范围：`deploy/nginx/conf.d/{www.conf,00-default.conf}`、`docker-compose.yml`、
  `scripts/deploy.sh`（state 快照）、`AGENTS.md` 共享面登记说明、文档。
- 不含：DNS、安全组、新增端口、官网正式内容、安装包下载、公安备案号。

## 2. 任务拆分

| 序号 | 任务 | 依赖 | 验收标准 | 状态 |
| --- | --- | --- | --- | --- |
| T1 | 证书就位：`openapp.asia` 双 SAN | — | `/srv/nginx/certs/openapp.asia.{pem,key}` 存在，SAN 含裸域与 www，私钥匹配 | done（2026-11-25） |
| T2 | 新建 `/srv/www/openapp` 与 `/srv/www/acme`，部署占位页 | T1 | 占位页含 ICP 备案号；目录属主 `ecs-user` | done（2026-11-25） |
| T3 | 新增 `deploy/nginx/conf.d/www.conf` | T1 | 80 跳转、443 静态站、裸域 301、404、CSP 齐全 | done（2026-11-25） |
| T4 | 修改 `00-default.conf`：80 兜底改跳官网 | T3 | 不再声明 `openapp.asia`/`www`，避免与 www.conf 冲突 | done（2026-11-25） |
| T5 | `docker-compose.yml` 挂载 `/srv/www:ro` 并重建 web | T2/T3 | 容器内 `ls /srv/www/openapp` 可见；atlas/quiz 无中断 | done（2026-11-25） |
| T6 | 线上同步配置 + `nginx -t` + reload | T4/T5 | `nginx -t` 通过；www 200、裸域 301、未知 SNI 拒绝 | done（2026-11-25 20:25） |
| T7 | `deploy.sh` 的 `nginxMd5` 扩到 4 项 | — | 下次发布 `state/quizgo.json` 含 `conf.d/www.conf` md5 | done（本次提交） |
| T8 | `AGENTS.md` 9.3 同步四文件登记 | T7 | 文档与脚本一致 | done（本次提交） |
| T9 | 设计 / 计划 / ROADMAP / 部署待办补档 | T6 | 文档可追溯，状态与线上一致 | done（本次提交） |
| T10 | `/srv/ops/bin/status.sh` 增加官网三项探测 | T6 | 200 / 裸域 301 / 证书剩余天数进状态快照 | todo（服务器侧，需与 Atlas 协调） |
| T11 | 正式站产物上线 + `/downloads/` 限速 | OpenApp 排期 | 首页无「建设中」；下载 206、并发 3 连接回 429 | todo（OpenApp） |
| T12 | HSTS 评估 | 一次证书续期后 | 决定是否加 `max-age` 或继续延后 | todo |

## 3. 依赖与顺序

T1 → T2/T3/T4（可并行）→ T5 → T6 → T7/T8/T9；T10 独立；T11/T12 后续。

## 4. 实施命令（已执行）

```bash
# 线上（2026-11-25）
ssh quizgo 'sudo mkdir -p /srv/www/openapp /srv/www/acme && sudo chown -R ecs-user:ecs-user /srv/www'
rsync -az deploy/nginx/conf.d/ quizgo:/srv/nginx/conf.d/       # 经备份后同步
ssh quizgo 'cd /srv/quizgo && docker compose up -d web'        # 使 /srv/www 挂载生效
ssh quizgo 'docker exec quizgo-web-1 nginx -t && docker exec quizgo-web-1 nginx -s reload'

# 占位页
rsync -az ../OpenApp/deploy/placeholder/ quizgo:/srv/www/openapp/
```

## 5. 验证记录（2026-11-25）

| 检查 | 结果 |
| --- | --- |
| `nginx -t` | `syntax is ok` / `test is successful` |
| `https://www.openapp.asia/` | 200，占位页，页脚 `粤ICP备2026058093号-2` |
| `https://openapp.asia/` | 301 → `https://www.openapp.asia/` |
| `http://www.openapp.asia/` | 301 → `https://www.openapp.asia/` |
| `http://8.163.122.39/` | 301 → `https://www.openapp.asia/`（D1） |
| `https://atlas.openapp.asia/` | 200，未受影响 |
| `https://quiz.openapp.asia/v1/meta` | 200，未受影响 |
| 证书 | `CN=openapp.asia`，SAN `openapp.asia, www.openapp.asia`，`notAfter 2027-02-22 23:59:59 GMT` |
| 404 | `https://www.openapp.asia/nope` → 404 |
| `/srv/nginx` 与仓库 | `www.conf`、`00-default.conf` 内容一致（本次提交后随 `--with-nginx` 保持） |

## 6. 里程碑

| 里程碑 | 任务 | 验收 | 计划时间 | 实际 |
| --- | --- | --- | --- | --- |
| M1 证书与目录就绪 | T1/T2 | 证书可读、占位页可放 | 2026-11-25 | 2026-11-25 |
| M2 官网 443 上线 | T3–T6 | 四条 curl 检查通过 | 2026-11-25 | 2026-11-25 |
| M3 配置纳入发布流程 | T7–T9 | 提交后 state 快照含 4 项 | 2026-11-25 | 2026-11-25（本次提交） |
| M4 运维可观测 | T10 | `status.sh` 出现官网三项 | 待排期 | — |
| M5 正式站 | T11/T12 | 正式内容 + 下载限速 + HSTS 决策 | OpenApp 排期 | — |

## 7. 回滚方案

1. 从仓库恢复 `00-default.conf`、删除 `www.conf`，`rsync` 后 `nginx -t && reload`；
   最坏情况从 `/srv/nginx.prev` 整目录恢复（`deploy.sh --with-nginx` 自动备份）。
2. `/srv/www` 挂载与目录可保留，对 atlas / quiz 无副作用。
3. 证书可保留；官网下线不影响其它站点。
4. 回滚后裸 IP / 未知 Host 兜底会回到 atlas（与 086 文档一致）。

## 8. 偏差与决策

- 与上游请求文档的差异：兜底改跳官网而非 atlas（D1）；暂缓 HSTS（D2）；
  暂无 `/downloads/` 限速（D3）；缓存规则按实际目录写为 `/images/` 而非 `/brand/`、`/media/`。
- 上游 `C4`（`status.sh`）与 `C6`（正式产物）未在本次完成，保留为 T10/T11。
- 本次改动发生在公安备案审核期：未改 DNS、未改公网 IP、未新增端口，仅新增 `server_name`。
