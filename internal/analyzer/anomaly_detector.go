package analyzer

import (
  "math"
  "sort"
  "time"
)

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
  // 错误率突增阈值 (标准差倍数)
  ErrorSpikeThreshold float64
  // 慢请求阈值 (秒)
  SlowRequestThreshold float64
  // 时间窗口大小 (分钟)
  TimeWindowMinutes int
  // 最小样本数
  MinSampleSize int
}

// NewAnomalyDetector 创建新的异常检测器
func NewAnomalyDetector() *AnomalyDetector {
  return &AnomalyDetector{
    ErrorSpikeThreshold:  2.0,  // 超过 2 倍标准差视为异常
    SlowRequestThreshold: 3.0,  // 响应时间超过 3 秒视为慢请求
    TimeWindowMinutes:    5,    // 5 分钟时间窗口
    MinSampleSize:        10,   // 最少需要 10 个样本
  }
}

// DetectErrorSpike 检测错误率突增
// 通过计算每个时间窗口的错误数，检测是否存在异常突增
func (ad *AnomalyDetector) DetectErrorSpike(logs []LogEntry) []Anomaly {
  if len(logs) < ad.MinSampleSize {
    return nil
  }

  // 按时间窗口统计错误数
  windowCounts := ad.countByTimeWindow(logs, func(entry LogEntry) bool {
    return entry.Level == "ERROR" || entry.Level == "FATAL" || entry.Level == "CRITICAL"
  })

  if len(windowCounts) < 3 {
    return nil
  }

  // 计算统计值
  values := make([]float64, 0, len(windowCounts))
  for _, count := range windowCounts {
    values = append(values, float64(count))
  }

  mean := ad.calculateMean(values)
  stdDev := ad.calculateStdDev(values, mean)

  // 检测异常
  var anomalies []Anomaly
  windows := ad.getSortedWindows(windowCounts)

  for _, window := range windows {
    count := windowCounts[window]
    deviation := (float64(count) - mean) / stdDev

    if stdDev > 0 && deviation > ad.ErrorSpikeThreshold {
      anomaly := Anomaly{
        Type:        "error_spike",
        Severity:    ad.getSeverityByDeviation(deviation),
        Description: "检测到错误率突增",
        DetectedAt:  time.Now(),
        StartTime:   window,
        EndTime:     window.Add(time.Duration(ad.TimeWindowMinutes) * time.Minute),
        Stats: AnomalyStats{
          CurrentValue:    float64(count),
          BaselineValue:   mean,
          StdDev:          stdDev,
          DeviationFactor: deviation,
          SampleCount:     len(values),
        },
        Suggestion: "检查该时间段的服务状态和相关日志",
      }

      // 收集相关日志条目
      anomaly.RelatedEntries = ad.getEntriesInWindow(logs, window, ad.TimeWindowMinutes)
      anomalies = append(anomalies, anomaly)
    }
  }

  return anomalies
}

// DetectSlowRequests 检测慢请求
// 针对 Nginx 日志，检测响应时间异常的请求
func (ad *AnomalyDetector) DetectSlowRequests(logs []NginxLogEntry) []Anomaly {
  if len(logs) < ad.MinSampleSize {
    return nil
  }

  // 收集所有响应时间
  var responseTimes []float64
  for _, log := range logs {
    if log.ResponseTime > 0 {
      responseTimes = append(responseTimes, log.ResponseTime)
    }
  }

  if len(responseTimes) < ad.MinSampleSize {
    return nil
  }

  mean := ad.calculateMean(responseTimes)
  stdDev := ad.calculateStdDev(responseTimes, mean)

  // 检测慢请求
  var anomalies []Anomaly
  var slowRequests []LogEntry

  for _, log := range logs {
    // 绝对阈值检测
    if log.ResponseTime > ad.SlowRequestThreshold {
      slowRequests = append(slowRequests, log.LogEntry)
    }
  }

  if len(slowRequests) > 0 {
    // 按时间窗口分组慢请求
    windowCounts := ad.countByTimeWindow(slowRequests, func(entry LogEntry) bool {
      return true
    })

    for window, count := range windowCounts {
      if count >= 3 { // 同一时间窗口内有 3 个以上慢请求
        anomaly := Anomaly{
          Type:        "slow_requests",
          Severity:    "medium",
          Description: "检测到集中的慢请求",
          DetectedAt:  time.Now(),
          StartTime:   window,
          EndTime:     window.Add(time.Duration(ad.TimeWindowMinutes) * time.Minute),
          Stats: AnomalyStats{
            CurrentValue:    float64(count),
            BaselineValue:   mean,
            StdDev:          stdDev,
            DeviationFactor: ad.SlowRequestThreshold / mean,
            SampleCount:     len(responseTimes),
          },
          RelatedEntries: ad.getEntriesInWindow(slowRequests, window, ad.TimeWindowMinutes),
          Suggestion:     "检查后端服务性能和数据库查询",
        }
        anomalies = append(anomalies, anomaly)
      }
    }
  }

  return anomalies
}

