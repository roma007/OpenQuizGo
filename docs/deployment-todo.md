# 部署待办（OpenChronicleAtlas + OpenQuizGo）

> 服务器：`ecs-user@8.163.122.39`（华南3 广州 / ALinux 4 / 2C2G / 40G）
> 登录：**`ssh quizgo`**（走 Tailscale，推荐）；`ssh quizgo-pub` 仅在安全组放行当前出口 IP 时可用
> 域名：`openapp.asia`
> 线上版本：`1.0.16`（`VERSION` = tag = `.env APP_VERSION` = `/v1/meta`）
> 最后更新：2026-11-14

---

## 一、已完成

| 项 | 状态 | 说明 |
|---|---|---|
| 服务器初始化 | ✅ | Docker 24.0.9 + Compose 2.26.1，镜像加速，4G swap，SSH 禁 root/密码 |
| Atlas 部署 | ✅ | 本地构建 → `/srv/atlas`，Nginx `:8081`，SPA 回退/gzip/缓存头/PWA 不缓存 |
| QuizGo 部署 | ✅ | 镜像 `openquizgo:local`，6 个服务（pg/redis/migrate/api/worker/web），43 表，最新迁移 `00031`（`goose_db_version` 共 32 行，含 version 0 的初始化行） |
| 精简配置 | ✅ | 去 Prometheus、PG/Redis 限内存、DB/Redis 不对宿主机发布、容器内存上限 |
| 密钥 | ✅ | `APP_ENV=prod` + 5 个强密钥，`/srv/quizgo/.env` 600 |
| 管理员 | ✅ | `admin` 账号已建（密码见首次部署记录，**建议立即修改**） |
| 数据库备份 | ✅ | systemd timer 每天 03:20，`/srv/backup/pg/`，保留 7 天，已做恢复演练 |
| 自动快照 | ✅ | 云盘每天 1 次，保留 7 天（购买时已开） |
| 安全组 | ✅ | 22 仅 `223.73.119.15/32`（临时）、删 3389、开 80/443、UDP 41641（Tailscale）、保留 ICMP |
| 远程访问 | ✅ | Tailscale：服务器 `100.65.110.80`、Mac `100.72.81.32`，**UDP 直连**，SSH 与页面都走内网 |
| Tailscale × 阿里云段冲突 | ✅ | `100.64.0.0/10`（仅 eth0）例外规则 + systemd timer 自愈（详见「五、已知问题 1」） |

---

## 二、待办

### 🔴 P0 — 需要你操作，越早越好

- [x] **安全组收紧**（控制台）✅ 2026-11-14 完成
  - 新增：`TCP 22` → `223.73.119.15/32`，**验证能连后再删**旧的 `0.0.0.0/0`
  - 删除：`TCP 3389`（Linux 不需要）
  - 新增：`TCP 80`、`TCP 443` → `0.0.0.0/0`（备案通过后要用）
  - ICMP：保留（便于排查）
  - ⚠️ 家庭宽带 IP 会变，变了用控制台 VNC 进去救，再改授权对象
  - 现状：当时 4 条（`22(限本机)` + `ICMP` + `80` + `443`）；加上 `UDP 41641` 后共 **5 条**
- [x] **新增 UDP 41641（Tailscale 直连）**（控制台）✅ 2026-11-14 完成
  - 规则：优先级 `1` / 允许 / **自定义 UDP** / `41641/41641` / `0.0.0.0/0` / `Tailscale 直连`
  - 验证（A/B 抓包）：发往 41641 的探测包 **5/5 到达服务器**；对照组 41642 **0/5**（被拦）
  - 注意：`tailscale netcheck` 显示的映射端口是随机的，那是 STUN 用临时端口，**不能**用来判断这条规则
  - 效果：两端可主动打洞，直连稳定，不会掉到 161ms 的洛杉矶中继

### 🗓 定时复核 —— 2026-11-21（一周后）

> 目标：确认 Tailscale 稳定后，删掉公网 SSH 规则，彻底消除动态 IP 的隐患。

