-- +goose Up

alter table family
    add column membership_shared boolean not null default true;

-- +goose Down

alter table family drop column if exists membership_shared;
