#!/bin/bash
#
# Owl Agent 卸载脚本
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

if [ "$EUID" -ne 0 ]; then
  log_error "请使用 root 权限运行此脚本"
fi

log_info "开始卸载 Owl Agent..."

# 停止服务
if systemctl is-active --quiet owl-agent; then
  log_info "停止服务..."
  systemctl stop owl-agent
fi

# 禁用服务
if systemctl is-enabled --quiet owl-agent 2>/dev/null; then
  log_info "禁用服务..."
  systemctl disable owl-agent
fi

# 删除服务文件
if [ -f /etc/systemd/system/owl-agent.service ]; then
  log_info "删除服务文件..."
  rm -f /etc/systemd/system/owl-agent.service
  systemctl daemon-reload
fi

# 删除二进制文件
if [ -f /usr/local/bin/owl-agent ]; then
  log_info "删除二进制文件..."
  rm -f /usr/local/bin/owl-agent
fi

# 询问是否删除配置和数据
read -p "是否删除配置文件和数据? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  log_info "删除配置和数据..."
  rm -rf /etc/owl-agent
  rm -rf /var/lib/owl-agent
fi

echo ""
echo -e "${GREEN}Owl Agent 已卸载完成${NC}"
