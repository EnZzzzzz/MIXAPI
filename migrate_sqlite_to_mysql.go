package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/glebarez/sqlite"
	_ "github.com/go-sql-driver/mysql"
)

// Config 迁移配置
type Config struct {
	SQLitePath string
	MySQLDSN   string
	BatchSize  int
}

// MigrationStats 迁移统计
type MigrationStats struct {
	Table       string
	TotalRows   int
	SuccessRows int
	FailedRows  int
	Duration    time.Duration
	Columns     []string // 实际迁移的字段
}

func main() {
	config := Config{
		SQLitePath: getEnv("SQLITE_PATH", "./one-api.db"),
		MySQLDSN:   getEnv("MYSQL_DSN", "root:123456@tcp(localhost:3306)/mixapi?parseTime=true&multiStatements=true"),
		BatchSize:  1000,
	}

	fmt.Println("========================================")
	fmt.Println("SQLite to MySQL Migration Tool")
	fmt.Println("========================================")
	fmt.Printf("SQLite: %s\n", config.SQLitePath)
	fmt.Printf("MySQL: %s\n", config.MySQLDSN)
	fmt.Println("========================================")

	// 连接数据库
	sqliteDB, err := sql.Open("sqlite", config.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open SQLite: %v", err)
	}
	defer sqliteDB.Close()

	mysqlDB, err := sql.Open("mysql", config.MySQLDSN)
	if err != nil {
		log.Fatalf("Failed to open MySQL: %v", err)
	}
	defer mysqlDB.Close()

	// 测试连接
	if err := sqliteDB.Ping(); err != nil {
		log.Fatalf("Failed to ping SQLite: %v", err)
	}
	if err := mysqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping MySQL: %v", err)
	}

	fmt.Println("✓ Database connections established")

	// 执行迁移 - 按照外键依赖顺序
	tables := []string{
		"users",
		"channels",
		"tokens",
		"abilities",
		"options",
		"logs",
		"redemptions",
		"top_ups",
		"midjourneys",
		"tasks",
		"setups",
		"quota_data",
		"token_usage_logs",
		"usage_statistics",
	}

	var allStats []MigrationStats
	startTime := time.Now()

	for _, table := range tables {
		stats := migrateTableDynamic(sqliteDB, mysqlDB, table, config.BatchSize)
		allStats = append(allStats, stats)
		printStats(stats)
	}

	totalDuration := time.Since(startTime)

	// 打印汇总
	fmt.Println("\n========================================")
	fmt.Println("Migration Summary")
	fmt.Println("========================================")
	totalRows := 0
	totalSuccess := 0
	totalFailed := 0
	for _, stats := range allStats {
		totalRows += stats.TotalRows
		totalSuccess += stats.SuccessRows
		totalFailed += stats.FailedRows
	}
	fmt.Printf("Total Tables: %d\n", len(allStats))
	fmt.Printf("Total Rows: %d\n", totalRows)
	fmt.Printf("Success: %d\n", totalSuccess)
	fmt.Printf("Failed: %d\n", totalFailed)
	fmt.Printf("Total Duration: %v\n", totalDuration)
	fmt.Println("========================================")
}

// getSQLiteColumns 获取 SQLite 表的字段列表
func getSQLiteColumns(db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk)
		if err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

// getMySQLColumns 获取 MySQL 表的字段列表
func getMySQLColumns(db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.Query(
		"SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ?",
		tableName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

// getCommonColumns 获取两个表的共同字段
func getCommonColumns(sqliteCols, mysqlCols []string) []string {
	mysqlColSet := make(map[string]bool)
	for _, col := range mysqlCols {
		mysqlColSet[col] = true
	}

	var common []string
	for _, col := range sqliteCols {
		if mysqlColSet[col] {
			common = append(common, col)
		}
	}

	// 按字母顺序排序，确保一致性
	sort.Strings(common)
	return common
}

// buildSelectSQL 构建 SELECT SQL
func buildSelectSQL(tableName string, columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	// 处理 SQLite 中的保留字，如 group
	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		if col == "group" || col == "key" || col == "order" {
			quotedCols[i] = fmt.Sprintf("`%s`", col)
		} else {
			quotedCols[i] = col
		}
	}
	return fmt.Sprintf("SELECT %s FROM %s", strings.Join(quotedCols, ", "), tableName)
}

// buildInsertSQL 构建 INSERT SQL
func buildInsertSQL(tableName string, columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	// 处理 MySQL 中的保留字
	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		if col == "group" || col == "key" || col == "order" {
			quotedCols[i] = fmt.Sprintf("`%s`", col)
		} else {
			quotedCols[i] = col
		}
	}

	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = "?"
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "))
}

