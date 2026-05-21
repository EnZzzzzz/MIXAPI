package common

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteLogFiles writes user input and response body to disk files.
// Returns the relative log directory (e.g. "20250521/123") and error.
func WriteLogFiles(root string, logId int, userInput string, responseBody string) (string, error) {
	dateStr := time.Now().Format("20060102")
	relDir := filepath.Join(dateStr, fmt.Sprintf("%d", logId))
	absDir := filepath.Join(root, relDir)

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return "", err
	}

	if userInput != "" {
		inputPath := filepath.Join(absDir, "input.json")
		if err := os.WriteFile(inputPath, []byte(userInput), 0644); err != nil {
			return "", err
		}
	}

	if responseBody != "" {
		respPath := filepath.Join(absDir, "response.json")
		if err := os.WriteFile(respPath, []byte(responseBody), 0644); err != nil {
			return "", err
		}
	}

	return relDir, nil
}

// ReadLogInput reads user input from the log file.
func ReadLogInput(root string, logDir string) string {
	if logDir == "" || root == "" {
		return ""
	}
	path := filepath.Join(root, logDir, "input.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadLogResponse reads model response from the log file.
func ReadLogResponse(root string, logDir string) string {
	if logDir == "" || root == "" {
		return ""
	}
	path := filepath.Join(root, logDir, "response.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// InitLogFilePath initializes the log file storage path from environment.
func InitLogFilePath() string {
	path := os.Getenv("LOG_FILE_PATH")
	if path == "" {
		path = "./logs"
	}
	LogFilePath = path
	if err := os.MkdirAll(path, 0755); err != nil {
		SysError("failed to create log file path: " + err.Error())
	}
	return path
}
