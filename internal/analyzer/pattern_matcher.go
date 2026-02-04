package analyzer

import (
  "regexp"
  "sync"
)

// PatternMatcher 错误模式匹配器
type PatternMatcher struct {
  // 已注册的模式
  patterns []Pattern
  // 编译后的正则表达式缓存
  compiled map[string]*regexp.Regexp
  // 互斥锁
  mu sync.RWMutex
}

// NewPatternMatcher 创建新的模式匹配器，包含内置模式
func NewPatternMatcher() *PatternMatcher {
  pm := &PatternMatcher{
    patterns: make([]Pattern, 0),
    compiled: make(map[string]*regexp.Regexp),
  }
  // 加载内置模式
  pm.loadBuiltinPatterns()
  return pm
}

// loadBuiltinPatterns 加载内置的常见错误模式
func (pm *PatternMatcher) loadBuiltinPatterns() {
  builtinPatterns := []Pattern{
    // ===== 内存相关 =====
    {
      Name:        "OOM Killer",
      Regex:       `Out of memory: Kill(ed)? process \d+`,
      Severity:    "critical",
      Description: "系统内存不足，OOM Killer 终止了进程",
      Suggestion:  "检查内存使用情况，考虑增加内存或优化应用内存占用",
      Category:    "memory",
    },
    {
      Name:        "OOM 进程被杀",
      Regex:       `oom-kill:.*task_memcg`,
      Severity:    "critical",
      Description: "进程因内存不足被 OOM Killer 终止",
      Suggestion:  "检查 cgroup 内存限制，调整容器内存配额",
      Category:    "memory",
    },
    {
      Name:        "内存分配失败",
      Regex:       `(cannot allocate memory|memory allocation failed|ENOMEM)`,
      Severity:    "high",
      Description: "内存分配失败",
      Suggestion:  "检查系统可用内存，释放不必要的内存占用",
      Category:    "memory",
    },

    // ===== 磁盘相关 =====
    {
      Name:        "磁盘空间不足",
      Regex:       `(No space left on device|ENOSPC|disk full)`,
      Severity:    "critical",
      Description: "磁盘空间已满",
      Suggestion:  "清理磁盘空间，删除不必要的文件或日志",
      Category:    "disk",
    },
    {
      Name:        "磁盘只读",
      Regex:       `(Read-only file system|EROFS|Remounting filesystem read-only)`,
      Severity:    "critical",
      Description: "文件系统变为只读模式，可能存在磁盘故障",
      Suggestion:  "检查磁盘健康状态，运行 fsck 检查文件系统",
      Category:    "disk",
    },
    {
      Name:        "磁盘 I/O 错误",
      Regex:       `(I/O error|Buffer I/O error|blk_update_request: I/O error)`,
      Severity:    "critical",
      Description: "磁盘 I/O 错误，可能存在硬件故障",
      Suggestion:  "检查磁盘 SMART 状态，考虑更换磁盘",
      Category:    "disk",
    },
    {
      Name:        "inode 耗尽",
      Regex:       `(No space left on device.*inode|inode.*exhausted)`,
      Severity:    "high",
      Description: "inode 数量耗尽",
      Suggestion:  "删除大量小文件或重新格式化分区",
      Category:    "disk",
    },

    // ===== 网络相关 =====
    {
      Name:        "连接超时",
      Regex:       `(Connection timed out|ETIMEDOUT|connect timeout)`,
      Severity:    "medium",
      Description: "网络连接超时",
      Suggestion:  "检查网络连通性和目标服务状态",
      Category:    "network",
    },
    {
      Name:        "连接被拒绝",
      Regex:       `(Connection refused|ECONNREFUSED)`,
      Severity:    "medium",
      Description: "连接被目标服务拒绝",
      Suggestion:  "检查目标服务是否运行，端口是否正确",
      Category:    "network",
    },
    {
      Name:        "连接重置",
      Regex:       `(Connection reset by peer|ECONNRESET)`,
      Severity:    "medium",
      Description: "连接被对端重置",
      Suggestion:  "检查网络稳定性和对端服务状态",
      Category:    "network",
    },
    {
      Name:        "DNS 解析失败",
      Regex:       `(Name or service not known|NXDOMAIN|DNS.*failed|could not resolve)`,
      Severity:    "high",
      Description: "DNS 解析失败",
      Suggestion:  "检查 DNS 配置和域名是否正确",
      Category:    "network",
    },
    {
      Name:        "端口已被占用",
      Regex:       `(Address already in use|EADDRINUSE)`,
      Severity:    "high",
      Description: "端口已被其他进程占用",
      Suggestion:  "检查端口占用情况，终止冲突进程或更换端口",
      Category:    "network",
    },

    // ===== 权限相关 =====
    {
      Name:        "权限拒绝",
      Regex:       `(Permission denied|EACCES|EPERM|Access denied)`,
      Severity:    "medium",
      Description: "权限不足，操作被拒绝",
      Suggestion:  "检查文件/目录权限和用户身份",
      Category:    "permission",
    },
    {
      Name:        "认证失败",
      Regex:       `(authentication fail|auth.*failed|invalid password|login failed)`,
      Severity:    "high",
      Description: "认证失败",
      Suggestion:  "检查用户名密码是否正确，是否存在暴力破解",
      Category:    "security",
    },

    // ===== Nginx 相关 =====
    {
      Name:        "Nginx 上游超时",
      Regex:       `upstream timed out.*while (reading|connecting)`,
      Severity:    "high",
      Description: "Nginx 上游服务响应超时",
      Suggestion:  "检查后端服务状态，考虑增加超时时间",
      Category:    "nginx",
    },
    {
      Name:        "Nginx 上游连接失败",
      Regex:       `connect\(\) failed.*while connecting to upstream`,
      Severity:    "high",
      Description: "Nginx 无法连接到上游服务",
      Suggestion:  "检查后端服务是否运行，网络是否通畅",
      Category:    "nginx",
    },
    {
      Name:        "Nginx 502 错误",
      Regex:       `upstream prematurely closed connection`,
      Severity:    "high",
      Description: "上游服务过早关闭连接，导致 502 错误",
      Suggestion:  "检查后端服务稳定性和资源使用情况",
      Category:    "nginx",
    },
    {
      Name:        "Nginx worker 异常退出",
      Regex:       `worker process \d+ exited on signal \d+`,
      Severity:    "high",
      Description: "Nginx worker 进程异常退出",
      Suggestion:  "检查 Nginx 错误日志，可能存在内存问题",
      Category:    "nginx",
    },

    // ===== Docker 相关 =====
    {
      Name:        "Docker 容器 OOM",
      Regex:       `container.*killed.*OOM|OOMKilled.*true`,
      Severity:    "critical",
      Description: "Docker 容器因内存不足被终止",
      Suggestion:  "增加容器内存限制或优化应用内存使用",
      Category:    "docker",
    },
    {
      Name:        "Docker 镜像拉取失败",
      Regex:       `(pull access denied|manifest.*not found|image.*not found)`,
      Severity:    "high",
      Description: "Docker 镜像拉取失败",
      Suggestion:  "检查镜像名称是否正确，是否需要登录认证",
      Category:    "docker",
    },
    {
      Name:        "Docker 容器启动失败",
      Regex:       `(container.*failed to start|Error starting container)`,
      Severity:    "high",
      Description: "Docker 容器启动失败",
      Suggestion:  "检查容器配置和依赖服务",
      Category:    "docker",
    },
    {
      Name:        "Docker 网络错误",
      Regex:       `(network.*not found|failed to create endpoint)`,
      Severity:    "medium",
      Description: "Docker 网络配置错误",
      Suggestion:  "检查 Docker 网络配置",
      Category:    "docker",
    },

    // ===== 系统服务相关 =====
    {
      Name:        "服务启动失败",
      Regex:       `(Failed to start|service.*failed|systemd.*failed)`,
      Severity:    "high",
      Description: "系统服务启动失败",
      Suggestion:  "检查服务配置和依赖",
      Category:    "systemd",
    },
    {
      Name:        "服务崩溃",
      Regex:       `(segfault|Segmentation fault|SIGSEGV|core dumped)`,
      Severity:    "critical",
      Description: "进程发生段错误崩溃",
      Suggestion:  "检查应用日志，可能存在内存访问错误",
      Category:    "crash",
    },
    {
      Name:        "进程被杀",
      Regex:       `(Killed|SIGKILL|signal 9)`,
      Severity:    "high",
      Description: "进程被强制终止",
      Suggestion:  "检查是否被 OOM Killer 终止或手动 kill",
      Category:    "process",
    },

    // ===== 数据库相关 =====
    {
      Name:        "数据库连接失败",
      Regex:       `(could not connect to.*database|database.*connection.*failed)`,
      Severity:    "high",
      Description: "数据库连接失败",
      Suggestion:  "检查数据库服务状态和连接配置",
      Category:    "database",
    },
    {
      Name:        "数据库连接池耗尽",
      Regex:       `(too many connections|connection pool exhausted|max.*connections)`,
      Severity:    "high",
      Description: "数据库连接数达到上限",
      Suggestion:  "增加连接池大小或优化连接使用",
      Category:    "database",
    },
    {
      Name:        "数据库死锁",
      Regex:       `(deadlock|Deadlock found)`,
      Severity:    "high",
      Description: "数据库发生死锁",
      Suggestion:  "检查事务逻辑，优化锁顺序",
      Category:    "database",
    },

    // ===== SSL/TLS 相关 =====
    {
      Name:        "SSL 证书过期",
      Regex:       `(certificate has expired|SSL.*expired|cert.*expired)`,
      Severity:    "critical",
      Description: "SSL 证书已过期",
      Suggestion:  "立即更新 SSL 证书",
      Category:    "ssl",
    },
    {
      Name:        "SSL 握手失败",
      Regex:       `(SSL handshake failed|SSL_ERROR|TLS.*error)`,
      Severity:    "high",
      Description: "SSL/TLS 握手失败",
      Suggestion:  "检查证书配置和 TLS 版本兼容性",
      Category:    "ssl",
    },
  }

  for _, p := range builtinPatterns {
    pm.AddPattern(p)
  }
}

