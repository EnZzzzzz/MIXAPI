package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	Table      string
	TotalRows  int
	SuccessRows int
	FailedRows  int
	Duration    time.Duration
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
	migrators := []func(*sql.DB, *sql.DB, int) MigrationStats{
		migrateUsers,
		migrateChannels,
		migrateTokens,
		migrateAbilities,
		migrateOptions,
		migrateLogs,
		migrateRedemptions,
		migrateTopUps,
		migrateMidjourneys,
		migrateTasks,
		migrateSetups,
		migrateQuotaData,
		migrateTokenUsageLogs,
		migrateUsageStatistics,
	}

	var allStats []MigrationStats
	startTime := time.Now()

	for _, migrator := range migrators {
		stats := migrator(sqliteDB, mysqlDB, config.BatchSize)
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

func migrateUsers(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "users", batchSize,
		"SELECT id, username, password, display_name, role, status, email, github_id, oidc_id, wechat_id, telegram_id, access_token, quota, used_quota, request_count, `group`, aff_code, aff_count, aff_quota, aff_history, inviter_id, deleted_at, linux_do_id, setting, remark, stripe_customer FROM users",
		"INSERT INTO users (id, username, password, display_name, role, status, email, github_id, oidc_id, wechat_id, telegram_id, access_token, quota, used_quota, request_count, `group`, aff_code, aff_count, aff_quota, aff_history, inviter_id, deleted_at, linux_do_id, setting, remark, stripe_customer) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, role, status, quota, usedQuota, requestCount, affCount, affQuota, affHistory, inviterId sql.NullInt64
			var username, password, displayName, email, githubId, oidcId, wechatId, telegramId, accessToken, groupName, affCode, linuxDoId, setting, remark, stripeCustomer sql.NullString
			var deletedAt sql.NullTime

			err := rows.Scan(&id, &username, &password, &displayName, &role, &status, &email, &githubId, &oidcId, &wechatId, &telegramId, &accessToken, &quota, &usedQuota, &requestCount, &groupName, &affCode, &affCount, &affQuota, &affHistory, &inviterId, &deletedAt, &linuxDoId, &setting, &remark, &stripeCustomer)
			if err != nil {
				return nil, err
			}

			// 处理零值时间
			var deletedAtPtr interface{}
			if deletedAt.Valid && !deletedAt.Time.IsZero() && deletedAt.Time.Year() > 1900 {
				deletedAtPtr = deletedAt.Time
			} else {
				deletedAtPtr = nil
			}

			return []interface{}{
				id.Int64, username.String, password.String, displayName.String, role.Int64, status.Int64,
				email.String, githubId.String, oidcId.String, wechatId.String, telegramId.String,
				accessToken.String, quota.Int64, usedQuota.Int64, requestCount.Int64, groupName.String,
				affCode.String, affCount.Int64, affQuota.Int64, affHistory.Int64, inviterId.Int64,
				deletedAtPtr, linuxDoId.String, setting.String, remark.String, stripeCustomer.String,
			}, nil
		})
}

