package analyzer

import (
  "bufio"
  "fmt"
  "os"
  "os/exec"
  "regexp"
  "runtime"
  "strconv"
  "strings"
  "time"
)

// LogAnalyzer 日志分析器核心
type LogAnalyzer struct {
  // 模式匹配器
  PatternMatcher *PatternMatcher
  // 异常检测器
  AnomalyDetector *AnomalyDetector
  // 分析结果
  Results []AnalysisResult
  // 配置选项
  Options AnalyzerOptions
}

// AnalyzerOptions 分析器配置选项
type AnalyzerOptions struct {
  // 最大分析行数 (0 表示不限制)
  MaxLines int
  // 时间范围过滤
  TimeRange *TimeRange
  // 是否启用异常检测
  EnableAnomalyDetection bool
  // 是否启用模式匹配
  EnablePatternMatching bool
}

// NewLogAnalyzer 创建新的日志分析器
func NewLogAnalyzer() *LogAnalyzer {
  return &LogAnalyzer{
    PatternMatcher:  NewPatternMatcher(),
    AnomalyDetector: NewAnomalyDetector(),
    Results:         make([]AnalysisResult, 0),
    Options: AnalyzerOptions{
      MaxLines:               0,
      EnableAnomalyDetection: true,
      EnablePatternMatching:  true,
    },
  }
}

// AnalyzeFile 分析单个日志文件
func (la *LogAnalyzer) AnalyzeFile(path string) (*AnalysisResult, error) {
  file, err := os.Open(path)
  if err != nil {
    return nil, fmt.Errorf("无法打开文件 %s: %w", path, err)
  }
  defer file.Close()

  result := &AnalysisResult{
    FilePath:    path,
    StartTime:   time.Now(),
    LevelCounts: make(map[string]int),
    HourlyErrors: make(map[int]int),
  }

  var entries []LogEntry
  scanner := bufio.NewScanner(file)
  lineNum := 0

  for scanner.Scan() {
    lineNum++
    if la.Options.MaxLines > 0 && lineNum > la.Options.MaxLines {
      break
    }

    line := scanner.Text()
    entry := la.parseLine(line, lineNum, path)

    // 时间范围过滤
    if la.Options.TimeRange != nil {
      if entry.Timestamp.Before(la.Options.TimeRange.Start) ||
        entry.Timestamp.After(la.Options.TimeRange.End) {
        continue
      }
    }

    entries = append(entries, entry)
    result.ParsedLines++

    // 统计日志级别
    if entry.Level != "" {
      result.LevelCounts[entry.Level]++
      if entry.Level == "ERROR" || entry.Level == "FATAL" {
        result.ErrorLines++
        result.HourlyErrors[entry.Timestamp.Hour()]++
      } else if entry.Level == "WARN" || entry.Level == "WARNING" {
        result.WarnLines++
      }
    }
  }

  result.TotalLines = lineNum

  if err := scanner.Err(); err != nil {
    return nil, fmt.Errorf("读取文件错误: %w", err)
  }

  // 模式匹配
  if la.Options.EnablePatternMatching {
    result.PatternMatches = la.PatternMatcher.MatchLines(entries)
  }

  // 异常检测
  if la.Options.EnableAnomalyDetection {
    result.Anomalies = la.AnomalyDetector.DetectErrorSpike(entries)
    result.Anomalies = append(result.Anomalies, la.AnomalyDetector.DetectTrafficAnomaly(entries)...)
  }

  result.EndTime = time.Now()
  result.Summary = la.generateSummary(result)

  la.Results = append(la.Results, *result)
  return result, nil
}

