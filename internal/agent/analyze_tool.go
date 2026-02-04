package agent

import (
	"fmt"
	"strings"
	"time"

	"serverowl/internal/analyzer"
)

// CreateAnalyzeLogsTool 创建日志分析工具
func CreateAnalyzeLogsTool() *Tool {
	return &Tool{
		Name:        "analyze_logs",
		Description: "分析日志文件，检测错误模式和异常。支持分析 Nginx 访问日志、系统日志等。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"analyze_file", "analyze_nginx", "analyze_system", "detect_patterns"},
					"description": "操作类型：analyze_file 分析指定文件，analyze_nginx 分析 Nginx 日志，analyze_system 分析系统日志，detect_patterns 检测错误模式",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "日志文件路径（analyze_file 和 analyze_nginx 需要）",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "日志内容（detect_patterns 时可直接传入内容）",
				},
			},
			"required": []string{"action"},
		},
		Handler:     analyzeLogsHandler,
		NeedConfirm: false,
		Timeout:     2 * time.Minute,
		RetryCount:  0,
	}
}

func analyzeLogsHandler(args map[string]any) (string, error) {
	action, ok := args["action"].(string)
	if !ok {
		return "", fmt.Errorf("action 参数必须是字符串")
	}

	logAnalyzer := analyzer.NewLogAnalyzer()

	switch action {
	case "analyze_file":
		return handleAnalyzeFile(logAnalyzer, args)
	case "analyze_nginx":
		return handleAnalyzeNginx(logAnalyzer, args)
	case "analyze_system":
		return handleAnalyzeSystem(logAnalyzer)
	case "detect_patterns":
		return handleDetectPatterns(args)
	default:
		return "", fmt.Errorf("未知操作: %s", action)
	}
}

func handleAnalyzeFile(la *analyzer.LogAnalyzer, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path 参数是必需的")
	}

	result, err := la.AnalyzeFile(path)
	if err != nil {
		return "", fmt.Errorf("分析文件失败: %w", err)
	}

	return formatAnalysisResult(result), nil
}

func handleAnalyzeNginx(la *analyzer.LogAnalyzer, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path 参数是必需的")
	}

	result, err := la.AnalyzeNginxLogs(path)
	if err != nil {
		return "", fmt.Errorf("分析 Nginx 日志失败: %w", err)
	}

	return formatNginxAnalysisResult(result), nil
}

func handleAnalyzeSystem(la *analyzer.LogAnalyzer) (string, error) {
	result, err := la.AnalyzeSystemLogs()
	if err != nil {
		return "", fmt.Errorf("分析系统日志失败: %w", err)
	}

	return formatAnalysisResult(result), nil
}

func handleDetectPatterns(args map[string]any) (string, error) {
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return "", fmt.Errorf("content 参数是必需的")
	}

	matcher := analyzer.NewPatternMatcher()
	lines := strings.Split(content, "\n")

	// 将字符串转换为 LogEntry
	var entries []analyzer.LogEntry
	for i, line := range lines {
		if line == "" {
			continue
		}
		entries = append(entries, analyzer.LogEntry{
			Raw:        line,
			LineNumber: i + 1,
			Message:    line,
		})
	}

	matches := matcher.MatchLines(entries)

	if len(matches) == 0 {
		return "未检测到已知错误模式。", nil
	}

	return formatPatternMatches(matches), nil
}

