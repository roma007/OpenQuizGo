-- +goose Up

create table membership_invoice (
    id           uuid        primary key default gen_random_uuid(),
    order_id     uuid        not null unique references membership_order (id) on delete cascade,
    account_id   uuid        not null references parent_account (id) on delete cascade,
    title_type   text        not null default 'personal',
    title        text        not null,
    tax_no       text        not null default '',
    email        text        not null default '',
    amount_cents integer     not null,
    status       text        not null default 'requested',
    remark       text        not null default '',
    created_at   timestamptz not null default now(),
    issued_at    timestamptz,
    updated_at   timestamptz not null default now(),
    constraint membership_invoice_type_chk check (title_type in ('personal', 'company')),
    constraint membership_invoice_status_chk check (status in ('requested', 'issued', 'rejected')),
    constraint membership_invoice_amount_chk check (amount_cents >= 0)
);

create index membership_invoice_account_idx on membership_invoice (account_id, created_at desc);
create index membership_invoice_status_idx on membership_invoice (status, created_at desc);

-- +goose Down

drop table if exists membership_invoice;
