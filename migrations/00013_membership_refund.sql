-- +goose Up

alter table membership_order add column if not exists refunded_at timestamptz;

alter table membership_order drop constraint if exists membership_order_status_check;
alter table membership_order
    add constraint membership_order_status_check
    check (status in ('pending', 'paid', 'cancelled', 'refunded'));

-- +goose Down

update membership_order set status = 'paid' where status = 'refunded';
alter table membership_order drop constraint if exists membership_order_status_check;
alter table membership_order
    add constraint membership_order_status_check
    check (status in ('pending', 'paid', 'cancelled'));
alter table membership_order drop column if exists refunded_at;
