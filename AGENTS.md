# OpenQuizGo 项目规约（AGENTS.md）

本文件是项目的最高约束。任何会话、任何代理、任何贡献者开始工作前，必须先阅读并遵守。

> **本文件由 AI 代理自动加载；`docs/` 下的文件不会。**
> 需要背景细节时按下表主动读取，不要凭记忆推断：
>
> | 场景 | 必读 |
> | --- | --- |
> | 发布 / 回滚 / 线上更新 | 本文件第八节（可执行流程） |
> | 线上环境、临时访问、已知坑 | 本文件第七节 |
> | 线上待办、备案、环境现状 | `docs/deployment-todo.md` |
> | 部署架构、容量、安全基线 | `docs/deployment.md` |
> | 接口契约 | `api/openapi.yaml` |
>
> **跨仓库操作**（改 Atlas 静态站）时，OpenCode 只会加载当前目录的 AGENTS.md，
> 必须先主动读 `../OpenChronicleAtlas/AGENTS.md`。

---

## 一、项目铁律

### 铁律 1：所有对话必须留档

- 每一次会话（无论长短、无论是否产生代码）都必须归档到 `docs/conversations/`。
- 归档内容 = **可读的 Markdown 记录** + **原始 JSON**（`docs/conversations/raw/`），二者缺一不可。
- 归档时机：
  - 会话结束前必须归档；
  - 长会话在每个里程碑完成后增量归档（重跑脚本会覆盖同一会话的旧归档）。
- 归档命令：

  ```bash
  node scripts/archive-conversation.mjs              # 归档当前项目最近一次会话
  node scripts/archive-conversation.mjs --session ses_xxx   # 指定会话
  node scripts/archive-conversation.mjs --all        # 全量补档
  node scripts/archive-conversation.mjs --sanitize   # 脱敏后归档
  ```

- **会话开始时先补档**：执行一次 `--all`，确保上一次会话的收尾消息也进入归档。
- 归档文件不得手工删改；如需更正，在文末追加「归档勘误」小节并注明时间。
- `docs/conversations/INDEX.md` 由脚本自动维护，是对话检索入口，不要手工编辑。

### 铁律 2：先设计、再计划、后实施

任何功能开发、重构、架构调整，必须依次产出并推进：

1. **设计方案** → `docs/design/NNN-<主题>.md`（模板：`docs/design/TEMPLATE.md`）
   - 必须包含：背景与目标、需求/验收标准、方案对比与选型、总体架构、数据模型、接口设计、非功能要求、风险与对策、未决问题。
2. **实施计划** → `docs/plans/NNN-<主题>.md`（模板：`docs/plans/TEMPLATE.md`）
   - 必须包含：任务拆分（含依赖与顺序）、每项任务的验收标准、验证方式、里程碑、回滚方案。
3. **按计划实施** → 在计划文件中实时更新任务状态、实际耗时、偏差与决策记录。
4. **验收与复盘** → 回填实际结果、遗留问题与后续建议。

**禁止跳过设计与计划直接写代码。** 计划未经确认（状态不是 `approved`）不得进入实施。

**例外（轻量变更）**：错别字、注释、纯文档修正、无行为变化的单行修改，可免设计与计划，但必须在 `docs/plans/BACKLOG.md` 登记一行（日期 / 内容 / 原因）。

### 铁律 3：每次提交必须升级版本（1.0.X）

- 全项目统一版本号为 `1.0.X`（主版本 1、次版本 0、修订号 X），**每提交一次，X 加 1**，从 `1.0.0` 起单调递增；禁止回退、复用或跳号。
- **唯一真源**是仓库根目录 `VERSION`，其它位置由脚本同步，不得手工改出不一致：
  - `VERSION` → `1.0.X`
  - `tv/app/build.gradle.kts` → `versionName = "1.0.X"`、`versionCode = 100 + X`
  - `parent/pubspec.yaml` → `version: 1.0.X+<100 + X>`
  - 服务端发布时在部署环境显式设置 `APP_VERSION=v1.0.X`