- [ ] **决定是否删除 `TCP 22 → 223.73.119.15/32`**
  - 实测记录 **2026-11-14 04:31**：Mac 出口 IP 从 `223.73.119.15` 变成 `183.48.71.64`
    - `ssh quizgo` 正常，直连自动重建（`183.48.71.64:35685`），延迟约 15ms
    - `ssh quizgo-pub` 超时（安全组只放了旧 IP）
    - **结论：动态 IP 的影响已被 Tailscale 完全消除，那条公网 22 规则此刻已经形同虚设**
  - 删除前逐条确认（三条都满足才删）：
    ```bash
    # 1. 这周用 ssh quizgo 有没有失败过（自己回想）
    # 2. 服务器重启后 Tailscale 自动起来
    ssh quizgo 'systemctl is-enabled tailscaled; systemctl is-active tailscaled; uptime -p'
    # 3. 现在是直连不是中继
    ssh quizgo 'tailscale status | grep macbook'
    ```
  - 判定：`enabled` + `active` + 对方显示 `direct` → **可以删**
  - 任何一条不满足 → 再等一周，公网后路先留着
  - 删除后立刻验证：`ssh quizgo 'echo ok'` 仍通；`nc -z 8.163.122.39 22` 应超时
  - ⚠️ 删完若 Tailscale 也出问题，唯一入口是 **控制台 → 远程连接 → VNC**


### 🟡 P1 — 备案相关（当前进度：ICP 备案申请中）

- [ ] **ICP 备案提交并通过**（你，阿里云）
  - 网站名称：`开放应用记录`
  - 网站内容：仅勾「其他」
  - 网站语言：中文简体
  - 备注：已按关键词要求精简（不要写"新闻/出版/教育/医疗/文化/金融/销售/广告/付费/注册/发帖"）
  - 云服务：`ECS` → 实例 `8.163.122.39`
  - 流程：阿里云初审 1–2 天 → 工信部短信核验（24h 内）→ 管局 1–2 周
- [ ] **域名解析**（你，备案通过后）
  - `@`、`www`、`atlas`、`quiz` → A → `8.163.122.39`
- [ ] **公安联网备案**（你，ICP 通过后 30 日内）
  - `beian.mps.gov.cn`，通过后把公安备案号加到页面底部

### 🟢 P2 — 等备案通过后我做

- [ ] **Nginx 切到 80/443 + HTTPS**
  - `deploy/nginx/conf.d/atlas.conf`：`listen 8081` → `80` + `443 ssl`
  - `deploy/nginx/conf.d/quiz.conf`：`listen 8082` → `80` + `443 ssl`
  - `server_name atlas.openapp.asia` / `quiz.openapp.asia`
  - 证书：Let's Encrypt（certbot）或阿里云免费证书
  - 80 → 301 跳 443
- [ ] **页面底部挂备案号**（两个项目）
  - ICP：链接 `https://beian.miit.gov.cn`
  - 公安：链接 `https://beian.mps.gov.cn`
- [ ] **上线后全链路验证**
  - `curl -I https://atlas.openapp.asia`、`https://quiz.openapp.asia/v1/meta`
  - 证书有效期、HSTS、gzip、缓存头

### ⚪ P3 — 可选增强

- [ ] **OSS 异地备份**（需要 RAM 子账号 AccessKey，只授权单 bucket）
  - 每天把 `/srv/backup/pg/*.sql.gz` 上传，防止整盘/实例级故障
  - ⚠️ **只传 `*.sql.gz`，不要传 `env-*.bak`**：那是 `.env` 快照，含全部生产密钥。
    如果坚持要备份配置，先 `gpg`/`age` 加密再上传，或改存到 OSS 的独立加密 bucket
- [ ] **短信通道接入**（当前 `SMS_MODE=disabled`，家长端短信登录不可用）
  - 接阿里云短信 → 改 `.env` → 重启 api
- [x] **更新发布流程固化** ✅ 2026-11-14
  - 已实现：`scripts/deploy.sh`（备份 → 打 `openquizgo:rollback` → rsync → nginx/units →
    `docker compose build` → **构建成功后才写 `.env`** → `up -d`（migrate 先跑）→ 自检 → 共享面登记）
  - 参数：`--with-admin-web`、`--with-nginx`、`--with-units`、`--skip-backup`、`--skip-build`、
    `--ack-open`、`--dry-run`
  - 解决了版本真空期：提交 hook 升 `VERSION` 后，脚本自动写入 `.env APP_VERSION` 并重建容器
  - 仍待做：CI 构建镜像推 ACR，服务器只 pull（当前 2C2G 上构建约 4.5 分钟）
  - ⚠️ `--skip-build` / `--skip-backup` 有硬保护：以服务器 `/srv/quizgo/.deployed-image`
    为基线，服务端代码/迁移有变化、工作区脏、或本机解析不到基线 commit 时直接 `exit 2`
  - ⚠️ 改 `deploy/nginx` 必须加 `--with-nginx`（会 `nginx -t` 通过后再 reload）；
    改 `deploy/systemd` 必须加 `--with-units`，否则会「发布成功但配置没生效」
  - 跨项目协作：发布前读 `/srv/ops/bin/inbox.sh quizgo`，有 open 请求直接 `exit 2`，
    确认无关才加 `--ack-open`；收尾自动覆盖 `/srv/ops/state/quizgo.json` 并追加 journal
  - ⚠️ 构建失败时 `.env` 保持上一版（版本写入在 build 成功之后），不会出现「报新版本、跑旧代码」
