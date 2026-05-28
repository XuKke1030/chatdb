-- ============================================================
-- ChatDB 最近三个月测试数据 (2026-03-01 ~ 2026-05-27)
-- 只写入5张原始表，聚合表由刷新逻辑自动填充
-- 每天、每类至少30条数据
-- 人流6个区域，车流4个卡口（有差异性），网格8个区域
-- ============================================================

SET NAMES utf8mb4;
SET UNIQUE_CHECKS = 0;
SET FOREIGN_KEY_CHECKS = 0;

-- ======================== 1. case_list (网格原始表) ========================

DROP PROCEDURE IF EXISTS gen_case_list;
DELIMITER //
CREATE PROCEDURE gen_case_list()
BEGIN
  DECLARE d DATE DEFAULT '2026-03-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE f DOUBLE;
  DECLARE seq INT DEFAULT 10000;
  DECLARE cnt INT;
  DECLARE i INT;
  DECLARE rep_h INT;
  DECLARE rep_m INT;
  DECLARE rep_dt DATETIME;
  DECLARE ct VARCHAR(200);
  DECLARE step_val VARCHAR(50);
  DECLARE region_v VARCHAR(200);
  DECLARE loc_v VARCHAR(500);
  DECLARE unit_v VARCHAR(100);
  DECLARE src_v VARCHAR(100);
  DECLARE desc_v TEXT;

  WHILE d <= end_d DO
    -- 工作日/周末/节假日因子
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.8; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.3; END IF;

    -- 高新-唐家湾-唐家 (scale=1.0, 约14条/天)
    SET cnt = FLOOR(14 * f + RAND()*4 - 2);
    IF cnt < 2 THEN SET cnt = 2; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*10), '城市管理/市政设施','城市管理/违法建设','环境卫生/垃圾清运','环境卫生/污水排放','市容市貌/乱摆卖','市容市貌/户外广告','市场监管/无证经营','市场监管/食品安全','社区服务/邻里纠纷','社区服务/公共设施');
      SET step_val = IF(RAND()<0.75, '结案', IF(RAND()<0.7, '办理中', '待受理'));
      SET region_v = '珠海市/高新区/唐家湾镇/唐家社区居委会';
      SET loc_v = CONCAT('高新区唐家湾镇唐家社区', ELT(1+FLOOR(RAND()*5),'后环街','山房路','唐淇路','港湾大道','金唐东路'), RAND()*100+1, '号');
      SET unit_v = '巡查公司';
      SET src_v = ELT(1+FLOOR(RAND()*3), '案件网格员快速上报', '市民热线', '视频监控');
      SET desc_v = CONCAT(ct, '相关问题');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES (unit_v,CONCAT('高新社管2026字第',LPAD(seq,5,'0'),'号'),src_v,rep_dt,step_val,ct,region_v,loc_v,desc_v);
      SET i = i + 1;
    END WHILE;

    -- 高新-唐家湾-淇澳 (scale=0.3)
    SET cnt = FLOOR(5 * f + RAND()*2);
    IF cnt < 1 THEN SET cnt = 1; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*4), '环境卫生/垃圾清运','城市管理/市政设施','市容市貌/乱摆卖','社区服务/公共设施');
      SET step_val = IF(RAND()<0.8, '结案', '办理中');
      SET region_v = '珠海市/高新区/唐家湾镇/淇澳社区居委会';
      SET loc_v = '高新区唐家湾镇淇澳社区居委会广东省唐家湾镇淇澳社区居委会';
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('高新社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rep_dt,step_val,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 香洲-拱北-粤华 (scale=0.8)
    SET cnt = FLOOR(11 * f + RAND()*4 - 2);
    IF cnt < 2 THEN SET cnt = 2; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*10), '城市管理/市政设施','城市管理/违法建设','环境卫生/垃圾清运','环境卫生/污水排放','市容市貌/乱摆卖','市容市貌/户外广告','市场监管/无证经营','市场监管/食品安全','社区服务/邻里纠纷','社区服务/公共设施');
      SET step_val = IF(RAND()<0.75, '结案', IF(RAND()<0.7, '办理中', '待受理'));
      SET region_v = '珠海市/香洲区/拱北街道/粤华社区居委会';
      SET loc_v = CONCAT('香洲区拱北街道粤华社区', ELT(1+FLOOR(RAND()*4),'迎宾南路','岭南路','桂花路','粤华路'), FLOOR(RAND()*200+1), '号');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('香洲社管2026字第',LPAD(seq,5,'0'),'号'),ELT(1+FLOOR(RAND()*3),'案件网格员快速上报','市民热线','视频监控'),rep_dt,step_val,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 香洲-翠香-紫荆 (scale=0.5)
    SET cnt = FLOOR(7 * f + RAND()*3 - 1);
    IF cnt < 1 THEN SET cnt = 1; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*4), '环境卫生/垃圾清运','城市管理/市政设施','社区服务/邻里纠纷','市容市貌/户外广告');
      SET step_val = IF(RAND()<0.8, '结案', '办理中');
      SET region_v = '珠海市/香洲区/翠香街道/紫荆社区居委会';
      SET loc_v = CONCAT('香洲区翠香街道紫荆社区', ELT(1+FLOOR(RAND()*3),'紫荆路','兴业路','银桦路'), FLOOR(RAND()*150+1), '号');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('香洲社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rep_dt,step_val,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 斗门-井岸-红旗 (scale=0.4)
    SET cnt = FLOOR(6 * f + RAND()*2);
    IF cnt < 1 THEN SET cnt = 1; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*4), '城市管理/市政设施','环境卫生/垃圾清运','社区服务/公共设施','市场监管/无证经营');
      SET step_val = IF(RAND()<0.8, '结案', '办理中');
      SET region_v = '珠海市/斗门区/井岸镇/红旗社区居委会';
      SET loc_v = CONCAT('斗门区井岸镇红旗社区', ELT(1+FLOOR(RAND()*3),'井岸大道','中兴路','港霞路'), FLOOR(RAND()*100+1), '号');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('斗门社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rep_dt,step_val,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 斗门-白蕉-城东 (scale=0.3)
    SET cnt = FLOOR(5 * f + RAND()*2);
    IF cnt < 1 THEN SET cnt = 1; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*3), '环境卫生/垃圾清运','城市管理/市政设施','社区服务/公共设施');
      SET step_val = IF(RAND()<0.85, '结案', '办理中');
      SET region_v = '珠海市/斗门区/白蕉镇/城东社区居委会';
      SET loc_v = '斗门区白蕉镇城东社区居委会';
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('斗门社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rep_dt,step_val,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 金湾-三灶-中心村 (scale=0.25)
    SET cnt = FLOOR(4 * f + RAND()*2);
    IF cnt < 1 THEN SET cnt = 1; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*3), '环境卫生/垃圾清运','城市管理/市政设施','社区服务/公共设施');
      SET step_val = IF(RAND()<0.85, '结案', '办理中');
      SET region_v = '珠海市/金湾区/三灶镇/中心村社区居委会';
      SET loc_v = '金湾区三灶镇中心村社区居委会';
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('金湾社管2026字第',LPAD(seq,5,'0'),'号'),'案件网格员快速上报',rep_dt,step_val,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    -- 横琴-横琴-荷塘 (scale=0.35)
    SET cnt = FLOOR(5 * f + RAND()*2);
    IF cnt < 1 THEN SET cnt = 1; END IF;
    SET i = 0;
    WHILE i < cnt DO
      SET seq = seq + 1;
      SET rep_h = 7 + FLOOR(RAND()*12);
      SET rep_m = FLOOR(RAND()*60);
      SET rep_dt = TIMESTAMP(d, MAKETIME(rep_h, rep_m, 0));
      SET ct = ELT(1+FLOOR(RAND()*4), '城市管理/市政设施','市容市貌/乱摆卖','市场监管/无证经营','社区服务/邻里纠纷');
      SET step_val = IF(RAND()<0.8, '结案', '办理中');
      SET region_v = '珠海市/横琴新区/横琴镇/荷塘社区居委会';
      SET loc_v = CONCAT('横琴新区横琴镇荷塘社区', ELT(1+FLOOR(RAND()*3),'环岛路','琴海东路','十字门大道'), FLOOR(RAND()*50+1), '号');
      INSERT INTO case_list (responsibility_unit,case_number,case_source,report_time,pending_step,case_type,region,case_location,description) VALUES ('巡查公司',CONCAT('横琴社管2026字第',LPAD(seq,5,'0'),'号'),ELT(1+FLOOR(RAND()*2),'案件网格员快速上报','市民热线'),rep_dt,step_val,ct,region_v,loc_v,CONCAT(ct,'相关问题'));
      SET i = i + 1;
    END WHILE;

    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_case_list();
