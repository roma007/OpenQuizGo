-- +goose Up

-- 家长远程解锁签发的授权没有答题会话；quiz 授权仍带 session_id。
alter table watch_grant alter column session_id drop not null;

-- 授权来源：quiz（答题）/ parent（家长远程解锁）。
alter table watch_grant add column if not exists source text not null default 'quiz';
alter table watch_grant add constraint watch_grant_source_chk
    check (source in ('quiz', 'parent'));

-- +goose Down

alter table watch_grant drop constraint if exists watch_grant_source_chk;
alter table watch_grant drop column if exists source;
delete from watch_grant where session_id is null;
alter table watch_grant alter column session_id set not null;