- [x] **清理构建缓存** ✅ 2026-11-14
  - 已执行 `docker builder prune -f`，回收 911.7MB，`docker system df` 里 Build Cache 归零
  - 同时删除了 `/srv/quizgo` 下已停止更新的陈旧副本 `bin`/`dist`/`parent`/`tv`/`android`（141M）
  - `openquizgo:local` 镜像本身 63MB，与构建缓存是不同口径
  - ⚠️ 缓存清空后**下次代码发布会全量编译，约 5 分钟**；发布窗口内不要并行做别的重活
- [ ] **Tailscale 加固**（见上方「定时复核 2026-11-21」）
  - 已完成：UDP 41641 放行（A/B 抓包验证）、直连建立、延迟 18–24ms
  - 待办：一周后决定是否删掉 `TCP 22 → 223.73.119.15/32`

---

## 三、常用命令

```bash
# SSH（走 Tailscale 内网，与公网 IP 无关）
ssh quizgo

# 或完整写法（等价，便于在别人的机器上临时用）
ssh -i ~/.ssh/gz-atlas-quizgo.pem ecs-user@100.65.110.80

# 页面直接用内网 IP 打开，不需要 SSH 隧道
# Atlas   http://100.65.110.80:8081
# 后台    http://100.65.110.80:8082

# Tailscale 状态（两端 IP、直连/中继）
tailscale status
/Applications/Tailscale.app/Contents/MacOS/Tailscale ip -4   # Mac 端
ssh quizgo 'tailscale ip -4; tailscale status | head -5'     # 服务器端

# 走公网 IP 的备用入口（动态 IP 变化后会失效）
ssh quizgo-pub

# 验证 UDP 41641 规则是否生效（A/B 对照：41641 应到达，41642 应被拦）
ssh quizgo 'sudo timeout 20 tcpdump -ni any -c 80 "udp and dst portrange 41640-41645"' &
sleep 4
python3 -c 'import socket as s;a=s.socket(2,2);[a.sendto(b"A"*77,("8.163.122.39",41641)) for _ in range(5)];b=s.socket(2,2);[b.sendto(b"B"*77,("8.163.122.39",41642)) for _ in range(5)]'
# 结果里应只有 length 77 且 dst 41641 的包，没有 41642

# 服务状态 / 日志
cd /srv/quizgo
docker compose ps
docker compose logs -f --tail=50 api

# 改管理员密码
docker compose run --rm --no-deps --entrypoint /app/admin api \
  passwd -username admin -password '新密码'

# 手动备份 / 看备份
sudo systemctl start quizgo-backup.service
ls -lh /srv/backup/pg/
sudo journalctl -u quizgo-backup.service -n 20 --no-pager

# 恢复
docker compose stop api worker
gunzip -c /srv/backup/pg/openquizgo-XXXX.sql.gz \
  | docker compose exec -T postgres psql -U openquizgo -d openquizgo
docker compose start api worker
```

---

## 四、端口与资源

| 服务 | 监听 | 对外可达性 | 内存上限 |
|---|---|---|---|
| nginx（Atlas + 后台/API） | `0.0.0.0:8081` / `8082`（host 网络） | 仅本机 + Tailscale 网段（`allow`/`deny`），公网安全组未放行 | 64m |
| quizgo api | `127.0.0.1:8080` | 仅宿主机（由 nginx 反代） | 256m |
| quizgo worker | 无 | — | 128m |
| postgres | 仅容器网络 | 容器内 | 512m |
| redis | 仅容器网络 | 容器内 | 128m |

> nginx 用 `network_mode: host`：经 docker-proxy 转发时源 IP 会被改写成 `172.19.0.1`，`allow/deny` 会误判；host 网络下日志和访问控制都拿到真实客户端 IP。
>
> 双重保护：安全组只放行 22 / 80 / 443 / 41641(UDP) / ICMP；即使 8081/8082 被外面扫到，nginx 也会返回 403。

