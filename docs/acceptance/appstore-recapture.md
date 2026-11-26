# 商店截图重截执行清单（2026-11-25）

> 目标：把 `docs/acceptance/appstore-screenshots/` 换成 076 之后的新布局，iPhone/iPad 各 4 张，供 App Store Connect 直接上传。
> 现状：现有截图 2026-11-11 生成，iPad 那套是旧底部导航；iPhone 那套的标题文案也需替换。

## 1. 需要产出的文件

| 槽位 | 目录 | 尺寸 | 张数 | 内容 |
| --- | --- | --- | --- | --- |
| 灵动岛 iPhone（中等屏） | `iphone-dynamic-island-medium/` | 1206×2622 | 4 | 01 首页档案与设备、02 规则、03 远程解锁、04 排行榜/对战 |
| iPad 13 英寸 | `ipad-13/` | 2064×2752 | 4 | 同上 |
| 大屏 iPhone（可选） | `iphone-6.9/` | 1320×2868 | 4 | 同上，非必需 |

标题沿用现有四句：

1. 管好电视时间，一个 App 就够
2. 答题换时长，规则由你定
3. 不在家，也能临时放行
4. 亲子对战，让学习更有趣

## 2. 本地环境（已验证部分）

| 项 | 状态 |
| --- | --- |
| PostgreSQL 15/16 | ✅ 本机运行，库 `openquizgo` 存在但**未迁移**（无表） |
| Redis | ✅ `PONG` |
| Docker | ❌ 不可用，不走 compose，直接跑 Go 二进制 |
| 模拟器 | ✅ `OQG-iPad13`（75DE4395…）、`SS-iPhone69`（E2F958EF…） |
| Flutter | ✅ `/opt/homebrew/bin/flutter`，`parent/ios/Pods` 已安装 |

## 3. 步骤

```bash
cd ~/我的文档/源码/OpenQuizGo

# 1) 迁移（.env 已指向 openquizgo 库）
make migrate-up

# 2) 起 API（mock 登录，固定验证码 123456）
make run        # 或 go run ./cmd/api

# 3) 起模拟器
xcrun simctl boot 75DE4395-4F38-4987-B7C8-BF52CEE5036C   # iPad 13
xcrun simctl boot E2F958EF-79B0-45F7-9987-8719CCF153E5   # iPhone 6.9

# 4) 跑 App（模拟器内访问本机用 127.0.0.1）
cd parent
flutter run -d 75DE4395-4F38-4987-B7C8-BF52CEE5036C --dart-define=API_BASE_URL=http://127.0.0.1:8080/
flutter run -d E2F958EF-79B0-45F7-9987-8719CCF153E5 --dart-define=API_BASE_URL=http://127.0.0.1:8080/

# 5) 登录 → 用 123456；首次登录自动建家庭
# 6) 逐屏截图
xcrun simctl io <udid> screenshot /tmp/shot-01.png
```

## 4. 数据准备（截图的真实性底线）

- 孩子昵称、设备名必须是演示用假数据，禁止出现真实手机号或真实儿童信息。
- 需要的状态：1 个孩子档案、1 台在线电视、1 套规则、若干答题记录、排行榜里至少 2 个家庭（亲子对战页）。
- 若题库为空，先在管理后台 `quiz.openapp.asia` 或在本地 `admin` 命令行建 1 个专题 + 5 道题。

## 5. 合成与验收

- 截图只允许"真实 App 界面 + 统一标题条 + 统一背景"，不得伪造功能、评分或用户数据。
- 合成后按上表尺寸核对：`sips -g pixelWidth -g pixelHeight <file>`。
- 验收通过后替换目录内容，更新 `appstore-assets.md` 的日期与说明，并把旧图挪到 `appstore-screenshots/archive-2026-11-11/`。
- 官网产品页的截图（`src/assets/products/openquizgo/`）同步换成同一批，避免商店与官网不一致。

## 6. 执行结果（2026-11-25）

- 已完成：iPhone 6.9（1320×2868）、灵动岛中等屏（1206×2622）、iPad 13（2064×2752）各 4 张，旧图归档 `appstore-screenshots/archive-2026-11-11/`。
- 演示数据：本地 API（mock 短信，验证码 123456）创建家庭「小宇（三年级）/ 小满（二年级）」、客厅电视、观看规则、6 场亲子对战与 1 次临时放行；无真实手机号或儿童信息。
- 官网 `src/assets/products/openquizgo/` 已同步同一批图（iPhone ×4、iPad ×2），构建并发布到 https://www.openapp.asia 。
- 环境备注：本机 Xcode 仅支持 iOS 15+ 部署目标，截图时临时把 `parent/ios/Podfile` 的平台与 Pods 部署目标升到 15.0，截取完成后已还原（未提交）。
