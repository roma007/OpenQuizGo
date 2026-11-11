# App Store 截图与 App 预览

| 项目 | 内容 |
| --- | --- |
| 截图目录 | [`appstore-screenshots/`](appstore-screenshots/) |
| App 预览 | [`appstore-preview/OpenQuizGo-AppPreview-886x1920.mp4`](appstore-preview/OpenQuizGo-AppPreview-886x1920.mp4) |
| 打包下载 | `dist/OpenQuizGo-AppStore-assets.zip`（本地生成，不在仓库） |
| 录制环境 | iPhone 6.9 英寸模拟器、iPad 13 英寸模拟器、真实 Flutter 构建 |

## 1. 截图

### iPhone 6.9 英寸

尺寸：1320×2868，共 4 张。

| 顺序 | 文件 | 标题 | 展示内容 |
| --- | --- | --- | --- |
| 1 | [`iphone-6.9/01.png`](appstore-screenshots/iphone-6.9/01.png) | 管好电视时间，一个 App 就够 | 孩子档案、设备状态、观看报告 |
| 2 | [`iphone-6.9/02.png`](appstore-screenshots/iphone-6.9/02.png) | 答题换时长，规则由你定 | 解锁时长、每日上限、允许时段 |
| 3 | [`iphone-6.9/03.png`](appstore-screenshots/iphone-6.9/03.png) | 不在家，也能临时放行 | 家长 PIN 远程解锁 |
| 4 | [`iphone-6.9/04.png`](appstore-screenshots/iphone-6.9/04.png) | 亲子对战，让学习更有趣 | 家庭排行榜、对战记录 |

### iPad 13 英寸

尺寸：2064×2752，共 4 张。

| 顺序 | 文件 |
| --- | --- |
| 1 | [`ipad-13/01.png`](appstore-screenshots/ipad-13/01.png) |
| 2 | [`ipad-13/02.png`](appstore-screenshots/ipad-13/02.png) |
| 3 | [`ipad-13/03.png`](appstore-screenshots/ipad-13/03.png) |
| 4 | [`ipad-13/04.png`](appstore-screenshots/ipad-13/04.png) |

说明：

- 截图来自真实 App UI，仅添加标题和统一背景；
- 未使用「设置 / 会员中心」页面，避免审核看到「模拟支付」；
- 未包含 TV 端画面，iOS 截图只展示 iOS App 本身。

## 2. App 预览

| 项目 | 内容 |
| --- | --- |
| 文件 | [`OpenQuizGo-AppPreview-886x1920.mp4`](appstore-preview/OpenQuizGo-AppPreview-886x1920.mp4) |
| 尺寸 | 886×1920（苹果推荐的 6.9 英寸预览尺寸） |
| 时长 | 27.4 秒 |
| 编码 | H.264，30fps，无音频 |
| 大小 | 约 1.1 MB |

内容顺序：

1. 手机号 + 验证码登录；
2. 设置观看规则；
3. 家长远程解锁；
4. 亲子对战与排行榜。

App 预览是**可选**的，不上传也可以提交；上传后会显示在产品页截图之前。

## 3. 上传到 App Store Connect

1. 打开 App Store Connect → 你的 App → iOS App 版本页；
2. 找到「App 预览和截屏」；
3. 先选择 **iPhone 6.9 英寸**，拖入 `iphone-6.9/01–04.png`；
4. 再选择 **iPad 13 英寸**，拖入 `ipad-13/01–04.png`；
5. 如需 App 预览，在 iPhone 6.9 英寸下上传 `OpenQuizGo-AppPreview-886x1920.mp4`；
6. 顺序按文件名排序，第一张会出现在搜索结果中。

如果页面上还出现 6.5 英寸或 12.9 英寸的槽位，可以用现有图片缩放生成；当前苹果以 6.9 英寸和 13 英寸为主，通常不需要额外尺寸。

## 4. 注意

- 截图中孩子昵称为测试数据「T2孩子」，上线前可替换为更中性的示例昵称；
- 第 3 张截图中的家长 PIN 为空，未展示真实 PIN；
- App 预览无音频，审核员静音观看也能理解；
- 截图和预览均未展示会员购买入口。