// DetectStatusCodeSpike 检测 HTTP 状态码异常
func (ad *AnomalyDetector) DetectStatusCodeSpike(logs []NginxLogEntry, statusCode int) []Anomaly {
  if len(logs) < ad.MinSampleSize {
    return nil
  }

  // 转换为 LogEntry 并过滤指定状态码
  var entries []LogEntry
  for _, log := range logs {
    entries = append(entries, log.LogEntry)
  }

  windowCounts := ad.countByTimeWindow(entries, func(entry LogEntry) bool {
    // 从 Fields 中获取状态码
    for _, log := range logs {
      if log.LogEntry.Raw == entry.Raw {
        return log.StatusCode == statusCode
      }
    }
    return false
  })

  if len(windowCounts) < 3 {
    return nil
  }

  values := make([]float64, 0, len(windowCounts))
  for _, count := range windowCounts {
    values = append(values, float64(count))
  }

  mean := ad.calculateMean(values)
  stdDev := ad.calculateStdDev(values, mean)

  var anomalies []Anomaly
  windows := ad.getSortedWindows(windowCounts)

  for _, window := range windows {
    count := windowCounts[window]
    if stdDev == 0 {
      continue
    }
    deviation := (float64(count) - mean) / stdDev

    if deviation > ad.ErrorSpikeThreshold {
      anomaly := Anomaly{
        Type:        "status_code_spike",
        Severity:    ad.getSeverityByStatusCode(statusCode),
        Description: "检测到 HTTP 状态码异常突增",
        DetectedAt:  time.Now(),
        StartTime:   window,
        EndTime:     window.Add(time.Duration(ad.TimeWindowMinutes) * time.Minute),
        Stats: AnomalyStats{
          CurrentValue:    float64(count),
          BaselineValue:   mean,
          StdDev:          stdDev,
          DeviationFactor: deviation,
          SampleCount:     len(values),
        },
        Suggestion: ad.getSuggestionByStatusCode(statusCode),
      }
      anomalies = append(anomalies, anomaly)
    }
  }

  return anomalies
}

// DetectTrafficAnomaly 检测流量异常 (突增或突降)
func (ad *AnomalyDetector) DetectTrafficAnomaly(logs []LogEntry) []Anomaly {
  if len(logs) < ad.MinSampleSize {
    return nil
  }

  // 统计每个时间窗口的请求数
  windowCounts := ad.countByTimeWindow(logs, func(entry LogEntry) bool {
    return true // 统计所有请求
  })

  if len(windowCounts) < 3 {
    return nil
  }

  values := make([]float64, 0, len(windowCounts))
  for _, count := range windowCounts {
    values = append(values, float64(count))
  }

  mean := ad.calculateMean(values)
  stdDev := ad.calculateStdDev(values, mean)

  var anomalies []Anomaly
  windows := ad.getSortedWindows(windowCounts)

  for _, window := range windows {
    count := windowCounts[window]
    if stdDev == 0 {
      continue
    }
    deviation := (float64(count) - mean) / stdDev

    // 检测突增
    if deviation > ad.ErrorSpikeThreshold {
      anomaly := Anomaly{
        Type:        "traffic_spike",
        Severity:    "medium",
        Description: "检测到流量突增",
        DetectedAt:  time.Now(),
        StartTime:   window,
        EndTime:     window.Add(time.Duration(ad.TimeWindowMinutes) * time.Minute),
        Stats: AnomalyStats{
          CurrentValue:    float64(count),
          BaselineValue:   mean,
          StdDev:          stdDev,
          DeviationFactor: deviation,
          SampleCount:     len(values),
        },
        Suggestion: "检查是否存在异常访问或 DDoS 攻击",
      }
      anomalies = append(anomalies, anomaly)
    }

    // 检测突降
    if deviation < -ad.ErrorSpikeThreshold {
      anomaly := Anomaly{
        Type:        "traffic_drop",
        Severity:    "high",
        Description: "检测到流量突降",
        DetectedAt:  time.Now(),
        StartTime:   window,
        EndTime:     window.Add(time.Duration(ad.TimeWindowMinutes) * time.Minute),
        Stats: AnomalyStats{
          CurrentValue:    float64(count),
          BaselineValue:   mean,
          StdDev:          stdDev,
          DeviationFactor: math.Abs(deviation),
          SampleCount:     len(values),
        },
        Suggestion: "检查服务是否正常运行，网络是否通畅",
      }
      anomalies = append(anomalies, anomaly)
    }
  }

  return anomalies
}

