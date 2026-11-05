-- +goose Up

alter table device_bind
    add column last_seen_at timestamptz not null default now();

create index device_bind_seen_idx
    on device_bind (last_seen_at)
    where status = 'active';

-- +goose Down

drop index if exists device_bind_seen_idx;
alter table device_bind drop column if exists last_seen_at;