func formatAnalysisResult(result *analyzer.AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 日志分析报告: %s\n\n", result.FilePath))
	sb.WriteString(fmt.Sprintf("**分析时间**: %s\n", result.EndTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**总行数**: %d\n", result.TotalLines))
	sb.WriteString(fmt.Sprintf("**错误数**: %d\n", result.ErrorLines))
	sb.WriteString(fmt.Sprintf("**警告数**: %d\n\n", result.WarnLines))

	// 错误模式匹配
	if len(result.PatternMatches) > 0 {
		sb.WriteString("### 检测到的错误模式\n\n")
		for _, match := range result.PatternMatches {
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", match.Pattern.Name, match.Pattern.Severity, match.Pattern.Suggestion))
			sb.WriteString(fmt.Sprintf("  - 匹配行: `%s`\n", truncateString(match.Entry.Raw, 100)))
		}
		sb.WriteString("\n")
	}

	// 异常检测
	if len(result.Anomalies) > 0 {
		sb.WriteString("### 检测到的异常\n\n")
		for _, anomaly := range result.Anomalies {
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", anomaly.Type, anomaly.Severity, anomaly.Description))
		}
		sb.WriteString("\n")
	}

	// 时间分布
	if len(result.HourlyErrors) > 0 {
		sb.WriteString("### 错误时间分布\n\n")
		for hour, count := range result.HourlyErrors {
			if count > 0 {
				sb.WriteString(fmt.Sprintf("- %02d:00 - %02d:59: %d 条\n", hour, hour, count))
			}
		}
	}

	return sb.String()
}

func formatNginxAnalysisResult(result *analyzer.AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Nginx 日志分析报告: %s\n\n", result.FilePath))
	sb.WriteString(fmt.Sprintf("**分析时间**: %s\n", result.EndTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**总请求数**: %d\n\n", result.TotalLines))

	// HTTP 状态码分布 (使用 LevelCounts，存储的是 2xx, 3xx, 4xx, 5xx 等)
	if len(result.LevelCounts) > 0 {
		sb.WriteString("### HTTP 状态码分布\n\n")
		for status, count := range result.LevelCounts {
			percentage := float64(count) / float64(result.TotalLines) * 100
			sb.WriteString(fmt.Sprintf("- %s: %d (%.1f%%)\n", status, count, percentage))
		}
		sb.WriteString("\n")
	}

	// 错误统计
	if result.ErrorLines > 0 || result.WarnLines > 0 {
		sb.WriteString("### 错误统计\n\n")
		sb.WriteString(fmt.Sprintf("- 5xx 错误: %d\n", result.ErrorLines))
		sb.WriteString(fmt.Sprintf("- 4xx 错误: %d\n", result.WarnLines))
		sb.WriteString("\n")
	}

	// 异常检测
	if len(result.Anomalies) > 0 {
		sb.WriteString("### 检测到的异常\n\n")
		for _, anomaly := range result.Anomalies {
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", anomaly.Type, anomaly.Severity, anomaly.Description))
		}
		sb.WriteString("\n")
	}

	// 模式匹配
	if len(result.PatternMatches) > 0 {
		sb.WriteString("### 检测到的错误模式\n\n")
		for _, match := range result.PatternMatches {
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", match.Pattern.Name, match.Pattern.Severity, match.Pattern.Suggestion))
		}
	}

	return sb.String()
}

func formatPatternMatches(matches []analyzer.PatternMatch) string {
	var sb strings.Builder

	sb.WriteString("## 错误模式检测结果\n\n")
	sb.WriteString(fmt.Sprintf("共检测到 %d 个匹配\n\n", len(matches)))

	// 按严重程度分组
	critical := []analyzer.PatternMatch{}
	high := []analyzer.PatternMatch{}
	medium := []analyzer.PatternMatch{}
	low := []analyzer.PatternMatch{}

	for _, m := range matches {
		switch m.Pattern.Severity {
		case "critical":
			critical = append(critical, m)
		case "high":
			high = append(high, m)
		case "medium":
			medium = append(medium, m)
		default:
			low = append(low, m)
		}
	}

	if len(critical) > 0 {
		sb.WriteString("### 严重 (Critical)\n\n")
		for _, m := range critical {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", m.Pattern.Name, m.Pattern.Suggestion))
			sb.WriteString(fmt.Sprintf("  - 行 %d: `%s`\n", m.Entry.LineNumber, truncateString(m.Entry.Raw, 80)))
		}
		sb.WriteString("\n")
	}

	if len(high) > 0 {
		sb.WriteString("### 高 (High)\n\n")
		for _, m := range high {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", m.Pattern.Name, m.Pattern.Suggestion))
			sb.WriteString(fmt.Sprintf("  - 行 %d: `%s`\n", m.Entry.LineNumber, truncateString(m.Entry.Raw, 80)))
		}
		sb.WriteString("\n")
	}

	if len(medium) > 0 {
		sb.WriteString("### 中 (Medium)\n\n")
		for _, m := range medium {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", m.Pattern.Name, m.Pattern.Suggestion))
		}
		sb.WriteString("\n")
	}

	if len(low) > 0 {
		sb.WriteString("### 低 (Low)\n\n")
		for _, m := range low {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", m.Pattern.Name, m.Pattern.Suggestion))
		}
	}

	return sb.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
