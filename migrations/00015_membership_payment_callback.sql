-- +goose Up

alter table membership_order add column if not exists external_trade_no text not null default '';

-- +goose Down

alter table membership_order drop column if exists external_trade_no;
