-- +goose Up

create table device_diagnostic (
    id         bigserial   primary key,
    device_id  text        not null,
    child_id   uuid        references child_profile (id) on delete cascade,
    content    text        not null,
    bytes      integer     not null,
    note       text        not null default '',
    created_at timestamptz not null default now(),
    constraint device_diagnostic_bytes_chk check (bytes >= 0)
);

create index device_diagnostic_device_idx
    on device_diagnostic (device_id, created_at desc);

-- +goose Down

drop table if exists device_diagnostic;