- **自动化（默认生效）**：
  - 每台开发机执行一次 `make hooks`（即 `git config core.hooksPath scripts/git-hooks`）。
  - `scripts/git-hooks/pre-commit` 在每次提交前调用 `scripts/bump-version.mjs`，自增版本并自动暂存 `VERSION`、TV、家长端三处。
  - 手动检查：`node scripts/bump-version.mjs --dry-run`；手动升级：`node scripts/bump-version.mjs`。
- **跳过仅限紧急情况**：`OQG_SKIP_VERSION_BUMP=1 git commit ...`。使用后必须在提交说明中写明原因，并在下一次提交补齐版本。
- **发布**：发布提交必须打 tag `v1.0.X`（与 `VERSION` 完全一致），随后同步两个远端。
- 文档、截图等非代码提交同样按本规则自增；历史文档中的 `v0.65.x` 等旧版本号是既有记录，保留不改写。
- 任何一次提交后，`git show HEAD:VERSION` 必须与 tag、TV `versionName`、家长端 `version` 一致。

---

## 二、文档体系

```
docs/
├── README.md                 # 文档导航与项目状态总览
├── ROADMAP.md                # 总路线图与进度勾选（唯一勾选入口）
├── conversations/            # 对话留档（铁律 1）
│   ├── INDEX.md              # 自动生成的归档索引
│   ├── raw/                  # 原始 JSON，完整无损
│   └── YYYY-MM-DD-NNN-<标题>.md
├── design/                   # 设计方案（铁律 2 第一步）
│   ├── TEMPLATE.md
│   └── NNN-<主题>.md
└── plans/                    # 实施计划（铁律 2 第二步）
    ├── TEMPLATE.md
    ├── BACKLOG.md            # 轻量变更登记
    └── NNN-<主题>.md
```

- 设计文档与实施计划**共用编号**：`NNN` 为三位递增序号，一个主题一份设计 + 一份计划。
- 文档内互相引用：设计文档头部写「对应计划」，计划文档头部写「对应设计」。
- **进度勾选**：每完成一项，必须在 `docs/ROADMAP.md` 对应条目把 `[ ]` 改为 `[x]`，并追加变更记录；`docs/README.md` 状态表同步更新。

## 三、状态流转

`draft`（草稿）→ `review`（待评审）→ `approved`（已确认）→ `in-progress`（实施中）→ `done`（已完成）

异常分支：`cancelled`（取消，需写明原因）。

- 只有 `approved` 的计划可以开始实施。
- 实施中发现方案需变更：先改设计与计划并重新确认，再继续编码，同时在「决策记录」中留痕。

## 四、命名与提交

- 对话归档：`YYYY-MM-DD-NNN-<会话标题>.md`（脚本自动生成）。
- 设计与计划：`NNN-<kebab-case 或中文主题>.md`，同一主题编号一致。
- 代码提交信息引用编号，例如：`feat(003): 实现题库导入`。
- 提交前自检：本次变更是否有对应设计与计划？计划状态是否已更新？

## 五、当前技术栈与约定

- **服务端**：Go（模块化单体）、PostgreSQL 16、Redis 7、S3 兼容对象存储。
- **电视端**：Android TV（Kotlin + Compose for TV + 无障碍/UsageStats），定位为**守护端**，不承载内容播放；答题时**优先拉起全屏答题 Activity** 使播放器失焦停止（创维实测可用），MediaSession/音频焦点/媒体键作为支持机型上的补充。
- **家长端**：Flutter（Android / iOS）。
- **管控对象**：第三方视频 App（播放器不可控），设备级管控方案见设计 001 第 4.6 节。
- 构建/测试：`make build`、`make test`、`make test-int`（需 `TEST_DATABASE_URL` / `TEST_REDIS_URL`）。
- 本地启动：`make dev`；迁移：`make migrate-up`。
- 接口契约：`api/openapi.yaml`；部署手册：`docs/deployment.md`。

## 六、版本管理与远端