// AddPattern 添加新的匹配模式
func (pm *PatternMatcher) AddPattern(pattern Pattern) error {
  pm.mu.Lock()
  defer pm.mu.Unlock()

  // 预编译正则表达式
  compiled, err := regexp.Compile(pattern.Regex)
  if err != nil {
    return err
  }

  pm.patterns = append(pm.patterns, pattern)
  pm.compiled[pattern.Name] = compiled
  return nil
}

// RemovePattern 移除指定名称的模式
func (pm *PatternMatcher) RemovePattern(name string) {
  pm.mu.Lock()
  defer pm.mu.Unlock()

  for i, p := range pm.patterns {
    if p.Name == name {
      pm.patterns = append(pm.patterns[:i], pm.patterns[i+1:]...)
      delete(pm.compiled, name)
      return
    }
  }
}

// MatchLine 对单行日志进行模式匹配
func (pm *PatternMatcher) MatchLine(entry LogEntry) []PatternMatch {
  pm.mu.RLock()
  defer pm.mu.RUnlock()

  var matches []PatternMatch

  for _, pattern := range pm.patterns {
    compiled := pm.compiled[pattern.Name]
    if compiled == nil {
      continue
    }

    if result := compiled.FindStringSubmatch(entry.Raw); result != nil {
      match := PatternMatch{
        Pattern:   pattern,
        Entry:     entry,
        Captures:  result,
        MatchedAt: entry.Timestamp,
      }
      matches = append(matches, match)
    }
  }

  return matches
}

