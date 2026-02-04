package agent

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"serverowl/internal/cluster"

	"gopkg.in/yaml.v3"
)

// Environment 服务器环境信息
type Environment struct {
	UpdatedAt  string                    `yaml:"updated_at"`
	Host       HostInfo                  `yaml:"host"`
	Containers map[string]*ContainerInfo `yaml:"containers"`
	Databases  map[string]*DatabaseInfo  `yaml:"databases,omitempty"`
	Proxies    map[string]*ProxyInfo     `yaml:"proxies,omitempty"`
}

// MultiNodeEnvironment 多节点环境信息
type MultiNodeEnvironment struct {
	Nodes map[string]*Environment `yaml:"nodes"`
}

type HostInfo struct {
	Hostname string `yaml:"hostname"`
	IP       string `yaml:"ip,omitempty"`
}

type ContainerInfo struct {
	Image   string            `yaml:"image"`
	Status  string            `yaml:"status"`
	Env     map[string]string `yaml:"env,omitempty"`
	Ports   []string          `yaml:"ports,omitempty"`
	Network string            `yaml:"network,omitempty"`
}

type DatabaseInfo struct {
	Type     string `yaml:"type"` // postgres, mysql, redis
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database,omitempty"`
	Source   string `yaml:"source"` // 来源：container env, config file
}

type ProxyInfo struct {
	Type       string     `yaml:"type"` // caddy, nginx
	ConfigFile string     `yaml:"config_file"`
	Sites      []SiteInfo `yaml:"sites,omitempty"`
	Source     string     `yaml:"source"`
}

type SiteInfo struct {
	Domain  string `yaml:"domain"`
	Backend string `yaml:"backend,omitempty"`
	Root    string `yaml:"root,omitempty"`
}

// EnvironmentManager 环境信息管理器
type EnvironmentManager struct {
	filePath      string
	configPath    string // ServerOwl 配置文件路径
	env           *Environment
	masterCluster *cluster.MasterCluster
	localNodeID   string // 本地节点 ID
}

// NewEnvironmentManager 创建环境管理器
func NewEnvironmentManager(filePath, configPath string) *EnvironmentManager {
	return &EnvironmentManager{
		filePath:   filePath,
		configPath: configPath,
	}
}

// SetMasterCluster 设置 MasterCluster 引用
func (m *EnvironmentManager) SetMasterCluster(mc *cluster.MasterCluster, localNodeID string) {
	m.masterCluster = mc
	m.localNodeID = localNodeID
}

// Load 加载环境信息（从文件或扫描）
func (m *EnvironmentManager) Load() error {
	// 尝试从文件加载
	if data, err := os.ReadFile(m.filePath); err == nil {
		var env Environment
		if err := yaml.Unmarshal(data, &env); err == nil {
			m.env = &env
			return nil
		}
	}

	// 文件不存在或解析失败，执行扫描
	return m.Refresh()
}

// Refresh 刷新环境信息
func (m *EnvironmentManager) Refresh() error {
	env := &Environment{
		UpdatedAt:  time.Now().Format("2006-01-02 15:04:05"),
		Containers: make(map[string]*ContainerInfo),
		Databases:  make(map[string]*DatabaseInfo),
		Proxies:    make(map[string]*ProxyInfo),
	}

	// 扫描主机信息
	m.scanHost(env)

	// 扫描容器
	m.scanContainers(env)

	// 从 ServerOwl 配置提取数据库信息
	m.extractFromConfig(env)

	// 扫描代理配置
	m.scanProxies(env)

	// 保存到文件
	m.env = env
	return m.save()
}

// Get 获取环境信息
func (m *EnvironmentManager) Get() *Environment {
	return m.env
}

// GetLocalNodeID 获取本地节点 ID
func (m *EnvironmentManager) GetLocalNodeID() string {
	if m.localNodeID == "" {
		return "local"
	}
	return m.localNodeID
}

// GetAllNodes 获取所有节点的环境信息
func (m *EnvironmentManager) GetAllNodes() map[string]*Environment {
	result := make(map[string]*Environment)

	// 添加本地节点
	if m.env != nil {
		nodeID := m.localNodeID
		if nodeID == "" {
			nodeID = "local"
		}
		result[nodeID] = m.env
	}

	// 从 MasterCluster 获取其他节点的环境信息
	if m.masterCluster != nil {
		envs := m.masterCluster.GetAllEnvironments()
		for agentID, agentEnv := range envs {
			// 转换 cluster.AgentEnvironment 到 Environment
			env := m.convertFromClusterEnv(agentEnv)
			result[agentID] = env
		}
	}

	return result
}

