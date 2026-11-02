-- +goose Up

-- 会话规则快照：保证一次会话期间判分口径稳定
alter table quiz_session add column rule_snapshot jsonb not null default '{}'::jsonb;
alter table quiz_session add column correct_count integer not null default 0;

-- 题目快照：答案、解析、学科、题型，避免会话中途改题影响判分
alter table quiz_session_item add column question_snapshot jsonb not null default '{}'::jsonb;

-- +goose Down

alter table quiz_session_item drop column if exists question_snapshot;
alter table quiz_session drop column if exists correct_count;
alter table quiz_session drop column if exists rule_snapshot;
