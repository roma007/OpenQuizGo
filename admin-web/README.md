# OpenQuizGo 题库治理后台

Vite + React + TypeScript + Ant Design。

## 开发

```bash
npm install
npm run dev -- --host 127.0.0.1 --port 5175
```

默认代理 `/v1` → `http://127.0.0.1:18080`，可通过 `VITE_PROXY_TARGET` 覆盖：

```bash
VITE_PROXY_TARGET=http://127.0.0.1:8080 npm run dev
```

管理员账号由服务端 CLI 创建：

```bash
go run ./cmd/admin create -username admin -password 'admin12345' -role super_admin -name 管理员
```

## 构建

```bash
npm run build      # 产物 admin-web/dist
```

生产建议与 API 同源托管（Nginx `/admin/` + `/v1/`），避免跨域。

## 页面

- 题目管理：筛选、分页、新建/编辑；
- 工作流：提交审核、通过/驳回、上线、下线、重新提交；
- 批量导入：JSON / CSV；
- 审计日志；
- 覆盖率看板（告警 + CSV 导出）；
- 难度校准（正确率建议、勾选应用）。