| 远端 | 地址 | 分支 |
| --- | --- | --- |
| `github` | `git@github.com:roma007/OpenQuizGo.git` | `main` |
| `gitee` | `git@gitee.com:roma007007/open-quiz-go.git` | `master` |

- 本地主干分支为 `main`，两个远端保持同步：
  ```bash
  git push github main
  git push gitee main:master
  ```
- 不要在单一远端上产生分叉；如发生冲突，先在本地合并解决，再同时推送。
- 版本号与 tag 规则见「铁律 3：每次提交必须升级版本（1.0.X）」。
- 公开仓库注意：归档前若对话含密钥、口令、个人信息，先用 `--sanitize` 归档并复查 `docs/conversations/raw/`，敏感内容不得提交。

---

## 七、线上环境与临时访问（调试用）

> **备案通过前，禁止把 nginx 切到 80/443 对外。** 域名解析已经指向公网 IP，
> 一旦在 80/443 上提供服务，就属于未备案对外，可能被云厂商封端口并影响备案审核。

### 环境信息

| 项 | 值 |
| --- | --- |
| 云厂商 / 地域 | 阿里云 华南3（广州） |
| 实例 | `ecs.e-c1m1.large` 2C2G / 40G ESSD |
| 公网 IP | `8.163.122.39` |
| Tailscale IP | `100.65.110.80`（hostname `quizgo-ecs`） |
| 系统 | ALinux 4（kernel 6.x）/ Docker 24 + Compose 2.26 |
| 部署目录 | `/srv/quizgo`（服务 + 配置）、`/srv/atlas`（Atlas 静态站）、`/srv/nginx`（nginx 配置） |
| 数据库备份 | `/srv/backup/pg/`，systemd timer 每天 03:20，保留 7 天 |

### SSH

```bash
ssh quizgo       # 走 Tailscale，优先用这个：公网出口 IP 变了也不受影响
ssh quizgo-pub   # 走公网 IP，仅当安全组放行了当前出口 IP 时可用
```

### 网页访问（临时调试通道，不涉及备案）

```bash
open http://quizgo-ecs:8081     # OpenChronicleAtlas 静态站
open http://quizgo-ecs:8082     # QuizGo 管理后台 + /v1 API
```

等效写法：`http://100.65.110.80:8081` / `:8082`。

- 访问方必须加入同一 tailnet（同一账号登录 Tailscale），手机 4G 也可以
- nginx 只放行 `127.0.0.1` 与 `100.64.0.0/10`，其余返回 403；证书、HSTS 等公网能力暂不启用
- 安全组未放行 8081/8082，公网无法直连（双重保护）

### 给非 tailnet 用户临时看效果

| 方式 | 做法 | 说明 |
| --- | --- | --- |
| Tailscale 共享节点（推荐） | Tailscale 后台 → Machines → Share | 只共享单个节点，对方无需加你账号 |
| 临时放开非标端口 | 安全组加 `TCP 8081/8082 → 对方 IP/32` | 非 80/443，一般不涉及备案；用完删掉 |
| 公网隧道（ngrok/Cloudflare Tunnel） | — | ⚠️ 不推荐，有合规风险 |

### 硬性约束

1. `deploy/nginx/conf.d/*.conf` 在备案通过前保持 `listen 8081/8082`，不得改成 80/443
2. 不得把 `.env`、密钥、管理员口令、备份文件写入仓库或对话归档；归档使用 `--sanitize`
3. 改动部署相关文件后，需同步到服务器，并在 `docs/deployment-todo.md` 留痕
4. 服务器只有 2C2G：不要在上面跑构建，镜像在本地或 CI 构建后传输

### 常用运维命令

```bash
ssh quizgo 'cd /srv/quizgo && docker compose ps'
ssh quizgo 'cd /srv/quizgo && docker compose logs -f --tail=50 api'
ssh quizgo 'sudo systemctl start quizgo-backup.service'    # 手动备份一次
ssh quizgo 'ls -lh /srv/backup/pg/'                        # 查看备份
ssh quizgo 'sudo iptables -S ts-input'                     # 检查 Tailscale 是否丢了阿里云网段例外
ssh quizgo 'cd /srv/quizgo && docker compose run --rm --no-deps --entrypoint /app/admin api list'
```

