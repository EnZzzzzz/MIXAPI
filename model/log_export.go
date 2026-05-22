package model

import (
	"time"
)

type LogExportTask struct {
	ID        int64  `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserId    int    `json:"user_id" gorm:"index"`
	Status    string `json:"status" gorm:"type:varchar(20);index"` // pending / processing / success / failed
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Username  string `json:"username" gorm:"type:varchar(100)"`
	ModelName string `json:"model_name" gorm:"type:varchar(100)"`
	Format    string `json:"format" gorm:"type:varchar(10)"` // json / csv
	FilePath        string `json:"file_path" gorm:"type:varchar(255)"`
	FileSize        int64  `json:"file_size"`
	ErrorMsg        string `json:"error_msg" gorm:"type:text"`
	BodyExportMode  string `json:"body_export_mode" gorm:"type:varchar(20);default:'full'"` // full / truncated / none
	BodyExportLength int   `json:"body_export_length" gorm:"default:500"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (LogExportTask) TableName() string {
	return "log_export_tasks"
}

func (task *LogExportTask) Insert() error {
	task.CreatedAt = time.Now().Unix()
	task.UpdatedAt = task.CreatedAt
	return DB.Create(task).Error
}

func (task *LogExportTask) Update() error {
	task.UpdatedAt = time.Now().Unix()
	return DB.Save(task).Error
}

func GetLogExportTaskByID(id int64) (*LogExportTask, error) {
	var task LogExportTask
	err := DB.First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func GetLogExportTasksByUserId(userId int, startIdx, pageSize int) ([]*LogExportTask, error) {
	var tasks []*LogExportTask
	err := DB.Where("user_id = ?", userId).Order("id desc").Limit(pageSize).Offset(startIdx).Find(&tasks).Error
	return tasks, err
}

func CountLogExportTasksByUserId(userId int) (int64, error) {
	var count int64
	err := DB.Model(&LogExportTask{}).Where("user_id = ?", userId).Count(&count).Error
	return count, err
}

func DeleteLogExportTaskByID(id int64) error {
	return DB.Delete(&LogExportTask{}, id).Error
}

func GetExpiredLogExportTasks(beforeTime int64) ([]*LogExportTask, error) {
	var tasks []*LogExportTask
	err := DB.Where("created_at < ?", beforeTime).Find(&tasks).Error
	return tasks, err
}
