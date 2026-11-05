# 045 管理令牌 HttpOnly Cookie 与 CSP 设计

| 项目 | 内容 |
| --- | --- |
| 编号 | 045 |
| 版本 | v1 |
| 状态 | done |
| 作者 | AI |
| 创建日期 | 2026-11-05 |
| 对应计划 | [045-管理令牌Cookie与CSP实施计划](../plans/045-管理令牌Cookie与CSP实施计划.md) |
| 前置 | 005 管理端、039 家庭设备 |

## 1. 目标

管理端令牌不再存入 `localStorage`，改用 HttpOnly Cookie，降低 XSS 窃取风险；同时为管理端页面增加 CSP。

- 登录成功后服务端 `Set-Cookie`；
- 浏览器请求自动携带 Cookie；
- Bearer 方式继续支持脚本/第三方客户端；
- Cookie 鉴权的写请求要求自定义头，防 CSRF；
- 管理端 `index.html` 增加 CSP meta。

## 2. 服务端

- Cookie 名 `openquizgo_admin`，`HttpOnly`、`SameSite=Strict`、`Path=/v1`、`Max-Age` 与令牌一致；
- HTTPS 或 `X-Forwarded-Proto=https` 时加 `Secure`；
- 鉴权顺序：`Authorization: Bearer` → Cookie；
- Cookie 鉴权的非 GET 请求必须带 `X-Admin-Request: 1`，否则 403；
- 新增 `POST /v1/admin/auth/logout` 清除 Cookie（幂等）；
- 登录响应保留 `token` 字段，兼容旧客户端。

## 3. 前端

- `api.ts` 移除 `localStorage` 令牌读写，统一 `credentials: 'include'`；
- 所有请求带 `X-Admin-Request: 1`；
- 登录成功不保存 token；退出调用 logout；
- `auth.setup.spec.ts` 仍通过 Cookie 复用登录态。

## 4. CSP

`index.html` 增加：

```text
default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:; font-src 'self' data:;
connect-src 'self' http: https: ws: wss:; object-src 'none'; base-uri 'self'
```

开发态允许 `ws:` 供 Vite HMR；生产部署文档补充反向代理安全头。

## 5. 风险

| 风险 | 对策 |
| --- | --- |
| CSRF | SameSite=Strict + 自定义头 |
| 旧客户端 | 保留 Bearer 与登录 token 字段 |
| 本地 HTTP 无 Secure | 仅 HTTPS 设置 Secure |
| CSP 阻断内联样式 | style-src 允许 'unsafe-inline' |

## 6. 后续

- 刷新令牌/会话吊销；
- 生产 nginx 安全头模板；
- 完整 CSRF token。

## 7. 变更记录

| 日期 | 变更 | 原因 |
| --- | --- | --- |
| 2026-11-05 | 初稿并批准 | 管理端安全加固 |
