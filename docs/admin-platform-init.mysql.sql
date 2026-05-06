-- 问答问数平台与管理后台初始化 SQL（MySQL 版本）
-- 用户端测试账号：
--   zhangsan / 123456，拥有网格 + 人流 + 车流权限
--   lisi / 123456，仅拥有人流权限
--   wangwu / 123456，仅拥有车流权限
--   grid_user / 123456，仅拥有网格权限
-- 管理后台测试账号：
--   admin / admin123

CREATE TABLE IF NOT EXISTS `user` (
    user_id INT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(64) NOT NULL,
    verify INT NOT NULL DEFAULT 0,
    rule_level INT NOT NULL DEFAULT 1,
    last_login_tme INT DEFAULT 0,
    create_time INT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
    update_time INT DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS database_conf (
    database_id INT PRIMARY KEY AUTO_INCREMENT,
    db_name VARCHAR(128) NOT NULL,
    user_name VARCHAR(128) NOT NULL,
    password VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INT NOT NULL,
    db_type VARCHAR(32) NOT NULL,
    user_id INT NOT NULL,
    create_time INT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
    update_time INT DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS chats (
    id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    messages LONGTEXT NOT NULL,
    create_time INT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
    update_time INT DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_account (
    id INT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(64) NOT NULL,
    verify INT NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'administrator',
    enabled TINYINT NOT NULL DEFAULT 1,
    create_time INT NOT NULL,
    update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_user_profile (
    user_id INT PRIMARY KEY,
    department VARCHAR(128),
    enabled TINYINT NOT NULL DEFAULT 1,
    rule_level INT NOT NULL DEFAULT 7,
    create_time INT NOT NULL,
    update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_example_question (
    id INT PRIMARY KEY AUTO_INCREMENT,
    topic VARCHAR(32) NOT NULL,
    question VARCHAR(255) NOT NULL,
    description TEXT,
    enabled TINYINT NOT NULL DEFAULT 1,
    sort INT NOT NULL DEFAULT 0,
    create_time INT NOT NULL,
    update_time INT NOT NULL,
    INDEX idx_topic_sort (topic, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_question_candidate (
    id INT PRIMARY KEY AUTO_INCREMENT,
    topic VARCHAR(32) NOT NULL,
    question VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    count INT NOT NULL DEFAULT 1,
    last_seen_at INT NOT NULL,
    create_time INT NOT NULL,
    update_time INT NOT NULL,
    INDEX idx_status_seen (status, last_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_data_source (
    id INT PRIMARY KEY AUTO_INCREMENT,
    source_type VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL,
    enabled TINYINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'closed',
    latest_sync INT NOT NULL DEFAULT 0,
    create_time INT NOT NULL,
    update_time INT NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_grid_import (
    id INT PRIMARY KEY AUTO_INCREMENT,
    month VARCHAR(16),
    file_name VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    total_rows INT NOT NULL DEFAULT 0,
    success_rows INT NOT NULL DEFAULT 0,
    failed_rows INT NOT NULL DEFAULT 0,
    operator VARCHAR(64),
    create_time INT NOT NULL,
    complete_time INT NOT NULL DEFAULT 0,
    INDEX idx_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS admin_grid_import_error (
    id INT PRIMARY KEY AUTO_INCREMENT,
    import_id INT NOT NULL,
    row_index INT NOT NULL,
    reason TEXT NOT NULL,
    raw_data TEXT,
    INDEX idx_import_id (import_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO `user` (username, password, verify, rule_level, create_time, update_time)
VALUES
('zhangsan', '65ba747efffca3c8acb1952b6ca6e417', 81371, 7, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('lisi', '4728ea0394876cda4bf1b870724ac106', 81372, 2, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('wangwu', '9b67706762d7b7d6c06df6717fbdfb33', 81373, 4, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('grid_user', '7e7f8e85bdff9991d0271c0ea8d2d938', 81374, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

INSERT IGNORE INTO admin_account (username, password, verify, role, enabled, create_time, update_time)
VALUES ('admin', 'e9125c6228bcd40b8d58098c62e5fd6f', 8137, 'administrator', 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '区治理中心', 1, rule_level, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()
FROM `user`
WHERE username = 'zhangsan';

REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '综合治理办', 1, rule_level, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()
FROM `user`
WHERE username = 'lisi';

REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '交通专班', 1, rule_level, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()
FROM `user`
WHERE username = 'wangwu';

REPLACE INTO admin_user_profile (user_id, department, enabled, rule_level, create_time, update_time)
SELECT user_id, '网格治理专班', 1, rule_level, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()
FROM `user`
WHERE username = 'grid_user';

INSERT IGNORE INTO admin_example_question (topic, question, description, enabled, sort, create_time, update_time)
VALUES
('grid', '本月高新区案件结案率是多少？', '查询网格案件办理成效', 1, 10, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('grid', '哪个网格的案件数量最多？', '查询网格案件排名', 1, 20, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('population', '过去一周人流进出趋势如何？', '展示每日进出人数对比', 1, 30, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('traffic', '今日港澳车辆占比是多少？', '统计重点卡口跨境车辆情况', 1, 40, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

INSERT IGNORE INTO admin_question_candidate (topic, question, status, count, last_seen_at, create_time, update_time)
VALUES
('grid', '本月哪个部门处理案件效率最高？', 'pending', 3, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('population', '节假日前后人流变化最大的区域是哪里？', 'pending', 5, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

INSERT IGNORE INTO admin_data_source (source_type, name, enabled, status, latest_sync, create_time, update_time)
VALUES
('population', '人流数据接入', 1, 'running', UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('traffic', '车流数据接入', 1, 'running', UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('grid', '网格月度导入', 1, 'ready', 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

INSERT IGNORE INTO admin_grid_import (month, file_name, status, total_rows, success_rows, failed_rows, operator, create_time, complete_time)
VALUES
('2026-04', 'grid-2026-04.xlsx', 'completed', 1280, 1276, 4, 'admin', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('2026-03', 'grid-2026-03.xlsx', 'completed', 1268, 1268, 0, 'admin', UNIX_TIMESTAMP() - 86400 * 30, UNIX_TIMESTAMP() - 86400 * 30);

INSERT IGNORE INTO admin_grid_import_error (import_id, row_index, reason, raw_data)
VALUES
(1, 18, '案件编号为空', '{"grid":"高新区第一网格","case_name":"占道经营"}'),
(1, 126, '结案时间格式不正确', '{"case_no":"GX-202604-0126","closed_at":"4月20日"}');
