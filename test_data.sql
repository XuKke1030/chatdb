-- ============================================================
-- ChatDB 测试数据 SQL
-- 包含：系统表数据 + 业务测试表数据
-- 适用于 MySQL
-- ============================================================

-- ============================================================
-- 1. 项目系统表测试数据
-- ============================================================

-- 插入测试用户（密码为 MD5("123456")）
INSERT INTO `user` (`username`, `password`, `verify`, `rule_level`, `last_login_tme`, `create_time`, `update_time`) VALUES
('admin', 'e10adc3949ba59abbe56e057f20f883e', 0, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('zhangsan', 'e10adc3949ba59abbe56e057f20f883e', 0, 2, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('lisi', 'e10adc3949ba59abbe56e057f20f883e', 0, 2, UNIX_TIMESTAMP(), UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('wangwu', 'e10adc3949ba59abbe56e057f20f883e', 0, 2, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- 插入数据库配置
INSERT INTO `database_conf` (`db_name`, `user_name`, `password`, `host`, `port`, `db_type`, `create_time`, `update_time`) VALUES
('chatdb', 'root', '123456', '127.0.0.1', 3306, 'mysql', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('erp_db', 'admin', 'admin123', '192.168.1.100', 3306, 'mysql', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('log_db', 'postgres', 'pg123456', '192.168.1.101', 5432, 'postgres', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('analytics', 'readonly', 'ro123456', '10.0.0.50', 3306, 'mysql', UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- ============================================================
-- 2. 业务测试表（用于 AI SQL 查询测试）
-- ============================================================

-- 部门表
CREATE TABLE IF NOT EXISTS `departments` (
    `id` INT PRIMARY KEY AUTO_INCREMENT,
    `dept_name` VARCHAR(50) NOT NULL COMMENT '部门名称',
    `location` VARCHAR(100) COMMENT '办公地点',
    `budget` DECIMAL(12,2) COMMENT '年度预算',
    `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO `departments` (`id`, `dept_name`, `location`, `budget`) VALUES
(1, '研发部', '3号楼5层', 5000000.00),
(2, '市场部', '1号楼2层', 3000000.00),
(3, '财务部', '2号楼3层', 1500000.00),
(4, '人力资源部', '1号楼1层', 1200000.00),
(5, '运维部', '3号楼6层', 2000000.00);

-- 员工表
CREATE TABLE IF NOT EXISTS `employees` (
    `id` INT PRIMARY KEY AUTO_INCREMENT,
    `name` VARCHAR(50) NOT NULL COMMENT '姓名',
    `gender` ENUM('男','女') DEFAULT '男' COMMENT '性别',
    `age` INT COMMENT '年龄',
    `phone` VARCHAR(20) COMMENT '电话',
    `email` VARCHAR(100) COMMENT '邮箱',
    `hire_date` DATE COMMENT '入职日期',
    `salary` DECIMAL(10,2) COMMENT '月薪',
    `dept_id` INT COMMENT '部门ID',
    `position` VARCHAR(50) COMMENT '职位',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1在职 0离职',
    `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO `employees` (`id`, `name`, `gender`, `age`, `phone`, `email`, `hire_date`, `salary`, `dept_id`, `position`, `status`) VALUES
(1, '张伟', '男', 32, '13800138001', 'zhangwei@company.com', '2020-03-15', 25000.00, 1, '高级后端工程师', 1),
(2, '李娜', '女', 28, '13800138002', 'lina@company.com', '2021-06-01', 18000.00, 1, '前端工程师', 1),
(3, '王强', '男', 35, '13800138003', 'wangqiang@company.com', '2019-01-10', 35000.00, 1, '技术总监', 1),
(4, '刘芳', '女', 29, '13800138004', 'liufang@company.com', '2022-02-20', 22000.00, 2, '市场专员', 1),
(5, '陈明', '男', 40, '13800138005', 'chenming@company.com', '2018-07-08', 30000.00, 2, '市场总监', 1),
(6, '赵敏', '女', 26, '13800138006', 'zhaomin@company.com', '2023-05-15', 15000.00, 3, '会计', 1),
(7, '孙涛', '男', 45, '13800138007', 'suntao@company.com', '2016-11-01', 28000.00, 3, '财务总监', 1),
(8, '周雪', '女', 31, '13800138008', 'zhouxue@company.com', '2020-09-12', 20000.00, 4, 'HR经理', 1),
(9, '吴磊', '男', 27, '13800138009', 'wulei@company.com', '2022-08-01', 16000.00, 5, '运维工程师', 1),
(10, '郑红', '女', 33, '13800138010', 'zhenghong@company.com', '2019-04-20', 24000.00, 5, '运维主管', 1),
(11, '杨帆', '男', 24, '13800138011', 'yangfan@company.com', '2024-01-05', 12000.00, 1, '初级工程师', 1),
(12, '黄磊', '男', 38, '13800138012', 'huanglei@company.com', '2017-06-18', 0.00, 2, '销售经理', 0);

-- 产品表
CREATE TABLE IF NOT EXISTS `products` (
    `id` INT PRIMARY KEY AUTO_INCREMENT,
    `product_name` VARCHAR(100) NOT NULL COMMENT '产品名称',
    `category` VARCHAR(50) COMMENT '分类',
    `price` DECIMAL(10,2) COMMENT '单价',
    `stock` INT DEFAULT 0 COMMENT '库存',
    `supplier` VARCHAR(100) COMMENT '供应商',
    `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO `products` (`id`, `product_name`, `category`, `price`, `stock`, `supplier`) VALUES
(1, 'MacBook Pro 14寸', '电子产品', 14999.00, 50, 'Apple中国'),
(2, 'iPhone 16 Pro', '电子产品', 8999.00, 120, 'Apple中国'),
(3, 'Dell XPS 15', '电子产品', 12999.00, 30, '戴尔中国'),
(4, '罗技MX Master 3S', '配件', 699.00, 200, '罗技中国'),
(5, '人体工学椅', '办公家具', 2999.00, 80, '西昊家具'),
(6, '升降办公桌', '办公家具', 3999.00, 45, '乐歌股份'),
(7, '4K显示器 27寸', '电子产品', 3499.00, 60, 'LG电子'),
(8, '机械键盘 Keychron', '配件', 899.00, 150, '京东京造');

-- 订单表
CREATE TABLE IF NOT EXISTS `orders` (
    `id` INT PRIMARY KEY AUTO_INCREMENT,
    `order_no` VARCHAR(50) NOT NULL UNIQUE COMMENT '订单编号',
    `customer_name` VARCHAR(50) COMMENT '客户名称',
    `total_amount` DECIMAL(12,2) COMMENT '订单总金额',
    `status` ENUM('待支付','已支付','已发货','已完成','已取消') DEFAULT '待支付' COMMENT '订单状态',
    `order_date` DATE COMMENT '下单日期',
    `emp_id` INT COMMENT '负责员工ID',
    `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO `orders` (`id`, `order_no`, `customer_name`, `total_amount`, `status`, `order_date`, `emp_id`) VALUES
(1, 'ORD-202604-0001', '腾讯科技', 59996.00, '已完成', '2026-04-01', 4),
(2, 'ORD-202604-0002', '阿里巴巴', 26997.00, '已发货', '2026-04-03', 4),
(3, 'ORD-202604-0003', '字节跳动', 8999.00, '已支付', '2026-04-05', 5),
(4, 'ORD-202604-0004', '美团', 14999.00, '待支付', '2026-04-08', 12),
(5, 'ORD-202604-0005', '京东', 44997.00, '已完成', '2026-04-10', 5),
(6, 'ORD-202604-0006', '小米', 12999.00, '已取消', '2026-04-12', 4),
(7, 'ORD-202604-0007', '华为', 23994.00, '已发货', '2026-04-15', 5),
(8, 'ORD-202604-0008', '百度', 6999.00, '已完成', '2026-04-18', 4);

-- 订单明细表
CREATE TABLE IF NOT EXISTS `order_items` (
    `id` INT PRIMARY KEY AUTO_INCREMENT,
    `order_id` INT COMMENT '订单ID',
    `product_id` INT COMMENT '产品ID',
    `quantity` INT DEFAULT 1 COMMENT '数量',
    `unit_price` DECIMAL(10,2) COMMENT '单价',
    `subtotal` DECIMAL(12,2) COMMENT '小计',
    `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO `order_items` (`id`, `order_id`, `product_id`, `quantity`, `unit_price`, `subtotal`) VALUES
(1, 1, 1, 4, 14999.00, 59996.00),
(2, 2, 2, 3, 8999.00, 26997.00),
(3, 3, 2, 1, 8999.00, 8999.00),
(4, 4, 1, 1, 14999.00, 14999.00),
(5, 5, 2, 5, 8999.00, 44995.00),
(6, 6, 3, 1, 12999.00, 12999.00),
(7, 7, 7, 4, 3499.00, 13996.00),
(8, 7, 4, 2, 699.00, 1398.00),
(9, 7, 8, 2, 899.00, 1798.00),
(10, 8, 4, 10, 699.00, 6990.00),
(11, 8, 8, 1, 0.00, 0.00);

-- 薪资发放记录表
CREATE TABLE IF NOT EXISTS `salary_records` (
    `id` INT PRIMARY KEY AUTO_INCREMENT,
    `emp_id` INT COMMENT '员工ID',
    `month` VARCHAR(7) COMMENT '月份',
    `base_salary` DECIMAL(10,2) COMMENT '基本工资',
    `bonus` DECIMAL(10,2) DEFAULT 0 COMMENT '奖金',
    `deduction` DECIMAL(10,2) DEFAULT 0 COMMENT '扣款',
    `net_salary` DECIMAL(10,2) COMMENT '实发工资',
    `pay_date` DATE COMMENT '发放日期',
    `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO `salary_records` (`id`, `emp_id`, `month`, `base_salary`, `bonus`, `deduction`, `net_salary`, `pay_date`) VALUES
(1, 1, '2026-03', 25000.00, 5000.00, 2000.00, 28000.00, '2026-04-05'),
(2, 2, '2026-03', 18000.00, 3000.00, 1000.00, 20000.00, '2026-04-05'),
(3, 3, '2026-03', 35000.00, 10000.00, 3000.00, 42000.00, '2026-04-05'),
(4, 4, '2026-03', 22000.00, 4000.00, 1500.00, 24500.00, '2026-04-05'),
(5, 5, '2026-03', 30000.00, 8000.00, 2500.00, 35500.00, '2026-04-05'),
(6, 6, '2026-03', 15000.00, 2000.00, 800.00, 16200.00, '2026-04-05'),
(7, 7, '2026-03', 28000.00, 6000.00, 2000.00, 32000.00, '2026-04-05'),
(8, 8, '2026-03', 20000.00, 3500.00, 1200.00, 22300.00, '2026-04-05'),
(9, 9, '2026-03', 16000.00, 2500.00, 900.00, 17600.00, '2026-04-05'),
(10, 10, '2026-03', 24000.00, 4500.00, 1800.00, 26700.00, '2026-04-05'),
(11, 1, '2026-04', 25000.00, 5500.00, 2000.00, 28500.00, '2026-04-22'),
(12, 3, '2026-04', 35000.00, 12000.00, 3000.00, 44000.00, '2026-04-22');

-- ============================================================
-- 3. 完成提示
-- ============================================================
-- 执行方式（在 MySQL 中）：
-- mysql -u root -p chatdb < test_data.sql
--
-- 测试时可向 AI 提问的示例：
-- 1. 查询每个部门的员工数量和平均工资
-- 2. 找出月薪最高的前3名员工
-- 3. 统计2026年4月的订单总额和订单数量
-- 4. 查询库存不足50的产品
-- 5. 计算研发部的年度人力成本
-- 6. 找出负责订单金额最多的员工
-- 7. 统计每个产品分类的总销售额
-- 8. 查询3月份实发工资超过30000的员工
-- 9. 找出已取消订单的客户名称
-- 10. 计算每个部门的年度预算使用率
-- ============================================================
