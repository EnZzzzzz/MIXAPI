package model

import (
	"one-api/common"

	"github.com/bytedance/gopkg/util/gopool"
)

type TokenIpLog struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	TokenId     int    `json:"token_id" gorm:"index"`
	Ip          string `json:"ip" gorm:"type:varchar(45)"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
}

func (TokenIpLog) TableName() string {
	return "token_ip_logs"
}

func InsertTokenIpLog(tokenId int, ip string) error {
	if tokenId <= 0 || ip == "" {
		return nil
	}
	gopool.Go(func() {
		log := TokenIpLog{
			TokenId:     tokenId,
			Ip:          ip,
			CreatedTime: common.GetTimestamp(),
		}
		if err := DB.Create(&log).Error; err != nil {
			common.SysError("failed to insert token ip log: " + err.Error())
		}
	})
	return nil
}

func GetTokenDistinctIpCount(tokenId int, sinceTime int64) (int64, error) {
	var count int64
	err := DB.Model(&TokenIpLog{}).Where("token_id = ? AND created_time >= ?", tokenId, sinceTime).Distinct("ip").Count(&count).Error
	return count, err
}

func CleanExpiredIpLogs(beforeTime int64) error {
	for {
		result := DB.Where("created_time < ?", beforeTime).Limit(1000).Delete(&TokenIpLog{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			break
		}
	}
	return nil
}

func GetTokenIpLogs(tokenId int, sinceTime int64, limit int) ([]*TokenIpLog, error) {
	var logs []*TokenIpLog
	err := DB.Where("token_id = ? AND created_time >= ?", tokenId, sinceTime).Order("created_time desc").Limit(limit).Find(&logs).Error
	return logs, err
}