### 已知环境坑（详细见 `docs/deployment-todo.md` 第五节）

1. **Tailscale 与阿里云 `100.64.0.0/10` 冲突**：`ts-input` 的防欺骗规则会丢掉阿里云内网响应，
   症状是 DNS 全废、元数据不可达、云安全中心离线、文件备份失败。
   已用 `-A ts-input -s 100.64.0.0/10 -i eth0 -j ACCEPT` 修复，由 systemd timer 每 5 分钟自愈。
2. `tailscale netcheck` 显示的端口是随机的，**不能**用来判断 UDP 41641 是否放行，要用 A/B 抓包对照。
3. zsh 里 `log` 是内置数学函数，查系统日志必须写 `/usr/bin/log`。
4. macOS 上 Tailscale 的「Launch Tailscale at login」开关会报 `Could not find the launch daemon`，
   属于 Tailscale 1.104.7 与 macOS 26 的兼容问题，改用系统登录项即可。

---

## 八、发布到服务器（什么时候发、怎么发）

### 8.1 什么时候必须更新服务器

判断口诀：**用户能不能看出区别？** 能 → 发；看不出且不涉及安全 → 攒到下次一起发。

| 变更类型 | 是否发布 | 说明 |
| --- | --- | --- |
| 页面 / 接口行为 / 校验规则 / 错误码 | ✅ 必须 | 客户端可能依赖 |
| 线上 bug 修复 | ✅ 必须 | |
| 安全修复 | ✅ **立刻**发 | 不等攒批 |
| 新增 goose 迁移（改表、加索引） | ✅ 必须，**先备份** | 迁移不可逆 |
| `.env` 配置变化（限额、开关、SMS_MODE） | ✅ 必须 | 要重建容器才生效 |
| 只改文档、注释、测试 | ❌ 不发 | |
| 纯重构且行为不变 | ⏳ 可攒 | 跟下次功能一起发 |
| feature 分支、本地实验 | ❌ 不发 | 未合并不发 |

### 8.2 版本一致性（四个地方必须对齐）

```bash
VERSION                     # 1.0.X
git tag                     # v1.0.X
/srv/quizgo/.env APP_VERSION # 1.0.X（服务器上手动改）
线上 /v1/meta 的 version      # 1.0.X
```

`VERSION` 由 pre-commit 钩子自增；服务器 `.env` 不会自动同步，**必须手动改**，否则 `/v1/meta` 仍报旧版本。

> **强制要求（078）**：提交会由钩子把 `VERSION` +1，但服务器 `.env` 不会跟着变。
> 任何一次发布都必须执行 `scripts/deploy.sh`（或手工完成它的全部步骤），
> 否则 `/v1/meta` 会一直报旧版本，排查时会被误导。

### 8.3 QuizGo 发布流程

一步到位（推荐，内部就是下面 0–5 步，外加版本对齐与自检）：

```bash
scripts/deploy.sh                   # 标准发布
scripts/deploy.sh --with-admin-web  # 管理端前端有改动
scripts/deploy.sh --with-units      # deploy/systemd 有改动（重装 unit + 重启 timer）
scripts/deploy.sh --skip-build      # 纯文档/注释提交：不重建镜像，只对齐版本号
scripts/deploy.sh --dry-run         # 只看会执行什么
```

> 脚本会先打回滚点、跑一次备份，再用 `docker compose up -d` 让 migrate 先执行。
> 下面分步流程用于理解或应急手工作业。

