# ServerOwl 🦉

AI 驱动的个人服务器运维助手，让服务器管理更智能、更简单。

**在线演示**: [https://owl.cyb1.org](https://owl.cyb1.org)

## 特性

- **智能对话** - 通过自然语言与服务器交互，执行运维任务
- **ReAct Agent** - 基于 ReAct 架构的 AI Agent，支持多步推理和工具调用
- **分层记忆系统** - 短期/长期/工作记忆，让 AI 记住上下文和历史操作
- **环境自动扫描** - 自动发现 Docker 容器、数据库配置、Nginx/Caddy 站点
- **多节点管理** - Master-Agent 架构，集中管理多台服务器
- **双主高可用** - 支持多 Master 热备，自动故障转移
- **技能系统** - 可扩展的技能定义，快速添加新能力
- **企业微信集成** - 通过企业微信对话管理服务器，接收告警通知
- **实时监控** - CPU、内存、磁盘、网络、容器状态实时采集
- **Web 管理面板** - 现代化 React 前端，可视化管理

## 架构

```
┌─────────────────────────────────────┐     ┌─────────────────────────────────────┐
│           Master A                  │◄───►│           Master B                  │
│  ┌─────────────────────────────┐   │ 同步 │  ┌─────────────────────────────┐   │
│  │  - ReAct Agent Engine       │   │     │  │  - ReAct Agent Engine       │   │
│  │  - Memory System            │   │     │  │  - Memory System            │   │
│  │  - Skill Manager            │   │     │  │  - Skill Manager            │   │
│  │  - Environment Scanner      │   │     │  │  - Environment Scanner      │   │
│  │  - Web UI (React)           │   │     │  │  - Web UI (React)           │   │
│  └─────────────────────────────┘   │     │  └─────────────────────────────┘   │
└─────────────────────────────────────┘     └─────────────────────────────────────┘
                  │                                       │
                  └───────────────┬───────────────────────┘
                                  │ WebSocket
                                  ▼
              ┌─────────┐    ┌─────────┐    ┌─────────┐
              │ Agent 1 │    │ Agent 2 │    │ Agent N │
              │ 服务器A  │    │ 服务器B  │    │ 服务器C  │
              └─────────┘    └─────────┘    └─────────┘
```

**特点**：
- 双主热备，自动故障转移
- Agent 可连接任一 Master
- 状态实时同步

## 核心功能

### 环境自动扫描

ServerOwl 会自动扫描服务器环境，发现并记录：

- **Docker 容器** - 容器名称、镜像、状态、环境变量、网络配置
- **数据库服务** - 从容器环境变量和配置文件中提取 PostgreSQL、MySQL、Redis 连接信息
- **反向代理** - 解析 Caddy/Nginx 配置，提取域名和后端映射关系

AI 可以直接查询这些信息，无需重复执行命令：

```
用户: PostgreSQL 的密码是什么？
AI: [查询环境信息] PostgreSQL 密码是 xxx，来源于容器 postgres 的环境变量。
```

### 企业微信集成

支持接入企业微信，实现：

- **对话式运维** - 在企业微信中直接与 AI 对话，执行服务器管理任务
- **告警通知** - 服务器异常时自动推送告警到企业微信
- **消息回调** - 支持企业微信消息回调，实时响应指令

配置示例：

```yaml
wechat:
  corp_id: "your-corp-id"
  agent_id: 1000002
  secret: "your-secret"
  token: "callback-token"
  encoding_aes_key: "your-aes-key"
  default_user: "@all"
```

### 内置技能

ServerOwl 内置了常用运维技能：

| 技能 | 说明 |
|------|------|
| `healthcheck` | 健康检查（容器、HTTP、TCP、进程） |
| `docker_stats` | Docker 容器状态和资源使用 |
| `backup` | 数据库备份（PostgreSQL、MySQL） |
| `cleanup` | 清理日志、临时文件、Docker 镜像 |
| `service` | 服务管理（启动、停止、重启） |
| `ssl_renew` | SSL 证书续期 |
| `firewall` | 防火墙规则管理 |
| `network_diag` | 网络诊断（ping、traceroute、DNS） |
| `log_tail` | 日志查看和搜索 |
| `caddy` | Caddy 配置管理 |
| `system_info` | 系统信息概览 |

### 分层记忆系统

- **短期记忆** - 当前对话上下文，最近 N 轮对话
- **长期记忆** - 持久化存储，支持向量检索相关历史
- **工作记忆** - 当前任务状态、中间结果

### 自定义技能

通过 YAML 定义技能，快速扩展 AI 能力：

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
  - name: check_docker
    tool: shell
    command: docker ps
```

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 18+ (前端开发)
- PostgreSQL 16+ with pgvector 扩展

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
│   ├── skills/          # 技能系统
│   │   └── builtin/     # 内置技能
│   ├── wecom/           # 企业微信回调
│   ├── notifier/        # 通知推送
│   ├── cluster/         # 集群管理
│   ├── scheduler/       # 任务调度
│   └── web/             # Web API
├── configs/             # 配置文件示例
├── deploy/              # 部署脚本和配置
├── skills/              # 自定义技能
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
