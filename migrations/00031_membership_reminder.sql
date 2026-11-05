-- +goose Up

create table membership_reminder (
    id          uuid        primary key default gen_random_uuid(),
    account_id  uuid        not null references parent_account (id) on delete cascade,
    plan_name   text        not null default '',
    expires_at  timestamptz not null,
    days_before integer     not null,
    status      text        not null default 'unread',
    created_at  timestamptz not null default now(),
    constraint membership_reminder_days_chk check (days_before in (30, 7, 1)),
    constraint membership_reminder_status_chk check (status in ('unread', 'read')),
    unique (account_id, expires_at, days_before)
);

create index membership_reminder_account_idx
    on membership_reminder (account_id, status, created_at desc);

-- +goose Down

drop table if exists membership_reminder;
