package agentnode

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"serverowl/internal/protocol"
)

// EnvironmentScanner 环境扫描器
type EnvironmentScanner struct {
	configPath string // Agent 配置文件路径
}

// NewEnvironmentScanner 创建环境扫描器
func NewEnvironmentScanner(configPath string) *EnvironmentScanner {
	return &EnvironmentScanner{
		configPath: configPath,
	}
}

// Scan 扫描环境信息
func (s *EnvironmentScanner) Scan() *protocol.EnvironmentPayload {
	env := &protocol.EnvironmentPayload{
		UpdatedAt:  time.Now().Format("2006-01-02 15:04:05"),
		Containers: make(map[string]*protocol.ContainerInfo),
		Databases:  make(map[string]*protocol.DatabaseInfo),
		Proxies:    make(map[string]*protocol.ProxyInfo),
	}

	// 扫描主机信息
	s.scanHost(env)

	// 扫描容器
	s.scanContainers(env)

	// 扫描代理配置
	s.scanProxies(env)

	return env
}

func (s *EnvironmentScanner) scanHost(env *protocol.EnvironmentPayload) {
	// 主机名
	if hostname, err := os.Hostname(); err == nil {
		env.Host.Hostname = hostname
	}

	// IP
	cmd := exec.Command("hostname", "-I")
	if output, err := cmd.Output(); err == nil {
		ips := strings.Fields(string(output))
		if len(ips) > 0 {
			env.Host.IP = ips[0]
		}
	}
}

func (s *EnvironmentScanner) scanContainers(env *protocol.EnvironmentPayload) {
	// 获取所有容器
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	names := strings.Fields(string(output))
	for _, name := range names {
		info := s.inspectContainer(name)
		if info != nil {
			env.Containers[name] = info
			// 从环境变量提取数据库信息
			s.extractDatabaseFromEnv(env, name, info)
		}
	}
}

func (s *EnvironmentScanner) inspectContainer(name string) *protocol.ContainerInfo {
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

	info := &protocol.ContainerInfo{
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
				if s.isRelevantEnv(key) {
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

func (s *EnvironmentScanner) isRelevantEnv(key string) bool {
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

func (s *EnvironmentScanner) extractDatabaseFromEnv(env *protocol.EnvironmentPayload, containerName string, info *protocol.ContainerInfo) {
	// PostgreSQL
	if user, ok := info.Env["POSTGRES_USER"]; ok {
		dbInfo := &protocol.DatabaseInfo{
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
		dbInfo := &protocol.DatabaseInfo{
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
		dbInfo := &protocol.DatabaseInfo{
			Type:     "redis",
			Host:     containerName,
			Port:     "6379",
			Password: pass,
			Source:   fmt.Sprintf("container:%s env", containerName),
		}
		env.Databases[containerName+"-redis"] = dbInfo
	}
}

func (s *EnvironmentScanner) scanProxies(env *protocol.EnvironmentPayload) {
	// 扫描 Caddy
	caddyPaths := []string{
		"/etc/caddy/Caddyfile",
		"/opt/caddy/Caddyfile",
	}
	for _, path := range caddyPaths {
		if data, err := os.ReadFile(path); err == nil {
			proxy := s.parseCaddyfile(string(data), path)
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
			entries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			var allSites []protocol.SiteInfo
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				filePath := path + "/" + entry.Name()
				if data, err := os.ReadFile(filePath); err == nil {
					sites := s.parseNginxConfig(string(data))
					allSites = append(allSites, sites...)
				}
			}
			if len(allSites) > 0 {
				env.Proxies["nginx"] = &protocol.ProxyInfo{
					Type:       "nginx",
					ConfigFile: path,
					Sites:      allSites,
					Source:     "file:" + path,
				}
			}
		} else {
			if data, err := os.ReadFile(path); err == nil {
				sites := s.parseNginxConfig(string(data))
				if len(sites) > 0 {
					env.Proxies["nginx"] = &protocol.ProxyInfo{
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

func (s *EnvironmentScanner) parseCaddyfile(content, path string) *protocol.ProxyInfo {
	proxy := &protocol.ProxyInfo{
		Type:       "caddy",
		ConfigFile: path,
		Sites:      []protocol.SiteInfo{},
		Source:     "file:" + path,
	}

	lines := strings.Split(content, "\n")
	var currentDomain string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !strings.HasPrefix(line, "{") && !strings.HasPrefix(line, "}") &&
			!strings.HasPrefix(line, "reverse_proxy") && !strings.HasPrefix(line, "root") &&
			!strings.HasPrefix(line, "file_server") && !strings.HasPrefix(line, "@") &&
			!strings.HasPrefix(line, "header") && !strings.HasPrefix(line, "encode") &&
			!strings.HasPrefix(line, "tls") && !strings.HasPrefix(line, "log") &&
			!strings.HasPrefix(line, "transport") {
			parts := strings.Fields(line)
			if len(parts) > 0 && (strings.Contains(parts[0], ".") || strings.HasPrefix(parts[0], ":")) {
				currentDomain = parts[0]
			}
		}

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
				proxy.Sites = append(proxy.Sites, protocol.SiteInfo{
					Domain:  currentDomain,
					Backend: backend,
				})
			}
		}

		if strings.HasPrefix(line, "root") && currentDomain != "" {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				proxy.Sites = append(proxy.Sites, protocol.SiteInfo{
					Domain: currentDomain,
					Root:   parts[2],
				})
			}
		}
	}

	return proxy
}

func (s *EnvironmentScanner) parseNginxConfig(content string) []protocol.SiteInfo {
	var sites []protocol.SiteInfo

	serverNameRegex := regexp.MustCompile(`server_name\s+([^;]+);`)
	proxyPassRegex := regexp.MustCompile(`proxy_pass\s+([^;]+);`)
	rootRegex := regexp.MustCompile(`root\s+([^;]+);`)

	serverBlocks := regexp.MustCompile(`server\s*\{`).Split(content, -1)

	for _, block := range serverBlocks[1:] {
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

		site := protocol.SiteInfo{Domain: domain}

		if match := proxyPassRegex.FindStringSubmatch(block); len(match) >= 2 {
			site.Backend = strings.TrimSpace(match[1])
		}

		if match := rootRegex.FindStringSubmatch(block); len(match) >= 2 {
			site.Root = strings.TrimSpace(match[1])
		}

		if site.Backend != "" || site.Root != "" {
			sites = append(sites, site)
		}
	}

	return sites
}
