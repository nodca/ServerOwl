package mcp

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogResourceProvider 日志资源提供者
type LogResourceProvider struct {
	logDirs []string
}

// NewLogResourceProvider 创建日志资源提供者
func NewLogResourceProvider(logDirs ...string) *LogResourceProvider {
	if len(logDirs) == 0 {
		logDirs = []string{"/var/log"}
	}
	return &LogResourceProvider{
		logDirs: logDirs,
	}
}

// ListResources 列出日志资源
func (p *LogResourceProvider) ListResources() []Resource {
	resources := []Resource{
		{
			URI:         "logs://system/syslog",
			Name:        "系统日志",
			Description: "系统日志 (/var/log/syslog 或 /var/log/messages)",
			MimeType:    "text/plain",
		},
		{
			URI:         "logs://system/auth",
			Name:        "认证日志",
			Description: "认证和安全日志",
			MimeType:    "text/plain",
		},
		{
			URI:         "logs://system/dmesg",
			Name:        "内核日志",
			Description: "内核环形缓冲区日志",
			MimeType:    "text/plain",
		},
		{
			URI:         "logs://nginx/access",
			Name:        "Nginx 访问日志",
			Description: "Nginx HTTP 访问日志",
			MimeType:    "text/plain",
		},
		{
			URI:         "logs://nginx/error",
			Name:        "Nginx 错误日志",
			Description: "Nginx 错误日志",
			MimeType:    "text/plain",
		},
	}

	// 动态发现日志文件
	for _, dir := range p.logDirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.log"))
		if err != nil {
			continue
		}
		for _, file := range files {
			name := filepath.Base(file)
			resources = append(resources, Resource{
				URI:         fmt.Sprintf("logs://file/%s", name),
				Name:        name,
				Description: fmt.Sprintf("日志文件: %s", file),
				MimeType:    "text/plain",
			})
		}
	}

	return resources
}

// ReadResource 读取日志资源
func (p *LogResourceProvider) ReadResource(uri string) (*ResourceContents, error) {
	var logPath string
	var lines int = 100 // 默认读取最后 100 行

	switch uri {
	case "logs://system/syslog":
		logPath = p.findLogFile("syslog", "messages")
	case "logs://system/auth":
		logPath = p.findLogFile("auth.log", "secure")
	case "logs://system/dmesg":
		return p.readDmesg()
	case "logs://nginx/access":
		logPath = p.findLogFile("nginx/access.log", "access.log")
	case "logs://nginx/error":
		logPath = p.findLogFile("nginx/error.log", "error.log")
	default:
		// 处理动态日志文件
		if strings.HasPrefix(uri, "logs://file/") {
			filename := strings.TrimPrefix(uri, "logs://file/")
			for _, dir := range p.logDirs {
				path := filepath.Join(dir, filename)
				if _, err := os.Stat(path); err == nil {
					logPath = path
					break
				}
			}
		}
	}

	if logPath == "" {
		return nil, fmt.Errorf("日志文件不存在: %s", uri)
	}

	content, err := p.tailFile(logPath, lines)
	if err != nil {
		return nil, err
	}

	return &ResourceContents{
		URI:      uri,
		MimeType: "text/plain",
		Text:     content,
	}, nil
}

func (p *LogResourceProvider) findLogFile(names ...string) string {
	for _, dir := range p.logDirs {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func (p *LogResourceProvider) tailFile(path string, lines int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	// 从文件末尾开始读取
	var result []string
	scanner := bufio.NewScanner(file)

	// 如果文件较小，直接读取全部
	if stat.Size() < 1024*1024 { // 小于 1MB
		for scanner.Scan() {
			result = append(result, scanner.Text())
		}
	} else {
		// 大文件：跳到末尾附近
		offset := stat.Size() - int64(lines*500) // 估算每行 500 字节
		if offset < 0 {
			offset = 0
		}
		file.Seek(offset, 0)

		// 跳过可能的不完整行
		if offset > 0 {
			scanner.Scan()
		}

		for scanner.Scan() {
			result = append(result, scanner.Text())
		}
	}

	// 只保留最后 N 行
	if len(result) > lines {
		result = result[len(result)-lines:]
	}

	return strings.Join(result, "\n"), nil
}

func (p *LogResourceProvider) readDmesg() (*ResourceContents, error) {
	// 读取 dmesg
	data, err := os.ReadFile("/var/log/dmesg")
	if err != nil {
		// 尝试使用 dmesg 命令
		return nil, fmt.Errorf("无法读取 dmesg: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 100 {
		lines = lines[len(lines)-100:]
	}

	return &ResourceContents{
		URI:      "logs://system/dmesg",
		MimeType: "text/plain",
		Text:     strings.Join(lines, "\n"),
	}, nil
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
}

// ParseLogLine 解析日志行
func ParseLogLine(line string) *LogEntry {
	entry := &LogEntry{
		Message: line,
	}

	// 尝试解析常见日志格式
	// syslog 格式: "Jan  1 00:00:00 hostname service: message"
	parts := strings.SplitN(line, " ", 6)
	if len(parts) >= 6 {
		// 尝试解析时间
		timeStr := strings.Join(parts[:3], " ")
		if t, err := time.Parse("Jan  2 15:04:05", timeStr); err == nil {
			entry.Timestamp = t
			entry.Source = parts[4]
			entry.Message = parts[5]
		}
	}

	// 检测日志级别
	lineLower := strings.ToLower(line)
	switch {
	case strings.Contains(lineLower, "error") || strings.Contains(lineLower, "fatal"):
		entry.Level = "error"
	case strings.Contains(lineLower, "warn"):
		entry.Level = "warning"
	case strings.Contains(lineLower, "info"):
		entry.Level = "info"
	case strings.Contains(lineLower, "debug"):
		entry.Level = "debug"
	default:
		entry.Level = "info"
	}

	return entry
}
