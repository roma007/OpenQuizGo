# 第三期家长端 App 验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.3.1 |
| 日期 | 2026-11-04 |
| 范围 | 第三期：家长端 App（S1–S3、T1–T10） |
| 设计基线 | [003-家长端 App 设计](../design/003-家长端%20App%20设计.md) |
| 计划基线 | [003-家长端 App 实施计划](../plans/003-家长端%20App%20实施计划.md) |
| 服务端基线 | v0.1.0 + 远程解锁接口 |
| TV 基线 | v0.2.0 |

## 1. 结论

**家长端 MVP 功能验收：通过（无真机）。**

- 登录、孩子档案、设备绑定/解绑、规则配置、成长报告、远程解锁、PIN、家庭成员全部实现；
- 服务端远程解锁接口完成并实测（200/403/404/429）；
- Android debug/release 构建成功；iOS 无签名构建成功；
- `flutter test` 18 项通过，`dart analyze` 无问题；
- 已在 Android TV 模拟器完成逐页目视走查（孩子/规则/预览/PIN/设备/远程解锁/报告/设置/成员）；
- 走查中发现并修复「FilledButton 最小宽度为无限大导致 Row 内整页空白」的 UI 缺陷（v0.3.1）；
- 限制：没有实体手机，真机尺寸/手感与 iOS 真机体验仍待补。

## 2. 交付物

| 类别 | 交付 |
| --- | --- |
| Android 包 | `dist/OpenQuizGo-Parent-0.3.1.apk`（sha256 见同目录） |
| iOS | `parent/build/ios/iphoneos/Runner.app`（未签名） |
| 源码 | `parent/`（Flutter + Riverpod + go_router + Dio） |
| 服务端 | 迁移 `00007_parent_unlock.sql`、`POST /v1/family/devices/{id}/unlock` |
| 文档 | 设计 003、计划 003、本报告 |

## 3. 功能清单

| 功能 | 状态 | 说明 |
| --- | --- | --- |
| 短信登录 / 令牌刷新 | ✅ | mock 验证码 `123456`；401 自动刷新并重试 |
| 登录态持久化 | ✅ | Keychain/Keystore；杀进程与覆盖安装后仍登录 |
| 孩子档案 | ✅ | 列表、新增/编辑（昵称/学段/生日） |
| 设备绑定 | ✅ | 6 位激活码 + 选择孩子；设备列表与解绑 |
| 观看规则 | ✅ | 预设/进阶权重、时长、门槛、每日上限、离线、时段、范围 |
| 规则预览 | ✅ | dry-run 显示题量与最高加权分，保存前 PIN |
| 成长报告 | ✅ | 周/月、观看时长、答题数、正确率、学科分布、历史 |
| 远程解锁 | ✅ | 5–60 分钟 + PIN，服务端签发授权，TV 轮询放行 |
| 家长 PIN | ✅ | 设置/修改、4–6 位、服务端错误锁定 |
| 家庭成员 | ✅ | 成员列表、邀请码、加入家庭 |
| 离线缓存 | ✅ | 孩子/规则/报告缓存，断网显示「离线数据 + 同步时间」 |

## 4. 验证结果

### 4.1 自动化

```text
dart analyze lib test      No issues found
flutter test               18 tests passed
go test ./...              passed（含远程解锁单测）
```

覆盖：登录态恢复、401 刷新、孩子/设备/规则/报告请求、PIN/成员、页面渲染、缓存读写。

### 4.2 构建

| 产物 | 结果 |
| --- | --- |
| Android debug（arm64） | ✅ 构建成功 |
| Android release（arm64） | ✅ versionCode 4 / 0.3.1 |
| iOS `flutter build ios --no-codesign` | ✅ Runner.app 18.1MB |

> release 包当前回退 debug 签名（未提供 `keystore.properties`），仅用于内部验收；生产签名方式见部署手册。

### 4.3 端到端

| 链路 | 证据 |
| --- | --- |
| 登录 | 模拟器实测：发码 → 登录 → 首页；API 200 |
| 登录持久化 | force-stop/覆盖安装后仍进入首页 |
| 孩子/设备/报告加载 | App 启动请求 `GET /children`、`/devices`、`/reports` 均 200 |
| 远程解锁 | 服务端实测：签发 200、错误 PIN 403、限流 429；写库 `source=parent`、`session_id=NULL`、审计记录 |
| TV 放行 | TV v0.2.0 阻挡页每 5s 轮询 active grant，实测 5s 内 `BLOCKED -> GRANTED` |
| 规则生效 | 服务端 `PUT /rule` 校验 PIN/可达性；TV 下次组卷使用新规则 |

### 4.4 目视走查（Android TV 模拟器）

截图见 [`docs/acceptance/phase3/`](phase3/)：

| 页面 | 截图 |
| --- | --- |
| 孩子列表 | `01-child-list.png` |
| 新增孩子 | `02-child-form.png` |
| 规则配置 | `03-rule.png` |
| 规则预览（5 题 / 最高 5.5 分） | `04-rule-preview.png` |
| 规则保存 PIN | `05-rule-pin.png` |
| 设备列表 | `06-devices.png` |
| 远程解锁 | `07-unlock.png` |
| 远程解锁成功（已解锁 10 分钟） | `08-unlock-success.png` |
| 报告空态 | `09-report.png` |
| 设置 | `10-settings.png` |
| 家庭邀请码 | `11-invite.png` |

走查同时验证了真实接口：
`PUT /rule 200`、`POST /rule/preview 200`、`POST /devices/{id}/unlock 200`、`POST /members/invite 200`。

## 5. 已知限制

1. 无实体手机：页面目视验收仅覆盖登录/首页；孩子、设备、规则、报告页的视觉走查待真机补。
2. 模拟器 headless 合成器偶尔冻结画面（Flutter Inspector 可见组件树正常），不影响真机逻辑。
3. Android release 为 debug 签名，正式发布需配置 `keystore.properties` 并使用 HTTPS 地址。
4. iOS 未签名，未做 TestFlight。
5. 推送通知、离线写操作队列未实现（设计中列为后续）。
6. 远程解锁忽略观看时段（产品确认），仅受每日额度与 3 次/小时限流约束。

## 6. 发布与回滚

- 构建：`flutter build apk --release --dart-define=API_BASE_URL=https://<生产域名>/`；
- 签名：仓库根目录 `keystore.properties`（gitignore）或 CI Secret 注入；
- 升级：同签名覆盖安装，登录态保留；
- 回滚：保留上一版 APK 覆盖安装；服务端可解绑设备/吊销授权。

## 7. 后续建议

1. 真机补视觉走查与推送通知；
2. iOS 签名与 TestFlight；
3. 远程解锁增加「忽略时段」开关与二次确认；
4. 离线写操作队列（规则保存、解锁补发）；
5. 与 TV 联合回归：规则变更 → 下次答题生效、远程解锁 → 阻挡页放行。
