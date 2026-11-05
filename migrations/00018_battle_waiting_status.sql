-- +goose Up

alter table family_battle drop constraint if exists family_battle_status_check;
alter table family_battle
    add constraint family_battle_status_check
    check (status in ('waiting', 'child_turn', 'parent_turn', 'finished'));

-- +goose Down

update family_battle set status = 'child_turn' where status = 'waiting';
alter table family_battle drop constraint if exists family_battle_status_check;
alter table family_battle
    add constraint family_battle_status_check
    check (status in ('child_turn', 'parent_turn', 'finished'));