实测占用约 47MB，系统可用 1.1GB，磁盘使用 26%。

### 安全组入方向（目标状态）

| 协议 | 端口 | 授权对象 | 用途 | 状态 |
|---|---|---|---|---|
| 自定义 TCP | `22/22` | `223.73.119.15/32` | SSH（公网后路） | 临时保留，2026-11-21 复核后删 |
| 自定义 UDP | `41641/41641` | `0.0.0.0/0` | Tailscale 直连打洞 | ✅ 已开（A/B 抓包验证） |
| 自定义 TCP | `80/80` | `0.0.0.0/0` | HTTP（等备案） | 已开 |
| 自定义 TCP | `443/443` | `0.0.0.0/0` | HTTPS（等备案） | 已开 |
| 全部 ICMP | `-1/-1` | `0.0.0.0/0` | ping 排查 | 已开 |

出方向保持默认全放行。`5432 / 6379 / 8080 / 8081 / 8082` 一律不放开。

---

## 五、已知问题（环境坑，别再重复排查）

### 1. Tailscale 与阿里云 `100.64.0.0/10` 冲突（DNS / 元数据 / 云安全中心 / 云备份 全挂）⚠️

**现象（2026-11-14 实际发生，两次短信告警）**

- 阿里云短信：**云安全中心客户端异常离线**
- 阿里云短信：**文件备份任务失败，备份点未生成**
- 服务器上 `curl https://任意域名` 全部失败，但 `curl https://223.5.5.5` 正常
- `getent hosts xxx` 解析失败
- `ping 100.100.100.200` 失败
- **抓包能看到 SYN-ACK 回来了**，但本机不会回应 ACK（对方连续重发三次）

**根因**

Tailscale 的防欺骗规则：

```
-A ts-input -s 100.64.0.0/10 ! -i tailscale0 -j DROP
```

从 `eth0` 进来、源地址在 `100.64.0.0/10` 的包**一律丢弃**。而阿里云这些服务都在该段内、且从 `eth0` 进来：

| 服务 | 地址 |
|---|---|
| 实例元数据 | `100.100.100.200` |
| 内网 DNS | `100.100.2.136` / `100.100.2.138` |
| 云安全中心 | `100.100.30.26` / `100.100.32.66` / `100.100.25.4` |
| 云备份 HBR | `100.100.0.28`（`hbr-vpc.<region>.aliyuncs.com`） |
| OSS 内网 | `100.115.33.x`（`oss-cn-<region>-internal.aliyuncs.com`） |

所以 DNS 响应、元数据响应、云盾心跳、备份数据流的返回包全在内核里被丢掉。
公网地址（如 `223.5.5.5`）不受影响，所以表现为「能 ping 外网但上不了网」。

> ⚠️ **踩过的坑**：第一版只放行 `100.100.0.0/16`，云盾恢复了，但**文件备份仍然失败**——
> 因为 OSS 内网用的是 `100.115.33.x`，不在 `/16` 里。最终改为放行整个 `100.64.0.0/10`。

**修复（已部署，无需再动）**

在 `ts-input` 链最前面插一条例外（只对默认路由网卡生效，`docker0` / `br-*` 仍受保护）：

```
-A ts-input -s 100.64.0.0/10 -i eth0 -j ACCEPT
```

持久化（仓库文件已同步）：

- `deploy/systemd/tailscale-aliyun-fix.service` → `ExecStart=/srv/quizgo/scripts/tailscale-aliyun-fix.sh`
- `deploy/systemd/tailscale-aliyun-fix.timer`（开机 30s 后 + 每 5 分钟检查，**tailscaled 重启后自动补回**，幂等）
- 不再复制到 `/usr/local/bin`：unit 直接指向仓库脚本，避免两份文件漂移（旧副本已删除）
- 安装/更新 unit：`scripts/deploy.sh --with-units`（内部 `daemon-reload` + `enable` + `restart`）
- 脚本会自动清理旧版 `100.100.0.0/16` 窄规则

**验证命令**

```bash
sudo iptables -S ts-input | grep "100.64.0.0/10 -i eth0"   # 应有且只有一条 ACCEPT
curl -s -o /dev/null -w "%{http_code}\n" http://100.100.100.200/latest/meta-data/   # 200
curl -s -o /dev/null -w "%{http_code}\n" https://oss-cn-guangzhou-internal.aliyuncs.com/   # 403=通
getent hosts hbr-vpc.cn-guangzhou.aliyuncs.com      # 能解析
sudo ss -tnp | grep -E "AliYunDun|hbrclient"        # 各有 ESTAB 连接
```

