# ServerOwl 部署指南

## 架构概述

ServerOwl 是一个智能服务器监控和管理系统，包含以下核心组件：

- **Master (serverowl)** - 主控节点，提供 Web UI、AI Agent、API 服务
- **Agent (owl-agent)** - 轻量级代理，部署在被监控服务器上

### 单主模式

```
┌─────────────────────────────────────┐
│           Master 节点                │
│  ┌─────────────────────────────────┐ │
│  │  serverowl                      │ │
│  │  - HTTP API + Web UI (:19528)   │ │
│  │  - WebSocket (:19527)           │ │
│  │  - AI Agent (LLM 集成)          │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
                  │
                  │ WebSocket
                  ▼
┌─────────┐  ┌─────────┐  ┌─────────┐
│ 服务器A  │  │ 服务器B  │  │ 服务器C  │
│owl-agent│  │owl-agent│  │owl-agent│
│(systemd)│  │(systemd)│  │(systemd)│
└─────────┘  └─────────┘  └─────────┘
```

### 双主高可用模式

```
┌─────────────────┐         ┌─────────────────┐
│   Master A      │◄───────►│   Master B      │
│  192.168.1.10   │  同步    │  192.168.1.11   │
│  (主节点)        │ (5秒)   │  (备用节点)      │
└────────┬────────┘         └────────┬────────┘
         │                           │
         └───────────┬───────────────┘
                     │
         ┌───────────┼───────────┐
         ▼           ▼           ▼
    ┌─────────┐ ┌─────────┐ ┌─────────┐
    │ Agent 1 │ │ Agent 2 │ │ Agent N │
    └─────────┘ └─────────┘ └─────────┘
```

**重要**：Agent 必须直接部署在宿主机上，不能使用 Docker，否则只能监控容器内部资源。

---

## 端口说明

| 端口 | 用途 |
|------|------|
| 19527 | WebSocket (Agent 连接) + Master 同步 |
| 19528 | Web UI + HTTP API |

---

## 1. 部署 Master

### 方式一：直接部署（推荐）

```bash
# 1. 编译
cd /path/to/Owl
go build -o serverowl ./cmd/serverowl

# 2. 构建前端
cd web-react
npm install
npm run build
cd ..

# 3. 创建目录
sudo mkdir -p /opt/serverowl/{configs,web,skills}

# 4. 复制文件
sudo cp serverowl /opt/serverowl/
sudo cp -r web-react/dist/* /opt/serverowl/web/
sudo cp configs/config.yaml /opt/serverowl/configs/
sudo cp configs/tool_whitelist.yaml /opt/serverowl/configs/
sudo cp -r skills/* /opt/serverowl/skills/ 2>/dev/null || true

# 5. 编辑配置
sudo vim /opt/serverowl/configs/config.yaml

# 6. 创建 systemd 服务
sudo tee /etc/systemd/system/serverowl.service > /dev/null <<EOF
[Unit]
Description=ServerOwl
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/serverowl
ExecStart=/opt/serverowl/serverowl -config /opt/serverowl/configs/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 7. 启动服务
sudo systemctl daemon-reload
sudo systemctl enable serverowl
sudo systemctl start serverowl
```

### 方式二：Docker Compose

```bash
cd deploy/docker
vim config/config.yaml  # 编辑配置
docker-compose up -d --build
```

---

## 2. Master 配置说明

完整配置示例 (`config.yaml`)：

```yaml
# 基础服务端口（WebSocket + Agent API）
server:
  port: 19527

# Web 管理面板
web:
  enabled: true
  port: 19528
  static_dir: "/opt/serverowl/web"
  auth:
    enabled: true
    username: "admin"
    password: "your-secure-password"
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"

# 企业微信通知（可选）
wechat:
  corp_id: ""
  agent_id: 0
  secret: ""
  token: ""
  encoding_aes_key: ""
  default_user: "@all"

# LLM 配置（AI Agent 功能）
llm:
  provider: "siliconflow"
  api_key: "your-api-key"
  base_url: "https://api.siliconflow.cn/v1"
  model: "deepseek-ai/DeepSeek-V3"
  timeout: 120s
  retries: 1

# 记忆系统（需要 PostgreSQL）
memory:
  postgres:
    dsn: "postgresql://user:pass@localhost:5432/serverowl?sslmode=disable"
  short_term:
    max_turns: 10
    max_age: "30m"
  embedding:
    api_key: "your-api-key"
    base_url: "https://api.siliconflow.cn/v1"
    model: "BAAI/bge-large-zh-v1.5"

# Agent 工具配置
agent:
  max_iterations: 10
  timeout: "5m"
  whitelist_path: "/opt/serverowl/configs/tool_whitelist.yaml"
  log_retention_days: 30
  log_cleanup_schedule: "@daily"

# 技能系统
skills:
  enabled: true
  dirs:
    - "/opt/serverowl/skills"
  auto_load: true

# 多主集群配置
master_cluster:
  enabled: true
  id: "master-01"
  name: "主节点"
  addr: "0.0.0.0:19527"
  sync_interval: "5s"
  peers: []
  # 双主模式添加对等节点：
  # peers:
  #   - id: "master-02"
  #     name: "备用节点"
  #     addr: "192.168.1.11:19527"

# 日志配置
logging:
  level: "info"
  format: "console"
  sanitize: true
```

---

## 3. 部署 Agent

### Linux 手动安装

