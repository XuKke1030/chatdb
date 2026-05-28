-- ============================================================
-- ChatDB 四月五月测试数据 (2026-04-01 ~ 2026-05-27)
-- case_list / population_flow_record / mobile_day_flow_tag /
-- pl_mobile_people_flow_data / traffic_gate_record
-- 各~1000条，只写原始表，聚合表由 RefreshAggregates 自动填充
-- ============================================================

SET NAMES utf8mb4;
SET UNIQUE_CHECKS = 0;
SET FOREIGN_KEY_CHECKS = 0;

-- ======================== case_list (~1026) ========================

DROP PROCEDURE IF EXISTS gen_case;
DELIMITER //
CREATE PROCEDURE gen_case()
BEGIN
  DECLARE d DATE DEFAULT '2026-04-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE seq INT DEFAULT 20000;
  DECLARE f DOUBLE;
  DECLARE i INT;
  DECLARE rh INT; DECLARE rm INT; DECLARE rdt DATETIME;
  DECLARE ct VARCHAR(200); DECLARE step_v VARCHAR(50);
  DECLARE region_v VARCHAR(200); DECLARE loc_v VARCHAR(500);

  DELETE FROM case_list WHERE case_number LIKE '%社管2026字%';
  WHILE d <= end_d DO
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.8; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.3; END IF;

    -- 高新-唐家 3条/天
    SET i = 0;
    WHILE i < 3 DO
      SET seq = seq + 1;
      SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
      SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
      SET ct = ELT(1+FLOOR(RAND()*10), '城市管理/市政设施','城市管理/违法建设','环境卫生/垃圾清运','环境卫生/污水排放','市容市貌/乱摆卖','市容市貌/户外广告','市场监管/无证经营','市场监管/食品安全','社区服务/邻里纠纷','社区服务/公共设施');
      SET step_v = IF(RAND()<0.75, '结案', IF(RAND()<0.7, '办理中', '待受理'));
      SET region_v = '珠海市/高新区/唐家湾镇/唐家社区居委会';
      SET loc_v = CONCAT('高新区唐家湾镇唐家社区', ELT(1+FLOOR(RAND()*5),'后环街','山房路','唐淇路','港湾大道','金唐东路'), FLOOR(RAND()*100+1), '号');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('高新社管2026字第',LPAD(seq,5,'0'),'号'),ELT(1+FLOOR(RAND()*3),'案件网格员快速上报','市民热线','视频监控'),rdt,step_v,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 高新-淇澳 1条/天
    SET seq = seq + 1;
    SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
    SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
    SET ct = ELT(1+FLOOR(RAND()*4), '环境卫生/垃圾清运','城市管理/市政设施','市容市貌/乱摆卖','社区服务/公共设施');
    SET step_v = IF(RAND()<0.8, '结案', '办理中');
    INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('高新社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rdt,step_v,ct,'珠海市/高新区/唐家湾镇/淇澳社区居委会','高新区唐家湾镇淇澳社区居委会',CONCAT(ct,'相关问题'));

    -- 香洲-拱北 3条/天
    SET i = 0;
    WHILE i < 3 DO
      SET seq = seq + 1;
      SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
      SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
      SET ct = ELT(1+FLOOR(RAND()*10), '城市管理/市政设施','城市管理/违法建设','环境卫生/垃圾清运','环境卫生/污水排放','市容市貌/乱摆卖','市容市貌/户外广告','市场监管/无证经营','市场监管/食品安全','社区服务/邻里纠纷','社区服务/公共设施');
      SET step_v = IF(RAND()<0.75, '结案', IF(RAND()<0.7, '办理中', '待受理'));
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('香洲社管2026字第',LPAD(seq,5,'0'),'号'),ELT(1+FLOOR(RAND()*3),'案件网格员快速上报','市民热线','视频监控'),rdt,step_v,ct,'珠海市/香洲区/拱北街道/粤华社区居委会',CONCAT('香洲区拱北街道粤华社区', ELT(1+FLOOR(RAND()*4),'迎宾南路','岭南路','桂花路','粤华路'), FLOOR(RAND()*200+1), '号'),CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 香洲-翠香 2条/天
    SET i = 0;
    WHILE i < 2 DO
      SET seq = seq + 1;
      SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
      SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
      SET ct = ELT(1+FLOOR(RAND()*4), '环境卫生/垃圾清运','城市管理/市政设施','社区服务/邻里纠纷','市容市貌/户外广告');
      SET step_v = IF(RAND()<0.8, '结案', '办理中');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('香洲社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rdt,step_v,ct,'珠海市/香洲区/翠香街道/紫荆社区居委会',CONCAT('香洲区翠香街道紫荆社区', ELT(1+FLOOR(RAND()*3),'紫荆路','兴业路','银桦路'), FLOOR(RAND()*150+1), '号'),CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 斗门-井岸 2条/天
    SET i = 0;
    WHILE i < 2 DO
      SET seq = seq + 1;
      SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
      SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
      SET ct = ELT(1+FLOOR(RAND()*4), '城市管理/市政设施','环境卫生/垃圾清运','社区服务/公共设施','市场监管/无证经营');
      SET step_v = IF(RAND()<0.8, '结案', '办理中');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('斗门社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rdt,step_v,ct,'珠海市/斗门区/井岸镇/红旗社区居委会',CONCAT('斗门区井岸镇红旗社区', ELT(1+FLOOR(RAND()*3),'井岸大道','中兴路','港霞路'), FLOOR(RAND()*100+1), '号'),CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 斗门-白蕉 1条/天
    SET seq = seq + 1;
    SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
    SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
    SET ct = ELT(1+FLOOR(RAND()*3), '环境卫生/垃圾清运','城市管理/市政设施','社区服务/公共设施');
    INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('斗门社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rdt,IF(RAND()<0.85,'结案','办理中'),ct,'珠海市/斗门区/白蕉镇/城东社区居委会','斗门区白蕉镇城东社区居委会',CONCAT(ct,'相关问题'));

    -- 金湾-三灶 1条/天
    SET seq = seq + 1;
    SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
    SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
    SET ct = ELT(1+FLOOR(RAND()*3), '环境卫生/垃圾清运','城市管理/市政设施','社区服务/公共设施');
    INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('金湾社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rdt,IF(RAND()<0.85,'结案','办理中'),ct,'珠海市/金湾区/三灶镇/中心村社区居委会','金湾区三灶镇中心村社区居委会',CONCAT(ct,'相关问题'));

    -- 横琴-荷塘 2条/天
    SET i = 0;
    WHILE i < 2 DO
      SET seq = seq + 1;
      SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
      SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
      SET ct = ELT(1+FLOOR(RAND()*4), '城市管理/市政设施','市容市貌/乱摆卖','市场监管/无证经营','社区服务/邻里纠纷');
      SET step_v = IF(RAND()<0.8, '结案', '办理中');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('横琴社管2026字第',LPAD(seq,5,'0'),'号'),ELT(1+FLOOR(RAND()*2),'案件网格员快速上报','市民热线'),rdt,step_v,ct,'珠海市/横琴新区/横琴镇/荷塘社区居委会',CONCAT('横琴新区横琴镇荷塘社区', ELT(1+FLOOR(RAND()*3),'环岛路','琴海东路','十字门大道'), FLOOR(RAND()*50+1), '号'),CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 随机补3条凑满18条/天
    SET i = 0;
    WHILE i < 3 DO
      SET seq = seq + 1;
      SET rh = 7 + FLOOR(RAND()*12); SET rm = FLOOR(RAND()*60);
      SET rdt = TIMESTAMP(d, MAKETIME(rh, rm, 0));
      SET ct = ELT(1+FLOOR(RAND()*10), '城市管理/市政设施','城市管理/违法建设','环境卫生/垃圾清运','环境卫生/污水排放','市容市貌/乱摆卖','市容市貌/户外广告','市场监管/无证经营','市场监管/食品安全','社区服务/邻里纠纷','社区服务/公共设施');
      SET step_v = IF(RAND()<0.75, '结案', IF(RAND()<0.7, '办理中', '待受理'));
      SET region_v = ELT(1+FLOOR(RAND()*8), '珠海市/高新区/唐家湾镇/唐家社区居委会','珠海市/高新区/唐家湾镇/淇澳社区居委会','珠海市/香洲区/拱北街道/粤华社区居委会','珠海市/香洲区/翠香街道/紫荆社区居委会','珠海市/斗门区/井岸镇/红旗社区居委会','珠海市/斗门区/白蕉镇/城东社区居委会','珠海市/金湾区/三灶镇/中心村社区居委会','珠海市/横琴新区/横琴镇/荷塘社区居委会');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('珠海社管2026字第',LPAD(seq,5,'0'),'号'),ELT(1+FLOOR(RAND()*3),'案件网格员快速上报','市民热线','视频监控'),rdt,step_v,ct,region_v,CONCAT('珠海市相关路段', FLOOR(RAND()*200+1), '号'),CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_case();
DROP PROCEDURE IF EXISTS gen_case;


-- ======================== population_flow_record (~1026) ========================

DROP PROCEDURE IF EXISTS gen_pop_rec;
DELIMITER //
CREATE PROCEDURE gen_pop_rec()
BEGIN
  DECLARE d DATE DEFAULT '2026-04-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE f DOUBLE;
  DECLARE mf DOUBLE;
  DECLARE area_idx INT;
  DECLARE area_name VARCHAR(100);
  DECLARE area_w DOUBLE;
  DECLARE io_r DOUBLE;
  DECLARE day_base INT;
  DECLARE h INT;
  DECLARE hw DOUBLE;
  DECLARE hour_in INT;
  DECLARE hour_out INT;
  DECLARE seq INT DEFAULT 1;

  DELETE FROM population_flow_record WHERE source_provider = 'test';
  WHILE d <= end_d DO
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.85; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.4; END IF;
    SET mf = 0.95 + (MONTH(d) - 4) * 0.03;

    -- 6区域x3个时段 = 18条/天 (每个时段代表全天1/3的人流)
    SET area_idx = 0;
    WHILE area_idx < 6 DO
      CASE area_idx
        WHEN 0 THEN SET area_name='高新';  SET area_w=0.40; SET io_r=1.03;
        WHEN 1 THEN SET area_name='香洲';  SET area_w=0.25; SET io_r=1.01;
        WHEN 2 THEN SET area_name='斗门';  SET area_w=0.15; SET io_r=0.98;
        WHEN 3 THEN SET area_name='金湾';  SET area_w=0.10; SET io_r=0.97;
        WHEN 4 THEN SET area_name='横琴';  SET area_w=0.07; SET io_r=1.05;
        WHEN 5 THEN SET area_name='淇澳岛'; SET area_w=0.03; SET io_r=1.10;
      END CASE;
      SET day_base = FLOOR(260000 * area_w * f * mf);

      -- 早(8点33%) 午(12点34%) 晚(18点33%) 代表全天
      SET h = 8;
      WHILE h <= 18 DO
        IF h = 8 THEN SET hw = 0.33;
        ELSEIF h = 12 THEN SET hw = 0.34;
        ELSEIF h = 18 THEN SET hw = 0.33;
        ELSE SET hw = 0.33;
        END IF;
        SET hour_in = FLOOR(day_base * hw * io_r * 0.49 * (1+RAND()*0.2-0.1));
        SET hour_out = FLOOR(day_base * hw / io_r * 0.51 * (1+RAND()*0.2-0.1));
        INSERT INTO population_flow_record (external_id,sync_version,metric_time,region,grid_name,in_count,out_count,net_in_count,floating_population_count,source_provider,raw_payload,create_time,update_time)
        VALUES (CONCAT('TEST-POP-',LPAD(seq,5,'0')), DATE_FORMAT(d,'%Y%m%d'), TIMESTAMP(d, MAKETIME(h, 0, 0)), area_name, '', hour_in, hour_out, hour_in-hour_out, FLOOR((hour_in+hour_out)*0.05), 'test', NULL, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());
        SET seq = seq + 1;
        IF h = 8 THEN SET h = 12;
        ELSEIF h = 12 THEN SET h = 18;
        ELSE SET h = 99;
        END IF;
      END WHILE;
      SET area_idx = area_idx + 1;
    END WHILE;
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_pop_rec();
DROP PROCEDURE IF EXISTS gen_pop_rec;


-- ======================== mobile_day_flow_tag (~1026) ========================

DROP PROCEDURE IF EXISTS gen_tag;
DELIMITER //
CREATE PROCEDURE gen_tag()
BEGIN
  DECLARE d DATE DEFAULT '2026-04-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE f DOUBLE;
  DECLARE mf DOUBLE;
  DECLARE area_idx INT;
  DECLARE area_name VARCHAR(100);
  DECLARE area_w DOUBLE;
  DECLARE area_base INT;
  DECLARE cnt INT;

  DELETE FROM mobile_day_flow_tag WHERE day >= '2026-04-01';
  WHILE d <= end_d DO
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.85; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.4; END IF;
    SET mf = 0.95 + (MONTH(d) - 4) * 0.03;

    -- 6区域 x (5年龄+2性别+3来源) x type=1 = 60条/天
    SET area_idx = 0;
    WHILE area_idx < 6 DO
      CASE area_idx
        WHEN 0 THEN SET area_name='高新';  SET area_w=0.40;
        WHEN 1 THEN SET area_name='香洲';  SET area_w=0.25;
        WHEN 2 THEN SET area_name='斗门';  SET area_w=0.15;
        WHEN 3 THEN SET area_name='金湾';  SET area_w=0.10;
        WHEN 4 THEN SET area_name='横琴';  SET area_w=0.07;
        WHEN 5 THEN SET area_name='淇澳岛'; SET area_w=0.03;
      END CASE;
      SET area_base = FLOOR(260000 * area_w * f * mf);

      -- 年龄 type=1, 5个label覆盖全年龄段
      SET cnt = FLOOR(area_base * 0.08 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','0-18',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.18 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','18-30',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.30 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','30-50',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.22 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','50-70',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.08 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','70+',1,d,NOW(3));

      -- 性别 type=1, 2个label
      SET cnt = FLOOR(area_base * 0.52 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'性别','男',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.48 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'性别','女',1,d,NOW(3));

      -- 来源地 type=1, 3个label
      SET cnt = FLOOR(area_base * 0.22 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','珠海',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.12 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','中山',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.10 * (1+RAND()*0.2-0.1));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广州',1,d,NOW(3));

      SET area_idx = area_idx + 1;
    END WHILE;
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_tag();
DROP PROCEDURE IF EXISTS gen_tag;


-- ======================== pl_mobile_people_flow_data (57条/天) ========================

DROP PROCEDURE IF EXISTS gen_pop_flow;
DELIMITER //
CREATE PROCEDURE gen_pop_flow()
BEGIN
  DECLARE d DATE DEFAULT '2026-04-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE f DOUBLE;
  DECLARE mf DOUBLE;
  DECLARE all_c INT;
  DECLARE in_c INT;
  DECLARE out_c INT;

  DELETE FROM pl_mobile_people_flow_data WHERE statistics_date >= '2026-04-01';
  WHILE d <= end_d DO
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.85; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.4; END IF;
    SET mf = 0.95 + (MONTH(d) - 4) * 0.03;
    SET all_c = FLOOR(260000 * f * mf * (1 + RAND()*0.16 - 0.08));
    SET in_c = FLOOR(all_c * (0.48 + RAND()*0.04));
    SET out_c = all_c - in_c;
    INSERT INTO pl_mobile_people_flow_data (all_count,in_count,out_count,statistics_date,activation,base_line_value) VALUES (all_c,in_c,out_c,d,ROUND(5.3+RAND()*0.5,2),ROUND(5.4+RAND()*0.2,2));
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_pop_flow();
DROP PROCEDURE IF EXISTS gen_pop_flow;


-- ======================== traffic_gate_record (~1026) ========================

DROP PROCEDURE IF EXISTS gen_gate;
DELIMITER //
CREATE PROCEDURE gen_gate()
BEGIN
  DECLARE d DATE DEFAULT '2026-04-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE gate_idx INT;
  DECLARE v_device_id VARCHAR(64);
  DECLARE v_device_name VARCHAR(128);
  DECLARE v_device_region VARCHAR(128);
  DECLARE v_hk_r DOUBLE;
  DECLARE v_in_r DOUBLE;
  DECLARE v_wknd_mul DOUBLE;
  DECLARE f DOUBLE;
  DECLARE is_h INT;
  DECLARE cnt INT;
  DECLARE i INT;
  DECLARE is_hk_val INT;
  DECLARE v_plate VARCHAR(32);
  DECLARE v_plate_orig VARCHAR(64);
  DECLARE v_plate_region_type VARCHAR(32);
  DECLARE snap_h INT;
  DECLARE snap_dt DATETIME;
  DECLARE v_plate_type VARCHAR(64);
  DECLARE v_plate_color VARCHAR(64);
  DECLARE v_car_color VARCHAR(64);
  DECLARE v_vehicle_dir VARCHAR(64);

  DELETE FROM traffic_gate_record WHERE device_id IN ('DVC-ZH-GQ-003','DVC-ZH-GB-005','DVC-ZH-HAM-IN-002','DVC-ZH-HAM-OUT-001');
  WHILE d <= end_d DO
    SET f = 1.0;
    SET is_h = 0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.9; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06') THEN SET f = 1.3; SET is_h = 1; END IF;
    IF d IN ('2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.5; SET is_h = 1; END IF;

    -- 4卡口 × ~5条 = ~20条/天
    SET gate_idx = 0;
    WHILE gate_idx < 4 DO
      CASE gate_idx
        WHEN 0 THEN SET v_device_id='DVC-ZH-GQ-003'; SET v_device_name='横琴口岸-北侧卡口'; SET v_device_region='横琴粤澳深度合作区'; SET v_hk_r=0.15; SET v_in_r=0.52; SET v_wknd_mul=1.3;
        WHEN 1 THEN SET v_device_id='DVC-ZH-GB-005'; SET v_device_name='拱北口岸-入方向'; SET v_device_region='香洲区'; SET v_hk_r=0.10; SET v_in_r=0.55; SET v_wknd_mul=1.2;
        WHEN 2 THEN SET v_device_id='DVC-ZH-HAM-IN-002'; SET v_device_name='洪澳岛-入方向'; SET v_device_region='香洲区'; SET v_hk_r=0.03; SET v_in_r=0.60; SET v_wknd_mul=0.7;
        WHEN 3 THEN SET v_device_id='DVC-ZH-HAM-OUT-001'; SET v_device_name='洪澳岛-出方向'; SET v_device_region='香洲区'; SET v_hk_r=0.03; SET v_in_r=0.40; SET v_wknd_mul=0.7;
      END CASE;

      IF DAYOFWEEK(d) IN (1,7) OR is_h = 1 THEN SET f = f * v_wknd_mul; END IF;

      SET cnt = 4 + FLOOR(RAND()*2);
      SET i = 0;
      WHILE i < cnt DO
        SET snap_h = 6 + FLOOR(RAND()*16);
        SET snap_dt = TIMESTAMP(d, MAKETIME(snap_h, FLOOR(RAND()*60), FLOOR(RAND()*60)));

        IF RAND() < v_hk_r THEN
          SET is_hk_val = 1;
          SET v_plate = CONCAT('粤Z', LPAD(FLOOR(RAND()*10000),4,'0'), '港');
          SET v_plate_orig = '港澳';
          SET v_plate_region_type = '港澳牌';
        ELSE
          SET is_hk_val = 0;
          SET v_plate = ELT(1+FLOOR(RAND()*8), '粤C12345','粤A67890','粤B11111','粤T22222','粤E33333','粤S44444','桂A55555','湘A66666');
          IF LEFT(v_plate, 1) = '粤' THEN
            SET v_plate_orig = '广东';
            SET v_plate_region_type = '普通';
          ELSE
            SET v_plate_orig = '大陆';
            SET v_plate_region_type = '普通';
          END IF;
        END IF;

        SET v_plate_type = ELT(1+FLOOR(RAND()*4), '小型汽车','中型汽车','新能源汽车','大型汽车');
        SET v_plate_color = ELT(1+FLOOR(RAND()*5), '白色','黑色','银灰','红色','蓝色');
        SET v_car_color = ELT(1+FLOOR(RAND()*5), '白色','黑色','银灰','红色','蓝色');
        SET v_vehicle_dir = ELT(1+FLOOR(RAND()*4), '东向西','西向东','南向北','北向南');

        INSERT INTO traffic_gate_record (device_id,device_name,camera_ip,plate_char,plate_normalized,plate_type,plate_color,vehicle_type,vehicle_color,vehicle_speed,in_dir,vehicle_dir,lane_id,snapshot_time,plate_origin,plate_region_type,is_hk_macau,raw_payload,payload_hash,create_time,update_time)
        VALUES (v_device_id, v_device_name, CONCAT('192.168.1.', 100+FLOOR(RAND()*50)),
          v_plate, v_plate, v_plate_type, v_plate_color, '小型汽车', v_car_color,
          20+FLOOR(RAND()*60), IF(RAND()<v_in_r,0,1),
          v_vehicle_dir,
          FLOOR(RAND()*4), snap_dt, v_plate_orig, v_plate_region_type, is_hk_val,
          NULL, NULL, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

        SET i = i + 1;
      END WHILE;
      SET gate_idx = gate_idx + 1;
    END WHILE;
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_gate();
DROP PROCEDURE IF EXISTS gen_gate;

SET UNIQUE_CHECKS = 1;
SET FOREIGN_KEY_CHECKS = 1;
