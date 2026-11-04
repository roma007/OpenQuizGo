-- +goose Up

alter table tamper_event add column if not exists read_at timestamptz;
create index if not exists tamper_event_child_reported_idx
    on tamper_event (child_id, reported_at desc);

-- +goose Down

drop index if exists tamper_event_child_reported_idx;
alter table tamper_event drop column if exists read_at;
