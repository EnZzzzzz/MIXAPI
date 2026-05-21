package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"one-api/common"
	"one-api/model"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	ExportDir           = "./exports/"
	ExportBatchSize     = 5000
	ExportCleanerPeriod = 1 * time.Hour
	ExportMaxAge        = 24 * time.Hour
)

func CreateExportTask(userId int, startTime, endTime int64, username, modelName, format string) (*model.LogExportTask, error) {
	if format != "csv" && format != "json" {
		format = "json"
	}

	task := &model.LogExportTask{
		UserId:    userId,
		Status:    "pending",
		StartTime: startTime,
		EndTime:   endTime,
		Username:  username,
		ModelName: modelName,
		Format:    format,
	}

	if err := task.Insert(); err != nil {
		return nil, err
	}

	gopool.Go(func() {
		processExportTask(task.ID)
	})

	return task, nil
}

func processExportTask(taskID int64) {
	task, err := model.GetLogExportTaskByID(taskID)
	if err != nil {
		common.SysError(fmt.Sprintf("log export task %d not found: %v", taskID, err))
		return
	}

	task.Status = "processing"
	if err := task.Update(); err != nil {
		common.SysError(fmt.Sprintf("log export task %d update processing failed: %v", taskID, err))
		return
	}

	if err := os.MkdirAll(ExportDir, 0755); err != nil {
		markTaskFailed(task, fmt.Sprintf("create export dir failed: %v", err))
		return
	}

	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("log_export_%d_%d.%s", task.ID, timestamp, task.Format)
	filePath := filepath.Join(ExportDir, filename)
	filePath = filepath.Clean(filePath)

	file, err := os.Create(filePath)
	if err != nil {
		markTaskFailed(task, fmt.Sprintf("create export file failed: %v", err))
		return
	}
	defer file.Close()

	if task.Format == "csv" {
		err = writeCSVExport(file, task)
	} else {
		err = writeJSONExport(file, task)
	}

	if err != nil {
		markTaskFailed(task, err.Error())
		_ = os.Remove(filePath)
		return
	}

	info, err := file.Stat()
	if err != nil {
		markTaskFailed(task, fmt.Sprintf("stat export file failed: %v", err))
		_ = os.Remove(filePath)
		return
	}

	task.Status = "success"
	task.FilePath = filePath
	task.FileSize = info.Size()
	if err := task.Update(); err != nil {
		common.SysError(fmt.Sprintf("log export task %d update success failed: %v", taskID, err))
	}
}

func markTaskFailed(task *model.LogExportTask, errMsg string) {
	task.Status = "failed"
	task.ErrorMsg = errMsg
	if err := task.Update(); err != nil {
		common.SysError(fmt.Sprintf("log export task %d update failed status failed: %v", task.ID, err))
	}
}

func writeCSVExport(writer io.Writer, task *model.LogExportTask) error {
	csvWriter := csv.NewWriter(writer)
	headers := []string{"id", "user_id", "created_at", "type", "username", "model_name",
		"prompt_tokens", "completion_tokens", "quota", "user_input", "response_body"}
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("write csv header failed: %w", err)
	}

	offset := 0
	for {
		var logs []*model.Log
		tx := buildExportQuery(task)
		tx = tx.Order("id desc").Limit(ExportBatchSize).Offset(offset)
		if err := tx.Find(&logs).Error; err != nil {
			return fmt.Errorf("query logs failed: %w", err)
		}
		if len(logs) == 0 {
			break
		}

		model.FillLogBodiesFromFiles(logs)
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
			if err := csvWriter.Write(record); err != nil {
				return fmt.Errorf("write csv record failed: %w", err)
			}
		}
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return fmt.Errorf("csv writer error: %w", err)
		}

		if len(logs) < ExportBatchSize {
			break
		}
		offset += ExportBatchSize
	}

	return nil
}

func writeJSONExport(writer io.Writer, task *model.LogExportTask) error {
	if _, err := writer.Write([]byte("[")); err != nil {
		return fmt.Errorf("write json start failed: %w", err)
	}

	first := true
	offset := 0
	for {
		var logs []*model.Log
		tx := buildExportQuery(task)
		tx = tx.Order("id desc").Limit(ExportBatchSize).Offset(offset)
		if err := tx.Find(&logs).Error; err != nil {
			return fmt.Errorf("query logs failed: %w", err)
		}
		if len(logs) == 0 {
			break
		}

		model.FillLogBodiesFromFiles(logs)
		for _, log := range logs {
			if !first {
				if _, err := writer.Write([]byte(",\n")); err != nil {
					return fmt.Errorf("write json separator failed: %w", err)
				}
			}
			first = false

			data, err := json.Marshal(log)
			if err != nil {
				return fmt.Errorf("marshal log failed: %w", err)
			}
			if _, err := writer.Write(data); err != nil {
				return fmt.Errorf("write json record failed: %w", err)
			}
		}

		if len(logs) < ExportBatchSize {
			break
		}
		offset += ExportBatchSize
	}

	if _, err := writer.Write([]byte("]")); err != nil {
		return fmt.Errorf("write json end failed: %w", err)
	}
	return nil
}

func buildExportQuery(task *model.LogExportTask) *gorm.DB {
	tx := model.LOG_DB.Model(&model.Log{})
	if task.StartTime > 0 {
		tx = tx.Where("created_at >= ?", task.StartTime)
	}
	if task.EndTime > 0 {
		tx = tx.Where("created_at <= ?", task.EndTime)
	}
	if task.Username != "" {
		tx = tx.Where("username = ?", task.Username)
	}
	if task.ModelName != "" {
		tx = tx.Where("model_name like ?", task.ModelName)
	}
	return tx
}

func StartLogExportCleaner() {
	ticker := time.NewTicker(ExportCleanerPeriod)
	defer ticker.Stop()

	for range ticker.C {
		cleanExpiredExportTasks()
	}
}

func cleanExpiredExportTasks() {
	beforeTime := time.Now().Add(-ExportMaxAge).Unix()
	tasks, err := model.GetExpiredLogExportTasks(beforeTime)
	if err != nil {
		common.SysError("get expired log export tasks failed: " + err.Error())
		return
	}

	for _, task := range tasks {
		if task.FilePath != "" {
			cleanPath := filepath.Clean(task.FilePath)
			if cleanPath != "" && cleanPath != "." && cleanPath != "/" {
				_ = os.Remove(cleanPath)
			}
		}
		if err := model.DeleteLogExportTaskByID(task.ID); err != nil {
			common.SysError(fmt.Sprintf("delete expired log export task %d failed: %v", task.ID, err))
		}
	}
}
