-- +goose Up

create table membership_auto_renew (
    account_id    uuid        primary key references parent_account (id) on delete cascade,
    plan_id       uuid        not null references membership_plan (id) on delete restrict,
    enabled       boolean     not null default false,
    last_order_at timestamptz,
    updated_at    timestamptz not null default now()
);

create index membership_auto_renew_due_idx
    on membership_auto_renew (enabled, last_order_at)
    where enabled;

-- +goose Down

drop table if exists membership_auto_renew;
