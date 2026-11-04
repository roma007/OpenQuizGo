-- +goose Up

create table if not exists membership_plan (
    id            uuid primary key default gen_random_uuid(),
    code          text not null unique,
    name          text not null,
    description   text not null default '',
    duration_days int not null check (duration_days between 1 and 3650),
    price_cents   int not null check (price_cents >= 0),
    features      jsonb not null default '[]'::jsonb check (jsonb_typeof(features) = 'array'),
    status        text not null default 'draft'
        check (status in ('draft', 'online', 'offline')),
    sort          int not null default 0,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create table if not exists membership_order (
    id            uuid primary key default gen_random_uuid(),
    order_no      text not null unique,
    account_id    uuid not null references parent_account (id) on delete cascade,
    plan_id       uuid not null references membership_plan (id),
    plan_name     text not null,
    amount_cents  int not null,
    duration_days int not null,
    status        text not null default 'pending'
        check (status in ('pending', 'paid', 'cancelled')),
    channel       text not null default 'mock',
    created_at    timestamptz not null default now(),
    paid_at       timestamptz
);

create index if not exists membership_order_account_idx
    on membership_order (account_id, created_at desc);

create table if not exists account_membership (
    account_id  uuid primary key references parent_account (id) on delete cascade,
    plan_id     uuid references membership_plan (id) on delete set null,
    plan_name   text not null,
    status      text not null default 'active'
        check (status in ('active', 'expired')),
    starts_at   timestamptz not null default now(),
    expires_at  timestamptz not null,
    updated_at  timestamptz not null default now()
);

insert into membership_plan (code, name, description, duration_days, price_cents, features, status, sort)
values
    ('monthly', '月度会员', '按月解锁全部会员权益', 30, 1990,
     '["专题题包优先体验", "成长报告进阶分析"]'::jsonb, 'online', 10),
    ('yearly', '年度会员', '全年畅享会员权益，更划算', 365, 16900,
     '["专题题包优先体验", "成长报告进阶分析", "专属客服"]'::jsonb, 'online', 20)
on conflict (code) do nothing;

insert into admin_permission (code, name) values
    ('membership.read',  '查看会员套餐'),
    ('membership.write', '管理会员套餐')
on conflict (code) do update set name = excluded.name;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r join admin_permission p
    on p.code in ('membership.read', 'membership.write')
where r.code = 'super_admin'
on conflict do nothing;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r join admin_permission p
    on p.code = 'membership.read'
where r.code = 'ops'
on conflict do nothing;

-- +goose Down

delete from admin_role_permission
where permission_id in (
    select id from admin_permission where code in ('membership.read', 'membership.write'));
delete from admin_permission where code in ('membership.read', 'membership.write');

drop index if exists membership_order_account_idx;
drop table if exists account_membership;
drop table if exists membership_order;
drop table if exists membership_plan;
