# 076-iOS 与 iPad 适配设计

| 项目 | 内容 |
| --- | --- |
| 编号 | 076 |
| 状态 | done |
| 作者 | AI |
| 创建日期 | 2026-11-12 |
| 最后更新 | 2026-11-12 |
| 对应计划 | [076-iOS与iPad适配实施计划](../plans/076-iOS与iPad适配实施计划.md) |
| 前置 | 008 家长端平板适配（v0.8.0）、家长端 1.0.1 |

## 1. 背景与目标

### 1.1 现状与问题

- 家长端是 Flutter 单代码库，Android 已发布，iOS 工程可直接编译，但只做了「能跑」级别适配：
  - `TARGETED_DEVICE_FAMILY = "1,2"`，iPad 已包含在安装目标内；
  - 导航仅在逻辑宽度 ≥ 1100 时切换为 `NavigationRail`，iPad 竖屏（744–1024）仍使用底部导航，横向空间被浪费；
  - 孩子/设备卡片要到 900 才两列、1400 才三列，iPad 竖屏在侧边栏存在时经常只有一列；
  - 报告页左右分栏阈值 1000，iPad 竖屏无法分栏；
  - iOS App 图标仍是 Flutter 默认图标，启动图是 1×1 透明占位，桌面名称是 `Openquizgo Parent`；
  - 008 的断点只按「宽度」判定，无法区分 iPhone 横屏与 iPad 竖屏（两者宽度区间重叠）。
- iPad 支持 Split View / Slide Over / Stage Manager，窗口宽度可窄至 320，需要随窗口尺寸实时重排。

### 1.2 本次目标（可验证）

1. iOS 工程具备可发布的品牌资产：App 图标、启动图、中文桌面名称。
2. 用「最短边」识别平板，iPad 全尺寸使用侧边导航；iPhone（含横屏）保持底部导航。
3. iPad 内容区自适应：竖屏两列、横屏三列、报告页宽屏左右分栏。
4. iPad 分屏 /  Slide Over 窄窗口自动退回手机布局，不出现挤压或溢出。
5. `flutter analyze`、`flutter test`、`flutter build ios --simulator` 全部通过。

### 1.3 非目标（明确不做什么）

- Apple 开发者签名、TestFlight、App Store 提交流程（需证书，仍属外部阻塞）；
- APNs 推送、Apple IAP、Sign in with Apple；
- 原生 SwiftUI 页面、iPad 多窗口（多 Scene）、Apple Pencil / 键盘快捷键专项优化；
- 改动服务端接口或数据结构。

## 2. 需求与验收标准

| 编号 | 需求 | 验收标准 | 优先级 |
| --- | --- | --- | --- |
| R1 | 设备形态识别 | 最短边 ≥ 600 判定为平板；iPhone 横屏（如 844×390）仍为手机 | P0 |
| R2 | iPad 导航 | 平板全部尺寸显示 `NavigationRail`，宽度 ≥ 1200 时展开显示文字标签 | P0 |
| R3 | iPhone 导航不回归 | 手机尺寸显示底部 `NavigationBar`，无 `NavigationRail` | P0 |
| R4 | 栅格自适应 | 内容宽度 < 640 单列、640–1079 两列、≥ 1080 三列 | P0 |
| R5 | 报告分栏 | 内容宽度 ≥ 900 且历史报告 ≥ 1 条时左右分栏 | P1 |
| R6 | 分屏窄窗口 | 窗口宽 320 时使用底部导航与单列，无溢出异常 | P1 |
| R7 | iOS 品牌资产 | 图标为项目 Logo（无 alpha）、启动图为 Logo、桌面名称为「OpenQuizGo 家长端」 | P0 |
| R8 | iPad 方向 | iPad 支持四个方向；iPhone 保持竖屏 + 左右横屏 | P1 |
| R9 | 质量门禁 | analyze 0 issue、全部单测通过、iOS 模拟器构建成功 | P0 |

## 3. 方案对比与选型

| 方案 | 优点 | 缺点 | 结论 |
| --- | --- | --- | --- |
| A. 仅按宽度断点（现状） | 改动最小 | iPhone 横屏与 iPad 竖屏混淆；iPad 竖屏仍用底部导航 | 否决 |
| B. 按最短边识别平板 + 宽度决定栅格 | 判定稳定，手机/平板语义清晰，兼容分屏 | 需要新增一个布局工具与测试 | **采用** |
| C. 为 iPad 单独维护一套原生/页面 | 最贴近 HIG | 双份代码，维护成本高，与 Flutter 单代码库策略冲突 | 否决 |
| D. 引入第三方 adaptive 框架 | 开箱即用 | 增加依赖与学习成本，本项目布局诉求简单 | 否决 |

选型理由：方案 B 在现有 `core/layout/adaptive.dart` 上增量演进，纯 Dart、无新依赖，可被 Widget 测试完整覆盖，同时保留 iPhone 现有体验。

## 4. 总体架构

### 4.1 模块划分与职责

