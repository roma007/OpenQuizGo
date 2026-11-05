-- +goose Up

create table coverage_snapshot (
    captured_on date        primary key,
    total       integer     not null default 0,
    online      integer     not null default 0,
    draft       integer     not null default 0,
    in_review   integer     not null default 0,
    offline     integer     not null default 0,
    created_at  timestamptz not null default now(),
    constraint coverage_snapshot_counts_chk check (
        total >= 0 and online >= 0 and draft >= 0 and in_review >= 0 and offline >= 0
    )
);

-- +goose Down

drop table if exists coverage_snapshot;
