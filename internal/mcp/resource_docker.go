package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DockerResourceProvider Docker 资源提供者
type DockerResourceProvider struct {
	timeout time.Duration
}

// NewDockerResourceProvider 创建 Docker 资源提供者
func NewDockerResourceProvider() *DockerResourceProvider {
	return &DockerResourceProvider{
		timeout: 30 * time.Second,
	}
}

// ListResources 列出 Docker 资源
func (p *DockerResourceProvider) ListResources() []Resource {
	resources := []Resource{
		{
			URI:         "docker://containers",
			Name:        "容器列表",
			Description: "所有 Docker 容器",
			MimeType:    "application/json",
		},
		{
			URI:         "docker://images",
			Name:        "镜像列表",
			Description: "所有 Docker 镜像",
			MimeType:    "application/json",
		},
		{
			URI:         "docker://networks",
			Name:        "网络列表",
			Description: "Docker 网络配置",
			MimeType:    "application/json",
		},
		{
			URI:         "docker://volumes",
			Name:        "卷列表",
			Description: "Docker 存储卷",
			MimeType:    "application/json",
		},
		{
			URI:         "docker://stats",
			Name:        "容器统计",
			Description: "容器资源使用统计",
			MimeType:    "application/json",
		},
		{
			URI:         "docker://system",
			Name:        "系统信息",
			Description: "Docker 系统信息和磁盘使用",
			MimeType:    "application/json",
		},
	}

	// 动态添加运行中的容器
	containers, err := p.listContainers()
	if err == nil {
		for _, c := range containers {
			resources = append(resources, Resource{
				URI:         fmt.Sprintf("docker://container/%s", c.Name),
				Name:        c.Name,
				Description: fmt.Sprintf("容器: %s (%s)", c.Name, c.Status),
				MimeType:    "application/json",
			})
		}
	}

	return resources
}

// ReadResource 读取 Docker 资源
func (p *DockerResourceProvider) ReadResource(uri string) (*ResourceContents, error) {
	switch uri {
	case "docker://containers":
		return p.getContainers()
	case "docker://images":
		return p.getImages()
	case "docker://networks":
		return p.getNetworks()
	case "docker://volumes":
		return p.getVolumes()
	case "docker://stats":
		return p.getStats()
	case "docker://system":
		return p.getSystemInfo()
	default:
		// 处理单个容器
		if strings.HasPrefix(uri, "docker://container/") {
			name := strings.TrimPrefix(uri, "docker://container/")
			return p.getContainerDetail(name)
		}
	}

	return nil, fmt.Errorf("未知资源: %s", uri)
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Created string `json:"created"`
	Ports   string `json:"ports"`
}

func (p *DockerResourceProvider) listContainers() ([]ContainerInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.State}}|{{.CreatedAt}}|{{.Ports}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 7 {
			containers = append(containers, ContainerInfo{
				ID:      parts[0],
				Name:    parts[1],
				Image:   parts[2],
				Status:  parts[3],
				State:   parts[4],
				Created: parts[5],
				Ports:   parts[6],
			})
		}
	}

	return containers, nil
}

func (p *DockerResourceProvider) getContainers() (*ResourceContents, error) {
	containers, err := p.listContainers()
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"timestamp":  time.Now().Format(time.RFC3339),
		"count":      len(containers),
		"containers": containers,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "docker://containers",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *DockerResourceProvider) getImages() (*ResourceContents, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "images", "--format", "{{.Repository}}:{{.Tag}}|{{.ID}}|{{.Size}}|{{.CreatedAt}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var images []map[string]string
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 4 {
			images = append(images, map[string]string{
				"name":    parts[0],
				"id":      parts[1],
				"size":    parts[2],
				"created": parts[3],
			})
		}
	}

	result := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"count":     len(images),
		"images":    images,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "docker://images",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *DockerResourceProvider) getNetworks() (*ResourceContents, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "{{.ID}}|{{.Name}}|{{.Driver}}|{{.Scope}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var networks []map[string]string
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 4 {
			networks = append(networks, map[string]string{
				"id":     parts[0],
				"name":   parts[1],
				"driver": parts[2],
				"scope":  parts[3],
			})
		}
	}

	result := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"count":     len(networks),
		"networks":  networks,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "docker://networks",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *DockerResourceProvider) getVolumes() (*ResourceContents, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "volume", "ls", "--format", "{{.Name}}|{{.Driver}}|{{.Mountpoint}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var volumes []map[string]string
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			volumes = append(volumes, map[string]string{
				"name":       parts[0],
				"driver":     parts[1],
				"mountpoint": parts[2],
			})
		}
	}

	result := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"count":     len(volumes),
		"volumes":   volumes,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "docker://volumes",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *DockerResourceProvider) getStats() (*ResourceContents, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}|{{.NetIO}}|{{.BlockIO}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var stats []map[string]string
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 6 {
			stats = append(stats, map[string]string{
				"name":        parts[0],
				"cpu_percent": parts[1],
				"mem_usage":   parts[2],
				"mem_percent": parts[3],
				"net_io":      parts[4],
				"block_io":    parts[5],
			})
		}
	}

	result := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"count":     len(stats),
		"stats":     stats,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "docker://stats",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *DockerResourceProvider) getSystemInfo() (*ResourceContents, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	result := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Docker 版本
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if out, err := cmd.Output(); err == nil {
		result["version"] = strings.TrimSpace(string(out))
	}

	// Docker 信息
	cmd = exec.CommandContext(ctx, "docker", "info", "--format", "{{.Containers}} {{.ContainersRunning}} {{.ContainersPaused}} {{.ContainersStopped}} {{.Images}}")
	if out, err := cmd.Output(); err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 5 {
			result["containers_total"] = parts[0]
			result["containers_running"] = parts[1]
			result["containers_paused"] = parts[2]
			result["containers_stopped"] = parts[3]
			result["images"] = parts[4]
		}
	}

	// 磁盘使用
	cmd = exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}")
	if out, err := cmd.Output(); err == nil {
		var diskUsage []map[string]string
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			parts := strings.Split(line, "\t")
			if len(parts) >= 3 {
				diskUsage = append(diskUsage, map[string]string{
					"type":        parts[0],
					"size":        parts[1],
					"reclaimable": parts[2],
				})
			}
		}
		result["disk_usage"] = diskUsage
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "docker://system",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *DockerResourceProvider) getContainerDetail(name string) (*ResourceContents, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", name)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("容器不存在: %s", name)
	}

	// 解析并简化输出
	var inspectData []map[string]any
	if err := json.Unmarshal(out, &inspectData); err != nil {
		return nil, err
	}

	if len(inspectData) == 0 {
		return nil, fmt.Errorf("容器不存在: %s", name)
	}

	container := inspectData[0]
	result := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"name":      name,
	}

	// 提取关键信息
	if id, ok := container["Id"].(string); ok {
		result["id"] = id[:12]
	}
	if config, ok := container["Config"].(map[string]any); ok {
		result["image"] = config["Image"]
		result["env"] = config["Env"]
	}
	if state, ok := container["State"].(map[string]any); ok {
		result["state"] = state
	}
	if networkSettings, ok := container["NetworkSettings"].(map[string]any); ok {
		if ports, ok := networkSettings["Ports"].(map[string]any); ok {
			result["ports"] = ports
		}
		if networks, ok := networkSettings["Networks"].(map[string]any); ok {
			result["networks"] = networks
		}
	}
	if mounts, ok := container["Mounts"].([]any); ok {
		result["mounts"] = mounts
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      fmt.Sprintf("docker://container/%s", name),
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}
