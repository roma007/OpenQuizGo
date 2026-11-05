-- +goose Up

create table experiment (
    id              uuid        primary key default gen_random_uuid(),
    key             text        not null unique,
    name            text        not null,
    variant_a       text        not null default 'control',
    variant_b       text        not null default 'treatment',
    traffic_percent integer     not null default 50,
    status          text        not null default 'active',
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now(),
    constraint experiment_traffic_chk check (traffic_percent between 1 and 100),
    constraint experiment_status_chk check (status in ('active', 'disabled'))
);

create table experiment_exposure (
    experiment_id  uuid        not null references experiment (id) on delete cascade,
    account_id     uuid        not null references parent_account (id) on delete cascade,
    variant        text        not null,
    first_exposed_at timestamptz not null default now(),
    primary key (experiment_id, account_id)
);

create index experiment_exposure_variant_idx on experiment_exposure (experiment_id, variant);

insert into admin_permission (code, name) values
    ('experiment.read', '查看 A/B 实验'),
    ('experiment.write', '管理 A/B 实验')
on conflict (code) do update set name = excluded.name;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r join admin_permission p
    on p.code in ('experiment.read', 'experiment.write')
where r.code in ('super_admin', 'ops')
on conflict do nothing;

-- +goose Down

delete from admin_role_permission
where permission_id in (select id from admin_permission where code in ('experiment.read', 'experiment.write'));
delete from admin_permission where code in ('experiment.read', 'experiment.write');
drop table if exists experiment_exposure;
drop table if exists experiment;
