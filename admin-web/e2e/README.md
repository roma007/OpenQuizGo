# 管理端 E2E（Playwright）

只读冒烟用例：登录 → 运营概览 → 题目管理 → 专题题包 → 会员套餐 → 家庭设备 → 审计日志。

## 前置

1. 启动 API（默认 `http://127.0.0.1:18080`）。
2. 启动管理端 dev server（默认 `http://127.0.0.1:5174`）。
3. 首次运行安装 Chromium：

   ```bash
   npx playwright install chromium
   ```

## 运行

```bash
cd admin-web
npm run test:e2e
```

可用环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `BASE_URL` | `http://127.0.0.1:5174` | 管理端地址 |
| `ADMIN_USER` | `admin` | 管理员账号 |
| `ADMIN_PASSWORD` | `admin12345` | 管理员密码 |

## 说明

- 单 worker 串行执行，用例之间互不依赖；
- 登录态由 `auth.setup.spec.ts` 写入 storageState（`node_modules/.cache/openquizgo-e2e/admin.json`），业务用例复用，避免每个用例重复登录；
- 失败时截图与 trace 写入 `node_modules/.cache/openquizgo-e2e/test-results/`（运行 `npx playwright show-trace <trace.zip>` 查看），避免产物进入仓库；
- 只断言标题、表头与静态文案，不依赖易变业务数据，不产生写操作。