func migrateChannels(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "channels", batchSize,
		`SELECT id, type, key, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models,
		"group", used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, channel_info FROM channels`,
		"INSERT INTO channels (id, type, `key`, open_ai_organization, test_model, status, name, weight, created_time, test_time, response_time, base_url, other, balance, balance_updated_time, models, `group`, used_quota, model_mapping, status_code_mapping, priority, auto_ban, other_info, tag, setting, param_override, channel_info) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, channelType, status, weight, createdTime, testTime, responseTime, usedQuota, priority, autoBan sql.NullInt64
			var key, openAiOrg, testModel, name, baseUrl, other, models, groupName, modelMapping, statusCodeMapping, otherInfo, tag, setting, paramOverride sql.NullString
			var balance sql.NullFloat64
			var balanceUpdatedTime sql.NullInt64
			var channelInfo sql.NullString

			err := rows.Scan(&id, &channelType, &key, &openAiOrg, &testModel, &status, &name, &weight, &createdTime, &testTime, &responseTime, &baseUrl, &other, &balance, &balanceUpdatedTime, &models, &groupName, &usedQuota, &modelMapping, &statusCodeMapping, &priority, &autoBan, &otherInfo, &tag, &setting, &paramOverride, &channelInfo)
			if err != nil {
				return nil, err
			}

			// 处理 JSON 字段
			var channelInfoJSON interface{}
			if channelInfo.Valid && channelInfo.String != "" {
				channelInfoJSON = channelInfo.String
			} else {
				channelInfoJSON = nil
			}

			return []interface{}{
				id.Int64, channelType.Int64, key.String, openAiOrg.String, testModel.String, status.Int64, name.String, weight.Int64,
				createdTime.Int64, testTime.Int64, responseTime.Int64, baseUrl.String, other.String, balance.Float64,
				balanceUpdatedTime.Int64, models.String, groupName.String, usedQuota.Int64, modelMapping.String,
				statusCodeMapping.String, priority.Int64, autoBan.Int64, otherInfo.String, tag.String, setting.String,
				paramOverride.String, channelInfoJSON,
			}, nil
		})
}

func migrateTokens(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "tokens", batchSize,
		`SELECT id, user_id, key, status, name, created_time, accessed_time, expired_time, remain_quota, unlimited_quota,
		model_limits_enabled, model_limits, allow_ips, used_quota, "group", daily_usage_count, total_usage_count, last_usage_date,
		rate_limit_per_minute, rate_limit_per_day, last_rate_limit_reset, channel_tag, total_usage_limit, deleted_at FROM tokens`,
		"INSERT INTO tokens (id, user_id, `key`, status, name, created_time, accessed_time, expired_time, remain_quota, unlimited_quota, model_limits_enabled, model_limits, allow_ips, used_quota, `group`, daily_usage_count, total_usage_count, last_usage_date, rate_limit_per_minute, rate_limit_per_day, last_rate_limit_reset, channel_tag, total_usage_limit, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, userId, status, createdTime, accessedTime, expiredTime, remainQuota, usedQuota, dailyUsageCount, totalUsageCount, rateLimitPerMinute, rateLimitPerDay, lastRateLimitReset sql.NullInt64
			var key, name, modelLimits, allowIps, groupName, lastUsageDate, channelTag sql.NullString
			var unlimitedQuota, modelLimitsEnabled sql.NullBool
			var totalUsageLimit sql.NullInt64
			var deletedAt sql.NullTime

			err := rows.Scan(&id, &userId, &key, &status, &name, &createdTime, &accessedTime, &expiredTime, &remainQuota, &unlimitedQuota, &modelLimitsEnabled, &modelLimits, &allowIps, &usedQuota, &groupName, &dailyUsageCount, &totalUsageCount, &lastUsageDate, &rateLimitPerMinute, &rateLimitPerDay, &lastRateLimitReset, &channelTag, &totalUsageLimit, &deletedAt)
			if err != nil {
				return nil, err
			}

			// 处理 total_usage_limit 可能为 NULL 的情况
			var totalUsageLimitPtr interface{}
			if totalUsageLimit.Valid {
				totalUsageLimitPtr = totalUsageLimit.Int64
			} else {
				totalUsageLimitPtr = nil
			}

			// 处理零值时间
			var deletedAtPtr interface{}
			if deletedAt.Valid && !deletedAt.Time.IsZero() && deletedAt.Time.Year() > 1900 {
				deletedAtPtr = deletedAt.Time
			} else {
				deletedAtPtr = nil
			}

			return []interface{}{
				id.Int64, userId.Int64, key.String, status.Int64, name.String, createdTime.Int64, accessedTime.Int64,
				expiredTime.Int64, remainQuota.Int64, unlimitedQuota.Bool, modelLimitsEnabled.Bool, modelLimits.String,
				allowIps.String, usedQuota.Int64, groupName.String, dailyUsageCount.Int64, totalUsageCount.Int64,
				lastUsageDate.String, rateLimitPerMinute.Int64, rateLimitPerDay.Int64, lastRateLimitReset.Int64,
				channelTag.String, totalUsageLimitPtr, deletedAtPtr,
			}, nil
		})
}

