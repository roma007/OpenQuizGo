-- +goose Up

-- ============================================================
-- 标签字典：所有学科/类目/学段/年级/难度/题型取值的唯一来源。
-- 新增取值只插数据，不改表结构。
-- ============================================================
create table tag_dict (
    tag_type   text        not null,
    code       text        not null,
    name       text        not null,
    sort       integer     not null default 0,
    enabled    boolean     not null default true,
    created_at timestamptz not null default now(),
    primary key (tag_type, code),
    constraint tag_dict_type_chk check (tag_type in ('subject', 'category', 'stage', 'grade', 'difficulty', 'type')),
    constraint tag_dict_code_chk check (code ~ '^[a-z0-9_]+$')
);

comment on table tag_dict is '标签字典：学科/类目/学段/年级/难度/题型';

-- ============================================================
-- 管理端账号与 RBAC
-- ============================================================
create table admin_user (
    id            uuid        primary key default gen_random_uuid(),
    username      text        not null unique,
    password_hash text        not null,
    display_name  text        not null default '',
    status        text        not null default 'active',
    last_login_at timestamptz,
    created_at    timestamptz not null default now(),
    constraint admin_user_status_chk check (status in ('active', 'disabled'))
);

create table admin_role (
    id         uuid primary key default gen_random_uuid(),
    code       text not null unique,
    name       text not null,
    created_at timestamptz not null default now()
);

create table admin_permission (
    id         uuid primary key default gen_random_uuid(),
    code       text not null unique,
    name       text not null,
    created_at timestamptz not null default now()
);

create table admin_role_permission (
    role_id       uuid not null references admin_role (id) on delete cascade,
    permission_id uuid not null references admin_permission (id) on delete cascade,
    primary key (role_id, permission_id)
);

create table admin_user_role (
    user_id uuid not null references admin_user (id) on delete cascade,
    role_id uuid not null references admin_role (id) on delete cascade,
    primary key (user_id, role_id)
);

-- ============================================================
-- 媒体资源
-- ============================================================
create table media_asset (
    id          uuid        primary key default gen_random_uuid(),
    kind        text        not null,
    url         text        not null,
    duration_ms integer,
    hash        text        not null default '',
    status      text        not null default 'draft',
    created_at  timestamptz not null default now(),
    constraint media_asset_kind_chk check (kind in ('image', 'audio', 'video')),
    constraint media_asset_status_chk check (status in ('draft', 'approved', 'rejected')),
    constraint media_asset_duration_chk check (duration_ms is null or duration_ms >= 0)
);

-- ============================================================
-- 专题题包
-- ============================================================
create table topic_pack (
    id             uuid        primary key default gen_random_uuid(),
    name           text        not null,
    type           text        not null,
    subject_scope  jsonb       not null default '[]'::jsonb,
    stage_scope    jsonb       not null default '[]'::jsonb,
    start_at       timestamptz,
    end_at         timestamptz,
    is_paid        boolean     not null default false,
    status         text        not null default 'draft',
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    constraint topic_pack_type_chk check (type in ('holiday', 'ip', 'vacation', 'daily')),
    constraint topic_pack_status_chk check (status in ('draft', 'online', 'offline')),
    constraint topic_pack_subject_scope_chk check (jsonb_typeof(subject_scope) = 'array'),
    constraint topic_pack_stage_scope_chk check (jsonb_typeof(stage_scope) = 'array'),
    constraint topic_pack_time_chk check (end_at is null or start_at is null or end_at > start_at)
);