```bash
# 1. 编译 Agent
go build -o owl-agent ./cmd/owl-agent

# 2. 复制到目标服务器
scp owl-agent root@target-server:/usr/local/bin/

# 3. 在目标服务器上创建配置
ssh root@target-server

mkdir -p /etc/owl-agent /var/lib/owl-agent
cat > /etc/owl-agent/agent.yaml <<EOF
agent:
  id: ""  # 留空自动生成
  name: "server-01"
  tags:
    - "production"
  labels:
    env: "prod"
  heartbeat_interval: 10s
  reconnect_interval: 5s
  command_timeout: 5m
  data_dir: "/var/lib/owl-agent"

masters:
  - id: "master-01"
    name: "主节点"
    addr: "your-master-ip:19527"
    priority: 1
    is_active: true

metrics:
  enabled: true
  collect_interval: 15s
  report_interval: 30s
  include_disk: true

rules:
  - id: "high-cpu-alert"
    name: "CPU 使用率过高"
    enabled: true
    condition:
      type: "metric"
      metric: "cpu_usage"
      operator: ">"
      threshold: 90
    actions:
      - type: "notify"
        command: "CPU 使用率超过 90%"
    cooldown: 5m

  - id: "high-memory-alert"
    name: "内存使用率过高"
    enabled: true
    condition:
      type: "metric"
      metric: "memory_usage"
      operator: ">"
      threshold: 85
    actions:
      - type: "notify"
        command: "内存使用率超过 85%"
    cooldown: 5m

  - id: "disk-space-alert"
    name: "磁盘空间不足"
    enabled: true
    condition:
      type: "metric"
      metric: "disk_usage"
      operator: ">"
      threshold: 90
    actions:
      - type: "shell"
        command: "find /var/log -name '*.log' -mtime +7 -delete"
        timeout: 60s
      - type: "notify"
        command: "磁盘使用率超过 90%，已清理 7 天前的日志"
    cooldown: 1h
EOF

# 4. 创建 systemd 服务
cat > /etc/systemd/system/owl-agent.service <<EOF
[Unit]
Description=Owl Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/owl-agent -config /etc/owl-agent/agent.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 5. 启动服务
systemctl daemon-reload
systemctl enable owl-agent
systemctl start owl-agent
```

### Windows 安装

```powershell
# 1. 复制 owl-agent.exe 到 C:\Program Files\OwlAgent\
# 2. 创建配置文件 C:\Program Files\OwlAgent\agent.yaml
# 3. 安装为 Windows 服务
sc create OwlAgent binPath= "C:\Program Files\OwlAgent\owl-agent.exe -config C:\Program Files\OwlAgent\agent.yaml" start= auto
sc start OwlAgent
```

---

## 4. 常用命令

### Master

```bash
# 启动/停止/重启
sudo systemctl start serverowl
sudo systemctl stop serverowl
sudo systemctl restart serverowl

# 查看状态
sudo systemctl status serverowl

# 查看日志
journalctl -u serverowl -f

# Docker 模式
docker-compose up -d
docker-compose down
docker-compose logs -f
```

### Agent

```bash
# Linux
sudo systemctl start owl-agent
sudo systemctl stop owl-agent
sudo systemctl restart owl-agent
sudo systemctl status owl-agent
journalctl -u owl-agent -f

# Windows
Start-Service OwlAgent
Stop-Service OwlAgent
Restart-Service OwlAgent
Get-Service OwlAgent
```

### 前端开发

```bash
cd web-react
npm run dev      # 开发模式 (热更新)
npm run build    # 生产构建
npm run preview  # 预览构建结果
```

---

## 5. 功能说明

### AI Agent

通过企业微信或 Web UI 与 AI Agent 对话，支持：
- 查询服务器状态、容器信息
- 执行 Docker 命令
- 分析日志
- 远程执行 Shell 命令
- 调用自定义技能

### 环境感知

Agent 会自动扫描服务器环境：
- Docker 容器及其配置
- 数据库连接信息
- Caddy/Nginx 代理配置

在设置页面可以查看和编辑环境配置。

### 自治规则

Agent 支持本地自治规则，无需 Master 即可执行：
- CPU/内存/磁盘告警
- 服务自动重启
- 自动清理脚本

---

## 6. 防火墙配置

### Master

```bash
# 开放端口
sudo firewall-cmd --permanent --add-port=19527/tcp
sudo firewall-cmd --permanent --add-port=19528/tcp
sudo firewall-cmd --reload

# 或使用 ufw
sudo ufw allow 19527/tcp
sudo ufw allow 19528/tcp
```

### Agent

Agent 只需要出站连接到 Master，通常不需要额外配置。

---

## 7. 故障排查

### Agent 无法连接 Master

1. 检查网络连通性：
   ```bash
   curl http://master-ip:19528/health
   ```
2. 检查防火墙规则
3. 检查 Master 日志：`journalctl -u serverowl -f`
4. 检查 Agent 日志：`journalctl -u owl-agent -f`

### Agent 显示离线

1. 检查 Agent 服务状态：`systemctl status owl-agent`
2. 检查 WebSocket 连接是否正常
3. 检查心跳超时配置

### Web UI 无法访问

1. 检查服务是否运行：`systemctl status serverowl`
2. 检查端口是否监听：`netstat -tlnp | grep 19528`
3. 检查静态文件目录配置

### AI Agent 无响应

1. 检查 LLM API 配置是否正确
2. 检查 API Key 是否有效
3. 查看日志中的错误信息

---

## 8. 更新部署

```bash
# 1. 编译新版本
go build -o serverowl ./cmd/serverowl
cd web-react && npm run build && cd ..

# 2. 上传到服务器
scp serverowl root@server:/opt/serverowl/serverowl.new
scp -r web-react/dist/* root@server:/opt/serverowl/web/

# 3. 在服务器上更新
ssh root@server
cd /opt/serverowl
systemctl stop serverowl
mv serverowl serverowl.bak
mv serverowl.new serverowl
chmod +x serverowl
systemctl start serverowl
```
