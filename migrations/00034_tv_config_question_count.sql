-- +goose Up

-- 082：家长端可直接配置每次答题题量（1–5 题，默认 1）。

alter table tv_config
    add column question_count integer not null default 1,
    add constraint tv_config_question_count_chk check (question_count between 1 and 5);

-- 存量电视不重置为 1：此前题量由 SessionSize 推导，最少 5 题（3+2，封顶 12），
-- 因此统一回填 5，保持与升级前最接近的出题量。
update tv_config set question_count = 5;

-- +goose Down

alter table tv_config
    drop constraint if exists tv_config_question_count_chk,
    drop column if exists question_count;
