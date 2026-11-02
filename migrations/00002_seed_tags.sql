-- +goose Up

insert into tag_dict (tag_type, code, name, sort) values
    ('subject', 'math',     '数学思维',       10),
    ('subject', 'chinese',  '语文素养',       20),
    ('subject', 'english',  '英语启蒙',       30),
    ('subject', 'science',  '科学常识',       40),
    ('subject', 'logic',    '逻辑推理',       50),
    ('subject', 'safety',   '生活/安全常识',  60),
    ('subject', 'fun',      '脑筋急转弯',     90),

    ('category', 'core',      '核心学科',     10),
    ('category', 'literacy',  '素养学科',     20),
    ('category', 'topic',     '专题内容',     30),
    ('category', 'fun',       '趣味类目',     90),

    ('stage', 'preschool', '启蒙 3–5',     10),
    ('stage', 'low',       '低年级 6–8',   20),
    ('stage', 'mid',       '中年级 9–11',  30),
    ('stage', 'high',      '高年级 12+',   40),

    ('grade', 'g1', '一年级', 10),
    ('grade', 'g2', '二年级', 20),
    ('grade', 'g3', '三年级', 30),
    ('grade', 'g4', '四年级', 40),
    ('grade', 'g5', '五年级', 50),
    ('grade', 'g6', '六年级', 60),
    ('grade', 'g7', '七年级', 70),
    ('grade', 'g8', '八年级', 80),
    ('grade', 'g9', '九年级', 90),

    ('difficulty', '1', '入门', 10),
    ('difficulty', '2', '简单', 20),
    ('difficulty', '3', '中等', 30),
    ('difficulty', '4', '较难', 40),
    ('difficulty', '5', '挑战', 50),

    ('type', 'choice', '选择', 10),
    ('type', 'voice',  '语音', 20),
    ('type', 'image',  '看图', 30),
    ('type', 'fill',   '填空', 40),
    ('type', 'sort',   '排序', 50),
    ('type', 'match',  '连线', 60)
on conflict (tag_type, code) do update
set name = excluded.name,
    sort = excluded.sort;

-- +goose Down

-- 种子数据不随回滚删除，避免误删运营新增的标签。
select 1;