// AnalyzeNginxLogs 专门分析 Nginx 访问日志
func (la *LogAnalyzer) AnalyzeNginxLogs(path string) (*AnalysisResult, error) {
  file, err := os.Open(path)
  if err != nil {
    return nil, fmt.Errorf("无法打开文件 %s: %w", path, err)
  }
  defer file.Close()

  result := &AnalysisResult{
    FilePath:     path,
    StartTime:    time.Now(),
    LevelCounts:  make(map[string]int),
    HourlyErrors: make(map[int]int),
  }

  var entries []LogEntry
  var nginxEntries []NginxLogEntry
  scanner := bufio.NewScanner(file)
  lineNum := 0

  // Nginx combined 日志格式正则
  // 格式: $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
  nginxPattern := regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "([^"]*)" (\d+) (\d+) "([^"]*)" "([^"]*)"`)

  // 扩展格式 (包含响应时间)
  nginxExtPattern := regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "([^"]*)" (\d+) (\d+) "([^"]*)" "([^"]*)" (\S+)`)

  for scanner.Scan() {
    lineNum++
    if la.Options.MaxLines > 0 && lineNum > la.Options.MaxLines {
      break
    }

    line := scanner.Text()
    nginxEntry := la.parseNginxLine(line, lineNum, path, nginxPattern, nginxExtPattern)

    if nginxEntry != nil {
      // 时间范围过滤
      if la.Options.TimeRange != nil {
        if nginxEntry.Timestamp.Before(la.Options.TimeRange.Start) ||
          nginxEntry.Timestamp.After(la.Options.TimeRange.End) {
          continue
        }
      }

      nginxEntries = append(nginxEntries, *nginxEntry)
      entries = append(entries, nginxEntry.LogEntry)
      result.ParsedLines++

      // 统计状态码
      statusGroup := fmt.Sprintf("%dxx", nginxEntry.StatusCode/100)
      result.LevelCounts[statusGroup]++

      if nginxEntry.StatusCode >= 500 {
        result.ErrorLines++
        result.HourlyErrors[nginxEntry.Timestamp.Hour()]++
      } else if nginxEntry.StatusCode >= 400 {
        result.WarnLines++
      }
    }
  }

  result.TotalLines = lineNum

  if err := scanner.Err(); err != nil {
    return nil, fmt.Errorf("读取文件错误: %w", err)
  }

  // 模式匹配
  if la.Options.EnablePatternMatching {
    result.PatternMatches = la.PatternMatcher.MatchLines(entries)
  }

  // 异常检测
  if la.Options.EnableAnomalyDetection {
    result.Anomalies = la.AnomalyDetector.DetectSlowRequests(nginxEntries)
    result.Anomalies = append(result.Anomalies, la.AnomalyDetector.DetectStatusCodeSpike(nginxEntries, 500)...)
    result.Anomalies = append(result.Anomalies, la.AnomalyDetector.DetectStatusCodeSpike(nginxEntries, 502)...)
    result.Anomalies = append(result.Anomalies, la.AnomalyDetector.DetectStatusCodeSpike(nginxEntries, 503)...)
    result.Anomalies = append(result.Anomalies, la.AnomalyDetector.DetectTrafficAnomaly(entries)...)
  }

  result.EndTime = time.Now()
  result.Summary = la.generateNginxSummary(result, nginxEntries)

  la.Results = append(la.Results, *result)
  return result, nil
}

