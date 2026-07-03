package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"omniproxy/internal/history"
)

func encodeHistoryCSV(entries []history.Entry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{
		"时间",
		"级别",
		"方法",
		"路径",
		"路由厂商",
		"协议",
		"编程工具",
		"模型",
		"状态码",
		"耗时(ms)",
		"账号",
		"输入Token",
		"输出Token",
		"总Token",
		"触发冷却",
		"错误摘要",
		"重试链路",
	}); err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if err := writer.Write([]string{
			entry.Time.Format(time.RFC3339),
			entry.Level,
			entry.Method,
			entry.Path,
			entry.Provider,
			entry.Protocol,
			entry.ClientName,
			entry.Model,
			fmt.Sprintf("%d", entry.Status),
			fmt.Sprintf("%d", entry.Duration),
			entry.TokenName,
			fmt.Sprintf("%d", entry.InputTokens),
			fmt.Sprintf("%d", entry.OutputTokens),
			fmt.Sprintf("%d", entry.TotalTokens),
			formatBoolCN(entry.CooldownTriggered),
			entry.Message,
			formatRetryChain(entry.RetryChain),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func formatBoolCN(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func formatRetryChain(chain []history.RetryAttempt) string {
	if len(chain) == 0 {
		return ""
	}
	parts := make([]string, 0, len(chain))
	for _, attempt := range chain {
		label := fmt.Sprintf("#%d %s", attempt.Attempt, attempt.Provider)
		if attempt.TokenName != "" {
			label += " " + attempt.TokenName
		}
		if attempt.Status != 0 {
			label += fmt.Sprintf(" %d", attempt.Status)
		}
		if attempt.Duration > 0 {
			label += fmt.Sprintf(" %dms", attempt.Duration)
		}
		if attempt.CooldownTriggered {
			label += " 冷却"
		}
		if attempt.Message != "" {
			label += " " + attempt.Message
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " | ")
}
