-- Traffic flow feature test data for MySQL.
-- Run this against the `chatdb` database after the backend has created the traffic tables.
-- The generated records are relative to CURDATE(), so "today" and "last 7 days" filters stay useful.

START TRANSACTION;

CREATE TEMPORARY TABLE IF NOT EXISTS traffic_test_seq (
  n INT PRIMARY KEY
);

DELETE FROM traffic_test_seq;

INSERT INTO traffic_test_seq (n) VALUES
(0),(1),(2),(3),(4),(5),(6),(7),(8),(9),
(10),(11),(12),(13),(14),(15),(16),(17),(18),(19),
(20),(21),(22),(23),(24),(25),(26),(27),(28),(29),
(30),(31),(32),(33),(34),(35),(36),(37),(38),(39),
(40),(41),(42),(43),(44),(45),(46),(47),(48),(49),
(50),(51),(52),(53),(54),(55),(56),(57),(58),(59),
(60),(61),(62),(63),(64),(65),(66),(67),(68),(69),
(70),(71),(72),(73),(74),(75),(76),(77),(78),(79),
(80),(81),(82),(83),(84),(85),(86),(87),(88),(89),
(90),(91),(92),(93),(94),(95),(96),(97),(98),(99),
(100),(101),(102),(103),(104),(105),(106),(107),(108),(109),
(110),(111),(112),(113),(114),(115),(116),(117),(118),(119);

