# Owl 部署指南

## 架构概述

### 单主模式

```
┌─────────────────────────────────────┐
│           Master 节点                │
│  ┌─────────────────────────────┐   │
│  │  Docker 容器                 │   │
│  │  - HTTP API (:19528)        │   │
│  │  - WebSocket (:19527)       │   │
│  │  - React 前端 (内嵌)         │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
                  │
                  │ WebSocket / HTTP
                  ▼
┌─────────┐  ┌─────────┐  ┌─────────┐
│ 服务器A  │  │ 服务器B  │  │ 服务器C  │
│ Agent   │  │ Agent   │  │ Agent   │
│(systemd)│  │(systemd)│  │(systemd)│
└─────────┘  └─────────┘  └─────────┘
```

### 双主高可用模式（推荐）

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

**特点**：
- 两个 Master 互为备份，自动同步 Agent 状态
- Agent 可连接任一 Master，支持故障转移
- 5 秒同步间隔，最终一致性

**重要**：Agent 必须直接部署在宿主机上，不能使用 Docker，否则只能监控容器内部资源。

---

## 1. 部署 Master

### 单主部署

#### 方式一：Docker Compose（推荐）

```bash
cd deploy/docker

# 编辑配置
vim config/master.yaml

# 构建并启动（包含前端）
docker-compose up -d --build

# 查看日志
docker-compose logs -f
```

#### 方式二：直接部署

```bash
# 1. 构建前端
cd web-react
npm install
npm run build

# 2. 编译后端
cd ..
go build -o owl-master ./cmd/serverowl

# 3. 复制文件
sudo cp owl-master /usr/local/bin/
sudo mkdir -p /etc/owl /var/lib/owl/static
sudo cp deploy/docker/config/master.yaml /etc/owl/

# 4. 复制前端构建产物
sudo cp -r web-react/dist/* /var/lib/owl/static/

# 5. 安装服务
sudo cp deploy/systemd/owl-master.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable owl-master
sudo systemctl start owl-master
```

部署完成后访问 `http://your-master:19528` 即可打开 Web UI。

### 双主高可用部署

两台服务器分别部署 Master，互相配置为对等节点。

**服务器 A (192.168.1.10) 配置** - `config-master-a.yaml`：

```yaml
server:
  port: 19527

web:
  enabled: true
  port: 19528

# 多主集群配置
master_cluster:
  enabled: true
  id: "master-01"
  name: "主节点-A"
  addr: "0.0.0.0:19527"
  sync_interval: "5s"
  peers:
    - id: "master-02"
      name: "主节点-B"
      addr: "192.168.1.11:19527"
```

**服务器 B (192.168.1.11) 配置** - `config-master-b.yaml`：

```yaml
server:
  port: 19527

web:
  enabled: true
  port: 19528

# 多主集群配置
master_cluster:
  enabled: true
  id: "master-02"
  name: "主节点-B"
  addr: "0.0.0.0:19527"
  sync_interval: "5s"
  peers:
    - id: "master-01"
      name: "主节点-A"
      addr: "192.168.1.10:19527"
```

**部署步骤**：

```bash
# 服务器 A
./owl-master -config /etc/owl/config-master-a.yaml

# 服务器 B
./owl-master -config /etc/owl/config-master-b.yaml
```

**端口说明**：
| 端口 | 用途 |
|------|------|
| 19527 | WebSocket + Master 同步 |
| 19528 | Web UI + HTTP API |

---

## 2. 前端开发

### 本地开发模式

前后端分离开发，支持热更新：

```bash
# 终端 1: 启动后端
go run ./cmd/serverowl

# 终端 2: 启动前端开发服务器
cd web-react
npm install
npm run dev
```

访问 `http://localhost:3000`，API 请求会自动代理到后端 `:8080`。

### Docker 开发模式

```bash
cd deploy/docker
docker-compose -f docker-compose.yaml -f docker-compose.dev.yaml up
```

---

## 3. 部署 Agent

### Linux 一键安装

