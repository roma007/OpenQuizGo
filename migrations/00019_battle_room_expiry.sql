-- +goose Up

alter table family_battle add column if not exists join_code_expires_at timestamptz;
create index if not exists family_battle_join_code_expires_at_idx
    on family_battle (join_code_expires_at) where join_code is not null;

-- +goose Down

drop index if exists family_battle_join_code_expires_at_idx;
alter table family_battle drop column if exists join_code_expires_at;
