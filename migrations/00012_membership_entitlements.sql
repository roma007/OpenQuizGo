-- +goose Up

alter table membership_plan
    add column if not exists entitlement_codes jsonb not null default '[]'::jsonb;

alter table membership_plan
    drop constraint if exists membership_plan_entitlement_codes_check;
alter table membership_plan
    add constraint membership_plan_entitlement_codes_check
    check (jsonb_typeof(entitlement_codes) = 'array');

update membership_plan
set entitlement_codes = '["topic_pack","report_advanced"]'::jsonb
where code in ('monthly', 'yearly');

-- +goose Down

alter table membership_plan drop constraint if exists membership_plan_entitlement_codes_check;
alter table membership_plan drop column if exists entitlement_codes;
