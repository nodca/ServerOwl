package agentnode

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RuleEngine 自治规则引擎
type RuleEngine struct {
	rules     []AutoRule
	metrics   *SystemMetrics
	mu        sync.RWMutex
	logger    Logger
	eventChan chan *RuleEvent
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// RuleEvent 规则触发事件
type RuleEvent struct {
	RuleID    string
	RuleName  string
	Condition string
	Action    string
	Success   bool
	Output    string
	Error     string
	Timestamp time.Time
}

// NewRuleEngine 创建规则引擎
func NewRuleEngine(logger Logger) *RuleEngine {
	return &RuleEngine{
		rules:     []AutoRule{},
		logger:    logger,
		eventChan: make(chan *RuleEvent, 100),
	}
}

// SetRules 设置规则
func (e *RuleEngine) SetRules(rules []AutoRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = rules
}

// UpdateMetrics 更新指标
func (e *RuleEngine) UpdateMetrics(metrics *SystemMetrics) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.metrics = metrics
}

// Events 返回事件通道
func (e *RuleEngine) Events() <-chan *RuleEvent {
	return e.eventChan
}

// Evaluate 评估所有规则
func (e *RuleEngine) Evaluate(ctx context.Context) {
	e.mu.RLock()
	rules := make([]AutoRule, len(e.rules))
	copy(rules, e.rules)
	metrics := e.metrics
	e.mu.RUnlock()

	for i := range rules {
		rule := &rules[i]
		if !rule.Enabled {
			continue
		}

		// 检查冷却时间
		if rule.Cooldown > 0 && time.Since(rule.LastTriggered) < rule.Cooldown {
			continue
		}

		// 评估条件
		triggered, conditionStr := e.evaluateCondition(rule, metrics)
		if !triggered {
			continue
		}

		e.logger.Info("规则触发: %s, 条件: %s", rule.Name, conditionStr)

		// 执行动作
		for _, action := range rule.Actions {
			event := &RuleEvent{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Condition: conditionStr,
				Action:    action.Command,
				Timestamp: time.Now(),
			}

			output, err := e.executeAction(ctx, &action)
			if err != nil {
				event.Success = false
				event.Error = err.Error()
				e.logger.Error("规则动作执行失败: %s, 错误: %v", rule.Name, err)
			} else {
				event.Success = true
				event.Output = output
				e.logger.Info("规则动作执行成功: %s", rule.Name)
			}

			// 发送事件
			select {
			case e.eventChan <- event:
			default:
				// 通道满了，丢弃旧事件
			}
		}

		// 更新触发时间
		e.mu.Lock()
		for j := range e.rules {
			if e.rules[j].ID == rule.ID {
				e.rules[j].LastTriggered = time.Now()
				e.rules[j].TriggerCount++
				break
			}
		}
		e.mu.Unlock()
	}
}

// evaluateCondition 评估条件
func (e *RuleEngine) evaluateCondition(rule *AutoRule, metrics *SystemMetrics) (bool, string) {
	cond := &rule.Condition

	switch cond.Type {
	case ConditionTypeMetric:
		return e.evaluateMetricCondition(cond, metrics)
	case ConditionTypeService:
		return e.evaluateServiceCondition(cond)
	case ConditionTypeProcess:
		return e.evaluateProcessCondition(cond)
	default:
		return false, ""
	}
}

func (e *RuleEngine) evaluateMetricCondition(cond *RuleCondition, metrics *SystemMetrics) (bool, string) {
	if metrics == nil {
		return false, ""
	}

	var value float64
	switch cond.Metric {
	case "cpu_usage":
		value = metrics.CPUUsage
	case "memory_usage":
		value = metrics.MemoryUsage
	case "disk_usage":
		value = metrics.DiskUsage
	case "load_avg_1":
		value = metrics.LoadAvg1
	case "load_avg_5":
		value = metrics.LoadAvg5
	case "load_avg_15":
		value = metrics.LoadAvg15
	default:
		return false, ""
	}

	condStr := fmt.Sprintf("%s %s %.2f (当前: %.2f)", cond.Metric, cond.Operator, cond.Threshold, value)

	switch cond.Operator {
	case ">":
		return value > cond.Threshold, condStr
	case ">=":
		return value >= cond.Threshold, condStr
	case "<":
		return value < cond.Threshold, condStr
	case "<=":
		return value <= cond.Threshold, condStr
	case "==":
		return value == cond.Threshold, condStr
	case "!=":
		return value != cond.Threshold, condStr
	default:
		return false, ""
	}
}

func (e *RuleEngine) evaluateServiceCondition(cond *RuleCondition) (bool, string) {
	// 从 metric 字段提取服务名，格式: service:nginx
	serviceName := strings.TrimPrefix(cond.Metric, "service:")
	if serviceName == cond.Metric {
		return false, ""
	}

	isActive, _ := CheckServiceStatus(serviceName)
	condStr := fmt.Sprintf("service:%s %s (当前: %v)", serviceName, cond.Operator, isActive)

	switch cond.Operator {
	case "==":
		if cond.Threshold == 1 {
			return isActive, condStr
		}
		return !isActive, condStr
	case "!=":
		if cond.Threshold == 1 {
			return !isActive, condStr
		}
		return isActive, condStr
	default:
		// down = 服务不活跃
		return !isActive, condStr
	}
}

func (e *RuleEngine) evaluateProcessCondition(cond *RuleCondition) (bool, string) {
	processName := strings.TrimPrefix(cond.Metric, "process:")
	if processName == cond.Metric {
		return false, ""
	}

	exists, _ := CheckProcessExists(processName)
	condStr := fmt.Sprintf("process:%s exists=%v", processName, exists)

	switch cond.Operator {
	case "==":
		if cond.Threshold == 1 {
			return exists, condStr
		}
		return !exists, condStr
	default:
		return !exists, condStr
	}
}

// executeAction 执行动作
func (e *RuleEngine) executeAction(ctx context.Context, action *RuleAction) (string, error) {
	timeout := action.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch action.Type {
	case ActionTypeShell:
		cmd := exec.CommandContext(ctx, "sh", "-c", action.Command)
		output, err := cmd.CombinedOutput()
		return string(output), err

	case ActionTypeRestart:
		// action.Command 是服务名
		cmd := exec.CommandContext(ctx, "systemctl", "restart", action.Command)
		output, err := cmd.CombinedOutput()
		return string(output), err

	case ActionTypeNotify:
		// 只记录，不执行
		return fmt.Sprintf("通知: %s", action.Command), nil

	default:
		return "", fmt.Errorf("未知动作类型: %s", action.Type)
	}
}

// GetRuleStats 获取规则统计
func (e *RuleEngine) GetRuleStats() map[string]int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := make(map[string]int)
	for _, rule := range e.rules {
		stats[rule.ID] = rule.TriggerCount
	}
	return stats
}