func migrateAbilities(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "abilities", batchSize,
		`SELECT "group", model, channel_id, enabled, priority, weight, tag FROM abilities`,
		"INSERT INTO abilities (`group`, model, channel_id, enabled, priority, weight, tag) VALUES (?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var groupName, model, tag sql.NullString
			var channelId, priority, weight sql.NullInt64
			var enabled sql.NullBool

			err := rows.Scan(&groupName, &model, &channelId, &enabled, &priority, &weight, &tag)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				groupName.String, model.String, channelId.Int64, enabled.Bool, priority.Int64, weight.Int64, tag.String,
			}, nil
		})
}

func migrateOptions(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "options", batchSize,
		"SELECT key, value FROM options",
		"INSERT INTO options (`key`, value) VALUES (?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var key, value sql.NullString

			err := rows.Scan(&key, &value)
			if err != nil {
				return nil, err
			}

			return []interface{}{key.String, value.String}, nil
		})
}

func migrateLogs(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "logs", batchSize,
		`SELECT id, user_id, created_at, type, content, user_input, username, token_name, model_name, quota, prompt_tokens, completion_tokens, use_time, is_stream, channel_id, channel_name, token_id, "group", ip, other, response_body FROM logs`,
		"INSERT INTO logs (id, user_id, created_at, type, content, user_input, username, token_name, model_name, quota, prompt_tokens, completion_tokens, use_time, is_stream, channel_id, channel_name, token_id, `group`, ip, other, response_body) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, userId, createdAt, logType, quota, promptTokens, completionTokens, useTime, channelId, tokenId sql.NullInt64
			var content, userInput, username, tokenName, modelName, channelName, groupName, ip, other, responseBody sql.NullString
			var isStream sql.NullBool

			err := rows.Scan(&id, &userId, &createdAt, &logType, &content, &userInput, &username, &tokenName, &modelName, &quota, &promptTokens, &completionTokens, &useTime, &isStream, &channelId, &channelName, &tokenId, &groupName, &ip, &other, &responseBody)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				id.Int64, userId.Int64, createdAt.Int64, logType.Int64, content.String, userInput.String,
				username.String, tokenName.String, modelName.String, quota.Int64, promptTokens.Int64,
				completionTokens.Int64, useTime.Int64, isStream.Bool, channelId.Int64, channelName.String,
				tokenId.Int64, groupName.String, ip.String, other.String, responseBody.String,
			}, nil
		})
}

func migrateRedemptions(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "redemptions", batchSize,
		"SELECT id, user_id, key, status, name, quota, created_time, redeemed_time, used_user_id, deleted_at, expired_time FROM redemptions",
		"INSERT INTO redemptions (id, user_id, `key`, status, name, quota, created_time, redeemed_time, used_user_id, deleted_at, expired_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, userId, status, quota, createdTime, redeemedTime, usedUserId, expiredTime sql.NullInt64
			var key, name sql.NullString
			var deletedAt sql.NullTime

			err := rows.Scan(&id, &userId, &key, &status, &name, &quota, &createdTime, &redeemedTime, &usedUserId, &deletedAt, &expiredTime)
			if err != nil {
				return nil, err
			}

			// 处理零值时间
			var deletedAtPtr interface{}
			if deletedAt.Valid && !deletedAt.Time.IsZero() && deletedAt.Time.Year() > 1900 {
				deletedAtPtr = deletedAt.Time
			} else {
				deletedAtPtr = nil
			}

			return []interface{}{
				id.Int64, userId.Int64, key.String, status.Int64, name.String, quota.Int64, createdTime.Int64,
				redeemedTime.Int64, usedUserId.Int64, deletedAtPtr, expiredTime.Int64,
			}, nil
		})
}

