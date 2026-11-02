-- +goose Up

-- 管理端角色与权限
insert into admin_role (code, name) values
    ('super_admin', '超级管理员'),
    ('editor',      '教研编辑'),
    ('reviewer',    '审核员'),
    ('ops',         '运营')
on conflict (code) do update set name = excluded.name;

insert into admin_permission (code, name) values
    ('question.read',    '查看题目'),
    ('question.write',   '创建/编辑题目'),
    ('question.review',  '审核题目'),
    ('question.publish', '上线/下线题目'),
    ('question.import',  '批量导入'),
    ('audit.read',       '查看审计日志')
on conflict (code) do update set name = excluded.name;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r cross join admin_permission p
where r.code = 'super_admin'
on conflict do nothing;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r join admin_permission p on p.code in
    ('question.read', 'question.write', 'question.import')
where r.code = 'editor'
on conflict do nothing;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r join admin_permission p on p.code in
    ('question.read', 'question.review')
where r.code = 'reviewer'
on conflict do nothing;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r join admin_permission p on p.code in
    ('question.read', 'question.publish', 'audit.read')
where r.code = 'ops'
on conflict do nothing;

-- 趣味题必须同时是 fun 学科
alter table question add constraint question_fun_category_chk
    check ((category_code = 'fun') = (subject_code = 'fun'));

-- 管理端常用筛选
create index question_admin_filter_idx
    on question (status, subject_code, stage_code, difficulty, type_code);

-- +goose Down

drop index if exists question_admin_filter_idx;
alter table question drop constraint if exists question_fun_category_chk;

delete from admin_role_permission
where role_id in (select id from admin_role where code in ('super_admin', 'editor', 'reviewer', 'ops'));
delete from admin_role where code in ('super_admin', 'editor', 'reviewer', 'ops');
delete from admin_permission where code in
    ('question.read', 'question.write', 'question.review', 'question.publish', 'question.import', 'audit.read');