-- ============================================================
-- 题目主表：唯一题库，学科只是标签。
-- ============================================================
create table question (
    id             uuid        primary key default gen_random_uuid(),
    subject_code   text        not null,
    category_code  text        not null,
    stage_code     text        not null,
    grade_tag      text,
    difficulty     smallint    not null,
    type_code      text        not null,
    stem           text        not null,
    stem_media     jsonb,
    options        jsonb,
    answer         jsonb       not null,
    answer_alias   jsonb,
    explain        text,
    duration_limit smallint,
    topic_id       uuid        references topic_pack (id) on delete set null,
    status         text        not null default 'draft',
    version        integer     not null default 1,
    schema_ver     integer     not null default 1,
    created_by     uuid        references admin_user (id) on delete set null,
    reviewed_by    uuid        references admin_user (id) on delete set null,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    constraint question_stage_chk check (stage_code in ('preschool', 'low', 'mid', 'high')),
    constraint question_category_chk check (category_code in ('core', 'literacy', 'topic', 'fun')),
    constraint question_difficulty_chk check (difficulty between 1 and 5),
    constraint question_status_chk check (status in ('draft', 'review', 'trial', 'online', 'offline')),
    constraint question_type_chk check (type_code in ('choice', 'voice', 'image', 'fill', 'sort', 'match')),
    constraint question_grade_chk check (grade_tag is null or grade_tag ~ '^g[1-9]$'),
    constraint question_options_chk check (options is null or jsonb_typeof(options) = 'array'),
    constraint question_alias_chk check (category_code <> 'fun' or jsonb_array_length(coalesce(answer_alias, '[]'::jsonb)) > 0),
    constraint question_duration_chk check (duration_limit is null or duration_limit between 5 and 120),
    constraint question_version_chk check (version > 0 and schema_ver > 0)
);

comment on table question is '题目主表：学科/类目/学段/难度/题型均为标签，禁止按学科拆表';

create index question_online_pool_idx on question (stage_code, subject_code, difficulty) where status = 'online';
create index question_online_category_idx on question (category_code, stage_code) where status = 'online';
create index question_topic_idx on question (topic_id) where topic_id is not null;
create index question_review_idx on question (status, updated_at desc);

-- ============================================================
-- 题包与题目关联
-- ============================================================
create table topic_question (
    topic_id    uuid    not null references topic_pack (id) on delete cascade,
    question_id uuid    not null references question (id) on delete cascade,
    position    integer not null default 0,
    primary key (topic_id, question_id)
);

create index topic_question_question_idx on topic_question (question_id);

-- ============================================================
-- 家长账号、家庭、孩子、设备绑定
-- ============================================================
create table parent_account (
    id            uuid        primary key default gen_random_uuid(),
    phone_cipher  text        not null,
    phone_hash    text        not null unique,
    password_hash text        not null default '',
    status        text        not null default 'active',
    last_login_at timestamptz,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    constraint parent_account_status_chk check (status in ('active', 'disabled'))
);

comment on column parent_account.phone_cipher is '手机号密文（应用层 AES-GCM）';
comment on column parent_account.phone_hash is '手机号 HMAC-SHA256，用于唯一索引与查询';

create table family (
    id            uuid        primary key default gen_random_uuid(),
    owner_id      uuid        not null references parent_account (id) on delete restrict,
    member_status text        not null default 'free',
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now(),
    constraint family_member_status_chk check (member_status in ('free', 'member', 'expired'))
);

create index family_owner_idx on family (owner_id);

create table family_member (
    family_id             uuid        not null references family (id) on delete cascade,
    account_id            uuid        not null references parent_account (id) on delete cascade,
    role                  text        not null default 'member',
    second_factor_verified boolean     not null default false,
    joined_at             timestamptz not null default now(),
    primary key (family_id, account_id),
    constraint family_member_role_chk check (role in ('owner', 'member'))
);

create index family_member_account_idx on family_member (account_id);

create table child_profile (
    id          uuid        primary key default gen_random_uuid(),
    family_id   uuid        not null references family (id) on delete cascade,
    nickname    text        not null,
    birth_year  smallint,
    birth_month smallint,
    stage_code  text        not null,
    grade_tag   text,
    avatar_url  text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    constraint child_profile_stage_chk check (stage_code in ('preschool', 'low', 'mid', 'high')),
    constraint child_profile_grade_chk check (grade_tag is null or grade_tag ~ '^g[1-9]$'),
    constraint child_profile_birth_chk check (
        (birth_year is null and birth_month is null)
        or (birth_year between 2000 and 2100 and birth_month between 1 and 12)
    )
);

