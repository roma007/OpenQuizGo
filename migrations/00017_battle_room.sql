-- +goose Up

alter table family_battle add column if not exists mode text not null default 'same_device';
alter table family_battle drop constraint if exists family_battle_mode_check;
alter table family_battle
    add constraint family_battle_mode_check
    check (mode in ('same_device', 'room'));

alter table family_battle add column if not exists join_code text;
create unique index if not exists family_battle_join_code_idx
    on family_battle (join_code) where join_code is not null;

-- +goose Down

drop index if exists family_battle_join_code_idx;
alter table family_battle drop column if exists join_code;
alter table family_battle drop constraint if exists family_battle_mode_check;
alter table family_battle drop column if exists mode;
