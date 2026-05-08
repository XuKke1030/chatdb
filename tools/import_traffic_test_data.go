package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	path := "docs/traffic-flow-test-data.mysql.sql"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	dsn := "root:123456@tcp(127.0.0.1:3306)/chatdb?charset=utf8mb4&parseTime=true&multiStatements=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	statements := splitSQL(string(content))
	for i, statement := range statements {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err = db.Exec(statement); err != nil {
			panic(fmt.Errorf("execute statement %d failed: %w\n%s", i+1, err, statement))
		}
	}
	fmt.Printf("imported %d statements from %s\n", len(statements), path)
}

func splitSQL(sqlText string) []string {
	lines := strings.Split(sqlText, "\n")
	statements := make([]string, 0)
	var builder strings.Builder
	inSingleQuote := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		for i := 0; i < len(line); i++ {
			ch := line[i]
			if ch == '\'' && (i == 0 || line[i-1] != '\\') {
				inSingleQuote = !inSingleQuote
			}
			if ch == ';' && !inSingleQuote {
				statements = append(statements, strings.TrimSpace(builder.String()))
				builder.Reset()
				continue
			}
			builder.WriteByte(ch)
		}
		builder.WriteByte('\n')
	}
	if statement := strings.TrimSpace(builder.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}