DROP PROCEDURE IF EXISTS gen_case_list;


-- ======================== 2. pl_mobile_people_flow_data (人流全站汇总) ========================

DROP PROCEDURE IF EXISTS gen_pop_flow;
DELIMITER //
CREATE PROCEDURE gen_pop_flow()
BEGIN
  DECLARE d DATE DEFAULT '2026-03-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE f DOUBLE;
  DECLARE mf DOUBLE;
  DECLARE all_c INT;
  DECLARE in_c INT;
  DECLARE out_c INT;
  DECLARE act_val DECIMAL(10,2);
  DECLARE bl_val DECIMAL(10,2);

  WHILE d <= end_d DO
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.85; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.4; END IF;
    SET mf = 0.9 + (MONTH(d) - 3) * 0.05;
    SET all_c = FLOOR(260000 * f * mf * (1 + RAND()*0.16 - 0.08));
    IF all_c < 180000 THEN SET all_c = 180000; END IF;
    SET in_c = FLOOR(all_c * (0.48 + RAND()*0.04));
    SET out_c = all_c - in_c;
    SET act_val = FLOOR((5.3 + RAND()*0.5)*100)/100;
    SET bl_val = FLOOR((5.4 + RAND()*0.2)*100)/100;
    INSERT INTO pl_mobile_people_flow_data (all_count,in_count,out_count,statistics_date,activation,base_line_value) VALUES (all_c,in_c,out_c,d,act_val,bl_val);
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_pop_flow();
DROP PROCEDURE IF EXISTS gen_pop_flow;


