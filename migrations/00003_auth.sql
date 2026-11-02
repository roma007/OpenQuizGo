-- +goose Up

-- 家长当前生效的家庭（支持被邀请后切换）
alter table parent_account add column active_family_id uuid references family (id) on delete set null;

-- 家长登录会话：刷新令牌哈希 + 轮换/吊销
create table parent_session (
    id         uuid        primary key default gen_random_uuid(),
    account_id uuid        not null references parent_account (id) on delete cascade,
    token_hash text        not null unique,
    user_agent text        not null default '',
    ip         inet,
    expires_at timestamptz not null,
    revoked_at timestamptz,
    created_at timestamptz not null default now(),
    constraint parent_session_expires_chk check (expires_at > created_at)
);

create index parent_session_account_idx on parent_session (account_id, created_at desc);

-- 家长 PIN：用于修改规则等敏感操作
alter table parent_account add column pin_hash text not null default '';
alter table parent_account add column pin_updated_at timestamptz;

-- 家庭邀请码：多家长加入
create table family_invite (
    id         uuid        primary key default gen_random_uuid(),
    family_id  uuid        not null references family (id) on delete cascade,
    code       text        not null,
    created_by uuid        not null references parent_account (id) on delete cascade,
    expires_at timestamptz not null,
    used_at    timestamptz,
    used_by    uuid        references parent_account (id) on delete set null,
    created_at timestamptz not null default now(),
    constraint family_invite_expires_chk check (expires_at > created_at),
    constraint family_invite_used_chk check (used_at is null or used_by is not null)
);

create unique index family_invite_pending_code_idx on family_invite (code) where used_at is null;
create index family_invite_family_idx on family_invite (family_id, created_at desc);

-- +goose Down

drop table if exists family_invite;
alter table parent_account drop column if exists pin_updated_at;
alter table parent_account drop column if exists pin_hash;
alter table parent_account drop column if exists active_family_id;
drop table if exists parent_session;