// AnalyzeSystemLogs 分析系统日志 (journalctl/dmesg)
func (la *LogAnalyzer) AnalyzeSystemLogs() (*AnalysisResult, error) {
  result := &AnalysisResult{
    FilePath:     "system",
    StartTime:    time.Now(),
    LevelCounts:  make(map[string]int),
    HourlyErrors: make(map[int]int),
  }

  var entries []LogEntry

  // 根据操作系统选择日志来源
  if runtime.GOOS == "linux" {
    // 尝试读取 journalctl
    journalEntries, err := la.readJournalctl()
    if err == nil {
      entries = append(entries, journalEntries...)
    }

    // 尝试读取 dmesg
    dmesgEntries, err := la.readDmesg()
    if err == nil {
      entries = append(entries, dmesgEntries...)
    }
  } else if runtime.GOOS == "windows" {
    // Windows 系统暂不支持
    return nil, fmt.Errorf("Windows 系统日志分析暂不支持")
  }

  // 统计
  for _, entry := range entries {
    result.ParsedLines++
    if entry.Level != "" {
      result.LevelCounts[entry.Level]++
      if entry.Level == "ERROR" || entry.Level == "FATAL" || entry.Level == "err" || entry.Level == "crit" {
        result.ErrorLines++
        result.HourlyErrors[entry.Timestamp.Hour()]++
      } else if entry.Level == "WARN" || entry.Level == "WARNING" || entry.Level == "warning" {
        result.WarnLines++
      }
    }
  }

  result.TotalLines = len(entries)

  // 模式匹配
  if la.Options.EnablePatternMatching {
    result.PatternMatches = la.PatternMatcher.MatchLines(entries)
  }

  // 异常检测
  if la.Options.EnableAnomalyDetection {
    result.Anomalies = la.AnomalyDetector.DetectErrorSpike(entries)
  }

  result.EndTime = time.Now()
  result.Summary = la.generateSummary(result)

  la.Results = append(la.Results, *result)
  return result, nil
}

// GenerateReport 生成分析报告
func (la *LogAnalyzer) GenerateReport() *AnalysisReport {
  report := &AnalysisReport{
    GeneratedAt: time.Now(),
    Results:     la.Results,
  }

  // 计算总体统计
  var overallStats OverallStats
  var minTime, maxTime time.Time

  for _, result := range la.Results {
    overallStats.FilesAnalyzed++
    overallStats.TotalLines += result.TotalLines
    overallStats.TotalErrors += result.ErrorLines
    overallStats.TotalWarnings += result.WarnLines
    overallStats.AnomaliesDetected += len(result.Anomalies)
    overallStats.PatternsMatched += len(result.PatternMatches)

    // 更新时间范围
    if minTime.IsZero() || result.StartTime.Before(minTime) {
      minTime = result.StartTime
    }
    if maxTime.IsZero() || result.EndTime.After(maxTime) {
      maxTime = result.EndTime
    }
  }

  report.OverallStats = overallStats
  report.TimeRange = TimeRange{Start: minTime, End: maxTime}

  // 生成关键发现
  report.KeyFindings = la.generateKeyFindings()

  // 生成建议
  report.Recommendations = la.generateRecommendations()

  return report
}

// parseLine 解析通用日志行
func (la *LogAnalyzer) parseLine(line string, lineNum int, source string) LogEntry {
  entry := LogEntry{
    Raw:        line,
    LineNumber: lineNum,
    Source:     source,
    Fields:     make(map[string]string),
  }

  // 尝试解析常见日志格式

  // 格式1: 2024-01-15 10:30:45 [ERROR] message
  pattern1 := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\s+\[(\w+)\]\s+(.*)`)
  if matches := pattern1.FindStringSubmatch(line); matches != nil {
    if t, err := time.Parse("2006-01-02 15:04:05", matches[1]); err == nil {
      entry.Timestamp = t
    }
    entry.Level = strings.ToUpper(matches[2])
    entry.Message = matches[3]
    return entry
  }

  // 格式2: [2024-01-15T10:30:45Z] ERROR: message
  pattern2 := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z\+\-\d:]*)\]\s+(\w+):\s+(.*)`)
  if matches := pattern2.FindStringSubmatch(line); matches != nil {
    if t, err := time.Parse(time.RFC3339, matches[1]); err == nil {
      entry.Timestamp = t
    }
    entry.Level = strings.ToUpper(matches[2])
    entry.Message = matches[3]
    return entry
  }

  // 格式3: Jan 15 10:30:45 hostname service[pid]: message (syslog)
  pattern3 := regexp.MustCompile(`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[\d+\])?:\s+(.*)`)
  if matches := pattern3.FindStringSubmatch(line); matches != nil {
    // 解析 syslog 时间格式
    currentYear := time.Now().Year()
    timeStr := fmt.Sprintf("%d %s", currentYear, matches[1])
    if t, err := time.Parse("2006 Jan 2 15:04:05", timeStr); err == nil {
      entry.Timestamp = t
    }
    entry.Fields["hostname"] = matches[2]
    entry.Fields["service"] = matches[3]
    entry.Message = matches[4]

    // 从消息中推断级别
    entry.Level = la.inferLogLevel(matches[4])
    return entry
  }

  // 无法解析，保留原始行
  entry.Message = line
  entry.Timestamp = time.Now()
  entry.Level = la.inferLogLevel(line)

  return entry
}

