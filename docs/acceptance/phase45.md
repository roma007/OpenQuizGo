# 045 管理令牌 HttpOnly Cookie 与 CSP 验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.40.0（管理端 0.40.0） |
| 日期 | 2026-11-05 |
| 范围 | 045：管理端令牌 Cookie 化、CSRF 头、Logout、CSP |
| 设计/计划 | [设计](../design/045-管理令牌Cookie与CSP设计.md) / [计划](../plans/045-管理令牌Cookie与CSP实施计划.md) |

## 1. 结论

**通过。** 管理端令牌改为 HttpOnly Cookie，前端不再读写 `localStorage`；Cookie 写请求要求 `X-Admin-Request: 1`，新增幂等 logout；管理端页面增加 CSP meta。Playwright 7/7 通过。

## 2. 交付

- 登录设置 `openquizgo_admin` Cookie：`HttpOnly`、`SameSite=Strict`、`Path=/v1`、`Max-Age=28800`，HTTPS 或 `X-Forwarded-Proto=https` 时 `Secure`；
- 鉴权 Bearer 优先、Cookie 兜底；
- Cookie 鉴权的非 GET/HEAD/OPTIONS 请求缺少 `X-Admin-Request: 1` 返回 403；
- `POST /v1/admin/auth/logout` 幂等清除 Cookie；
- 前端 `credentials: 'include'`，移除 localStorage 令牌；
- `index.html` CSP：`default-src 'self'`，样式允许 `'unsafe-inline'`，开发态允许 ws/wss。

## 3. 验证

```text
gofmt -w internal/admin                    passed
go test ./internal/admin                   passed
go build ./...                             passed
cd admin-web && npm run build              passed
cd admin-web && npm run test:e2e           7 passed
```

接口实测：

```text
Set-Cookie: openquizgo_admin=...; Path=/v1; Max-Age=28800; HttpOnly; SameSite=Strict
登录后仅 Cookie 访问 /admin/me             → 200
Cookie 创建题目但缺 X-Admin-Request         → 403
带 X-Admin-Request 创建题目                 → 201
logout                                     → 200
logout 后 /admin/me                        → 401
```

单元测试覆盖 Cookie 属性、Secure 判定、Bearer 优先、CSRF 头、logout。

## 4. 限制

- 未实现刷新令牌与会话吊销；
- `Secure` 依赖反向代理正确传递 `X-Forwarded-Proto`；
- CSP 为 meta 级，生产建议在反向代理补充安全头；
- 旧版浏览器 localStorage 中的历史令牌不再被读取，但不会主动清除。

## 5. 后续

- 刷新令牌/会话管理；
- 生产 nginx 安全头模板；
- 完整 CSRF token（如需跨站场景）。
