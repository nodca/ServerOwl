package main

import (
	"fmt"
	"strconv"
	"strings"
)

func parseConfirmCancel(s string) (cmd string, id string, ok bool) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return "", "", false
	}

	parts := strings.Fields(raw)
	head := parts[0]
	if len(parts) == 0 {
		return "", "", false
	}
	switch head {
	case "确认", "同意", "执行", "yes", "YES":
		cmd = "confirm"
	case "取消", "拒绝", "不执行", "no", "NO":
		cmd = "cancel"
	default:
		if strings.HasPrefix(raw, "确认") {
			cmd = "confirm"
			parts = []string{"确认", strings.TrimSpace(strings.TrimPrefix(raw, "确认"))}
		} else if strings.HasPrefix(raw, "取消") {
			cmd = "cancel"
			parts = []string{"取消", strings.TrimSpace(strings.TrimPrefix(raw, "取消"))}
		} else {
			return "", "", false
		}
	}
	if len(parts) >= 2 {
		token := strings.TrimSpace(parts[1])
		for _, r := range token {
			if r >= '0' && r <= '9' {
				id += string(r)
			}
		}
	} else {
		for _, r := range raw {
			if r >= '0' && r <= '9' {
				id += string(r)
			}
		}
	}
	if id == "" {
		return "", "", false
	}
	if _, err := strconv.Atoi(id); err != nil {
		return "", "", false
	}
	return cmd, id, true
}

func main() {
	testCases := []string{
		"确认 74184317",
		"确认74184317",
		"确认  74184317",
		"取消 74184317",
	}
	
	for _, tc := range testCases {
		cmd, id, ok := parseConfirmCancel(tc)
		fmt.Printf("Input: %q -> cmd=%q, id=%q, ok=%v\n", tc, cmd, id, ok)
	}
}
