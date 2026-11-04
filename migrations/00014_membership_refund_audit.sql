-- +goose Up

alter table membership_order add column if not exists refund_reason text not null default '';
alter table membership_order add column if not exists refunded_by uuid;

-- +goose Down

alter table membership_order drop column if exists refunded_by;
alter table membership_order drop column if exists refund_reason;