func migrateTopUps(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "top_ups", batchSize,
		"SELECT id, user_id, amount, money, trade_no, create_time, complete_time, status FROM top_ups",
		"INSERT INTO top_ups (id, user_id, amount, money, trade_no, create_time, complete_time, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, userId, amount, createTime, completeTime sql.NullInt64
			var money sql.NullFloat64
			var tradeNo, status sql.NullString

			err := rows.Scan(&id, &userId, &amount, &money, &tradeNo, &createTime, &completeTime, &status)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				id.Int64, userId.Int64, amount.Int64, money.Float64, tradeNo.String, createTime.Int64,
				completeTime.Int64, status.String,
			}, nil
		})
}

func migrateMidjourneys(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "midjourneys", batchSize,
		"SELECT id, code, user_id, action, mj_id, prompt, prompt_en, description, state, submit_time, start_time, finish_time, image_url, video_url, video_urls, status, progress, fail_reason, channel_id, quota, buttons, properties FROM midjourneys",
		"INSERT INTO midjourneys (id, code, user_id, action, mj_id, prompt, prompt_en, description, state, submit_time, start_time, finish_time, image_url, video_url, video_urls, status, progress, fail_reason, channel_id, quota, buttons, properties) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, code, userId, submitTime, startTime, finishTime, channelId, quota sql.NullInt64
			var action, mjId, prompt, promptEn, description, state, imageUrl, videoUrl, videoUrls, status, progress, failReason, buttons, properties sql.NullString

			err := rows.Scan(&id, &code, &userId, &action, &mjId, &prompt, &promptEn, &description, &state, &submitTime, &startTime, &finishTime, &imageUrl, &videoUrl, &videoUrls, &status, &progress, &failReason, &channelId, &quota, &buttons, &properties)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				id.Int64, code.Int64, userId.Int64, action.String, mjId.String, prompt.String, promptEn.String,
				description.String, state.String, submitTime.Int64, startTime.Int64, finishTime.Int64,
				imageUrl.String, videoUrl.String, videoUrls.String, status.String, progress.String,
				failReason.String, channelId.Int64, quota.Int64, buttons.String, properties.String,
			}, nil
		})
}

func migrateTasks(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "tasks", batchSize,
		"SELECT id, created_at, updated_at, task_id, platform, user_id, channel_id, quota, action, status, fail_reason, submit_time, start_time, finish_time, progress, properties, data FROM tasks",
		"INSERT INTO tasks (id, created_at, updated_at, task_id, platform, user_id, channel_id, quota, action, status, fail_reason, submit_time, start_time, finish_time, progress, properties, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, createdAt, updatedAt, userId, channelId, quota, submitTime, startTime, finishTime sql.NullInt64
			var taskId, platform, action, status, failReason, progress sql.NullString
			var properties, data sql.NullString

			err := rows.Scan(&id, &createdAt, &updatedAt, &taskId, &platform, &userId, &channelId, &quota, &action, &status, &failReason, &submitTime, &startTime, &finishTime, &progress, &properties, &data)
			if err != nil {
				return nil, err
			}

			// 处理 JSON 字段
			var propertiesJSON, dataJSON interface{}
			if properties.Valid && properties.String != "" {
				propertiesJSON = properties.String
			}
			if data.Valid && data.String != "" {
				dataJSON = data.String
			}

			return []interface{}{
				id.Int64, createdAt.Int64, updatedAt.Int64, taskId.String, platform.String, userId.Int64,
				channelId.Int64, quota.Int64, action.String, status.String, failReason.String,
				submitTime.Int64, startTime.Int64, finishTime.Int64, progress.String, propertiesJSON, dataJSON,
			}, nil
		})
}

func migrateSetups(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "setups", batchSize,
		"SELECT id, version, initialized_at FROM setups",
		"INSERT INTO setups (id, version, initialized_at) VALUES (?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, initializedAt sql.NullInt64
			var version sql.NullString

			err := rows.Scan(&id, &version, &initializedAt)
			if err != nil {
				return nil, err
			}

			return []interface{}{id.Int64, version.String, initializedAt.Int64}, nil
		})
}

