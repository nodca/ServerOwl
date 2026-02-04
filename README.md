# ServerOwl 🦉

AI 驱动的个人服务器运维助手，让服务器管理更智能、更简单。

## 特性

- **智能对话** - 通过自然语言与服务器交互，执行运维任务
- **ReAct Agent** - 基于 ReAct 架构的 AI Agent，支持多步推理和工具调用
- **分层记忆系统** - 短期/长期/工作记忆，让 AI 记住上下文和历史操作
- **多节点管理** - Master-Agent 架构，集中管理多台服务器
- **双主高可用** - 支持多 Master 热备，自动故障转移
- **技能系统** - 可扩展的 YAML 技能定义，快速添加新能力
- **实时监控** - CPU、内存、磁盘、容器状态实时采集
- **Web 管理面板** - 现代化 React 前端，可视化管理

## 架构

```
┌─────────────────────────────────────┐
│           Master 节点                │
│  ┌─────────────────────────────┐   │
│  │  - ReAct Agent Engine       │   │
│  │  - Memory System            │   │
│  │  - Skill Manager            │   │
│  │  - Web UI (React)           │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
                  │
                  │ WebSocket
                  ▼
┌─────────┐  ┌─────────┐  ┌─────────┐
│ Agent 1 │  │ Agent 2 │  │ Agent N │
│ 服务器A  │  │ 服务器B  │  │ 服务器C  │
└─────────┘  └─────────┘  └─────────┘
```

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 18+ (前端开发)
- PostgreSQL 15+ (可选，用于记忆系统)
- Redis (可选，用于缓存)

### 编译

```bash
# 克隆仓库
git clone https://github.com/nodca/ServerOwl.git
cd ServerOwl

# 编译 Master
go build -o serverowl ./cmd/serverowl

# 编译 Agent
go build -o owl-agent ./cmd/owl-agent

# 编译前端
cd web-react
npm install
npm run build
```

### 配置

复制示例配置并修改：

```bash
cp configs/config.example.yaml configs/config.yaml
```

主要配置项：

```yaml
server:
  port: 9090

llm:
  provider: "openai"  # 或 siliconflow, deepseek 等
  api_key: "your-api-key"
  base_url: "https://api.openai.com/v1"
  model: "gpt-4"

web:
  enabled: true
  port: 8080
  auth:
    username: "admin"
    password: "your-password"
```

### 运行

```bash
# 启动 Master
./serverowl -config configs/config.yaml

# 在其他服务器上启动 Agent
./owl-agent -config configs/agent.yaml
```

访问 `http://localhost:8080` 打开 Web 管理面板。

## 部署

详细部署指南请参考 [deploy/README.md](deploy/README.md)。

### Docker 部署

```bash
cd deploy/docker
docker-compose up -d
```

### Systemd 服务

```bash
# Master
sudo cp deploy/systemd/owl-master.service /etc/systemd/system/
sudo systemctl enable owl-master
sudo systemctl start owl-master

# Agent
sudo cp deploy/systemd/owl-agent.service /etc/systemd/system/
sudo systemctl enable owl-agent
sudo systemctl start owl-agent
```

## 技能系统

ServerOwl 支持通过 YAML 定义技能，快速扩展 AI 能力：

```yaml
# skills/healthcheck.yaml
name: system_healthcheck
description: 执行系统健康检查
steps:
  - name: check_disk
    tool: shell
    command: df -h
  - name: check_memory
    tool: shell
    command: free -m
```

## 项目结构

```
.
├── cmd/
│   ├── serverowl/       # Master 入口
│   └── owl-agent/       # Agent 入口
├── internal/
│   ├── agent/           # ReAct Agent 引擎
│   ├── agent-node/      # 分布式 Agent 节点
│   ├── memory/          # 分层记忆系统
│   ├── llm/             # LLM 客户端
│   ├── monitor/         # 监控采集
│   ├── skill/           # 技能系统
│   └── web/             # Web API
├── configs/             # 配置文件示例
├── deploy/              # 部署脚本和配置
├── skills/              # 技能定义
└── web-react/           # React 前端
```

## 开发

### 本地开发

```bash
# 启动后端
go run ./cmd/serverowl

# 启动前端开发服务器（另一个终端）
cd web-react
npm run dev
```

### 代码风格

- Go: 遵循标准 Go 代码风格
- TypeScript/React: 无分号，单引号，2 空格缩进

## 许可证

[MIT License](LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request！