-- ======================== 3. population_flow_record (人流原始记录) ========================

DROP PROCEDURE IF EXISTS gen_pop_flow_rec;
DELIMITER //
CREATE PROCEDURE gen_pop_flow_rec()
BEGIN
  DECLARE d DATE DEFAULT '2026-03-01';
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
  DECLARE hour_net INT;
  DECLARE hour_flt INT;
  DECLARE seq INT DEFAULT 1;

  WHILE d <= end_d DO
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.85; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.4; END IF;
    SET mf = 0.9 + (MONTH(d) - 3) * 0.05;

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

      -- 每区域每天24条(每小时1条汇总记录)
      SET h = 0;
      WHILE h < 24 DO
        IF h BETWEEN 0 AND 5 THEN SET hw = 0.01;
        ELSEIF h = 7 THEN SET hw = 0.065;
        ELSEIF h = 8 THEN SET hw = 0.08;
        ELSEIF h BETWEEN 9 AND 11 THEN SET hw = 0.055;
        ELSEIF h = 12 THEN SET hw = 0.05;
        ELSEIF h BETWEEN 13 AND 16 THEN SET hw = 0.05;
        ELSEIF h = 17 THEN SET hw = 0.07;
        ELSEIF h = 18 THEN SET hw = 0.09;
        ELSEIF h = 19 THEN SET hw = 0.06;
        ELSE SET hw = 0.03;
        END IF;

        SET hour_in = FLOOR(day_base * hw * io_r * 0.49 * (1+RAND()*0.2-0.1));
        SET hour_out = FLOOR(day_base * hw / io_r * 0.51 * (1+RAND()*0.2-0.1));
        SET hour_net = hour_in - hour_out;
        SET hour_flt = FLOOR((hour_in + hour_out) * 0.05);

        INSERT INTO population_flow_record (external_id,sync_version,metric_time,region,grid_name,in_count,out_count,net_in_count,floating_population_count,source_provider,raw_payload,create_time,update_time)
        VALUES (CONCAT('TEST-POP-',LPAD(seq,6,'0')), DATE_FORMAT(d,'%Y%m%d'), TIMESTAMP(d, MAKETIME(h, 0, 0)), area_name, '', hour_in, hour_out, hour_net, hour_flt, 'test', NULL, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

        SET seq = seq + 1;
        SET h = h + 1;
      END WHILE;
      SET area_idx = area_idx + 1;
    END WHILE;
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_pop_flow_rec();
DROP PROCEDURE IF EXISTS gen_pop_flow_rec;


-- ======================== 4. mobile_day_flow_tag (人流标签原始数据) ========================

DROP PROCEDURE IF EXISTS gen_flow_tag;
DELIMITER //
CREATE PROCEDURE gen_flow_tag()
BEGIN
  DECLARE d DATE DEFAULT '2026-03-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE f DOUBLE;
  DECLARE mf DOUBLE;
  DECLARE area_idx INT;
  DECLARE area_name VARCHAR(100);
  DECLARE area_w DOUBLE;
  DECLARE area_base INT;
  DECLARE cnt INT;
  DECLARE tp INT;
  DECLARE type_mul DOUBLE;

  WHILE d <= end_d DO
    SET f = 1.0;
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.85; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06','2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.4; END IF;
    SET mf = 0.9 + (MONTH(d) - 3) * 0.05;

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

      -- 年龄标签 type=1,2,3
      SET tp = 1;
      WHILE tp <= 3 DO
        SET type_mul = IF(tp=1, 1.0, IF(tp=2, 0.49, 0.51));
        SET cnt = FLOOR(area_base * 0.03 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(0,18]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.06 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(19,22]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.08 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(23,25]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.12 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(26,30]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.14 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(31,35]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.15 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(36,40]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.12 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(41,45]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.10 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(46,50]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.07 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(51,55]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.05 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','(56,60]',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.04 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','>60',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.04 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'年龄','未知',tp,d,NOW(3));
        SET tp = tp + 1;
      END WHILE;

      -- 性别标签 type=1,2,3
      SET tp = 1;
      WHILE tp <= 3 DO
        SET type_mul = IF(tp=1, 1.0, IF(tp=2, 0.49, 0.51));
        SET cnt = FLOOR(area_base * 0.45 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'性别','男',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.33 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'性别','女',tp,d,NOW(3));
        SET cnt = FLOOR(area_base * 0.22 * type_mul * (1+RAND()*0.2-0.1));
        INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'性别','未知',tp,d,NOW(3));
        SET tp = tp + 1;
      END WHILE;

      -- 省内城市来源 type=1
      SET cnt = FLOOR(area_base * 0.55 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东珠海',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.12 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东广州',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.10 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东中山',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.07 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东深圳',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.04 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东湛江',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.03 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东佛山',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.03 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东东莞',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.02 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东茂名',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.02 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东江门',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.01 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东汕头',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.01 * 0.65 * (1+RAND()*0.24-0.12));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省内城市来源','广东阳江',1,d,NOW(3));

      -- 省外来源 type=1
      SET cnt = FLOOR(area_base * 0.22 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','广西',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.20 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','湖南',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.13 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','河南',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.10 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','四川',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.09 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','湖北',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.08 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','江西',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.07 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','贵州',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.05 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','云南',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.03 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','北京',1,d,NOW(3));
      SET cnt = FLOOR(area_base * 0.03 * 0.15 * (1+RAND()*0.3-0.15));
      INSERT INTO mobile_day_flow_tag (area,label_cnt,tag,label,type,day,create_time) VALUES (area_name,cnt,'省外来源','江苏',1,d,NOW(3));

      SET area_idx = area_idx + 1;
    END WHILE;
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_flow_tag();
DROP PROCEDURE IF EXISTS gen_flow_tag;


-- ======================== 5. traffic_gate_record (车流原始记录) ========================

DROP PROCEDURE IF EXISTS gen_gate_rec;
DELIMITER //
CREATE PROCEDURE gen_gate_rec()
BEGIN
  DECLARE d DATE DEFAULT '2026-03-01';
  DECLARE end_d DATE DEFAULT '2026-05-27';
  DECLARE gate_idx INT;
  DECLARE dev_id VARCHAR(64);
  DECLARE dev_name VARCHAR(128);
  DECLARE dev_region VARCHAR(128);
  DECLARE hk_r DOUBLE;
  DECLARE in_r DOUBLE;
  DECLARE wknd_mul DOUBLE;
  DECLARE f DOUBLE;
  DECLARE is_h INT;
  DECLARE h_name VARCHAR(64);
  DECLARE cnt INT;
  DECLARE i INT;
  DECLARE is_hk_val INT;
  DECLARE plate VARCHAR(32);
  DECLARE plate_orig VARCHAR(64);
  DECLARE plate_region_type VARCHAR(32);
  DECLARE snap_h INT;
  DECLARE snap_dt DATETIME;
  DECLARE p_type VARCHAR(64);
  DECLARE p_color VARCHAR(64);
  DECLARE car_color VARCHAR(64);

  WHILE d <= end_d DO
    SET f = 1.0;
    SET is_h = 0;
    SET h_name = '';
    IF DAYOFWEEK(d) IN (1,7) THEN SET f = 0.9; END IF;
    IF d IN ('2026-04-04','2026-04-05','2026-04-06') THEN SET f = 1.3; SET is_h = 1; SET h_name = '清明节'; END IF;
    IF d IN ('2026-05-01','2026-05-02','2026-05-03','2026-05-04','2026-05-05') THEN SET f = 1.5; SET is_h = 1; SET h_name = '劳动节'; END IF;

    SET gate_idx = 0;
    WHILE gate_idx < 4 DO
      CASE gate_idx
        -- 横琴口岸：量大，港澳15%，周末多(1.3x)
        WHEN 0 THEN SET dev_id='DVC-ZH-GQ-003'; SET dev_name='横琴口岸-北侧卡口'; SET dev_region='横琴粤澳深度合作区'; SET hk_r=0.15; SET in_r=0.52; SET wknd_mul=1.3;
        -- 拱北口岸：量大，港澳10%，周末多(1.2x)
        WHEN 1 THEN SET dev_id='DVC-ZH-GB-005'; SET dev_name='拱北口岸-入方向'; SET dev_region='香洲区'; SET hk_r=0.10; SET in_r=0.55; SET wknd_mul=1.2;
        -- 洪澳岛入：量小，港澳3%，周末少(0.7x)
        WHEN 2 THEN SET dev_id='DVC-ZH-HAM-IN-002'; SET dev_name='洪澳岛-入方向'; SET dev_region='香洲区'; SET hk_r=0.03; SET in_r=0.60; SET wknd_mul=0.7;
        -- 洪澳岛出：量小，港澳3%，周末少(0.7x)
        WHEN 3 THEN SET dev_id='DVC-ZH-HAM-OUT-001'; SET dev_name='洪澳岛-出方向'; SET dev_region='香洲区'; SET hk_r=0.03; SET in_r=0.40; SET wknd_mul=0.7;
      END CASE;

      -- 周末/节假日应用卡口倍率
      IF DAYOFWEEK(d) IN (1,7) OR is_h = 1 THEN SET f = f * wknd_mul; END IF;

      SET cnt = FLOOR(35 * f);
      IF cnt < 20 THEN SET cnt = 20; END IF;
      SET i = 0;
      WHILE i < cnt DO
        SET snap_h = 6 + FLOOR(RAND()*16);
        SET snap_dt = TIMESTAMP(d, MAKETIME(snap_h, FLOOR(RAND()*60), FLOOR(RAND()*60)));

        IF RAND() < hk_r THEN
          SET is_hk_val = 1;
          SET plate = CONCAT('粤Z', LPAD(FLOOR(RAND()*10000),4,'0'), '港');
          SET plate_orig = '港澳';
          SET plate_region_type = '港澳牌';
        ELSE
          SET is_hk_val = 0;
          SET plate = ELT(1+FLOOR(RAND()*8), '粤C12345','粤A67890','粤B11111','粤T22222','粤E33333','粤S44444','桂A55555','湘A66666');
          SET plate_orig = '大陆';
          SET plate_region_type = '普通';
        END IF;

        SET p_type = ELT(1+FLOOR(RAND()*4), '小型汽车','中型汽车','新能源汽车','大型汽车');
        SET p_color = ELT(1+FLOOR(RAND()*5), '白色','黑色','银灰','红色','蓝色');
        SET car_color = ELT(1+FLOOR(RAND()*5), '白色','黑色','银灰','红色','蓝色');

        INSERT INTO traffic_gate_record (device_id,device_name,camera_ip,plate_char,plate_normalized,plate_type,plate_color,vehicle_type,vehicle_color,vehicle_speed,in_dir,vehicle_dir,lane_id,snapshot_time,plate_origin,plate_region_type,is_hk_macau,raw_payload,payload_hash,create_time,update_time)
        VALUES (dev_id, dev_name, CONCAT('192.168.1.', 100+FLOOR(RAND()*50)),
          plate, plate, p_type, p_color, '小型汽车', car_color,
          20+FLOOR(RAND()*60), IF(RAND()<in_r,0,1),
          ELT(1+FLOOR(RAND()*4), '东向西','西向东','南向北','北向南'),
          FLOOR(RAND()*4), snap_dt, plate_orig, plate_region_type, is_hk_val,
          NULL, NULL, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

        SET i = i + 1;
      END WHILE;
      SET gate_idx = gate_idx + 1;
    END WHILE;
    SET d = d + INTERVAL 1 DAY;
  END WHILE;
END //
DELIMITER ;
CALL gen_gate_rec();
DROP PROCEDURE IF EXISTS gen_gate_rec;

SET UNIQUE_CHECKS = 1;
SET FOREIGN_KEY_CHECKS = 1;