create index child_profile_family_idx on child_profile (family_id);

create table device_bind (
    id               uuid        primary key default gen_random_uuid(),
    device_id        text        not null,
    child_id         uuid        not null references child_profile (id) on delete cascade,
    platform         text        not null,
    status           text        not null default 'active',
    token_hash       text        not null default '',
    token_expires_at timestamptz,
    bound_at         timestamptz not null default now(),
    unbound_at       timestamptz,
    constraint device_bind_platform_chk check (platform in ('tv', 'phone', 'pad', 'miniapp')),
    constraint device_bind_status_chk check (status in ('active', 'unbound')),
    constraint device_bind_unbound_chk check (
        (status = 'active' and unbound_at is null)
        or (status = 'unbound' and unbound_at is not null)
    )
);

create unique index device_bind_active_device_idx on device_bind (device_id) where status = 'active';
create index device_bind_child_idx on device_bind (child_id) where status = 'active';

-- ============================================================
-- 规则配置
-- ============================================================
create table watch_rule (
    id                    uuid          primary key default gen_random_uuid(),
    child_id              uuid          not null unique references child_profile (id) on delete cascade,
    unlock_minutes        integer       not null default 20,
    pass_threshold        numeric(6, 2) not null default 3,
    subject_weight        jsonb         not null default '{}'::jsonb,
    fun_ratio_cap         integer       not null default 2,
    fun_enabled           boolean       not null default true,
    time_window           jsonb         not null default '{"allow":[{"start":"07:00","end":"21:00"}]}'::jsonb,
    daily_limit           integer       not null default 60,
    offline_fallback_min  integer       not null default 10,
    scope_mode            text          not null default 'media',
    allowed_apps          jsonb         not null default '[]'::jsonb,
    weight_preset         text          not null default 'balanced',
    timezone              text          not null default 'Asia/Shanghai',
    version               integer       not null default 1,
    updated_by            uuid          references parent_account (id) on delete set null,
    updated_at            timestamptz   not null default now(),
    constraint watch_rule_unlock_chk check (unlock_minutes between 1 and 240),
    constraint watch_rule_threshold_chk check (pass_threshold > 0),
    constraint watch_rule_weight_chk check (jsonb_typeof(subject_weight) = 'object'),
    constraint watch_rule_fun_cap_chk check (fun_ratio_cap between 0 and 10),
    constraint watch_rule_daily_chk check (daily_limit between 1 and 720),
    constraint watch_rule_offline_chk check (offline_fallback_min between 0 and 120),
    constraint watch_rule_scope_chk check (scope_mode in ('media', 'custom', 'all_apps')),
    constraint watch_rule_apps_chk check (jsonb_typeof(allowed_apps) = 'array'),
    constraint watch_rule_preset_chk check (weight_preset in ('balanced', 'math', 'literacy', 'custom')),
    constraint watch_rule_version_chk check (version > 0)
);

-- ============================================================
-- 答题会话
-- ============================================================
create table quiz_session (
    id             uuid          primary key default gen_random_uuid(),
    child_id       uuid          not null references child_profile (id) on delete cascade,
    device_id      text          not null,
    rule_version   integer       not null,
    state          text          not null default 'active',
    question_count integer       not null default 0,
    pass_threshold numeric(6, 2) not null,
    score          numeric(6, 2) not null default 0,
    paused_app     text,
    was_playing    boolean       not null default false,
    paused_at      timestamptz,
    resumed_at     timestamptz,
    started_at     timestamptz   not null default now(),
    expires_at     timestamptz   not null,
    finished_at    timestamptz,
    constraint quiz_session_state_chk check (state in ('active', 'passed', 'failed', 'expired', 'cancelled')),
    constraint quiz_session_count_chk check (question_count between 0 and 30),
    constraint quiz_session_score_chk check (score >= 0)
);