// countByTimeWindow 按时间窗口统计符合条件的日志数
func (ad *AnomalyDetector) countByTimeWindow(logs []LogEntry, filter func(LogEntry) bool) map[time.Time]int {
  windowCounts := make(map[time.Time]int)
  windowDuration := time.Duration(ad.TimeWindowMinutes) * time.Minute

  for _, log := range logs {
    if !filter(log) {
      continue
    }
    // 将时间对齐到窗口起始
    windowStart := log.Timestamp.Truncate(windowDuration)
    windowCounts[windowStart]++
  }

  return windowCounts
}

// getEntriesInWindow 获取指定时间窗口内的日志条目
func (ad *AnomalyDetector) getEntriesInWindow(logs []LogEntry, windowStart time.Time, minutes int) []LogEntry {
  windowEnd := windowStart.Add(time.Duration(minutes) * time.Minute)
  var entries []LogEntry

  for _, log := range logs {
    if !log.Timestamp.Before(windowStart) && log.Timestamp.Before(windowEnd) {
      entries = append(entries, log)
    }
  }

  return entries
}

// getSortedWindows 获取排序后的时间窗口列表
func (ad *AnomalyDetector) getSortedWindows(windowCounts map[time.Time]int) []time.Time {
  windows := make([]time.Time, 0, len(windowCounts))
  for w := range windowCounts {
    windows = append(windows, w)
  }
  sort.Slice(windows, func(i, j int) bool {
    return windows[i].Before(windows[j])
  })
  return windows
}

// calculateMean 计算均值
func (ad *AnomalyDetector) calculateMean(values []float64) float64 {
  if len(values) == 0 {
    return 0
  }
  var sum float64
  for _, v := range values {
    sum += v
  }
  return sum / float64(len(values))
}

// calculateStdDev 计算标准差
func (ad *AnomalyDetector) calculateStdDev(values []float64, mean float64) float64 {
  if len(values) < 2 {
    return 0
  }
  var sumSquares float64
  for _, v := range values {
    diff := v - mean
    sumSquares += diff * diff
  }
  variance := sumSquares / float64(len(values)-1)
  return math.Sqrt(variance)
}

// getSeverityByDeviation 根据偏离程度确定严重性
func (ad *AnomalyDetector) getSeverityByDeviation(deviation float64) string {
  if deviation > 4.0 {
    return "critical"
  } else if deviation > 3.0 {
    return "high"
  } else if deviation > 2.0 {
    return "medium"
  }
  return "low"
}

// getSeverityByStatusCode 根据状态码确定严重性
func (ad *AnomalyDetector) getSeverityByStatusCode(statusCode int) string {
  switch {
  case statusCode >= 500:
    return "high"
  case statusCode >= 400:
    return "medium"
  default:
    return "low"
  }
}

// getSuggestionByStatusCode 根据状态码给出建议
func (ad *AnomalyDetector) getSuggestionByStatusCode(statusCode int) string {
  switch statusCode {
  case 500:
    return "检查应用日志，排查内部错误"
  case 502:
    return "检查上游服务状态"
  case 503:
    return "检查服务是否过载或正在维护"
  case 504:
    return "检查上游服务响应时间"
  case 400:
    return "检查客户端请求格式"
  case 401:
    return "检查认证配置"
  case 403:
    return "检查权限配置"
  case 404:
    return "检查路由配置和资源是否存在"
  case 429:
    return "检查限流配置"
  default:
    return "检查相关服务和配置"
  }
}

// CalculatePercentile 计算百分位数
func (ad *AnomalyDetector) CalculatePercentile(values []float64, percentile float64) float64 {
  if len(values) == 0 {
    return 0
  }

  sorted := make([]float64, len(values))
  copy(sorted, values)
  sort.Float64s(sorted)

  index := (percentile / 100.0) * float64(len(sorted)-1)
  lower := int(math.Floor(index))
  upper := int(math.Ceil(index))

  if lower == upper {
    return sorted[lower]
  }

  // 线性插值
  weight := index - float64(lower)
  return sorted[lower]*(1-weight) + sorted[upper]*weight
}
