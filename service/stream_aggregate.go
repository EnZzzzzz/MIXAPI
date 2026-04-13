package service

import (
	"encoding/json"
	"one-api/dto"
	"strings"
)

// AggregateStreamChunks 将流式响应的多个 chunks 聚合成一个标准 OpenAI 响应格式
func AggregateStreamChunks(responseBody string, usage *dto.Usage, model string) string {
	// 按行分割，每行是一个 chunk
	lines := strings.Split(responseBody, "\n")
	var streamItems []string
	var lastStreamData string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 解析 SSE 格式: data: {...}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}
			streamItems = append(streamItems, data)
			lastStreamData = data
		} else {
			// 非 SSE 格式，直接作为 JSON 处理
			streamItems = append(streamItems, line)
			lastStreamData = line
		}
	}

	if len(streamItems) == 0 {
		return responseBody
	}

	// 解析最后一个 chunk 获取基本信息
	var lastResponse dto.ChatCompletionsStreamResponse
	if err := json.Unmarshal([]byte(lastStreamData), &lastResponse); err != nil {
		// 如果解析失败，返回原始数据
		return "[" + strings.Join(streamItems, ",") + "]"
	}

	// 构建聚合的响应
	aggregated := dto.OpenAITextResponse{
		Id:      lastResponse.Id,
		Object:  "chat.completion",
		Created: lastResponse.Created,
		Model:   model,
		Usage:   *usage,
	}

	// 收集每个 choice 的内容
	type choiceContent struct {
		content          string
		reasoningContent string
		reasoning        string
		toolCalls        []dto.ToolCallResponse
		finishReason     string
	}
	choiceMap := make(map[int]*choiceContent)

	for _, item := range streamItems {
		var streamResp dto.ChatCompletionsStreamResponse
		if err := json.Unmarshal([]byte(item), &streamResp); err != nil {
			continue
		}

		for _, choice := range streamResp.Choices {
			if _, exists := choiceMap[choice.Index]; !exists {
				choiceMap[choice.Index] = &choiceContent{}
			}

			// 累加 content
			if choice.Delta.Content != nil {
				choiceMap[choice.Index].content += *choice.Delta.Content
			}
			// 累加 reasoning_content
			if choice.Delta.ReasoningContent != nil {
				choiceMap[choice.Index].reasoningContent += *choice.Delta.ReasoningContent
			}
			if choice.Delta.Reasoning != nil {
				choiceMap[choice.Index].reasoning += *choice.Delta.Reasoning
			}
			// 处理 tool_calls
			if len(choice.Delta.ToolCalls) > 0 {
				choiceMap[choice.Index].toolCalls = append(
					choiceMap[choice.Index].toolCalls,
					choice.Delta.ToolCalls...,
				)
			}
			// 记录 finish_reason
			if choice.FinishReason != nil {
				choiceMap[choice.Index].finishReason = *choice.FinishReason
			}
		}
	}

	// 将 choiceMap 转换为有序的 Choices 数组
	for i := 0; i < len(choiceMap); i++ {
		if cc, exists := choiceMap[i]; exists {
			choice := dto.OpenAITextResponseChoice{
				Index: i,
				Message: dto.Message{
					Role:             "assistant",
					Content:          cc.content,
					ReasoningContent: cc.reasoningContent,
					Reasoning:        cc.reasoning,
				},
				FinishReason: cc.finishReason,
			}
			// 处理 tool_calls
			if len(cc.toolCalls) > 0 {
				toolCallsJSON, _ := json.Marshal(cc.toolCalls)
				choice.Message.ToolCalls = toolCallsJSON
			}
			aggregated.Choices = append(aggregated.Choices, choice)
		}
	}

	result, err := json.Marshal(aggregated)
	if err != nil {
		return "[" + strings.Join(streamItems, ",") + "]"
	}

	return string(result)
}
