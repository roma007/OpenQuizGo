-- +goose Up

insert into admin_permission (code, name) values
    ('family.read', '查看家庭与设备')
on conflict (code) do update set name = excluded.name;

insert into admin_role_permission (role_id, permission_id)
select r.id, p.id from admin_role r join admin_permission p
    on p.code = 'family.read'
where r.code in ('super_admin', 'ops')
on conflict do nothing;

-- +goose Down

delete from admin_role_permission
where permission_id in (select id from admin_permission where code = 'family.read');
delete from admin_permission where code = 'family.read';