func migrateQuotaData(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "quota_data", batchSize,
		"SELECT id, user_id, username, model_name, created_at, token_used, count, quota FROM quota_data",
		"INSERT INTO quota_data (id, user_id, username, model_name, created_at, token_used, count, quota) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, userId, createdAt, tokenUsed, count, quota sql.NullInt64
			var username, modelName sql.NullString

			err := rows.Scan(&id, &userId, &username, &modelName, &createdAt, &tokenUsed, &count, &quota)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				id.Int64, userId.Int64, username.String, modelName.String, createdAt.Int64, tokenUsed.Int64, count.Int64, quota.Int64,
			}, nil
		})
}

func migrateTokenUsageLogs(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "token_usage_logs", batchSize,
		"SELECT id, token_id, created_at FROM token_usage_logs",
		"INSERT INTO token_usage_logs (id, token_id, created_at) VALUES (?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, tokenId, createdAt sql.NullInt64

			err := rows.Scan(&id, &tokenId, &createdAt)
			if err != nil {
				return nil, err
			}

			return []interface{}{id.Int64, tokenId.Int64, createdAt.Int64}, nil
		})
}

func migrateUsageStatistics(sqliteDB, mysqlDB *sql.DB, batchSize int) MigrationStats {
	return migrateTable(sqliteDB, mysqlDB, "usage_statistics", batchSize,
		"SELECT id, date, token_id, token_name, model_name, total_requests, successful_requests, failed_requests, total_tokens, prompt_tokens, completion_tokens, total_quota, created_time, updated_time FROM usage_statistics",
		"INSERT INTO usage_statistics (id, date, token_id, token_name, model_name, total_requests, successful_requests, failed_requests, total_tokens, prompt_tokens, completion_tokens, total_quota, created_time, updated_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		func(rows *sql.Rows) ([]interface{}, error) {
			var id, tokenId, totalRequests, successfulRequests, failedRequests, totalTokens, promptTokens, completionTokens, totalQuota, createdTime, updatedTime sql.NullInt64
			var date, tokenName, modelName sql.NullString

			err := rows.Scan(&id, &date, &tokenId, &tokenName, &modelName, &totalRequests, &successfulRequests, &failedRequests, &totalTokens, &promptTokens, &completionTokens, &totalQuota, &createdTime, &updatedTime)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				id.Int64, date.String, tokenId.Int64, tokenName.String, modelName.String, totalRequests.Int64,
				successfulRequests.Int64, failedRequests.Int64, totalTokens.Int64, promptTokens.Int64,
				completionTokens.Int64, totalQuota.Int64, createdTime.Int64, updatedTime.Int64,
			}, nil
		})
}

// migrateTable 通用迁移函数
func migrateTable(sqliteDB, mysqlDB *sql.DB, tableName string, batchSize int, selectSQL, insertSQL string, scanFunc func(*sql.Rows) ([]interface{}, error)) MigrationStats {
	stats := MigrationStats{Table: tableName}
	startTime := time.Now()

	fmt.Printf("\n[%s] Starting migration...\n", tableName)

	// 先清空目标表数据（避免重复插入）
	_, err := mysqlDB.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		fmt.Printf("[%s] Warning: Failed to clear table: %v\n", tableName, err)
	}

	// 如果是 abilities 表，需要特殊处理（没有 id 主键）
	if tableName == "abilities" {
		_, err = mysqlDB.Exec("DELETE FROM abilities")
		if err != nil {
			fmt.Printf("[%s] Warning: Failed to clear abilities table: %v\n", tableName, err)
		}
	}

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

	// 批量插入
	count := 0
	batch := make([][]interface{}, 0, batchSize)

	for rows.Next() {
		values, err := scanFunc(rows)
		if err != nil {
			fmt.Printf("[%s] Error scanning row: %v\n", tableName, err)
			stats.FailedRows++
			continue
		}

		batch = append(batch, values)
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
	fmt.Printf("%s %s: %d rows (%d success, %d failed) in %v\n",
		status, stats.Table, stats.TotalRows, stats.SuccessRows, stats.FailedRows, stats.Duration)
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
