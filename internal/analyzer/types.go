package analyzer

import (
  "time"
)

// LogEntry 表示一条日志记录
type LogEntry struct {
  // 时间戳
  Timestamp time.Time
  // 日志级别 (INFO, WARN, ERROR, DEBUG 等)
  Level string
  // 日志来源 (文件路径或服务名)
  Source string
  // 日志内容
  Message string
  // 原始日志行
  Raw string
  // 行号
  LineNumber int
  // 额外字段 (如 HTTP 状态码、响应时间等)
  Fields map[string]string
}

// Pattern 表示一个错误模式
type Pattern struct {
  // 模式名称
  Name string
  // 正则表达式
  Regex string
  // 严重程度 (critical, high, medium, low)
  Severity string
  // 问题描述
  Description string
  // 修复建议
  Suggestion string
  // 分类标签
  Category string
}

// PatternMatch 表示一次模式匹配结果
type PatternMatch struct {
  // 匹配的模式
  Pattern Pattern
  // 匹配的日志条目
  Entry LogEntry
  // 正则捕获组
  Captures []string
  // 匹配时间
  MatchedAt time.Time
}

// Anomaly 表示检测到的异常
type Anomaly struct {
  // 异常类型
  Type string
  // 严重程度
  Severity string
  // 异常描述
  Description string
  // 检测时间
  DetectedAt time.Time
  // 开始时间
  StartTime time.Time
  // 结束时间
  EndTime time.Time
  // 相关日志条目
  RelatedEntries []LogEntry
  // 统计数据
  Stats AnomalyStats
  // 建议操作
  Suggestion string
}

// AnomalyStats 异常统计数据
type AnomalyStats struct {
  // 当前值
  CurrentValue float64
  // 基准值 (均值)
  BaselineValue float64
  // 标准差
  StdDev float64
  // 偏离倍数
  DeviationFactor float64
  // 样本数量
  SampleCount int
}

// AnalysisResult 表示分析结果
type AnalysisResult struct {
  // 分析的文件路径
  FilePath string
  // 分析开始时间
  StartTime time.Time
  // 分析结束时间
  EndTime time.Time
  // 总行数
  TotalLines int
  // 解析成功的行数
  ParsedLines int
  // 错误行数
  ErrorLines int
  // 警告行数
  WarnLines int
  // 模式匹配结果
  PatternMatches []PatternMatch
  // 检测到的异常
  Anomalies []Anomaly
  // 日志级别统计
  LevelCounts map[string]int
  // 每小时错误分布
  HourlyErrors map[int]int
  // 摘要信息
  Summary string
}

// NginxLogEntry Nginx 访问日志条目
type NginxLogEntry struct {
  LogEntry
  // 客户端 IP
  ClientIP string
  // 请求方法
  Method string
  // 请求路径
  Path string
  // HTTP 协议版本
  Protocol string
  // 状态码
  StatusCode int
  // 响应大小 (字节)
  BodyBytes int
  // 引用页
  Referer string
  // 用户代理
  UserAgent string
  // 响应时间 (秒)
  ResponseTime float64
  // 上游响应时间
  UpstreamTime float64
}

// AnalysisReport 分析报告
type AnalysisReport struct {
  // 报告生成时间
  GeneratedAt time.Time
  // 分析的时间范围
  TimeRange TimeRange
  // 各文件分析结果
  Results []AnalysisResult
  // 总体统计
  OverallStats OverallStats
  // 关键发现
  KeyFindings []Finding
  // 建议操作
  Recommendations []string
}

// TimeRange 时间范围
type TimeRange struct {
  Start time.Time
  End   time.Time
}

// OverallStats 总体统计
type OverallStats struct {
  // 分析的文件数
  FilesAnalyzed int
  // 总日志行数
  TotalLines int
  // 总错误数
  TotalErrors int
  // 总警告数
  TotalWarnings int
  // 检测到的异常数
  AnomaliesDetected int
  // 匹配的模式数
  PatternsMatched int
}

// Finding 关键发现
type Finding struct {
  // 发现类型
  Type string
  // 严重程度
  Severity string
  // 描述
  Description string
  // 影响
  Impact string
  // 建议
  Recommendation string
  // 相关文件
  RelatedFiles []string
}