```bash
# 0. 本地自检
make test && make vet

# 1. 打回滚点：旧镜像 + 立刻备份数据库
ssh quizgo 'docker tag openquizgo:local openquizgo:rollback || true'
ssh quizgo 'sudo systemctl start quizgo-backup.service'

# 2. 同步源码（不要 --delete，避免删掉服务器上的东西；.env 必须排除）
#    注意排除 GB 级的客户端构建产物，否则会把 3Mbps 的发布拖死
rsync -az --exclude '.git' --exclude '.env' --exclude 'node_modules' \
  --exclude 'parent' --exclude 'tv' --exclude 'android' --exclude 'admin-web' \
  --exclude 'bin' --exclude 'dist' --exclude 'docs/conversations' \
  ./ quizgo:/srv/quizgo/
```

```bash
# 3. 版本号（本次升了版本才需要）
ssh quizgo 'sudo sed -i "s|^APP_VERSION=.*|APP_VERSION=1.0.7|" /srv/quizgo/.env'

# 4. 构建镜像 + 启动（migrate 先跑，失败则 api/worker 不启动）
ssh quizgo 'cd /srv/quizgo && docker compose build api && docker compose up -d'

# 5. 验证
ssh quizgo 'cd /srv/quizgo && docker compose ps'
curl -s http://quizgo-ecs:8082/v1/meta; echo
curl -s -o /dev/null -w "健康 %{http_code}\n" http://quizgo-ecs:8082/healthz
ssh quizgo 'cd /srv/quizgo && docker compose logs --tail=50 api | grep -iE "error|panic" || echo "无错误"'
```

镜像构建两种方式：

| 方式 | 命令 | 适用 |
| --- | --- | --- |
| 服务器构建（默认） | `docker compose build api` | 日常改动，有缓存，1–3 分钟 |
| 本地构建再传 | `docker build --platform linux/amd64 -t openquizgo:local .`<br>`docker save openquizgo:local \| ssh quizgo 'docker load'` | 依赖大改或服务器内存紧张时 |

服务器 2C2G，构建有 4G swap 兜底；**构建期间不要并行跑其他重活**。

### 8.4 管理端前端（改了才需要）

```bash
pnpm --filter admin-web build
rsync -az --delete admin-web/dist/ quizgo:/srv/quizgo/admin-web/dist/
```

### 8.5 Atlas 静态站（同一台服务器，另一个仓库）

```bash
cd ~/我的文档/源码/OpenChronicleAtlas
pnpm build
ssh quizgo 'rm -rf /srv/atlas.prev && cp -a /srv/atlas /srv/atlas.prev'
rsync -az --delete apps/web/dist/ quizgo:/srv/atlas/
```

详细规则见该仓库的 `AGENTS.md` 第 7 节；这里只需记住：**静态站回滚是秒级的，发布前必须备份上一版**。

### 8.6 回滚

```bash
# 只回代码（不回库）
ssh quizgo 'docker tag openquizgo:rollback openquizgo:local'
ssh quizgo 'cd /srv/quizgo && docker compose up -d --no-build api worker'

# 需要回库时才用备份（先停 api/worker，见第七节恢复命令）
```

迁移是单向的（goose `up`）。旧代码面对新表结构通常能跑（迁移应尽量只加不删）；
真要回库只能用备份恢复，**所以带迁移的发布必须先备份**。

### 8.7 发布后自检（四条全过才算成功）

1. `docker compose ps` 全部 `Up`/`healthy`，`migrate` 为 `Exited (0)`
2. `/v1/meta` 版本号与 `VERSION`、tag、`.env` 一致
3. 日志无 `error` / `panic`
4. `free -m` 可用内存 > 200MB，`df -h /` 使用率正常

### 8.8 常见坑

| 坑 | 说明 |
| --- | --- |
| 只同步代码不重建镜像 | 容器仍跑旧镜像，改动不生效 |
| 改了 `.env` 只 `up -d` | Compose 未必检测到 env_file 变化，用 `docker compose up -d --force-recreate api worker` |
| 迁移失败 | `migrate` 非 0 退出，api/worker 不会启动；`docker compose ps` 会显示 `Exited (1)` |
| 忘记改 `APP_VERSION` | `/v1/meta` 仍显示旧版本，排查时会被误导 |
| `up -d` 失败（migrate/依赖） | 脚本会自动回写旧 `APP_VERSION` 并 `exit 1`；先看 migrate 日志再重试，不要手工改 `.env` |
| 在服务器上构建 Atlas | 没必要，静态站本地构建秒级完成 |
| 构建后磁盘涨 | 每次构建留一层缓存，定期 `docker builder prune -f` |