```bash
# 单主模式
curl -sSL http://master:19528/install.sh | bash -s -- -m http://master:19527

# 双主模式（指定主 Master，Agent 会自动发现备用节点）
curl -sSL http://192.168.1.10:19528/install.sh | bash -s -- -m http://192.168.1.10:19527

# 指定名称和标签
curl -sSL http://master:19528/install.sh | bash -s -- \
  -m http://master:19527 \
  -n web-server-01 \
  -t production,web
```

**参数说明**：
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-m, --master` | Master 地址（必填） | - |
| `-n, --name` | Agent 名称 | 主机名 |
| `-t, --tags` | 标签，逗号分隔 | - |
| `-v, --version` | 安装版本 | latest |

### Windows 一键安装

```powershell
# PowerShell (管理员)
.\install-agent.ps1 -MasterUrl "http://master:19527"

# 指定名称和标签
.\install-agent.ps1 -MasterUrl "http://master:19527" -AgentName "win-server-01" -Tags "production,windows"
```

### 手动安装

```bash
# 1. 下载二进制文件
wget http://your-master:8080/download/owl-agent-linux-amd64
chmod +x owl-agent-linux-amd64
sudo mv owl-agent-linux-amd64 /usr/local/bin/owl-agent

# 2. 创建配置
sudo mkdir -p /etc/owl
sudo vim /etc/owl/agent.yaml

# 3. 安装服务
sudo cp deploy/systemd/owl-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable owl-agent
sudo systemctl start owl-agent
```

---

## 4. 常用命令

### Master

```bash
# Docker
docker-compose up -d --build  # 构建并启动
docker-compose down           # 停止
docker-compose logs -f        # 日志
docker-compose restart        # 重启

# Systemd
sudo systemctl start owl-master
sudo systemctl stop owl-master
sudo systemctl restart owl-master
sudo systemctl status owl-master
journalctl -u owl-master -f
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
Get-Content "C:\ProgramData\Owl\logs\agent.log" -Wait
```

### 前端

```bash
cd web-react

npm run dev      # 开发模式
npm run build    # 生产构建
npm run preview  # 预览构建结果
```

---

## 5. 卸载

### Linux

```bash
sudo bash deploy/scripts/uninstall-agent.sh
```

### Windows

```powershell
.\uninstall-agent.ps1

# 同时删除配置
.\uninstall-agent.ps1 -RemoveConfig
```

---

## 6. 配置说明

### Master 配置 (`master.yaml`)

```yaml
server:
  http_port: 8080           # HTTP API 端口
  ws_port: 8081             # WebSocket 端口
  static_dir: /var/lib/owl/static  # 前端静态文件目录

auth:
  enabled: false            # 是否启用认证
  jwt_secret: "secret"      # JWT 密钥

cluster:
  heartbeat_timeout: 90s    # 心跳超时
  cleanup_interval: 60s     # 清理间隔
```

### Agent 配置 (`agent.yaml`)

```yaml
agent:
  name: "server-01"         # Agent 名称
  tags:                     # 标签
    - production
    - web

master:
  url: "http://master:8080" # Master 地址
  heartbeat_interval: 30s   # 心跳间隔

websocket:
  enabled: true             # 启用 WebSocket
  ping_interval: 30s        # Ping 间隔

metrics:
  collect_interval: 60s     # 采集间隔
  include_processes: true   # 包含进程信息
  process_top_n: 10         # Top N 进程
```

---

## 7. 防火墙配置

### Master

```bash
# 开放端口
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp
sudo firewall-cmd --reload
```

### Agent

Agent 只需要出站连接到 Master，通常不需要额外配置。

---

## 8. 故障排查

### Agent 无法连接 Master

1. 检查网络连通性：`curl http://master:8080/api/health`
2. 检查防火墙规则
3. 检查 Master 日志：`docker-compose logs owl-master`
4. 检查 Agent 日志：`journalctl -u owl-agent -f`

### Agent 离线

1. 检查 Agent 服务状态：`systemctl status owl-agent`
2. 检查心跳超时配置
3. 检查网络稳定性

### 指标采集异常

1. 检查 Agent 权限
2. 检查 `/proc` 文件系统（Linux）
3. 检查 WMI 服务（Windows）

### 前端页面空白

1. 检查静态文件是否存在：`ls /var/lib/owl/static/`
2. 检查 `static_dir` 配置是否正确
3. 检查浏览器控制台错误
