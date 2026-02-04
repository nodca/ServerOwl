package monitor

import (
	"net/http"
	"time"
)

type HTTPCheckResult struct {
	Name       string
	URL        string
	StatusCode int
	Latency    time.Duration
	Healthy    bool
	Error      string
}

// 检查单个HTTP端点
func CheckHTTP(name, url string, timeout time.Duration) HTTPCheckResult {
	client := http.Client{
		Timeout: timeout,
	}
	result := HTTPCheckResult{
		Name: name,
		URL:  url,
	}
	now := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		result.Healthy = false
		result.Error = err.Error()
		result.Latency = time.Since(now)
	} else {
		defer resp.Body.Close()
		result.StatusCode = resp.StatusCode
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			result.Healthy = true
		} else {
			result.Healthy = false
		}
		result.Latency = time.Since(now)
	}
	return result
}
