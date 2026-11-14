# Prometheus 配置（预留，当前未启用）

生产服务器是 2C2G，Prometheus 常驻约 80–120MB，已从 `docker-compose.yml` 移除，
本目录的 `prometheus.yml` / `alerts.yml` 仅作后续扩容时的参考，**当前不参与部署**。

- 现在的指标入口：`api` 的 `/metrics`，默认只允许容器网络/本机访问；
  nginx 的 `quiz.conf` 里 `location /metrics` 显式 `deny all`。
- 需要临时看指标时，用 SSH 隧道或 `docker compose run` 抓一次，不要常驻。
- 扩容后再启用：把 Prometheus 服务加回 compose，并给容器设置 `mem_limit`。

相关文档：[docs/deployment.md](../../docs/deployment.md) 第 7 节。