// MatchLines 对多行日志进行模式匹配
func (pm *PatternMatcher) MatchLines(entries []LogEntry) []PatternMatch {
  var allMatches []PatternMatch

  for _, entry := range entries {
    matches := pm.MatchLine(entry)
    allMatches = append(allMatches, matches...)
  }

  return allMatches
}

// MatchByCategory 按分类匹配
func (pm *PatternMatcher) MatchByCategory(entries []LogEntry, category string) []PatternMatch {
  pm.mu.RLock()
  defer pm.mu.RUnlock()

  var matches []PatternMatch

  for _, entry := range entries {
    for _, pattern := range pm.patterns {
      if pattern.Category != category {
        continue
      }

      compiled := pm.compiled[pattern.Name]
      if compiled == nil {
        continue
      }

      if result := compiled.FindStringSubmatch(entry.Raw); result != nil {
        match := PatternMatch{
          Pattern:   pattern,
          Entry:     entry,
          Captures:  result,
          MatchedAt: entry.Timestamp,
        }
        matches = append(matches, match)
      }
    }
  }

  return matches
}

// MatchBySeverity 按严重程度匹配
func (pm *PatternMatcher) MatchBySeverity(entries []LogEntry, severity string) []PatternMatch {
  pm.mu.RLock()
  defer pm.mu.RUnlock()

  var matches []PatternMatch

  for _, entry := range entries {
    for _, pattern := range pm.patterns {
      if pattern.Severity != severity {
        continue
      }

      compiled := pm.compiled[pattern.Name]
      if compiled == nil {
        continue
      }

      if result := compiled.FindStringSubmatch(entry.Raw); result != nil {
        match := PatternMatch{
          Pattern:   pattern,
          Entry:     entry,
          Captures:  result,
          MatchedAt: entry.Timestamp,
        }
        matches = append(matches, match)
      }
    }
  }

  return matches
}

// GetPatterns 获取所有已注册的模式
func (pm *PatternMatcher) GetPatterns() []Pattern {
  pm.mu.RLock()
  defer pm.mu.RUnlock()

  result := make([]Pattern, len(pm.patterns))
  copy(result, pm.patterns)
  return result
}

// GetPatternsByCategory 按分类获取模式
func (pm *PatternMatcher) GetPatternsByCategory(category string) []Pattern {
  pm.mu.RLock()
  defer pm.mu.RUnlock()

  var result []Pattern
  for _, p := range pm.patterns {
    if p.Category == category {
      result = append(result, p)
    }
  }
  return result
}

// GetCategories 获取所有分类
func (pm *PatternMatcher) GetCategories() []string {
  pm.mu.RLock()
  defer pm.mu.RUnlock()

  categoryMap := make(map[string]bool)
  for _, p := range pm.patterns {
    categoryMap[p.Category] = true
  }

  var categories []string
  for cat := range categoryMap {
    categories = append(categories, cat)
  }
  return categories
}