INSERT INTO traffic_gate_device (
  device_id, device_sn, device_name, category, model, connection_status, work_status,
  region, address, longitude, latitude, owner_name, owner_phone, enabled,
  latest_seen_at, last_reported_at, device_created_at, remark, create_time, update_time
) VALUES
('DVC-ZH-HAM-OUT-001', 'QC000700010901000000', '洪澳岛-出方向', '影像', '车辆识别 AI 相机', '在线', '启动监测道路卡口', '香洲区',
 '广东省珠海市香洲区洪澳南主湾公园-停车场', 113.626915, 22.393198, '平台管理员', '18666111122', 1,
 CONCAT(CURDATE(), ' 11:08:02'), CONCAT(CURDATE(), ' 11:08:02'), '2022-09-11 18:09:52', '设备档案截图样例', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('DVC-ZH-HAM-IN-002', 'QC000700010901000001', '洪澳岛-入方向', '影像', '车辆识别 AI 相机', '在线', '启动监测道路卡口', '香洲区',
 '广东省珠海市香洲区洪澳南主湾公园-入口', 113.627812, 22.394088, '平台管理员', '18666111122', 1,
 CONCAT(CURDATE(), ' 11:12:35'), CONCAT(CURDATE(), ' 11:12:35'), '2022-09-11 18:12:15', '入方向测试设备', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('DVC-ZH-GQ-003', 'QC000700010901000002', '横琴口岸-北侧卡口', '影像', '车辆识别 AI 相机', '在线', '启动监测道路卡口', '横琴粤澳深度合作区',
 '广东省珠海市横琴口岸北侧道路', 113.543520, 22.136940, '运维一组', '18666111123', 1,
 CONCAT(CURDATE(), ' 10:55:01'), CONCAT(CURDATE(), ' 10:55:01'), '2023-04-18 09:30:00', '港澳车辆高频测试设备', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('DVC-ZH-XW-004', 'QC000700010901000003', '新湾路-双向卡口', '影像', '车辆识别 AI 相机', '离线', '最近心跳异常', '金湾区',
 '广东省珠海市金湾区新湾路', 113.371820, 22.145210, '运维二组', '18666111124', 0,
 DATE_SUB(CONCAT(CURDATE(), ' 08:15:00'), INTERVAL 1 DAY), DATE_SUB(CONCAT(CURDATE(), ' 08:15:00'), INTERVAL 1 DAY),
 '2023-07-05 14:20:00', '离线设备测试数据', UNIX_TIMESTAMP(), UNIX_TIMESTAMP())
ON DUPLICATE KEY UPDATE
  device_sn = VALUES(device_sn),
  device_name = VALUES(device_name),
  category = VALUES(category),
  model = VALUES(model),
  connection_status = VALUES(connection_status),
  work_status = VALUES(work_status),
  region = VALUES(region),
  address = VALUES(address),
  longitude = VALUES(longitude),
  latitude = VALUES(latitude),
  owner_name = VALUES(owner_name),
  owner_phone = VALUES(owner_phone),
  enabled = VALUES(enabled),
  latest_seen_at = VALUES(latest_seen_at),
  last_reported_at = VALUES(last_reported_at),
  device_created_at = VALUES(device_created_at),
  remark = VALUES(remark),
  update_time = UNIX_TIMESTAMP();

INSERT IGNORE INTO traffic_gate_record (
  device_id, device_name, camera_ip, plate_char, plate_normalized, plate_type, plate_color,
  vehicle_type, vehicle_type_ext, vehicle_color, vehicle_speed, in_dir, vehicle_dir, car_drv_dir,
  lane_id, lane_desc, lane_dir_desc, snapshot_time, collect_time, source_insert_time,
  plate_picture, panorama_picture, vehicle_picture, car_pre_brand, car_sub_brand, car_year_brand,
  plate_origin, plate_region_type, is_hk_macau, raw_payload, payload_hash, create_time, update_time
)
SELECT
  p.device_id,
  p.device_name,
  CONCAT('192.168.20.', 10 + (p.n % 80)) AS camera_ip,
  p.plate,
  p.plate,
  p.plate_type,
  p.plate_color,
  p.vehicle_type,
  p.vehicle_type_ext,
  p.vehicle_color,
  p.vehicle_speed,
  p.in_dir,
  IF(p.in_dir = 0, '入城方向', '出城方向') AS vehicle_dir,
  IF(p.in_dir = 0, '由南向北', '由北向南') AS car_drv_dir,
  1 + (p.n % 4) AS lane_id,
  CONCAT(1 + (p.n % 4), '号车道') AS lane_desc,
  IF(p.in_dir = 0, '入口车道', '出口车道') AS lane_dir_desc,
  p.snapshot_time,
  DATE_ADD(p.snapshot_time, INTERVAL 1 SECOND) AS collect_time,
  DATE_ADD(p.snapshot_time, INTERVAL 2 SECOND) AS source_insert_time,
  CONCAT('https://example.test/traffic/plate/', p.n, '.jpg') AS plate_picture,
  CONCAT('https://example.test/traffic/panorama/', p.n, '.jpg') AS panorama_picture,
  CONCAT('https://example.test/traffic/vehicle/', p.n, '.jpg') AS vehicle_picture,
  p.car_pre_brand,
  p.car_sub_brand,
  CAST(2018 + (p.n % 7) AS CHAR) AS car_year_brand,
  p.plate_origin,
  p.plate_region_type,
  p.is_hk_macau,
  CONCAT('{"source":"test-sql","deviceId":"', p.device_id, '","plate":"', p.plate, '","snapshotTime":"', DATE_FORMAT(p.snapshot_time, '%Y-%m-%d %H:%i:%s'), '"}') AS raw_payload,
  SHA2(CONCAT('traffic-test-', p.device_id, '-', p.plate, '-', DATE_FORMAT(p.snapshot_time, '%Y-%m-%d %H:%i:%s')), 256) AS payload_hash,
  UNIX_TIMESTAMP(p.snapshot_time) AS create_time,
  UNIX_TIMESTAMP() AS update_time
FROM (
  SELECT
    s.n,
    CASE s.n % 4
      WHEN 0 THEN 'DVC-ZH-HAM-OUT-001'
      WHEN 1 THEN 'DVC-ZH-HAM-IN-002'
      WHEN 2 THEN 'DVC-ZH-GQ-003'
      ELSE 'DVC-ZH-XW-004'
    END AS device_id,
    CASE s.n % 4
      WHEN 0 THEN '洪澳岛-出方向'
      WHEN 1 THEN '洪澳岛-入方向'
      WHEN 2 THEN '横琴口岸-北侧卡口'
      ELSE '新湾路-双向卡口'
    END AS device_name,
    CASE s.n % 12
      WHEN 0 THEN '粤C8T23X'
      WHEN 1 THEN '粤C5K92D'
      WHEN 2 THEN '粤Z1234港'
      WHEN 3 THEN '粤Z5678澳'
      WHEN 4 THEN '粤B7P31Q'
      WHEN 5 THEN 'AB1234'
      WHEN 6 THEN 'MA1234'
      WHEN 7 THEN '粤C2M68R'
      WHEN 8 THEN '粤Z8899港'
      WHEN 9 THEN '粤C9N18L'
      WHEN 10 THEN 'HK6688'
      ELSE '粤Z3366澳'
    END AS plate,
    CASE s.n % 12
      WHEN 2 THEN '香港跨境车牌'
      WHEN 3 THEN '澳门跨境车牌'
      WHEN 5 THEN '香港本地车牌'
      WHEN 6 THEN '澳门本地车牌'
      WHEN 8 THEN '香港跨境车牌'
      WHEN 10 THEN '香港本地车牌'
      WHEN 11 THEN '澳门跨境车牌'
      ELSE '大陆车牌'
    END AS plate_type,
    CASE s.n % 5
      WHEN 0 THEN '蓝'
      WHEN 1 THEN '绿'
      WHEN 2 THEN '黄'
      WHEN 3 THEN '黑'
      ELSE '白'
    END AS plate_color,
    CASE s.n % 6
      WHEN 0 THEN '小型客车'
      WHEN 1 THEN '新能源小型客车'
      WHEN 2 THEN '大型客车'
      WHEN 3 THEN '轻型货车'
      WHEN 4 THEN '出租车'
      ELSE '网约车'
    END AS vehicle_type,
    CASE s.n % 4
      WHEN 0 THEN '私家车'
      WHEN 1 THEN '营运车'
      WHEN 2 THEN '公务车'
      ELSE '货运车'
    END AS vehicle_type_ext,
    CASE s.n % 6
      WHEN 0 THEN '白色'
      WHEN 1 THEN '黑色'
      WHEN 2 THEN '银色'
      WHEN 3 THEN '蓝色'
      WHEN 4 THEN '灰色'
      ELSE '红色'
    END AS vehicle_color,
    18 + (s.n * 7 % 54) AS vehicle_speed,
    IF(s.n % 3 = 0, 0, 1) AS in_dir,
    CASE s.n % 4
      WHEN 0 THEN '比亚迪'
      WHEN 1 THEN '丰田'
      WHEN 2 THEN '特斯拉'
      ELSE '本田'
    END AS car_pre_brand,
    CASE s.n % 4
      WHEN 0 THEN '宋PLUS'
      WHEN 1 THEN '凯美瑞'
      WHEN 2 THEN 'Model 3'
      ELSE '雅阁'
    END AS car_sub_brand,
    CASE s.n % 12
      WHEN 2 THEN '香港'
      WHEN 3 THEN '澳门'
      WHEN 5 THEN '香港'
      WHEN 6 THEN '澳门'
      WHEN 8 THEN '香港'
      WHEN 10 THEN '香港'
      WHEN 11 THEN '澳门'
      ELSE '广东珠海'
    END AS plate_origin,
    CASE s.n % 12
      WHEN 2 THEN 'hong_kong'
      WHEN 3 THEN 'macau'
      WHEN 5 THEN 'hong_kong'
      WHEN 6 THEN 'macau'
      WHEN 8 THEN 'hong_kong'
      WHEN 10 THEN 'hong_kong'
      WHEN 11 THEN 'macau'
      ELSE 'mainland'
    END AS plate_region_type,
    CASE s.n % 12
      WHEN 2 THEN 1
      WHEN 3 THEN 1
      WHEN 5 THEN 1
      WHEN 6 THEN 1
      WHEN 8 THEN 1
      WHEN 10 THEN 1
      WHEN 11 THEN 1
      ELSE 0
    END AS is_hk_macau,
    DATE_ADD(
      DATE_SUB(CONCAT(CURDATE(), ' 07:00:00'), INTERVAL FLOOR(s.n / 18) DAY),
      INTERVAL ((s.n % 18) * 47 + (s.n % 5) * 3) MINUTE
    ) AS snapshot_time
  FROM traffic_test_seq s
) p;

INSERT INTO traffic_ingest_status (
  source, enabled, connected, topic, latest_received_at, today_received, latest_error, create_time, update_time
) VALUES (
  'mqtt', 1, 1, 'zh-iot-jinshan-kakou', CONCAT(CURDATE(), ' 11:08:02'), 18, '', UNIX_TIMESTAMP(), UNIX_TIMESTAMP()
)
ON DUPLICATE KEY UPDATE
  enabled = VALUES(enabled),
  connected = VALUES(connected),
  topic = VALUES(topic),
  latest_received_at = VALUES(latest_received_at),
  today_received = VALUES(today_received),
  latest_error = VALUES(latest_error),
  update_time = UNIX_TIMESTAMP();

INSERT INTO traffic_ingest_log (
  source, topic, status, message, device_id, payload_hash, raw_payload, create_time
)
SELECT
  'mqtt',
  'zh-iot-jinshan-kakou',
  CASE s.n % 10 WHEN 9 THEN 'duplicate' ELSE 'success' END,
  CASE s.n % 10 WHEN 9 THEN '测试数据：重复消息已跳过' ELSE '测试数据：入库成功' END,
  CASE s.n % 4
    WHEN 0 THEN 'DVC-ZH-HAM-OUT-001'
    WHEN 1 THEN 'DVC-ZH-HAM-IN-002'
    WHEN 2 THEN 'DVC-ZH-GQ-003'
    ELSE 'DVC-ZH-XW-004'
  END,
  SHA2(CONCAT('traffic-test-log-', s.n), 256),
  CONCAT('{"source":"test-sql","sequence":', s.n, '}'),
  UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL s.n MINUTE))
FROM traffic_test_seq s
WHERE s.n < 30;

DROP TEMPORARY TABLE IF EXISTS traffic_test_seq;

COMMIT;
