-- +goose Up

-- 081：电视端从「激活码绑定到孩子」改为「登录到家庭」，配置按电视独立下发。

create table tv_device (
    id               uuid primary key default gen_random_uuid(),
    family_id        uuid not null references family (id) on delete cascade,
    child_id         uuid references child_profile (id) on delete set null,
    device_id        text not null,
    name             text not null default '电视',
    platform         text not null default 'tv',
    token_hash       text not null default '',
    token_expires_at timestamptz,
    last_seen_at     timestamptz,
    status           text not null default 'active',
    created_at       timestamptz not null default now(),
    constraint tv_device_status_chk check (status in ('active', 'revoked')),
    constraint tv_device_platform_chk check (platform in ('tv', 'pad'))
);

create unique index tv_device_device_idx on tv_device (device_id) where status = 'active';
create index tv_device_family_idx on tv_device (family_id) where status = 'active';
create index tv_device_token_idx on tv_device (token_hash) where status = 'active';

create table tv_config (
    device_pk        uuid primary key references tv_device (id) on delete cascade,
    quiz_enabled     boolean not null default true,
    limit_enabled    boolean not null default true,
    unlock_minutes   integer not null default 15,
    daily_limit      integer not null default 60,
    subjects         jsonb not null default '[]'::jsonb,
    pass_threshold   numeric(6, 2) not null default 3,
    quiz_scope       text not null default 'random',
    hard_stop_grace  integer not null default 10,
    version          integer not null default 1,
    updated_by       uuid references parent_account (id) on delete set null,
    updated_at       timestamptz not null default now(),
    constraint tv_config_unlock_chk check (unlock_minutes between 1 and 240),
    constraint tv_config_daily_chk check (daily_limit between 1 and 720),
    constraint tv_config_grace_chk check (hard_stop_grace between 1 and 120),
    constraint tv_config_scope_chk check (quiz_scope in ('random', 'grade')),
    constraint tv_config_subjects_chk check (jsonb_typeof(subjects) = 'array'),
    constraint tv_config_threshold_chk check (pass_threshold > 0)
);

alter table child_profile
    add column is_placeholder boolean not null default false,
    add column grade_set_at date;

create index child_profile_placeholder_idx on child_profile (family_id, is_placeholder);

-- 存量迁移：已绑定的电视转成 tv_device，配置从孩子规则复制一份。
insert into tv_device (family_id, child_id, device_id, name, platform,
                       token_hash, token_expires_at, status, created_at)
select c.family_id, d.child_id, d.device_id, c.nickname, d.platform,
       d.token_hash, d.token_expires_at, 'active', d.bound_at
from device_bind d
join child_profile c on c.id = d.child_id
where d.status = 'active'
on conflict do nothing;

insert into tv_config (device_pk, quiz_enabled, limit_enabled, unlock_minutes,
                       daily_limit, subjects, pass_threshold, quiz_scope, hard_stop_grace)
select t.id, true, true,
       coalesce(wr.unlock_minutes, 15),
       coalesce(wr.daily_limit, 60),
       '[]'::jsonb,
       coalesce(wr.pass_threshold, 3),
       'grade', 10
from tv_device t
left join watch_rule wr on wr.child_id = t.child_id
on conflict (device_pk) do nothing;

-- +goose Down

delete from tv_config;
delete from tv_device;

drop index if exists child_profile_placeholder_idx;

alter table child_profile
    drop column if exists grade_set_at,
    drop column if exists is_placeholder;

drop table if exists tv_config;
drop table if exists tv_device;
