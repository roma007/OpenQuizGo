# 043 管理端 Playwright E2E 设计

| 项目 | 内容 |
| --- | --- |
| 编号 | 043 |
| 版本 | v1 |
| 状态 | done |
| 作者 | AI |
| 创建日期 | 2026-11-05 |
| 对应计划 | [043-管理端Playwright-E2E实施计划](../plans/043-管理端Playwright-E2E实施计划.md) |
| 前置 | 005 管理端、039 家庭设备 |

## 1. 目标

为管理端增加 Playwright 冒烟 E2E：

- 登录；
- 运营概览；
- 题目列表；
- 专题题包；
- 会员套餐/订单；
- 家庭设备；
- 审计日志。

只读用例，不修改业务数据。

## 2. 运行

```bash
cd admin-web
npm run test:e2e
```

- 默认 `BASE_URL=http://127.0.0.1:5174`；
- 需要 API 与 dev server 已启动；
- `ADMIN_USER`/`ADMIN_PASSWORD` 默认 admin/admin12345。

## 3. 方案

- `@playwright/test` + Chromium；
- `playwright.config.ts` 单 worker、无重试、失败截图；
- 登录后复用 storageState；
- 每个页面断言标题/关键文案，不依赖易变数据。

## 4. 风险

| 风险 | 对策 |
| --- | --- |
| 浏览器未安装 | 文档说明 `npx playwright install chromium` |
| 数据变化 | 只断言标题/表头 |
| 服务未启动 | 配置 `BASE_URL` 与 README |

## 5. 后续

- CI 集成；
- 写操作用例与清理；
- 视觉回归。

## 6. 变更记录

| 日期 | 变更 | 原因 |
| --- | --- | --- |
| 2026-11-05 | 初稿并批准 | 管理端质量 |
