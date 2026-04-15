package controller

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/model"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// statCache 统计接口缓存（30秒）
type statCache struct {
	mu      sync.RWMutex
	data    map[string]statCacheItem
	ttl     time.Duration
}

type statCacheItem struct {
	stat      model.Stat
	expiresAt time.Time
}

var logStatCache = &statCache{
	data: make(map[string]statCacheItem),
	ttl:  30 * time.Second,
}

func (c *statCache) key(parts ...string) string {
	return fmt.Sprintf("%v", parts)
}

func (c *statCache) Get(key string) (model.Stat, bool) {
	c.mu.RLock()
	item, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(item.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.data, key)
			c.mu.Unlock()
		}
		return model.Stat{}, false
	}
	return item.stat, true
}

func (c *statCache) Set(key string, stat model.Stat) {
	c.mu.Lock()
	c.data[key] = statCacheItem{stat: stat, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetAllLogsCursor(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	cursor, _ := strconv.Atoi(c.Query("cursor"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize <= 0 {
		pageSize = common.ItemsPerPage
	}
	if pageSize > 100 {
		pageSize = 100
	}
	logs, nextCursor, err := model.GetAllLogsCursor(logType, startTimestamp, endTimestamp, modelName, username, tokenName, cursor, pageSize, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "",
		"data":        logs,
		"next_cursor": nextCursor,
	})
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogsCursor(c *gin.Context) {
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	cursor, _ := strconv.Atoi(c.Query("cursor"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize <= 0 {
		pageSize = common.ItemsPerPage
	}
	if pageSize > 100 {
		pageSize = 100
	}
	logs, nextCursor, err := model.GetUserLogsCursor(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, cursor, pageSize, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "",
		"data":        logs,
		"next_cursor": nextCursor,
	})
	return
}

func SearchAllLogs(c *gin.Context) {
	keyword := c.Query("keyword")
	logs, err := model.SearchAllLogs(keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
	return
}

func SearchUserLogs(c *gin.Context) {
	keyword := c.Query("keyword")
	userId := c.GetInt("id")
	logs, err := model.SearchUserLogs(userId, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
	return
}

func GetLogByKey(c *gin.Context) {
	key := c.Query("key")
	logs, err := model.GetLogByKey(key)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	cacheKey := logStatCache.key("all", strconv.Itoa(logType), strconv.FormatInt(startTimestamp, 10), strconv.FormatInt(endTimestamp, 10), modelName, username, tokenName, strconv.Itoa(channel), group)
	stat, hit := logStatCache.Get(cacheKey)
	if !hit {
		stat = model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
		logStatCache.Set(cacheKey, stat)
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	cacheKey := logStatCache.key("self", username, strconv.Itoa(logType), strconv.FormatInt(startTimestamp, 10), strconv.FormatInt(endTimestamp, 10), modelName, tokenName, strconv.Itoa(channel), group)
	quotaNum, hit := logStatCache.Get(cacheKey)
	if !hit {
		quotaNum = model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
		logStatCache.Set(cacheKey, quotaNum)
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

func DeleteHistoryLogs(c *gin.Context) {
	// 获取开始和结束时间戳参数
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	// 兼容旧的target_timestamp参数
	if startTimestamp == 0 && endTimestamp == 0 {
		targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
		if targetTimestamp != 0 {
			// 如果只提供了target_timestamp，则将其作为结束时间，开始时间为0（删除该时间之前的所有日志）
			endTimestamp = targetTimestamp
		}
	}

	// 验证参数
	if startTimestamp == 0 && endTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "start timestamp or end timestamp is required",
		})
		return
	}

	// 获取清理模式参数，默认为 "all"
	cleanMode := c.DefaultQuery("clean_mode", "all")

	var count int64
	var err error

	// 根据清理模式执行不同的操作
	switch cleanMode {
	case "body_only":
		// 仅清理 user_input 和 response_body 字段
		count, err = model.CleanLogBodiesOnly(c.Request.Context(), startTimestamp, endTimestamp, 100)
	default:
		// 默认模式：删除整行（现有行为）
		count, err = model.DeleteLogsByTimeRange(c.Request.Context(), startTimestamp, endTimestamp, 100)
	}

	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}

func ExportLogs(c *gin.Context) {
	// 只有管理员可以导出
	userRole := c.GetInt("role")
	if userRole < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "only admin can export logs",
		})
		return
	}

	// 解析参数
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	format := c.DefaultQuery("format", "json")
	username := c.Query("username")
	modelName := c.Query("model_name")

	if startTimestamp == 0 || endTimestamp == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "start_timestamp and end_timestamp are required",
		})
		return
	}

	// 查询数据
	logs, err := model.ExportLogs(startTimestamp, endTimestamp, username, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 生成文件名
	startStr := time.Unix(startTimestamp, 0).Format("20060102")
	endStr := time.Unix(endTimestamp, 0).Format("20060102")
	filename := fmt.Sprintf("logs_%s_%s.%s", startStr, endStr, format)

	// 根据格式导出
	var data []byte
	if format == "csv" {
		data, err = convertLogsToCSV(logs)
	} else {
		data, err = json.MarshalIndent(logs, "", "  ")
	}

	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func convertLogsToCSV(logs []*model.Log) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 写入表头
	headers := []string{"id", "user_id", "created_at", "type", "username", "model_name",
		"prompt_tokens", "completion_tokens", "quota", "user_input", "response_body"}
	writer.Write(headers)

	// 写入数据
	for _, log := range logs {
		record := []string{
			strconv.Itoa(log.Id),
			strconv.Itoa(log.UserId),
			strconv.FormatInt(log.CreatedAt, 10),
			strconv.Itoa(log.Type),
			log.Username,
			log.ModelName,
			strconv.Itoa(log.PromptTokens),
			strconv.Itoa(log.CompletionTokens),
			strconv.Itoa(log.Quota),
			log.UserInput,
			log.ResponseBody,
		}
		writer.Write(record)
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}
