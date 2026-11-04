-- +goose Up

create table if not exists family_battle (
    id           uuid primary key default gen_random_uuid(),
    family_id    uuid not null references family (id) on delete cascade,
    child_id     uuid not null references child_profile (id) on delete cascade,
    status       text not null default 'child_turn'
        check (status in ('child_turn', 'parent_turn', 'finished')),
    question_ids jsonb not null check (jsonb_typeof(question_ids) = 'array'),
    child_score  int not null default 0,
    parent_score int not null default 0,
    created_by   uuid,
    created_at   timestamptz not null default now(),
    finished_at  timestamptz
);

create index if not exists family_battle_child_idx
    on family_battle (child_id, created_at desc);

create table if not exists family_battle_answer (
    id          bigserial primary key,
    battle_id   uuid not null references family_battle (id) on delete cascade,
    side        text not null check (side in ('child', 'parent')),
    question_id uuid not null references question (id),
    answer      jsonb not null,
    correct     boolean not null,
    position    int not null,
    created_at  timestamptz not null default now(),
    unique (battle_id, side, question_id)
);

create index if not exists family_battle_answer_battle_idx
    on family_battle_answer (battle_id, position);

-- +goose Down

drop index if exists family_battle_answer_battle_idx;
drop table if exists family_battle_answer;
drop index if exists family_battle_child_idx;
drop table if exists family_battle;