// parseNginxLine 解析 Nginx 日志行
func (la *LogAnalyzer) parseNginxLine(line string, lineNum int, source string, pattern, extPattern *regexp.Regexp) *NginxLogEntry {
  entry := &NginxLogEntry{
    LogEntry: LogEntry{
      Raw:        line,
      LineNumber: lineNum,
      Source:     source,
      Fields:     make(map[string]string),
    },
  }

  // 尝试扩展格式 (包含响应时间)
  if matches := extPattern.FindStringSubmatch(line); matches != nil {
    entry.ClientIP = matches[1]
    entry.Fields["remote_user"] = matches[2]

    // 解析时间
    if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", matches[3]); err == nil {
      entry.Timestamp = t
    }

    // 解析请求
    requestParts := strings.SplitN(matches[4], " ", 3)
    if len(requestParts) >= 2 {
      entry.Method = requestParts[0]
      entry.Path = requestParts[1]
      if len(requestParts) == 3 {
        entry.Protocol = requestParts[2]
      }
    }

    entry.StatusCode, _ = strconv.Atoi(matches[5])
    entry.BodyBytes, _ = strconv.Atoi(matches[6])
    entry.Referer = matches[7]
    entry.UserAgent = matches[8]
    entry.ResponseTime, _ = strconv.ParseFloat(matches[9], 64)

    entry.Message = fmt.Sprintf("%s %s %d", entry.Method, entry.Path, entry.StatusCode)
    return entry
  }

  // 尝试标准 combined 格式
  if matches := pattern.FindStringSubmatch(line); matches != nil {
    entry.ClientIP = matches[1]
    entry.Fields["remote_user"] = matches[2]

    if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", matches[3]); err == nil {
      entry.Timestamp = t
    }

    requestParts := strings.SplitN(matches[4], " ", 3)
    if len(requestParts) >= 2 {
      entry.Method = requestParts[0]
      entry.Path = requestParts[1]
      if len(requestParts) == 3 {
        entry.Protocol = requestParts[2]
      }
    }

    entry.StatusCode, _ = strconv.Atoi(matches[5])
    entry.BodyBytes, _ = strconv.Atoi(matches[6])
    entry.Referer = matches[7]
    entry.UserAgent = matches[8]

    entry.Message = fmt.Sprintf("%s %s %d", entry.Method, entry.Path, entry.StatusCode)
    return entry
  }

  return nil
}

