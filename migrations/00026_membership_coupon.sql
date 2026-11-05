-- +goose Up

create table membership_coupon (
    id               uuid        primary key default gen_random_uuid(),
    code             text        not null unique,
    name             text        not null,
    amount_cents     integer     not null,
    min_amount_cents integer     not null default 0,
    starts_at        timestamptz,
    expires_at       timestamptz,
    max_uses         integer     not null default 0,
    used_count       integer     not null default 0,
    status           text        not null default 'active',
    created_at       timestamptz not null default now(),
    updated_at       timestamptz not null default now(),
    constraint membership_coupon_amount_chk check (amount_cents > 0),
    constraint membership_coupon_min_chk check (min_amount_cents >= 0),
    constraint membership_coupon_max_chk check (max_uses >= 0 and used_count >= 0),
    constraint membership_coupon_status_chk check (status in ('active', 'disabled')),
    constraint membership_coupon_time_chk check (
        starts_at is null or expires_at is null or expires_at > starts_at
    )
);

alter table membership_order
    add column original_amount_cents integer not null default 0,
    add column discount_cents integer not null default 0,
    add column coupon_code text not null default '';

update membership_order set original_amount_cents = amount_cents where original_amount_cents = 0;

-- +goose Down

alter table membership_order
    drop column if exists coupon_code,
    drop column if exists discount_cents,
    drop column if exists original_amount_cents;
drop table if exists membership_coupon;
