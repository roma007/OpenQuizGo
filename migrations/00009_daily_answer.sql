-- +goose Up

create table if not exists child_daily_answer (
    id          bigserial primary key,
    child_id    uuid not null references child_profile (id) on delete cascade,
    answer_date date not null,
    question_id uuid not null references question (id),
    correct     boolean not null,
    points      int not null default 0,
    answered_at timestamptz not null default now(),
    unique (child_id, answer_date)
);

create index if not exists child_daily_child_date_idx
    on child_daily_answer (child_id, answer_date desc);

-- +goose Down

drop index if exists child_daily_child_date_idx;
drop table if exists child_daily_answer;