create index quiz_session_child_idx on quiz_session (child_id, started_at desc);
create index quiz_session_active_idx on quiz_session (device_id, state) where state = 'active';

create table quiz_session_item (
    id              uuid        primary key default gen_random_uuid(),
    session_id      uuid        not null references quiz_session (id) on delete cascade,
    question_id     uuid        not null references question (id) on delete restrict,
    question_version integer    not null,
    answer_snapshot jsonb       not null,
    position        integer     not null,
    answer_token_jti text       not null unique,
    issued_at       timestamptz not null default now(),
    answered_at     timestamptz,
    answer_given    jsonb,
    is_correct      boolean,
    duration_ms     integer,
    constraint quiz_session_item_position_chk check (position >= 0),
    constraint quiz_session_item_duration_chk check (duration_ms is null or duration_ms >= 0),
    unique (session_id, position),
    unique (session_id, question_id)
);

create index quiz_session_item_session_idx on quiz_session_item (session_id, position);

-- ============================================================
-- 答题记录（校准与报告数据源）
-- ============================================================
create table answer_log (
    id           bigserial   primary key,
    child_id     uuid        not null references child_profile (id) on delete cascade,
    question_id  uuid        not null references question (id) on delete restrict,
    session_id   uuid        references quiz_session (id) on delete set null,
    device_id    text        not null,
    answer_given jsonb,
    is_correct   boolean     not null,
    duration_ms  integer,
    answered_at  timestamptz not null default now()
);

-- 首期普通表；数据量达到阈值后按 answered_at 月分区（见设计 6.4）
create index answer_log_child_idx on answer_log (child_id, answered_at desc);
create index answer_log_question_idx on answer_log (question_id, answered_at desc);

-- ============================================================
-- 观看授权（服务端签发，客户端执行）
-- ============================================================
create table watch_grant (
    id               uuid          primary key default gen_random_uuid(),
    child_id         uuid          not null references child_profile (id) on delete cascade,
    device_id        text          not null,
    session_id       uuid          not null unique references quiz_session (id) on delete restrict,
    minutes          integer       not null,
    valid_from       timestamptz   not null default now(),
    valid_until      timestamptz   not null,
    jti              text          not null unique,
    consumed_seconds integer       not null default 0,
    status           text          not null default 'active',
    created_at       timestamptz   not null default now(),
    constraint watch_grant_minutes_chk check (minutes between 1 and 240),
    constraint watch_grant_consumed_chk check (consumed_seconds >= 0),
    constraint watch_grant_status_chk check (status in ('active', 'exhausted', 'revoked', 'expired')),
    constraint watch_grant_time_chk check (valid_until > valid_from)
);

create index watch_grant_child_idx on watch_grant (child_id, valid_from desc);
create index watch_grant_active_idx on watch_grant (device_id, status) where status = 'active';

-- ============================================================
-- 应用使用记录与离线租约
-- ============================================================
create table watch_log (
    id              bigserial   primary key,
    grant_id        uuid        references watch_grant (id) on delete set null,
    child_id        uuid        not null references child_profile (id) on delete cascade,
    device_id       text        not null,
    target_type     text        not null default 'app',
    target_id       text        not null default '',
    start_at        timestamptz not null,
    end_at          timestamptz not null,
    watched_seconds integer     not null default 0,
    paused_seconds  integer     not null default 0,
    created_at      timestamptz not null default now(),
    constraint watch_log_target_chk check (target_type in ('app', 'content')),
    constraint watch_log_time_chk check (end_at >= start_at),
    constraint watch_log_seconds_chk check (watched_seconds >= 0 and paused_seconds >= 0)
);

-- 首期普通表；数据量达到阈值后按 start_at 月分区（见设计 6.4）
create index watch_log_child_idx on watch_log (child_id, start_at desc);
create index watch_log_grant_idx on watch_log (grant_id);