| 模块 | 职责 |
| --- | --- |
| `core/layout/adaptive.dart` | `FormFactor` 判定、断点常量、`gridColumns`、`ContentFrame` |
| `core/layout/adaptive_shell.dart` | 统一导航骨架：手机底部导航 / 平板侧边导航，供 `HomeShell` 使用 |
| `features/shell/home_shell.dart` | 组装导航项、消息入口、`MembershipBanner` 与页面 `IndexedStack` |
| `features/child`、`features/device` | 使用统一 `gridColumns` 决定卡片列数 |
| `features/report` | 使用 `Breakpoints.twoPane` 决定左右分栏 |
| `ios/Runner` | 图标、启动图、Info.plist 名称与方向声明 |

### 4.2 关键流程

```text
MediaQuery.size
   │
   ├─ shortestSide ≥ 600 ? ── 否 ──> 手机：底部 NavigationBar + 单列
   │        是
   └─> 平板：NavigationRail
             ├─ width ≥ 1200 → 展开（图标 + 文字）
             └─ width < 1200 → 紧凑（仅图标，仍显示标签）
       内容区宽度 W
             ├─ W < 640    → 1 列
             ├─ 640–1079   → 2 列
             └─ ≥ 1080     → 3 列
```

窗口尺寸变化（旋转、分屏、Stage Manager 缩放）由 `MediaQuery`/`LayoutBuilder` 驱动，无需监听原生事件。

### 4.3 断点定义

| 常量 | 值 | 用途 |
| --- | --- | --- |
| `tabletShortestSide` | 600 | 平板识别（最短边） |
| `twoColumn` | 640 | 卡片两列 |
| `twoPane` | 900 | 报告左右分栏 |
| `threeColumn` | 1080 | 卡片三列 |
| `extendedRail` | 1200 | 侧边栏展开 |
| `contentMax` | 1200 | 内容最大宽度 |
| `formMax` | 720 | 表单最大宽度 |
| `listMax` | 1000 | 长列表最大宽度 |

> 008 中「720–1099 使用底部导航」的规则由本设计替代；008 的手机/表单/限宽结论继续有效。

### 4.4 目录结构影响

```text
parent/lib/core/layout/
├── adaptive.dart          # 扩展：FormFactor / gridColumns / 新断点
└── adaptive_shell.dart    # 新增：统一导航骨架
parent/test/adaptive_test.dart      # 新增
parent/test/home_shell_test.dart    # 由 smoke_test 中的宽屏用例迁出并扩充
```

无服务端、数据库、接口契约变化。

## 5. 数据模型

不涉及持久化数据模型；仅新增 Dart 枚举 `FormFactor { phone, tablet }` 与常量。

## 6. 接口设计

不涉及 HTTP 接口。内部组件约定：

| 类型 | 名称 | 入参 | 出参 | 说明 |
| --- | --- | --- | --- | --- |
| Dart 函数 | `formFactorOf` | `Size` | `FormFactor` | 按最短边判定 |
| Dart 函数 | `gridColumns` | `double width` | `int` | 1/2/3 列 |
| Widget | `AdaptiveShell` | `appBar`、`body`、`items`、`selectedIndex`、`onSelected` | Widget | 手机/平板导航切换 |

错误码与异常约定：无。

## 7. 非功能要求

- 性能：仅增加一次 `MediaQuery.sizeOf` 与轻量 `LayoutBuilder`，无额外重建热点。
- 安全与权限：不新增权限；继续使用 Keychain（`flutter_secure_storage`）与 HTTPS。
- 可观测性：不新增埋点；布局异常通过 Flutter 溢出报错在测试中暴露。
- 兼容性与可维护性：iOS 13+；所有断点集中在一个文件，新增页面复用同一套函数。

## 8. 风险与对策

| 风险 | 影响 | 概率 | 对策 |
| --- | --- | --- | --- |
| 平板判定导致 iPhone 横屏误用侧栏 | 体验回退 | 低 | 以最短边判定；补 844×390 用例 |
| 侧栏占用宽度后卡片过窄 | 文本折行 | 中 | 栅格阈值按「内容区宽度」而非屏幕宽度计算 |
| 图标含 alpha 被 App Store 拒绝 | 上架失败 | 中 | 生成时压平到品牌深色底并校验 `hasAlpha: no` |
| 启动图拉伸 | 首屏变形 | 低 | 输出正方形 1x/2x/3x，`contentMode=center` 居中 |
| 分屏恢复后状态错乱 | 页面错位 | 低 | 导航索引保存在 State，布局只做响应式重排 |

## 9. 未决问题

- [ ] Apple 开发者账号与签名（外部阻塞，见 BACKLOG 3.2）。
- [ ] 是否上架 App Store 及 iPad 截图素材（依赖签名与发行策略）。
- [ ] 后续是否启用 iPad 多窗口（多 Scene）。

## 10. 变更记录

| 日期 | 变更内容 | 原因 |
| --- | --- | --- |
| 2026-11-12 | 初稿并批准 | iOS/iPad 适配启动 |