// convertFromClusterEnv 从 cluster.AgentEnvironment 转换
func (m *EnvironmentManager) convertFromClusterEnv(agentEnv *cluster.AgentEnvironment) *Environment {
	env := &Environment{
		UpdatedAt: agentEnv.UpdatedAt,
		Host: HostInfo{
			Hostname: agentEnv.Host.Hostname,
			IP:       agentEnv.Host.IP,
		},
		Containers: make(map[string]*ContainerInfo),
		Databases:  make(map[string]*DatabaseInfo),
		Proxies:    make(map[string]*ProxyInfo),
	}

	for name, c := range agentEnv.Containers {
		env.Containers[name] = &ContainerInfo{
			Image:   c.Image,
			Status:  c.Status,
			Env:     c.Env,
			Ports:   c.Ports,
			Network: c.Network,
		}
	}

	for name, db := range agentEnv.Databases {
		env.Databases[name] = &DatabaseInfo{
			Type:     db.Type,
			Host:     db.Host,
			Port:     db.Port,
			User:     db.User,
			Password: db.Password,
			Database: db.Database,
			Source:   db.Source,
		}
	}

	for name, p := range agentEnv.Proxies {
		proxy := &ProxyInfo{
			Type:       p.Type,
			ConfigFile: p.ConfigFile,
			Source:     p.Source,
		}
		for _, site := range p.Sites {
			proxy.Sites = append(proxy.Sites, SiteInfo{
				Domain:  site.Domain,
				Backend: site.Backend,
				Root:    site.Root,
			})
		}
		env.Proxies[name] = proxy
	}

	return env
}