// readJournalctl 读取 journalctl 日志
func (la *LogAnalyzer) readJournalctl() ([]LogEntry, error) {
  // 获取最近 1 小时的日志
  cmd := exec.Command("journalctl", "--since", "1 hour ago", "--no-pager", "-o", "short-iso")
  output, err := cmd.Output()
  if err != nil {
    return nil, err
  }

  var entries []LogEntry
  lines := strings.Split(string(output), "\n")

  // journalctl 格式: 2024-01-15T10:30:45+0800 hostname service[pid]: message
  pattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+\-]\d{4})\s+(\S+)\s+(\S+?)(?:\[\d+\])?:\s+(.*)`)

  for i, line := range lines {
    if line == "" {
      continue
    }

    entry := LogEntry{
      Raw:        line,
      LineNumber: i + 1,
      Source:     "journalctl",
      Fields:     make(map[string]string),
    }

    if matches := pattern.FindStringSubmatch(line); matches != nil {
      if t, err := time.Parse("2006-01-02T15:04:05-0700", matches[1]); err == nil {
        entry.Timestamp = t
      }
      entry.Fields["hostname"] = matches[2]
      entry.Fields["service"] = matches[3]
      entry.Message = matches[4]
      entry.Level = la.inferLogLevel(matches[4])
    } else {
      entry.Message = line
      entry.Timestamp = time.Now()
      entry.Level = la.inferLogLevel(line)
    }

    entries = append(entries, entry)
  }

  return entries, nil
}

// readDmesg 读取 dmesg 日志
func (la *LogAnalyzer) readDmesg() ([]LogEntry, error) {
  cmd := exec.Command("dmesg", "--time-format=iso", "-l", "err,warn,crit")
  output, err := cmd.Output()
  if err != nil {
    // 尝试不带参数
    cmd = exec.Command("dmesg")
    output, err = cmd.Output()
    if err != nil {
      return nil, err
    }
  }

  var entries []LogEntry
  lines := strings.Split(string(output), "\n")

  for i, line := range lines {
    if line == "" {
      continue
    }

    entry := LogEntry{
      Raw:        line,
      LineNumber: i + 1,
      Source:     "dmesg",
      Fields:     make(map[string]string),
      Timestamp:  time.Now(),
    }

    entry.Message = line
    entry.Level = la.inferLogLevel(line)

    entries = append(entries, entry)
  }

  return entries, nil
}

// inferLogLevel 从日志内容推断日志级别
func (la *LogAnalyzer) inferLogLevel(message string) string {
  msg := strings.ToLower(message)

  if strings.Contains(msg, "error") || strings.Contains(msg, "fail") ||
    strings.Contains(msg, "fatal") || strings.Contains(msg, "crit") {
    return "ERROR"
  }
  if strings.Contains(msg, "warn") {
    return "WARN"
  }
  if strings.Contains(msg, "debug") {
    return "DEBUG"
  }
  if strings.Contains(msg, "info") {
    return "INFO"
  }

  return ""
}

// generateSummary 生成分析摘要
func (la *LogAnalyzer) generateSummary(result *AnalysisResult) string {
  var sb strings.Builder

  sb.WriteString(fmt.Sprintf("分析文件: %s\n", result.FilePath))
  sb.WriteString(fmt.Sprintf("总行数: %d, 解析成功: %d\n", result.TotalLines, result.ParsedLines))
  sb.WriteString(fmt.Sprintf("错误: %d, 警告: %d\n", result.ErrorLines, result.WarnLines))
  sb.WriteString(fmt.Sprintf("匹配模式: %d, 检测异常: %d\n", len(result.PatternMatches), len(result.Anomalies)))

  if len(result.PatternMatches) > 0 {
    sb.WriteString("\n关键问题:\n")
    // 按严重程度分组
    criticalCount := 0
    highCount := 0
    for _, match := range result.PatternMatches {
      if match.Pattern.Severity == "critical" {
        criticalCount++
      } else if match.Pattern.Severity == "high" {
        highCount++
      }
    }
    if criticalCount > 0 {
      sb.WriteString(fmt.Sprintf("  - 严重问题: %d 个\n", criticalCount))
    }
    if highCount > 0 {
      sb.WriteString(fmt.Sprintf("  - 高危问题: %d 个\n", highCount))
    }
  }

  return sb.String()
}

// generateNginxSummary 生成 Nginx 日志分析摘要
func (la *LogAnalyzer) generateNginxSummary(result *AnalysisResult, entries []NginxLogEntry) string {
  var sb strings.Builder

  sb.WriteString(fmt.Sprintf("分析文件: %s\n", result.FilePath))
  sb.WriteString(fmt.Sprintf("总请求数: %d\n", result.TotalLines))

  // 状态码统计
  sb.WriteString("\n状态码分布:\n")
  for status, count := range result.LevelCounts {
    sb.WriteString(fmt.Sprintf("  %s: %d\n", status, count))
  }

  // 计算响应时间统计
  if len(entries) > 0 {
    var totalTime float64
    var maxTime float64
    for _, e := range entries {
      totalTime += e.ResponseTime
      if e.ResponseTime > maxTime {
        maxTime = e.ResponseTime
      }
    }
    avgTime := totalTime / float64(len(entries))
    sb.WriteString(fmt.Sprintf("\n响应时间: 平均 %.3fs, 最大 %.3fs\n", avgTime, maxTime))
  }

  if len(result.Anomalies) > 0 {
    sb.WriteString(fmt.Sprintf("\n检测到 %d 个异常\n", len(result.Anomalies)))
  }

  return sb.String()
}

// generateKeyFindings 生成关键发现
func (la *LogAnalyzer) generateKeyFindings() []Finding {
  var findings []Finding

  // 从所有结果中提取关键发现
  for _, result := range la.Results {
    // 从模式匹配中提取
    criticalPatterns := make(map[string]int)
    for _, match := range result.PatternMatches {
      if match.Pattern.Severity == "critical" || match.Pattern.Severity == "high" {
        criticalPatterns[match.Pattern.Name]++
      }
    }

    for name, count := range criticalPatterns {
      finding := Finding{
        Type:           "pattern_match",
        Severity:       "high",
        Description:    fmt.Sprintf("检测到 %d 次 '%s' 问题", count, name),
        RelatedFiles:   []string{result.FilePath},
        Recommendation: la.getPatternRecommendation(name),
      }
      findings = append(findings, finding)
    }

    // 从异常中提取
    for _, anomaly := range result.Anomalies {
      finding := Finding{
        Type:           anomaly.Type,
        Severity:       anomaly.Severity,
        Description:    anomaly.Description,
        RelatedFiles:   []string{result.FilePath},
        Recommendation: anomaly.Suggestion,
      }
      findings = append(findings, finding)
    }
  }

  return findings
}

// generateRecommendations 生成建议
func (la *LogAnalyzer) generateRecommendations() []string {
  var recommendations []string
  recommendationSet := make(map[string]bool)

  for _, result := range la.Results {
    // 从模式匹配中收集建议
    for _, match := range result.PatternMatches {
      if match.Pattern.Suggestion != "" && !recommendationSet[match.Pattern.Suggestion] {
        recommendations = append(recommendations, match.Pattern.Suggestion)
        recommendationSet[match.Pattern.Suggestion] = true
      }
    }

    // 从异常中收集建议
    for _, anomaly := range result.Anomalies {
      if anomaly.Suggestion != "" && !recommendationSet[anomaly.Suggestion] {
        recommendations = append(recommendations, anomaly.Suggestion)
        recommendationSet[anomaly.Suggestion] = true
      }
    }
  }

  return recommendations
}

// getPatternRecommendation 获取模式对应的建议
func (la *LogAnalyzer) getPatternRecommendation(patternName string) string {
  for _, pattern := range la.PatternMatcher.GetPatterns() {
    if pattern.Name == patternName {
      return pattern.Suggestion
    }
  }
  return ""
}

// Reset 重置分析器状态
func (la *LogAnalyzer) Reset() {
  la.Results = make([]AnalysisResult, 0)
}

// SetTimeRange 设置时间范围过滤
func (la *LogAnalyzer) SetTimeRange(start, end time.Time) {
  la.Options.TimeRange = &TimeRange{Start: start, End: end}
}

// SetMaxLines 设置最大分析行数
func (la *LogAnalyzer) SetMaxLines(maxLines int) {
  la.Options.MaxLines = maxLines
}
