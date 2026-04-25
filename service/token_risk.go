package service

import (
	"fmt"
	"one-api/common"
	"one-api/model"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
)

func analyzeTokenRisk() {
	sinceTime := time.Now().Add(-24 * time.Hour).Unix()

	var results []struct {
		TokenId int
		IpCount int64
	}

	model.DB.Model(&model.TokenIpLog{}).
		Select("token_id, COUNT(DISTINCT ip) as ip_count").
		Where("created_time > ?", sinceTime).
		Group("token_id").
		Scan(&results)

	for _, result := range results {
		var riskLevel int
		var reason string

		switch {
		case result.IpCount <= 2:
			riskLevel = 0
			reason = ""
		case result.IpCount <= 5:
			riskLevel = 1
			reason = fmt.Sprintf("24h内出现 %d 个不同IP", result.IpCount)
		case result.IpCount <= 10:
			riskLevel = 2
			reason = fmt.Sprintf("24h内出现 %d 个不同IP", result.IpCount)
		default:
			riskLevel = 3
			reason = fmt.Sprintf("24h内出现 %d 个不同IP，存在滥用可能", result.IpCount)
		}

		err := model.DB.Model(&model.Token{}).Where("id = ?", result.TokenId).Updates(map[string]interface{}{
			"risk_level":  riskLevel,
			"risk_reason": reason,
		}).Error
		if err != nil {
			common.SysError(fmt.Sprintf("failed to update token risk level for token %d: %v", result.TokenId, err))
		}
	}
}

func cleanExpiredIpLogs() {
	beforeTime := time.Now().Add(-7 * 24 * time.Hour).Unix()

	for {
		result := model.DB.Where("created_time < ?", beforeTime).Limit(1000).Delete(&model.TokenIpLog{})
		if result.Error != nil {
			common.SysError("failed to clean expired ip logs: " + result.Error.Error())
			break
		}
		if result.RowsAffected == 0 {
			break
		}
	}
}

func StartTokenRiskAnalyzer() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			gopool.Go(func() {
				analyzeTokenRisk()
				cleanExpiredIpLogs()
			})
		}
	}()
}