// GetOverview 获取概览
func (m *EnvironmentManager) GetOverview() string {
	allNodes := m.GetAllNodes()
	if len(allNodes) == 0 {
		return "环境信息未加载"
	}

	var sb strings.Builder

	for nodeID, env := range allNodes {
		sb.WriteString(fmt.Sprintf("## 节点: %s\n", nodeID))
		sb.WriteString(fmt.Sprintf("主机: %s", env.Host.Hostname))
		if env.Host.IP != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", env.Host.IP))
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("更新时间: %s\n", env.UpdatedAt))

		if len(env.Containers) > 0 {
			sb.WriteString(fmt.Sprintf("容器: %d 个\n", len(env.Containers)))
		}
		if len(env.Databases) > 0 {
			sb.WriteString(fmt.Sprintf("数据库: %d 个\n", len(env.Databases)))
		}
		if len(env.Proxies) > 0 {
			sb.WriteString(fmt.Sprintf("代理: %d 个\n", len(env.Proxies)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetContainer 获取指定容器信息
func (m *EnvironmentManager) GetContainer(name string) string {
	allNodes := m.GetAllNodes()
	if len(allNodes) == 0 {
		return "环境信息未加载"
	}

	// 在所有节点中搜索
	for nodeID, env := range allNodes {
		for n, c := range env.Containers {
			if strings.EqualFold(n, name) || strings.Contains(strings.ToLower(n), strings.ToLower(name)) {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("节点: %s\n", nodeID))
				sb.WriteString(fmt.Sprintf("容器: %s\n", n))
				sb.WriteString(fmt.Sprintf("镜像: %s\n", c.Image))
				sb.WriteString(fmt.Sprintf("状态: %s\n", c.Status))

				if c.Network != "" {
					sb.WriteString(fmt.Sprintf("网络: %s\n", c.Network))
				}

				if len(c.Ports) > 0 {
					sb.WriteString(fmt.Sprintf("端口: %s\n", strings.Join(c.Ports, ", ")))
				}

				if len(c.Env) > 0 {
					sb.WriteString("\n环境变量:\n")
					for k, v := range c.Env {
						sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
					}
				}

				return sb.String()
			}
		}
	}

	return fmt.Sprintf("未找到容器: %s", name)
}

// GetDatabase 获取指定数据库信息
func (m *EnvironmentManager) GetDatabase(name string) string {
	allNodes := m.GetAllNodes()
	if len(allNodes) == 0 {
		return "环境信息未加载"
	}

	// 在所有节点中搜索
	for nodeID, env := range allNodes {
		for n, db := range env.Databases {
			if strings.EqualFold(n, name) || strings.Contains(strings.ToLower(n), strings.ToLower(name)) {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("节点: %s\n", nodeID))
				sb.WriteString(fmt.Sprintf("数据库: %s\n", n))
				sb.WriteString(fmt.Sprintf("类型: %s\n", db.Type))
				sb.WriteString(fmt.Sprintf("主机: %s\n", db.Host))
				sb.WriteString(fmt.Sprintf("端口: %s\n", db.Port))
				sb.WriteString(fmt.Sprintf("用户: %s\n", db.User))
				sb.WriteString(fmt.Sprintf("密码: %s\n", db.Password))
				if db.Database != "" {
					sb.WriteString(fmt.Sprintf("数据库名: %s\n", db.Database))
				}
				sb.WriteString(fmt.Sprintf("来源: %s\n", db.Source))
				return sb.String()
			}
		}
	}

	return fmt.Sprintf("未找到数据库: %s", name)
}

// GetByCategory 按类别获取
func (m *EnvironmentManager) GetByCategory(category string) string {
	allNodes := m.GetAllNodes()
	if len(allNodes) == 0 {
		return "环境信息未加载"
	}

	var sb strings.Builder

	switch strings.ToLower(category) {
	case "host":
		for nodeID, env := range allNodes {
			sb.WriteString(fmt.Sprintf("## 节点: %s\n", nodeID))
			sb.WriteString(fmt.Sprintf("主机名: %s\n", env.Host.Hostname))
			if env.Host.IP != "" {
				sb.WriteString(fmt.Sprintf("IP: %s\n", env.Host.IP))
			}
			sb.WriteString("\n")
		}
		return sb.String()

	case "containers":
		for nodeID, env := range allNodes {
			if len(env.Containers) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("## 节点: %s\n", nodeID))
			for name, c := range env.Containers {
				sb.WriteString(fmt.Sprintf("### %s\n", name))
				sb.WriteString(fmt.Sprintf("镜像: %s\n", c.Image))
				sb.WriteString(fmt.Sprintf("状态: %s\n", c.Status))
				if len(c.Env) > 0 {
					sb.WriteString("环境变量:\n")
					for k, v := range c.Env {
						sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
					}
				}
				sb.WriteString("\n")
			}
		}
		return sb.String()

	case "databases":
		for nodeID, env := range allNodes {
			if len(env.Databases) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("## 节点: %s\n", nodeID))
			for name, db := range env.Databases {
				sb.WriteString(fmt.Sprintf("### %s\n", name))
				sb.WriteString(fmt.Sprintf("类型: %s\n", db.Type))
				sb.WriteString(fmt.Sprintf("连接: %s:%s\n", db.Host, db.Port))
				sb.WriteString(fmt.Sprintf("用户: %s\n", db.User))
				sb.WriteString(fmt.Sprintf("密码: %s\n", db.Password))
				sb.WriteString("\n")
			}
		}
		return sb.String()

	case "proxies":
		for nodeID, env := range allNodes {
			if len(env.Proxies) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("## 节点: %s\n", nodeID))
			for name, p := range env.Proxies {
				sb.WriteString(fmt.Sprintf("### %s\n", name))
				sb.WriteString(fmt.Sprintf("类型: %s\n", p.Type))
				sb.WriteString(fmt.Sprintf("配置文件: %s\n", p.ConfigFile))
				if len(p.Sites) > 0 {
					sb.WriteString("站点:\n")
					for _, site := range p.Sites {
						if site.Backend != "" {
							sb.WriteString(fmt.Sprintf("  - %s -> %s\n", site.Domain, site.Backend))
						} else if site.Root != "" {
							sb.WriteString(fmt.Sprintf("  - %s (静态: %s)\n", site.Domain, site.Root))
						}
					}
				}
				sb.WriteString("\n")
			}
		}
		return sb.String()

	default:
		return m.GetOverview()
	}
}

func (m *EnvironmentManager) save() error {
	data, err := yaml.Marshal(m.env)
	if err != nil {
		return err
	}

	header := "# ServerOwl 环境信息（自动生成，可手动编辑）\n# 刷新命令: curl -X POST http://localhost:19528/api/v1/environment/refresh\n\n"
	return os.WriteFile(m.filePath, []byte(header+string(data)), 0644)
}

