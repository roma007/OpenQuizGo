-- +goose Up

-- 080：TV 端回归服务端模式后，答对解锁时长默认值与电视端历史体验（15 分钟）对齐。
-- 仅影响之后新建的 watch_rule；已有家庭的规则值保持不变，家长可在家长端自行调整。
alter table watch_rule alter column unlock_minutes set default 15;

-- +goose Down

alter table watch_rule alter column unlock_minutes set default 20;
