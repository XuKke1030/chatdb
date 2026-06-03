package model

type DistributedLock struct {
	LockKey     string `json:"lockKey"`
	LockedBy    string `json:"lockedBy"`
	LockedUntil int    `json:"lockedUntil"`
}

const DistributedLockTable = "distributed_lock"

var DLockSQL = []string{
	`CREATE TABLE IF NOT EXISTS distributed_lock (
		lock_key    VARCHAR(128) PRIMARY KEY,
		locked_by   VARCHAR(128) DEFAULT '',
		locked_until INT DEFAULT 0,
		INDEX idx_expired (locked_until)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}