// SaveRaw 保存原始 YAML 内容
func (m *EnvironmentManager) SaveRaw(content string) error {
	// 验证 YAML 格式
	var env Environment
	if err := yaml.Unmarshal([]byte(content), &env); err != nil {
		return fmt.Errorf("YAML 格式错误: %w", err)
	}

	// 保存到文件
	if err := os.WriteFile(m.filePath, []byte(content), 0644); err != nil {
		return err
	}

	// 更新内存中的环境信息
	m.env = &env
	return nil
}

func (m *EnvironmentManager) scanHost(env *Environment) {
	// 主机名
	if hostname, err := os.Hostname(); err == nil {
		env.Host.Hostname = hostname
	}

	// IP（尝试获取主要 IP）
	cmd := exec.Command("hostname", "-I")
	if output, err := cmd.Output(); err == nil {
		ips := strings.Fields(string(output))
		if len(ips) > 0 {
			env.Host.IP = ips[0]
		}
	}
}

func (m *EnvironmentManager) scanContainers(env *Environment) {
	// 获取所有容器
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	names := strings.Fields(string(output))
	for _, name := range names {
		info := m.inspectContainer(name)
		if info != nil {
			env.Containers[name] = info

			// 从环境变量提取数据库信息
			m.extractDatabaseFromEnv(env, name, info)
		}
	}
}

func (m *EnvironmentManager) inspectContainer(name string) *ContainerInfo {
	format := `{{.Config.Image}}|{{.State.Status}}|{{range .Config.Env}}{{.}}||{{end}}|{{range $net, $cfg := .NetworkSettings.Networks}}{{$net}}{{end}}`
	cmd := exec.Command("docker", "inspect", "--format", format, name)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) < 2 {
		return nil
	}

	info := &ContainerInfo{
		Image:  parts[0],
		Status: parts[1],
		Env:    make(map[string]string),
	}

	// 解析环境变量
	if len(parts) > 2 {
		envStr := parts[2]
		envPairs := strings.Split(envStr, "||")
		for _, pair := range envPairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			if idx := strings.Index(pair, "="); idx > 0 {
				key := pair[:idx]
				value := pair[idx+1:]
				// 只保留有意义的环境变量
				if m.isRelevantEnv(key) {
					info.Env[key] = value
				}
			}
		}
	}

	// 网络
	if len(parts) > 3 && parts[len(parts)-1] != "" {
		info.Network = parts[len(parts)-1]
	}

	return info
}

func (m *EnvironmentManager) isRelevantEnv(key string) bool {
	// 保留数据库相关、密码相关、端口相关的环境变量
	relevantPrefixes := []string{
		"POSTGRES", "MYSQL", "REDIS", "MONGO", "DB_",
		"DATABASE", "PASSWORD", "USER", "HOST", "PORT",
		"SECRET", "KEY", "TOKEN", "DSN", "URL",
	}
	keyUpper := strings.ToUpper(key)
	for _, prefix := range relevantPrefixes {
		if strings.Contains(keyUpper, prefix) {
			return true
		}
	}
	return false
}

func (m *EnvironmentManager) extractDatabaseFromEnv(env *Environment, containerName string, info *ContainerInfo) {
	// PostgreSQL
	if user, ok := info.Env["POSTGRES_USER"]; ok {
		dbInfo := &DatabaseInfo{
			Type:     "postgres",
			Host:     containerName,
			Port:     "5432",
			User:     user,
			Password: info.Env["POSTGRES_PASSWORD"],
			Database: info.Env["POSTGRES_DB"],
			Source:   fmt.Sprintf("container:%s env", containerName),
		}
		env.Databases[containerName+"-postgres"] = dbInfo
	}

	// MySQL
	if user, ok := info.Env["MYSQL_USER"]; ok {
		dbInfo := &DatabaseInfo{
			Type:     "mysql",
			Host:     containerName,
			Port:     "3306",
			User:     user,
			Password: info.Env["MYSQL_PASSWORD"],
			Database: info.Env["MYSQL_DATABASE"],
			Source:   fmt.Sprintf("container:%s env", containerName),
		}
		env.Databases[containerName+"-mysql"] = dbInfo
	}

	// Redis
	if pass, ok := info.Env["REDIS_PASSWORD"]; ok {
		dbInfo := &DatabaseInfo{
			Type:     "redis",
			Host:     containerName,
			Port:     "6379",
			Password: pass,
			Source:   fmt.Sprintf("container:%s env", containerName),
		}
		env.Databases[containerName+"-redis"] = dbInfo
	}
}