create table offline_lease (
    id               uuid        primary key default gen_random_uuid(),
    device_id        text        not null,
    child_id         uuid        not null references child_profile (id) on delete cascade,
    jti              text        not null unique,
    max_minutes      integer     not null default 10,
    allowed_apps     jsonb       not null default '[]'::jsonb,
    issued_at        timestamptz not null default now(),
    expires_at       timestamptz not null,
    reported_minutes integer     not null default 0,
    status           text        not null default 'active',
    constraint offline_lease_minutes_chk check (max_minutes between 0 and 120),
    constraint offline_lease_reported_chk check (reported_minutes >= 0),
    constraint offline_lease_status_chk check (status in ('active', 'reported', 'expired')),
    constraint offline_lease_time_chk check (expires_at > issued_at)
);

create index offline_lease_device_idx on offline_lease (device_id, expires_at desc);

-- ============================================================
-- 设备能力与篡改事件
-- ============================================================
create table device_capability (
    device_id        text        primary key,
    guard_mode       text        not null default 'standard',
    permissions      jsonb       not null default '{}'::jsonb,
    os_version       text        not null default '',
    app_version      text        not null default '',
    last_reported_at timestamptz not null default now(),
    constraint device_capability_mode_chk check (guard_mode in ('standard', 'device_owner')),
    constraint device_capability_permissions_chk check (jsonb_typeof(permissions) = 'object')
);

create table tamper_event (
    id          bigserial   primary key,
    device_id   text        not null,
    child_id    uuid        references child_profile (id) on delete set null,
    type        text        not null,
    detail      jsonb       not null default '{}'::jsonb,
    reported_at timestamptz not null default now(),
    constraint tamper_event_type_chk check (type in ('permission_revoked', 'uninstall_attempt', 'package_tampered', 'clock_skew', 'other'))
);

create index tamper_event_device_idx on tamper_event (device_id, reported_at desc);
create index tamper_event_child_idx on tamper_event (child_id, reported_at desc);

-- ============================================================
-- 成长报告与审计
-- ============================================================
create table growth_report (
    id            uuid        primary key default gen_random_uuid(),
    child_id      uuid        not null references child_profile (id) on delete cascade,
    period        text        not null,
    period_start  date        not null,
    subject_stats jsonb       not null default '{}'::jsonb,
    fun_stats     jsonb       not null default '{}'::jsonb,
    watch_minutes integer     not null default 0,
    generated_at  timestamptz not null default now(),
    constraint growth_report_period_chk check (period in ('week', 'month')),
    constraint growth_report_minutes_chk check (watch_minutes >= 0),
    unique (child_id, period, period_start)
);

create table audit_log (
    id          bigserial   primary key,
    actor_type  text        not null,
    actor_id    text        not null default '',
    action      text        not null,
    target_type text        not null default '',
    target_id   text        not null default '',
    before_data jsonb,
    after_data  jsonb,
    ip          inet,
    user_agent  text        not null default '',
    created_at  timestamptz not null default now(),
    constraint audit_log_actor_chk check (actor_type in ('parent', 'admin', 'device', 'system'))
);

create index audit_log_target_idx on audit_log (target_type, target_id);
create index audit_log_created_idx on audit_log (created_at desc);

-- +goose Down

drop table if exists audit_log;
drop table if exists growth_report;
drop table if exists tamper_event;
drop table if exists device_capability;
drop table if exists offline_lease;
drop table if exists watch_log;
drop table if exists watch_grant;
drop table if exists answer_log;
drop table if exists quiz_session_item;
drop table if exists quiz_session;
drop table if exists watch_rule;
drop table if exists device_bind;
drop table if exists child_profile;
drop table if exists family_member;
drop table if exists family;
drop table if exists parent_account;
drop table if exists topic_question;
drop table if exists question;
drop table if exists topic_pack;
drop table if exists media_asset;
drop table if exists admin_user_role;
drop table if exists admin_role_permission;
drop table if exists admin_permission;
drop table if exists admin_role;
drop table if exists admin_user;
drop table if exists tag_dict;
