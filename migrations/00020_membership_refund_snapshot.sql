-- +goose Up

alter table membership_order add column if not exists refund_before jsonb;
alter table membership_order add column if not exists refund_after jsonb;

-- +goose Down

alter table membership_order drop column if exists refund_after;
alter table membership_order drop column if exists refund_before;
