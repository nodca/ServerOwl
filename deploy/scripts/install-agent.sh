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
CONFIG_DIR="/etc/owl"
LOG_DIR="/var/log/owl"
DATA_DIR="/var/lib/owl"
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
      echo "  -m, --master <url>    Master 服务器地址 (必填)"
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

# 检测操作系统
detect_os() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    echo "$ID"
  else
    log_error "无法检测操作系统"
  fi
}

ARCH=$(detect_arch)
OS=$(detect_os)

log_info "检测到系统: $OS ($ARCH)"

# 设置默认 Agent 名称
if [ -z "$AGENT_NAME" ]; then
  AGENT_NAME=$(hostname)
fi

log_info "Agent 名称: $AGENT_NAME"
log_info "Master 地址: $MASTER_URL"

# 创建 owl 用户
create_user() {
  if ! id "owl" &>/dev/null; then
    log_info "创建 owl 用户..."
    useradd -r -s /sbin/nologin -d /var/lib/owl owl
  fi
}

# 创建目录
create_dirs() {
  log_info "创建目录..."
  mkdir -p "$CONFIG_DIR" "$LOG_DIR" "$DATA_DIR"
  chown -R owl:owl "$LOG_DIR" "$DATA_DIR"
}

# 下载二进制文件
download_binary() {
  log_info "下载 Owl Agent..."

  local download_url="${MASTER_URL}/download/owl-agent-linux-${ARCH}"

  if command -v curl &>/dev/null; then
    curl -sSL -o "${INSTALL_DIR}/owl-agent" "$download_url" || {
      log_warn "从 Master 下载失败，尝试从 GitHub 下载..."
      curl -sSL -o "${INSTALL_DIR}/owl-agent" \
        "https://github.com/your-org/owl/releases/${VERSION}/download/owl-agent-linux-${ARCH}"
    }
  elif command -v wget &>/dev/null; then
    wget -q -O "${INSTALL_DIR}/owl-agent" "$download_url" || {
      log_warn "从 Master 下载失败，尝试从 GitHub 下载..."
      wget -q -O "${INSTALL_DIR}/owl-agent" \
        "https://github.com/your-org/owl/releases/${VERSION}/download/owl-agent-linux-${ARCH}"
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
    tags_yaml="tags:"
    IFS=',' read -ra TAG_ARRAY <<< "$TAGS"
    for tag in "${TAG_ARRAY[@]}"; do
      tags_yaml="${tags_yaml}\n  - ${tag}"
    done
  else
    tags_yaml="tags: []"
  fi

  cat > "${CONFIG_DIR}/agent.yaml" << EOF
# Owl Agent 配置文件
# 由安装脚本自动生成

agent:
  name: "${AGENT_NAME}"
  $(echo -e "$tags_yaml")

master:
  url: "${MASTER_URL}"
  heartbeat_interval: 30s
  retry_interval: 5s

websocket:
  enabled: true
  ping_interval: 30s
  reconnect_interval: 5s

metrics:
  collect_interval: 60s
  include_processes: true
  process_top_n: 10

logging:
  level: info
  file: "${LOG_DIR}/agent.log"
  max_size: 100
  max_backups: 3
  max_age: 7
EOF

  chown owl:owl "${CONFIG_DIR}/agent.yaml"
  chmod 640 "${CONFIG_DIR}/agent.yaml"
  log_info "配置文件已生成: ${CONFIG_DIR}/agent.yaml"
}

# 安装 systemd 服务
install_service() {
  log_info "安装 systemd 服务..."

  cat > /etc/systemd/system/owl-agent.service << 'EOF'
[Unit]
Description=Owl Agent - Server Monitoring Agent
Documentation=https://github.com/your-org/owl
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=owl
Group=owl
ExecStart=/usr/local/bin/owl-agent -config /etc/owl/agent.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
StartLimitInterval=60
StartLimitBurst=3

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/log/owl /var/lib/owl
PrivateTmp=true

LimitNOFILE=65536
LimitNPROC=4096

Environment=OWL_LOG_LEVEL=info

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
  echo "日志文件: ${LOG_DIR}/agent.log"
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

  create_user
  create_dirs
  download_binary
  generate_config
  install_service
  start_service
  show_status
}

main
