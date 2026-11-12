# 076-iOS 与 iPad 适配实施计划

| 项目 | 内容 |
| --- | --- |
| 编号 | 076 |
| 状态 | done |
| 负责人 | AI |
| 创建日期 | 2026-11-12 |
| 最后更新 | 2026-11-12 |
| 对应设计 | [076-iOS与iPad适配设计](../design/076-iOS与iPad适配设计.md) |

## 1. 目标与范围

- 交付目标：家长端在 iPad 全尺寸与分屏下使用侧边导航和自适应栅格；iOS 品牌资产与显示名称补齐；iPhone 布局不回归。
- 范围边界：仅 `parent/lib`、`parent/test`、`parent/ios`、相关文档；不改服务端、TV 端、管理端与接口。
- 前置条件：Flutter 3.44.4、Xcode 26.6、CocoaPods 可用；现有 51 个单测通过。

## 2. 任务拆分

| 序号 | 任务 | 依赖 | 产出物 | 验收标准 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T1 | 设计与计划 | — | 076 设计、计划 | 状态 approved | done |
| T2 | 扩展自适应工具 | T1 | `adaptive.dart`、`adaptive_shell.dart` | `formFactorOf`/`gridColumns` 可用，analyze 通过 | done |
| T3 | 导航与页面改造 | T2 | `home_shell.dart`、`child_page.dart`、`device_page.dart`、`report_page.dart` | iPad 竖屏两列 + 侧栏；横屏三列；报告宽屏分栏 | done |
| T4 | iOS 品牌资产与配置 | T1 | 图标 15 张（18 个槽位）、启动图 3 张、`Info.plist` | 图标无 alpha；名称为「OpenQuizGo 家长端」；启动图居中 | done |
| T5 | 自动化测试 | T3 | `adaptive_test.dart`、`home_shell_test.dart` | 手机/平板/分屏/横屏用例全绿 | done |
| T6 | 构建与验收 | T4、T5 | analyze/test/build 结果 | analyze 0 issue，测试全绿，iOS 模拟器构建成功 | done |
| T7 | 文档、版本与归档 | T6 | ROADMAP、README、008 修订、归档 | 版本自增、双远端同步 | done |

状态取值：`todo` / `doing` / `done` / `blocked` / `cancelled`

## 3. 里程碑

| 里程碑 | 包含任务 | 完成标准 | 计划日期 | 实际日期 |
| --- | --- | --- | --- | --- |
| M1 | T1–T3 | 布局改造完成，本地测试通过 | 2026-11-12 | 2026-11-12 |
| M2 | T4–T5 | iOS 资产 + 测试覆盖 | 2026-11-12 | 2026-11-12 |
| M3 | T6–T7 | 构建、文档、归档与发布 | 2026-11-12 | 2026-11-12 |

## 4. 验证方式

- 单元/Widget 测试：`cd parent && flutter test`
- 静态检查：`cd parent && flutter analyze`
- iOS 构建：`cd parent && flutter build ios --simulator --debug`
- 手工验收：
  1. iPhone 15（390×844）→ 底部导航、单列；
  2. iPhone 横屏（844×390）→ 底部导航；
  3. iPad 10.9 竖屏（820×1180）→ 侧栏 + 两列；
  4. iPad Pro 13 横屏（1366×1024）→ 展开侧栏 + 三列；
  5. iPad Slide Over（320×1024）→ 底部导航、单列；
  6. 桌面图标与启动图为项目 Logo，桌面名称中文。

## 5. 回滚方案

- 触发条件：iPad 布局出现溢出、iPhone 布局回归或 iOS 构建失败且无法在 1 小时内修复。
- 回滚步骤：`git revert` 本次提交；或回退 `adaptive_shell.dart`、`home_shell.dart`、三处栅格阈值与 iOS 资产到 1.0.1 状态。
- 数据兼容性：无数据结构、无接口、无本地存储格式变化，回滚无需迁移。

## 6. 执行记录

> 每完成一项任务或发生偏差时追加，不删除历史记录。

| 日期 | 内容 | 结果/偏差 |
| --- | --- | --- |
| 2026-11-12 | T1 设计与计划创建 | approved |
| 2026-11-12 | 基线检查 | analyze/test 通过（51 个单测）；iOS 工程包含 iPad 设备族，图标与启动图仍为 Flutter 默认资源 |
| 2026-11-12 | T2 自适应工具 | 新增 `FormFactor`/`gridColumns`/`AdaptiveShell`，断点集中管理 |
| 2026-11-12 | T3 导航与页面 | 平板统一侧栏；孩子/设备改为 640/1080 两档；报告 900 分栏 |
| 2026-11-12 | T4 iOS 资产 | Logo 压平生成 15 张图标（18 个槽位）；启动图 168/336/504；名称改为「OpenQuizGo 家长端」 |
| 2026-11-12 | T5 测试 | 新增 `adaptive_test.dart`、`home_shell_test.dart`，覆盖手机/平板/分屏 |
| 2026-11-12 | T6 构建验收 | `dart analyze lib test` 0 issue（`flutter analyze` 在本机中文路径下 LSP 崩溃，改用 dart analyze）；59 个测试全绿；`flutter build ios --simulator` 成功；iPad 13 模拟器安装运行正常 |
| 2026-11-12 | T7 文档发布 | ROADMAP/README/008 更新，版本 1.0.2，双远端同步 |

## 7. 验收结论

- 结论：通过
- 遗留问题：Apple 签名 / TestFlight / App Store 素材仍依赖开发者账号；未做真机走查。
- 后续建议：取得证书后补 iPad 真机与 TestFlight 验收；评估 iPad 多窗口与键盘快捷键。
