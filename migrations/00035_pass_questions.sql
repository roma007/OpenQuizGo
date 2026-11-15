-- +goose Up

-- 083：通过标准从「加权门槛（分）」改为「答对题数」。

alter table watch_rule
    add column pass_questions integer not null default 2,
    add constraint watch_rule_pass_questions_chk check (pass_questions between 1 and 12);

alter table tv_config
    add column pass_questions integer not null default 1,
    add constraint tv_config_pass_questions_chk check (pass_questions between 1 and 12);

alter table quiz_session
    add column pass_questions integer not null default 2;

-- 存量电视：通过题数不能超过每次题量（1 题场次要求答对 1 题）。
update tv_config set pass_questions = least(2, question_count);

-- +goose Down

alter table quiz_session drop column if exists pass_questions;

alter table tv_config
    drop constraint if exists tv_config_pass_questions_chk,
    drop column if exists pass_questions;

alter table watch_rule
    drop constraint if exists watch_rule_pass_questions_chk,
    drop column if exists pass_questions;
