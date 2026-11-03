-- +goose Up

-- 上次心跳时间：服务端据此计算计费增量，避免客户端虚报
alter table watch_grant add column last_heartbeat_at timestamptz not null default now();

-- +goose Down

alter table watch_grant drop column if exists last_heartbeat_at;