func (m *EnvironmentManager) extractFromConfig(env *Environment) {
	if m.configPath == "" {
		return
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return
	}

	content := string(data)

	// 提取 PostgreSQL DSN
	// 格式: postgresql://user:pass@host:port/dbname
	pgDSNRegex := regexp.MustCompile(`postgresql://([^:]+):([^@]+)@([^:]+):(\d+)/([^\s?]+)`)
	matches := pgDSNRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 6 {
			dbName := match[5]
			// 避免重复
			key := "config-" + dbName
			if _, exists := env.Databases[key]; !exists {
				env.Databases[key] = &DatabaseInfo{
					Type:     "postgres",
					Host:     match[3],
					Port:     match[4],
					User:     match[1],
					Password: match[2],
					Database: dbName,
					Source:   "config:" + m.configPath,
				}
			}
		}
	}

	// 提取 Redis
	// 格式: addr: host:port 或 redis://...
	redisAddrRegex := regexp.MustCompile(`addr:\s*([^:\s]+):(\d+)`)
	if match := redisAddrRegex.FindStringSubmatch(content); len(match) >= 3 {
		redisPassRegex := regexp.MustCompile(`password:\s*(\S+)`)
		pass := ""
		if passMatch := redisPassRegex.FindStringSubmatch(content); len(passMatch) >= 2 {
			pass = passMatch[1]
		}
		env.Databases["config-redis"] = &DatabaseInfo{
			Type:     "redis",
			Host:     match[1],
			Port:     match[2],
			Password: pass,
			Source:   "config:" + m.configPath,
		}
	}
}

// scanProxies 扫描代理配置（Caddy、Nginx）
func (m *EnvironmentManager) scanProxies(env *Environment) {
	// 扫描 Caddy
	caddyPaths := []string{
		"/etc/caddy/Caddyfile",
		"/opt/caddy/Caddyfile",
	}
	for _, path := range caddyPaths {
		if data, err := os.ReadFile(path); err == nil {
			proxy := m.parseCaddyfile(string(data), path)
			if proxy != nil && len(proxy.Sites) > 0 {
				env.Proxies["caddy"] = proxy
			}
			break
		}
	}

	// 扫描 Nginx
	nginxPaths := []string{
		"/etc/nginx/nginx.conf",
		"/etc/nginx/sites-enabled",
	}
	for _, path := range nginxPaths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			// 扫描目录下的配置文件
			entries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			var allSites []SiteInfo
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				filePath := path + "/" + entry.Name()
				if data, err := os.ReadFile(filePath); err == nil {
					sites := m.parseNginxConfig(string(data))
					allSites = append(allSites, sites...)
				}
			}
			if len(allSites) > 0 {
				env.Proxies["nginx"] = &ProxyInfo{
					Type:       "nginx",
					ConfigFile: path,
					Sites:      allSites,
					Source:     "file:" + path,
				}
			}
		} else {
			// 单个配置文件
			if data, err := os.ReadFile(path); err == nil {
				sites := m.parseNginxConfig(string(data))
				if len(sites) > 0 {
					env.Proxies["nginx"] = &ProxyInfo{
						Type:       "nginx",
						ConfigFile: path,
						Sites:      sites,
						Source:     "file:" + path,
					}
				}
			}
		}
		break
	}
}

