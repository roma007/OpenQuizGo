# 轻量变更登记（BACKLOG）

按 `AGENTS.md` 铁律 2 的例外条款，免设计与计划的轻量变更在此登记一行。

| 日期 | 变更内容 | 类型 | 原因 | 执行人 |
| --- | --- | --- | --- | --- |
| 2026-11-02 | 建立项目规范、文档模板与对话归档脚本 | 规范/文档 | 落实项目铁律 | AI |
| 2026-11-04 | 修复 GitHub Actions：升级 Actions、固定 ubuntu-24.04、config 测试解耦环境 | CI/测试 | CI 红；root cause 为 APP_ENV=ci 时缺少 JWT_SECRET | AI |
| 2026-11-05 | 同步 009–022 新增接口到 OpenAPI 契约 | 文档 | 实现已先行、契约补档 | AI |
| 2026-11-05 | 新增 `scripts/auto-continue.sh` 与使用说明 | 工具/文档 | 10 分钟无人值守自动推进，不改变业务行为 | AI |
| 2026-11-06 | 退款时间戳截断到微秒 | 缺陷修复 | CI 中 PostgreSQL 微秒精度导致幂等退款断言失败（macOS 本地为微秒精度未复现） | AI |
| 2026-11-08 | tv-ci Gradle 下载 504 时切换镜像重试 | CI/构建 | 官方分发包重定向 GitHub Releases 返回 504；同时修正 bash -e 导致重试未执行 | AI |
| 2026-11-08 | 答题提示补充 90 秒自动答题与重播风险 | 文案/TV | 让用户提前知道不按 OK 的后果；TV 0.4.2 | AI |
| 2026-11-11 | iOS Info.plist 声明 `ITSAppUsesNonExemptEncryption=false` | 构建/上架 | App 仅使用 HTTPS/系统 Keychain 标准加密，避免每次上传被要求提供加密文稿 | AI |
| 2026-11-11 | 首页孩子卡片改为「身份信息 + 操作按钮」两行布局 | UI 优化 | 四个操作按钮挤压标题，iPhone 上副标题折行 | AI |

类型：错别字 / 注释 / 文档 / 无行为变化修改