---

## 九、跨项目协作（`/srv/ops` 共享面）

同一台 ECS 上还有 OpenChronicleAtlas（静态站），两边共用一个 nginx 容器与一份磁盘。
跨项目协调**不再经人转达**，统一走服务器上的 `/srv/ops/`。

### 9.1 先读契约

- 动共享面之前：`ssh quizgo 'cat /srv/ops/CONTRACT.md'`（归属表、动共享面四步、共同红线）。
- 一屏看双方状态：`ssh quizgo '/srv/ops/bin/status.sh'`。
- 看发给自己的未处理请求：`ssh quizgo '/srv/ops/bin/inbox.sh quizgo'`。

### 9.2 改 `/srv/nginx/**` 或重启 `web` 前必须拿锁

```bash
ssh quizgo 'flock /srv/ops/locks/nginx.lock -c "<命令>"'
```

- 拿不到锁说明 Atlas 正在动，**等**，不要强抢。
- 经 `scripts/deploy.sh --with-nginx` 的改动已包含备份、`nginx -t`、失败自动回滚与 reload，优先用它。

### 9.3 每次发布后登记 `state/quizgo.json`

`scripts/deploy.sh` 收尾自动覆盖，字段固定：

```json
{
  "project": "quizgo",
  "updatedAt": "2026-11-14T18:20:00+08:00",
  "updatedBy": "quizgo-ai",
  "release": { "version": "1.0.16", "commit": "<sha>", "image": "sha256:<id>" },
  "nginxMd5": {
    "nginx.conf": "<md5>",
    "conf.d/atlas.conf": "<md5>",
    "conf.d/quiz.conf": "<md5>"
  }
}
```

- `nginxMd5` **三个文件都要记**，少一个 `status.sh` 就查不出那部分漂移。
- 只写 `state/quizgo.json`，**不得修改 `state/atlas.json`**（Atlas 写的）。
- 登记失败不影响发布结果，但会打印告警，需要手工补登。

### 9.4 发布完追加 journal

`/srv/ops/journal/YYYY-MM.log`，一行一条，只追加：

```
2026-11-14T18:20:00+08:00 [quizgo] deploy | quizgo 1.0.16 | image sha256:… | ok
```

类别：`deploy` / `nginx` / `request` / `incident` / `ack` / `note`。`deploy.sh` 自动写 `deploy` 行。

### 9.5 需要 Atlas 做事就写 requests

- 在 `/srv/ops/requests/` 新建 `YYYY-MM-DD-quizgo-to-atlas-<主题>.md`，格式见该目录 `README.md`。
- 不要等人转达；对方处理后会把 `status` 改成 `done` 并追加 `## 回复`。

### 9.6 闸门挂在哪

| 路径 | 行为 |
| --- | --- |
| `scripts/git-hooks/pre-commit` | 列出 inbox(quizgo) 的 open 请求，**只提示不阻塞**（3 秒超时，失败静默） |
| `scripts/deploy.sh` 开头 | 有 open 请求 → `exit 2`；确认无关或本次就是处理它时加 `--ack-open` |
| `scripts/deploy.sh` 收尾 | 运行一次 `status.sh`，出现 `⚠️` 只告警；随后自动登记 state + journal |

### 9.7 红线

- 备案通过前，任何一方不得把 nginx 切到 80/443，不得新增公网端口。
- Tailscale / iptables / 阿里云安全组只能人工改，AI 不动。
- `/srv/atlas`、`/srv/atlas.prev`、`state/atlas.json` 属 Atlas，QuizGo 只读。
- `atlas.conf` 的**源文件在本仓库** `deploy/nginx/conf.d/`；Atlas 只能通过 requests 交付改动，由 QuizGo 提交并 `--with-nginx` 同步。