// parseCaddyfile 解析 Caddyfile
func (m *EnvironmentManager) parseCaddyfile(content, path string) *ProxyInfo {
	proxy := &ProxyInfo{
		Type:       "caddy",
		ConfigFile: path,
		Sites:      []SiteInfo{},
		Source:     "file:" + path,
	}

	lines := strings.Split(content, "\n")
	var currentDomain string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 域名行（不以特殊字符开头，可能包含端口）
		if !strings.HasPrefix(line, "{") && !strings.HasPrefix(line, "}") &&
			!strings.HasPrefix(line, "reverse_proxy") && !strings.HasPrefix(line, "root") &&
			!strings.HasPrefix(line, "file_server") && !strings.HasPrefix(line, "@") &&
			!strings.HasPrefix(line, "header") && !strings.HasPrefix(line, "encode") &&
			!strings.HasPrefix(line, "tls") && !strings.HasPrefix(line, "log") {
			// 可能是域名
			parts := strings.Fields(line)
			if len(parts) > 0 && (strings.Contains(parts[0], ".") || strings.HasPrefix(parts[0], ":")) {
				currentDomain = parts[0]
			}
		}

		// reverse_proxy 行
		if strings.HasPrefix(line, "reverse_proxy") && currentDomain != "" {
			parts := strings.Fields(line)
			backend := ""
			for _, p := range parts[1:] {
				if !strings.HasPrefix(p, "@") && !strings.HasPrefix(p, "{") {
					backend = p
					break
				}
			}
			if backend != "" {
				proxy.Sites = append(proxy.Sites, SiteInfo{
					Domain:  currentDomain,
					Backend: backend,
				})
			}
		}

		// root 行（静态文件）
		if strings.HasPrefix(line, "root") && currentDomain != "" {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				proxy.Sites = append(proxy.Sites, SiteInfo{
					Domain: currentDomain,
					Root:   parts[2],
				})
			}
		}
	}

	return proxy
}

// parseNginxConfig 解析 Nginx 配置
func (m *EnvironmentManager) parseNginxConfig(content string) []SiteInfo {
	var sites []SiteInfo

	// 简单解析 server_name 和 proxy_pass / root
	serverNameRegex := regexp.MustCompile(`server_name\s+([^;]+);`)
	proxyPassRegex := regexp.MustCompile(`proxy_pass\s+([^;]+);`)
	rootRegex := regexp.MustCompile(`root\s+([^;]+);`)

	// 按 server { } 块分割（简化处理）
	serverBlocks := regexp.MustCompile(`server\s*\{`).Split(content, -1)

	for _, block := range serverBlocks[1:] { // 跳过第一个空块
		// 找到对应的 }
		depth := 1
		endIdx := 0
		for i, ch := range block {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					endIdx = i
					break
				}
			}
		}
		if endIdx > 0 {
			block = block[:endIdx]
		}

		// 提取 server_name
		var domain string
		if match := serverNameRegex.FindStringSubmatch(block); len(match) >= 2 {
			names := strings.Fields(match[1])
			if len(names) > 0 && names[0] != "_" {
				domain = names[0]
			}
		}

		if domain == "" {
			continue
		}

		site := SiteInfo{Domain: domain}

		// 提取 proxy_pass
		if match := proxyPassRegex.FindStringSubmatch(block); len(match) >= 2 {
			site.Backend = strings.TrimSpace(match[1])
		}

		// 提取 root
		if match := rootRegex.FindStringSubmatch(block); len(match) >= 2 {
			site.Root = strings.TrimSpace(match[1])
		}

		if site.Backend != "" || site.Root != "" {
			sites = append(sites, site)
		}
	}

	return sites
}

// CreateEnvironmentTool 创建环境查询工具
func CreateEnvironmentTool(envManager *EnvironmentManager) *Tool {
	return &Tool{
		Name:        "get_environment",
		Description: "查询服务器环境信息（容器配置、数据库密码、代理站点等）。优先使用此工具获取已知信息，避免重复执行命令。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"category": map[string]any{
					"type":        "string",
					"enum":        []string{"all", "host", "containers", "databases", "proxies"},
					"description": "查询类别：all=概览，host=主机信息，containers=所有容器，databases=所有数据库，proxies=代理配置",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "具体名称（容器名或数据库名），用于查询单个对象的详细信息",
				},
			},
		},
		Handler: func(args map[string]any) (string, error) {
			if envManager == nil || envManager.env == nil {
				return "环境信息未加载", nil
			}

			name, _ := args["name"].(string)
			category, _ := args["category"].(string)

			// 如果指定了名称，优先按名称查询
			if name != "" {
				// 先尝试容器
				result := envManager.GetContainer(name)
				if !strings.HasPrefix(result, "未找到") {
					return result, nil
				}
				// 再尝试数据库
				result = envManager.GetDatabase(name)
				if !strings.HasPrefix(result, "未找到") {
					return result, nil
				}
				return fmt.Sprintf("未找到名为 %s 的容器或数据库", name), nil
			}

			// 按类别查询
			if category != "" && category != "all" {
				return envManager.GetByCategory(category), nil
			}

			// 默认返回概览
			return envManager.GetOverview(), nil
		},
		NeedConfirm: false,
	}
}
