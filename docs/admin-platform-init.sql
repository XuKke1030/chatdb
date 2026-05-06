-- 问答问数平台与管理后台初始化 SQL（SQLite 版本）
-- 用户端测试账号：
--   zhangsan / 123456，拥有网格 + 人流 + 车流权限
--   lisi / 123456，仅拥有人流权限
--   wangwu / 123456，仅拥有车流权限
--   grid_user / 123456，仅拥有网格权限
-- 管理后台测试账号：
--   admin / admin123

CREATE TABLE IF NOT EXISTS user (
    user_id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    verify INTEGER NOT NULL DEFAULT 0,
    rule_level INTEGER NOT NULL DEFAULT 1,
    last_login_tme INTEGER DEFAULT 0,
    create_time INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    update_time INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS database_conf (
    database_id INTEGER PRIMARY KEY AUTOINCREMENT,
    db_name TEXT NOT NULL,
    user_name TEXT NOT NULL,
    password TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    db_type TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    create_time INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    update_time INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    messages TEXT NOT NULL,
    create_time INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    update_time INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS admin_account (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    verify INTEGER NOT NULL,
    role TEXT NOT NULL DEFAULT 'administrator',
    enabled INTEGER NOT NULL DEFAULT 1,
    create_time INTEGER NOT NULL,
    update_time INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_user_profile (
    user_id INTEGER PRIMARY KEY,
    department TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    rule_level INTEGER NOT NULL DEFAULT 7,
    create_time INTEGER NOT NULL,
    update_time INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_example_question (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic TEXT NOT NULL,
    question TEXT NOT NULL,
    description TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    sort INTEGER NOT NULL DEFAULT 0,
    create_time INTEGER NOT NULL,
    update_time INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_question_candidate (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic TEXT NOT NULL,
    question TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    count INTEGER NOT NULL DEFAULT 1,
    last_seen_at INTEGER NOT NULL,
    create_time INTEGER NOT NULL,
    update_time INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_data_source (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'closed',
    latest_sync INTEGER NOT NULL DEFAULT 0,
    create_time INTEGER NOT NULL,
    update_time INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_grid_import (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    month TEXT,
    file_name TEXT NOT NULL,
    status TEXT NOT NULL,
    total_rows INTEGER NOT NULL DEFAULT 0,
    success_rows INTEGER NOT NULL DEFAULT 0,
    failed_rows INTEGER NOT NULL DEFAULT 0,
    operator TEXT,
    create_time INTEGER NOT NULL,
    complete_time INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS admin_grid_import_error (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    import_id INTEGER NOT NULL,
    row_index INTEGER NOT NULL,
    reason TEXT NOT NULL,
    raw_data TEXT
);

INSERT OR IGNORE INTO user (username, password, verify, rule_level, create_time, update_time)
VALUES
('zhangsan', '65ba747efffca3c8acb1952b6ca6e417', 81371, 7, strftime('%s', 'now'), strftime('%s', 'now')),
('lisi', '4728ea0394876cda4bf1b870724ac106', 81372, 2, strftime('%s', 'now'), strftime('%s', 'now')),
('wangwu', '9b67706762d7b7d6c06df6717fbdfb33', 81373, 4, strftime('%s', 'now'), strftime('%s', 'now')),
('grid_user', '7e7f8e85bdff9991d0271c0ea8d2d938', 81374, 1, strftime('%s', 'now'), strftime('%s', 'now'));

INSERT OR IGNORE INTO admin_account (username, password, verify, role, enabled, create_time, update_time)
VALUES ('admin', 'e9125c6228bcd40b8d58098c62e5fd6f', 8137, 'administrator', 1, strftime('%s', 'now'), strftime('%s', 'now'));

INSERT OR REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '区治理中心', 1, rule_level, strftime('%s', 'now'), strftime('%s', 'now')
FROM user
WHERE username = 'zhangsan';

INSERT OR REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '综合治理办', 1, rule_level, strftime('%s', 'now'), strftime('%s', 'now')
FROM user
WHERE username = 'lisi';

INSERT OR REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '交通专班', 1, rule_level, strftime('%s', 'now'), strftime('%s', 'now')
FROM user
WHERE username = 'wangwu';

INSERT OR REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '网格治理专班', 1, rule_level, strftime('%s', 'now'), strftime('%s', 'now')
FROM user
WHERE username = 'grid_user';

INSERT OR IGNORE INTO admin_example_question (topic, question, description, enabled, sort, create_time, update_time)
VALUES
('grid', '本月高新区案件结案率是多少？', '查询网格案件办理成效', 1, 10, strftime('%s', 'now'), strftime('%s', 'now')),
('grid', '哪个网格的案件数量最多？', '查询网格案件排名', 1, 20, strftime('%s', 'now'), strftime('%s', 'now')),
('population', '过去一周人流进出趋势如何？', '展示每日进出人数对比', 1, 30, strftime('%s', 'now'), strftime('%s', 'now')),
('traffic', '今日港澳车辆占比是多少？', '统计重点卡口跨境车辆情况', 1, 40, strftime('%s', 'now'), strftime('%s', 'now'));

INSERT OR IGNORE INTO admin_question_candidate (topic, question, status, count, last_seen_at, create_time, update_time)
VALUES
('grid', '本月哪个部门处理案件效率最高？', 'pending', 3, strftime('%s', 'now'), strftime('%s', 'now'), strftime('%s', 'now')),
('population', '节假日前后人流变化最大的区域是哪里？', 'pending', 5, strftime('%s', 'now'), strftime('%s', 'now'), strftime('%s', 'now'));

INSERT OR IGNORE INTO admin_data_source (source_type, name, enabled, status, latest_sync, create_time, update_time)
VALUES
('population', '人流数据接入', 1, 'running', strftime('%s', 'now'), strftime('%s', 'now'), strftime('%s', 'now')),
('traffic', '车流数据接入', 1, 'running', strftime('%s', 'now'), strftime('%s', 'now'), strftime('%s', 'now')),
('grid', '网格月度导入', 1, 'ready', 0, strftime('%s', 'now'), strftime('%s', 'now'));

INSERT OR IGNORE INTO admin_grid_import (month, file_name, status, total_rows, success_rows, failed_rows, operator, create_time, complete_time)
VALUES
('2026-04', 'grid-2026-04.xlsx', 'completed', 1280, 1276, 4, 'admin', strftime('%s', 'now'), strftime('%s', 'now')),
('2026-03', 'grid-2026-03.xlsx', 'completed', 1268, 1268, 0, 'admin', strftime('%s', 'now') - 86400 * 30, strftime('%s', 'now') - 86400 * 30);

INSERT OR IGNORE INTO admin_grid_import_error (import_id, row_index, reason, raw_data)
VALUES
(1, 18, '案件编号为空', '{"grid":"高新区第一网格","case_name":"占道经营"}'),
(1, 126, '结案时间格式不正确', '{"case_no":"GX-202604-0126","closed_at":"4月20日"}');
