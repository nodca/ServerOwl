package commander

type Intent struct {
	Type   string            `json:"type"`   // status, restart, logs, help, unknown
	Target string            `json:"target"` // 容器名/服务名
	Params map[string]string `json:"params"` // 额外参数
}
