package mcp

import (
	"testing"
)

func TestIsReadOnlySQL_Allowed(t *testing.T) {
	allowed := []string{
		"SELECT * FROM users",
		"select id, name from orders where id > 10",
		"WITH cte AS (SELECT 1) SELECT * FROM cte",
		"SHOW TABLES",
		"show databases",
		"DESCRIBE users",
		"DESC orders",
		"EXPLAIN SELECT * FROM users",
	}
	for _, sql := range allowed {
		if !isReadOnlySQL(sql) {
			t.Errorf("expected %q to be allowed (readonly)", sql)
		}
	}
}

func TestIsReadOnlySQL_Blocked(t *testing.T) {
	blocked := []string{
		"INSERT INTO users VALUES (1)",
		"UPDATE users SET name='x'",
		"DELETE FROM users",
		"DROP TABLE users",
		"ALTER TABLE users ADD col INT",
		"CREATE TABLE foo (id INT)",
		"TRUNCATE TABLE users",
		"REPLACE INTO users VALUES (1)",
		"GRANT SELECT ON db.* TO user",
		"REVOKE SELECT ON db.* FROM user",
		"EXEC sp_help",
		"CALL my_proc()",
		"MERGE INTO target USING src ON (1=1)",
		"LOCK TABLES users READ",
		"UNLOCK TABLES",
		"RENAME TABLE old TO new",
		"SELECT * INTO OUTFILE '/tmp/data.csv' FROM users",
		"SELECT * INTO DUMPFILE '/tmp/data' FROM users",
		"SELECT * FROM INFORMATION_SCHEMA.TABLES",
		"SELECT LOAD_FILE('/etc/passwd')",
		"SELECT BENCHMARK(1000000, SHA1('test'))",
		"SELECT SLEEP(5)",
		"SELECT 1; DROP TABLE users",
		"",
	}
	for _, sql := range blocked {
		if isReadOnlySQL(sql) {
			t.Errorf("expected %q to be blocked (not readonly)", sql)
		}
	}
}
