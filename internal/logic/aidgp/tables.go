package aidgp

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

func CreateTables(ctx context.Context) error {
	db := g.DB("master")
	for _, statement := range aidgpTableSQL() {
		if _, err := db.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func aidgpTableSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS aidgp_sync_record (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	sync_type VARCHAR(64) NOT NULL,
	external_id VARCHAR(128) NOT NULL,
	sync_version VARCHAR(64),
	payload_hash VARCHAR(64) NOT NULL,
	raw_payload LONGTEXT NOT NULL,
	last_sync_time INT NOT NULL,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	UNIQUE KEY uk_aidgp_record (sync_type, external_id),
	INDEX idx_sync_type_time (sync_type, last_sync_time)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS population_flow_record (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	external_id VARCHAR(128) NOT NULL,
	sync_version VARCHAR(64),
	metric_time DATETIME NOT NULL,
	region VARCHAR(128),
	grid_name VARCHAR(128),
	in_count INT NOT NULL DEFAULT 0,
	out_count INT NOT NULL DEFAULT 0,
	net_in_count INT NOT NULL DEFAULT 0,
	floating_population_count INT NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	raw_payload LONGTEXT,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	UNIQUE KEY uk_population_external (external_id),
	INDEX idx_population_time_region (metric_time, region)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS population_metric_hourly (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_hour DATETIME NOT NULL,
	region VARCHAR(128),
	grid_name VARCHAR(128),
	in_count INT NOT NULL DEFAULT 0,
	out_count INT NOT NULL DEFAULT 0,
	net_in_count INT NOT NULL DEFAULT 0,
	floating_population_count INT NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_pop_hour (metric_hour, region, grid_name),
	INDEX idx_pop_hour_region (metric_hour, region)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS population_metric_daily (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_date DATE NOT NULL,
	region VARCHAR(128),
	grid_name VARCHAR(128),
	in_count INT NOT NULL DEFAULT 0,
	out_count INT NOT NULL DEFAULT 0,
	net_in_count INT NOT NULL DEFAULT 0,
	floating_population_count INT NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_pop_daily (metric_date, region, grid_name),
	INDEX idx_pop_region_date (region, metric_date)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS population_floating_daily (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_date DATE NOT NULL,
	region VARCHAR(128),
	grid_name VARCHAR(128),
	floating_population_count INT NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_pop_float_daily (metric_date, region, grid_name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS grid_case_record (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	external_id VARCHAR(128) NOT NULL,
	sync_version VARCHAR(64),
	case_number VARCHAR(128),
	region VARCHAR(128),
	community VARCHAR(128),
	grid_name VARCHAR(128),
	case_type1 VARCHAR(128),
	case_type2 VARCHAR(128),
	case_title VARCHAR(255),
	case_status VARCHAR(64),
	report_time DATETIME NULL,
	close_time DATETIME NULL,
	major_score DECIMAL(10,2) NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	raw_payload LONGTEXT,
	create_time INT NOT NULL,
	update_time INT NOT NULL,
	UNIQUE KEY uk_grid_external (external_id),
	INDEX idx_grid_report_region (report_time, region),
	INDEX idx_grid_status (case_status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS grid_metric_daily (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_date DATE NOT NULL,
	region VARCHAR(128),
	community VARCHAR(128),
	grid_name VARCHAR(128),
	case_type1 VARCHAR(128),
	case_type2 VARCHAR(128),
	case_count INT NOT NULL DEFAULT 0,
	closed_count INT NOT NULL DEFAULT 0,
	close_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
	avg_handle_hours DECIMAL(10,2) NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_grid_day (metric_date, region, community, grid_name, case_type1, case_type2)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS grid_metric_monthly (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_month VARCHAR(7) NOT NULL,
	region VARCHAR(128),
	community VARCHAR(128),
	grid_name VARCHAR(128),
	case_type1 VARCHAR(128),
	case_type2 VARCHAR(128),
	case_count INT NOT NULL DEFAULT 0,
	closed_count INT NOT NULL DEFAULT 0,
	close_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
	avg_handle_hours DECIMAL(10,2) NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_grid_month (metric_month, region, community, grid_name, case_type1, case_type2)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS traffic_metric_hourly (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_hour DATETIME NOT NULL,
	region VARCHAR(128),
	device_id VARCHAR(64),
	device_name VARCHAR(128),
	total INT NOT NULL DEFAULT 0,
	in_count INT NOT NULL DEFAULT 0,
	out_count INT NOT NULL DEFAULT 0,
	hk_macau_count INT NOT NULL DEFAULT 0,
	mainland_count INT NOT NULL DEFAULT 0,
	foreign_count INT NOT NULL DEFAULT 0,
	province_inside_count INT NOT NULL DEFAULT 0,
	province_outside_count INT NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_traffic_hour (metric_hour, region, device_id),
	INDEX idx_traffic_hour_region (metric_hour, region)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS traffic_metric_daily (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_date DATE NOT NULL,
	region VARCHAR(128),
	device_id VARCHAR(64),
	device_name VARCHAR(128),
	total INT NOT NULL DEFAULT 0,
	in_count INT NOT NULL DEFAULT 0,
	out_count INT NOT NULL DEFAULT 0,
	hk_macau_count INT NOT NULL DEFAULT 0,
	mainland_count INT NOT NULL DEFAULT 0,
	foreign_count INT NOT NULL DEFAULT 0,
	province_inside_count INT NOT NULL DEFAULT 0,
	province_outside_count INT NOT NULL DEFAULT 0,
	is_holiday TINYINT(1) NOT NULL DEFAULT 0,
	holiday_name VARCHAR(64) DEFAULT '',
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_traffic_daily (metric_date, region, device_id),
	INDEX idx_traffic_date_region (metric_date, region)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS traffic_vehicle_stay_daily (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_date DATE NOT NULL,
	plate_normalized VARCHAR(32) NOT NULL,
	region VARCHAR(128),
	device_count INT NOT NULL DEFAULT 0,
	stay_minutes INT NOT NULL DEFAULT 0,
	first_seen_at DATETIME NULL,
	last_seen_at DATETIME NULL,
	source_provider VARCHAR(32) NOT NULL DEFAULT 'aidgp',
	update_time INT NOT NULL,
	UNIQUE KEY uk_traffic_stay (metric_date, plate_normalized, region),
	INDEX idx_traffic_stay_date (metric_date, stay_minutes)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS traffic_stay_distribution_daily (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_date DATE NOT NULL,
	region VARCHAR(64) NOT NULL DEFAULT '',
	bucket VARCHAR(32) NOT NULL,
	vehicle_count BIGINT NOT NULL DEFAULT 0,
	avg_stay_minutes DECIMAL(10,1) DEFAULT 0,
	source_provider VARCHAR(32) DEFAULT 'aidgp',
	update_time BIGINT NOT NULL DEFAULT 0,
	UNIQUE KEY uk_stay_dist (metric_date, region, bucket)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS traffic_origin_daily (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	metric_date DATE NOT NULL,
	province VARCHAR(32) NOT NULL,
	city VARCHAR(64) NOT NULL DEFAULT '',
	vehicle_count BIGINT NOT NULL DEFAULT 0,
	is_hk_macau TINYINT NOT NULL DEFAULT 0,
	source_provider VARCHAR(32) DEFAULT 'aidgp',
	update_time BIGINT NOT NULL DEFAULT 0,
	UNIQUE KEY uk_origin (metric_date, province, city)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS traffic_holiday (
	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	holiday_date DATE NOT NULL,
	holiday_name VARCHAR(64) NOT NULL,
	holiday_type VARCHAR(16) NOT NULL,
	UNIQUE KEY uk_holiday_date (holiday_date)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
}