// migrateTableDynamic 动态迁移表 - 只迁移共同字段
func migrateTableDynamic(sqliteDB, mysqlDB *sql.DB, tableName string, batchSize int) MigrationStats {
	stats := MigrationStats{Table: tableName}
	startTime := time.Now()

	fmt.Printf("\n[%s] Starting migration...\n", tableName)

	// 获取两个表的字段
	sqliteCols, err := getSQLiteColumns(sqliteDB, tableName)
	if err != nil {
		fmt.Printf("[%s] Error getting SQLite columns: %v\n", tableName, err)
		stats.Duration = time.Since(startTime)
		return stats
	}

	mysqlCols, err := getMySQLColumns(mysqlDB, tableName)
	if err != nil {
		fmt.Printf("[%s] Error getting MySQL columns: %v\n", tableName, err)
		stats.Duration = time.Since(startTime)
		return stats
	}

	// 获取共同字段
	commonCols := getCommonColumns(sqliteCols, mysqlCols)
	stats.Columns = commonCols

	if len(commonCols) == 0 {
		fmt.Printf("[%s] No common columns found, skipping\n", tableName)
		stats.Duration = time.Since(startTime)
		return stats
	}

	fmt.Printf("[%s] SQLite columns: %d, MySQL columns: %d, Common: %d\n",
		tableName, len(sqliteCols), len(mysqlCols), len(commonCols))
	fmt.Printf("[%s] Common columns: %v\n", tableName, commonCols)

	// 先清空目标表数据（避免重复插入）
	_, err = mysqlDB.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		fmt.Printf("[%s] Warning: Failed to clear table: %v\n", tableName, err)
	}

	// 构建 SQL
	selectSQL := buildSelectSQL(tableName, commonCols)
	insertSQL := buildInsertSQL(tableName, commonCols)

	fmt.Printf("[%s] SELECT: %s\n", tableName, selectSQL)
	fmt.Printf("[%s] INSERT: %s\n", tableName, insertSQL)

	// 查询源数据
	rows, err := sqliteDB.Query(selectSQL)
	if err != nil {
		fmt.Printf("[%s] Error querying SQLite: %v\n", tableName, err)
		stats.Duration = time.Since(startTime)
		return stats
	}
	defer rows.Close()

	// 准备插入语句
	stmt, err := mysqlDB.Prepare(insertSQL)
	if err != nil {
		fmt.Printf("[%s] Error preparing MySQL statement: %v\n", tableName, err)
		stats.Duration = time.Since(startTime)
		return stats
	}
	defer stmt.Close()

	// 获取列类型信息用于扫描
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		fmt.Printf("[%s] Error getting column types: %v\n", tableName, err)
		stats.Duration = time.Since(startTime)
		return stats
	}

	// 批量插入
	count := 0
	batch := make([][]interface{}, 0, batchSize)

	for rows.Next() {
		// 动态创建扫描目标
		values := make([]interface{}, len(colTypes))
		valuePtrs := make([]interface{}, len(colTypes))
		for i := range colTypes {
			valuePtrs[i] = &values[i]
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			fmt.Printf("[%s] Error scanning row: %v\n", tableName, err)
			stats.FailedRows++
			continue
		}

		// 处理特殊类型的转换
		processedValues := make([]interface{}, len(values))
		for i, v := range values {
			processedValues[i] = convertValue(v)
		}

		batch = append(batch, processedValues)
		count++

		if len(batch) >= batchSize {
			success, failed := executeBatch(stmt, batch)
			stats.SuccessRows += success
			stats.FailedRows += failed
			batch = batch[:0]

			if count%10000 == 0 {
				fmt.Printf("[%s] Migrated %d rows...\n", tableName, count)
			}
		}
	}

	// 插入剩余数据
	if len(batch) > 0 {
		success, failed := executeBatch(stmt, batch)
		stats.SuccessRows += success
		stats.FailedRows += failed
	}

	stats.TotalRows = count
	stats.Duration = time.Since(startTime)

	fmt.Printf("[%s] Completed: %d rows in %v\n", tableName, count, stats.Duration)
	return stats
}

// convertValue 处理值转换，处理 SQLite 到 MySQL 的类型差异
func convertValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []byte:
		// SQLite 返回的字节数组转为字符串
		return string(val)
	case int64:
		return val
	case float64:
		return val
	case bool:
		return val
	case string:
		return val
	case time.Time:
		// 处理时间零值
		if val.IsZero() || val.Year() < 1900 {
			return nil
		}
		return val
	default:
		return v
	}
}

func executeBatch(stmt *sql.Stmt, batch [][]interface{}) (int, int) {
	success := 0
	failed := 0

	for _, values := range batch {
		_, err := stmt.Exec(values...)
		if err != nil {
			// 尝试打印错误但不终止
			failed++
			if failed <= 5 {
				fmt.Printf("  Error inserting row: %v\n", err)
			}
		} else {
			success++
		}
	}

	return success, failed
}

func printStats(stats MigrationStats) {
	status := "✓"
	if stats.FailedRows > 0 {
		status = "⚠"
	}
	if len(stats.Columns) == 0 {
		status = "○"
	}
	fmt.Printf("%s %s: %d rows (%d success, %d failed) in %v [columns: %d]\n",
		status, stats.Table, stats.TotalRows, stats.SuccessRows, stats.FailedRows, stats.Duration, len(stats.Columns))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// validateJSON 验证并返回有效的 JSON 字符串
func validateJSON(s string) string {
	if s == "" {
		return "{}"
	}
	var js interface{}
	if err := json.Unmarshal([]byte(s), &js); err != nil {
		return "{}"
	}
	return s
}
