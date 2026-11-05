-- +goose Up

alter table membership_order
    add column external_refund_no text not null default '';

create unique index membership_order_refund_no_idx
    on membership_order (external_refund_no)
    where external_refund_no <> '';

-- +goose Down

drop index if exists membership_order_refund_no_idx;
alter table membership_order drop column if exists external_refund_no;
