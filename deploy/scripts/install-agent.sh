#!/bin/bash
#
# Owl Agent 一键安装脚本
# 用法: curl -sSL http://your-master:19528/install.sh | bash -s -- -m <master-url>
#

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# 默认配置
MASTER_URL=""
AGENT_NAME=""
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/owl-agent"
DATA_DIR="/var/lib/owl-agent"
VERSION="latest"
TAGS=""

# 解析参数
while [[ $# -gt 0 ]]; do
  case $1 in
    -m|--master)
      MASTER_URL="$2"
      shift 2
      ;;
    -n|--name)
      AGENT_NAME="$2"
      shift 2
      ;;
    -v|--version)
      VERSION="$2"
      shift 2
      ;;
    -t|--tags)
      TAGS="$2"
      shift 2
      ;;
    -h|--help)
      echo "Owl Agent 安装脚本"
      echo ""
      echo "用法: $0 [选项]"
      echo ""
      echo "选项:"
      echo "  -m, --master <url>    Master 服务器地址 (必填，如 192.168.1.10:19527)"
      echo "  -n, --name <name>     Agent 名称 (默认: 主机名)"
      echo "  -v, --version <ver>   安装版本 (默认: latest)"
      echo "  -t, --tags <tags>     标签，逗号分隔"
      echo "  -h, --help            显示帮助"
      exit 0
      ;;
    *)
      log_error "未知参数: $1"
      ;;
  esac
done

# 检查必填参数
if [ -z "$MASTER_URL" ]; then
  log_error "请指定 Master 地址: -m <master-url>"
fi

# 检查 root 权限
if [ "$EUID" -ne 0 ]; then
  log_error "请使用 root 权限运行此脚本"
fi

# 检测系统架构
detect_arch() {
  local arch=$(uname -m)
  case $arch in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      log_error "不支持的架构: $arch"
      ;;
  esac
}

ARCH=$(detect_arch)

log_info "检测到架构: $ARCH"

# 设置默认 Agent 名称
if [ -z "$AGENT_NAME" ]; then
  AGENT_NAME=$(hostname)
fi

log_info "Agent 名称: $AGENT_NAME"
log_info "Master 地址: $MASTER_URL"

# 创建目录
create_dirs() {
  log_info "创建目录..."
  mkdir -p "$CONFIG_DIR" "$DATA_DIR"
}

# 下载二进制文件
download_binary() {
  log_info "下载 Owl Agent..."

  # 尝试从 Master 下载
  local download_url="http://${MASTER_URL%:*}:19528/download/owl-agent-linux-${ARCH}"

  if command -v curl &>/dev/null; then
    curl -sSL -o "${INSTALL_DIR}/owl-agent" "$download_url" 2>/dev/null || {
      log_warn "从 Master 下载失败"
      log_error "请手动下载 owl-agent 并放置到 ${INSTALL_DIR}/"
    }
  elif command -v wget &>/dev/null; then
    wget -q -O "${INSTALL_DIR}/owl-agent" "$download_url" 2>/dev/null || {
      log_warn "从 Master 下载失败"
      log_error "请手动下载 owl-agent 并放置到 ${INSTALL_DIR}/"
    }
  else
    log_error "请安装 curl 或 wget"
  fi

  chmod +x "${INSTALL_DIR}/owl-agent"
  log_info "二进制文件已安装到 ${INSTALL_DIR}/owl-agent"
}

# 生成配置文件
generate_config() {
  log_info "生成配置文件..."

  # 解析 tags
  local tags_yaml=""
  if [ -n "$TAGS" ]; then
    IFS=',' read -ra TAG_ARRAY <<< "$TAGS"
    for tag in "${TAG_ARRAY[@]}"; do
      tags_yaml="${tags_yaml}    - \"${tag}\"\n"
    done
  fi

  cat > "${CONFIG_DIR}/agent.yaml" << EOF
# Owl Agent 配置文件
# 由安装脚本自动生成

agent:
  id: ""
  name: "${AGENT_NAME}"
  tags:
$(if [ -n "$tags_yaml" ]; then echo -e "$tags_yaml"; else echo "    - \"default\""; fi)
  labels:
    installed_by: "script"
  heartbeat_interval: 10s
  reconnect_interval: 5s
  command_timeout: 5m
  data_dir: "${DATA_DIR}"

masters:
  - id: "master-01"
    name: "主节点"
    addr: "${MASTER_URL}"
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
      - type: "notify"
        command: "磁盘使用率超过 90%"
    cooldown: 1h
EOF

  chmod 640 "${CONFIG_DIR}/agent.yaml"
  log_info "配置文件已生成: ${CONFIG_DIR}/agent.yaml"
}

# 安装 systemd 服务
install_service() {
  log_info "安装 systemd 服务..."

  cat > /etc/systemd/system/owl-agent.service << EOF
[Unit]
Description=Owl Agent - Server Monitoring Agent
Documentation=https://github.com/your-org/serverowl
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/owl-agent -config ${CONFIG_DIR}/agent.yaml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=5
StartLimitInterval=60
StartLimitBurst=3

LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable owl-agent
  log_info "服务已安装并设置为开机启动"
}

# 启动服务
start_service() {
  log_info "启动 Owl Agent..."
  systemctl start owl-agent

  sleep 2

  if systemctl is-active --quiet owl-agent; then
    log_info "Owl Agent 启动成功!"
  else
    log_error "Owl Agent 启动失败，请检查日志: journalctl -u owl-agent"
  fi
}

# 显示状态
show_status() {
  echo ""
  echo "=========================================="
  echo -e "${GREEN}Owl Agent 安装完成!${NC}"
  echo "=========================================="
  echo ""
  echo "配置文件: ${CONFIG_DIR}/agent.yaml"
  echo "服务状态: systemctl status owl-agent"
  echo ""
  echo "常用命令:"
  echo "  启动: systemctl start owl-agent"
  echo "  停止: systemctl stop owl-agent"
  echo "  重启: systemctl restart owl-agent"
  echo "  日志: journalctl -u owl-agent -f"
  echo ""
}

# 主流程
main() {
  log_info "开始安装 Owl Agent..."

  create_dirs
  download_binary
  generate_config
  install_service
  start_service
  show_status
}

main