**注意事项**

- 只在「云厂商内网也用 `100.64.0.0/10`」的环境出现（阿里云、腾讯云等），Mac 端不受影响
- 这类问题**不要**在控制台点「重装云安全中心客户端」——重装也连不上，根因在网络层
- 云备份失败时同理：**不要**重装备份客户端，先按上面命令验证 OSS 内网是否可达
- 放行范围只限 `eth0`，`docker0` / `br-*` 上来自 `100.64/10` 的包仍被丢弃
- 环境里新出现 `100.64/10` 的内网服务不用再改规则，`/10` 已全部覆盖
- Tailscale 升级后若链名变化，脚本找不到 `ts-input` 会自动退出，不会报错刷日志

### 2. Tailscale「Launch Tailscale at login」开关报错（macOS 26.6.2 + Tailscale 1.104.7）

**现象**

```
Start on Login could not be set.
Enable Tailscale in the System Settings app ('General' -> 'Login Items').
```

**已确认原因**（2026-11-14 排查）

点开关时的系统日志：

```
Tailscale: (ServiceManagement) Service <private> status: 3
Tailscale: [Tailscale:General] Critical error: Could not find the launch daemon.
```

- `status: 3` = `SMAppService.Status.notFound`
- Tailscale 的登录启动依赖包里自带的特权守护进程 `tssentineld`
  （`Contents/Library/LaunchDaemons/io.tailscale.ipn.macsys.tssentineld.plist`）
- 这个 daemon 在本机注册失败，导致整个开关被拒绝
- 已在系统设置里手动添加的登录项与它**不是同一个对象**，所以加了也没用

**已排除的可能**：安装位置、代码签名、公证、包完整性、daemon plist 路径、
daemon 二进制、SIP、版本新旧（1.104.7 已是 stable 最新）——全部正常。

**当前解法**：用系统登录项代替（等价于开关打开）

```
系统设置 → 通用 → 登录项 → 登录时打开
  Tailscale → /Applications/Tailscale.app
```

已确认 BTM 记录：`Disposition: [enabled, allowed, notified]`。

**注意事项**

- Tailscale 设置里那个开关会**一直报错**，忽略，不影响开机自启
- **不要重复添加**登录项，会启动两次
- 撤销：登录项里选中 Tailscale → 点 `−`

**以后彻底修复**

| 时机 | 做法 |
|---|---|
| Tailscale 发新版 | 菜单栏 About 里更新，升级后开关大概率恢复 |
| 想立刻试 | `sudo sfltool resetbtm`（会清掉所有 App 的后台项授权，需重新批准，不建议） |
| 上报 | Tailscale GitHub issue，附 `status: 3 / Could not find the launch daemon / macOS 26.6.2` |

**验证开机自启是否生效**：重启后看菜单栏有没有 Tailscale 图标，或

```bash
pgrep -fl "Tailscale.app/Contents/MacOS/Tailscale"
```

### 3. 发布基线 `/srv/quizgo/.deployed-image`（不要删、不要提交）

`scripts/deploy.sh` 每次成功发布后写入两行：上次构建的 commit 与镜像 ID。
`--skip-build` / `--skip-backup` 靠它判断「代码有没有变」，没有它就会拒绝跳过类参数。

- 不要手工编辑或删除；删掉后下一次只能做完整发布
- 不要提交进仓库（已在 `deploy.sh` 的 rsync 排除项里）
- 内容形如：
  ```
  c2b09ae6ea65645dfbbc46334447ca4be1189440
  sha256:8bcb5c3136352e7acd28ffeed8cfa0b254b37982d5df2378a55c1c5c130e3938
  ```

### 4. `tailscale netcheck` 的端口不能用来判断 UDP 规则

netcheck 用临时端口做 STUN 探测，显示的映射端口永远是随机高位值（如 56881），
**不能**据此判断安全组里 `UDP 41641` 是否放行。正确方法是 A/B 抓包对照，
命令见「三、常用命令」。

### 5. 本机 `log` 命令要写全路径

zsh 里 `log` 是内置数学函数，直接写 `log show ...` 会报
`zsh:log: too many arguments`。查系统日志必须用 `/usr/bin/log`。
