  1. 服务器上停服务

  sudo systemctl stop serverowl

  2. 本机重新编译（Windows，当前 PowerShell 会话生效）

  $env:GOOS="linux"; $env:GOARCH="amd64"; go build -trimpath -ldflags "-s -w" -o serverowl ./cmd/serverowl

  3. 上传新二进制到服务器

  scp ./serverowl root@149.104.30.171:/tmp/serverowl

  4. 服务器上替换并启动

  ssh root@149.104.30.171
  sudo mv /tmp/serverowl /opt/serverowl/serverowl
  sudo chmod +x /opt/serverowl/serverowl
  sudo systemctl start serverowl
  sudo systemctl status serverowl --no-pager

  5. 看日志确认起来了

  sudo journalctl -u serverowl -n 100 --no-pager

  journalctl -u serverowl -f







# ServerOwl 部署流程（Ubuntu + Caddy + 企业微信回调）

本文档用于把 `serverowl` 部署到一台 Ubuntu 服务器上，使用 Caddy 提供 HTTPS，并接入企业微信自建应用的回调（私聊应用 → 服务器收到消息 → 主动回消息）。

## 0. 前置条件

- 一台 Ubuntu 服务器，已安装并运行 Caddy（80/443 已放行）
- 你的企业微信管理后台可创建“自建应用”
- 你有一个可用域名（示例：`owl.cyb1.org`），并能在权威 DNS 处添加记录

## 1. 企业微信后台准备（自建应用）

在企业微信管理后台创建自建应用，并记录以下信息：

- `corp_id`（企业 ID）
- `agent_id`（应用 AgentId）
- `secret`（应用 Secret）

然后在应用里配置「接收消息服务器配置 / API 接收」：

- URL：`https://owl.cyb1.org/wecom/callback`
- Token：自定义随机串
- EncodingAESKey：后台生成的 43 位串

注意：后续服务器主动调用 OpenAPI 发消息，需要把服务器出口公网 IP 加到“可信 IP / IP 白名单”（否则会 `errcode=60020`）。

## 2. DNS 配置

确认你的域名权威 DNS 在哪里（Cloudflare/DNSPod 等），并只在“权威 DNS”里添加记录。

- 添加 A 记录：`owl.cyb1.org` → `149.104.30.171`（替换为你的服务器 IP）

验证解析（在你本机）：

```powershell
nslookup -type=A owl.cyb1.org 1.1.1.1
```

## 3. 本机（Windows）交叉编译

在项目根目录执行（只对当前 PowerShell 窗口生效）：

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -trimpath -ldflags "-s -w" -o serverowl ./cmd/serverowl
```

产物：当前目录下 `serverowl`（Linux 二进制）

## 4. 上传到服务器并安装

在本机上传：

```bash
scp ./serverowl root@149.104.30.171:/tmp/serverowl
scp ./configs/config.yaml root@149.104.30.171:/tmp/config.yaml
```

在服务器上安装：

```bash
ssh root@149.104.30.171
mkdir -p /opt/serverowl
mv /tmp/serverowl /opt/serverowl/serverowl
mv /tmp/config.yaml /opt/serverowl/config.yaml
chmod +x /opt/serverowl/serverowl
```

## 5. 配置 `config.yaml`

编辑 `/opt/serverowl/config.yaml`（重点字段）：

```yaml
server:
  port: 9090

wechat:
  corp_id: "wwxxxxxxxxxxxxxx"
  agent_id: 1000002
  secret: "your_secret"
  token: "callback_token"
  encoding_aes_key: "your_encoding_aes_key"
```

说明：

- `server.port`：serverowl 本地监听端口（示例 9090）
- `encoding_aes_key`：必须是企业微信后台生成的 43 位串

## 6. 先前台跑通（可选但推荐）

```bash
/opt/serverowl/serverowl -config /opt/serverowl/config.yaml
```

本机健康检查：

```bash
curl -i http://127.0.0.1:9090/health
```

## 7. 配 systemd 常驻

```bash
cat >/etc/systemd/system/serverowl.service <<'EOF'
[Unit]
Description=ServerOwl
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/serverowl
ExecStart=/opt/serverowl/serverowl -config /opt/serverowl/config.yaml
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now serverowl
systemctl status serverowl --no-pager
```

看日志：

```bash
journalctl -u serverowl -f
```

## 8. Caddy 增量添加反代（不覆盖现有站点）

目标：只新增一个站点块，不影响 `cyb1.org / blog.cyb1.org / s.cyb1.org` 等已有配置。

在 `/etc/caddy/Caddyfile` 末尾追加：

```caddy
owl.cyb1.org {
  reverse_proxy 127.0.0.1:9090
}
```

应用配置：

```bash
caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy
```

验证：

```bash
curl -i https://owl.cyb1.org/health
```

## 9. 验证企业微信回调链路

### 9.1 验证回调入口是否命中 serverowl

```bash
curl -i https://owl.cyb1.org/wecom/callback | head -n 10
```

正常会返回 `403 invalid signature`（因为你没带企业微信签名参数），这代表“请求确实到达 serverowl”。

### 9.2 私聊应用测试收消息

在企业微信里私聊该应用，发送任意文字；服务器日志应出现类似：

```
wecom text from=<UserID> content="..."
```

其中 `from=<UserID>` 就是你的企业微信 `userid`。

### 9.3 主动回消息（必要：IP 白名单）

若 serverowl 回消息失败，常见为：

- `errcode=60020 not allow to access from your ip`

解决：在企业微信后台把服务器出口公网 IP 加入可信 IP/白名单。

查询出口公网 IP（服务器上）：

```bash
curl -s https://api.ipify.org; echo
```

## 10. 更新/重启

- 更新二进制：上传新 `serverowl` 覆盖 `/opt/serverowl/serverowl` 后：

```bash
systemctl restart serverowl
systemctl status serverowl --no-pager
```

- 查看最近日志：

```bash
journalctl -u serverowl -n 200 --no-pager
```

## 11. 安全提示（强烈建议）

- 不要在聊天/截图中泄露：`secret/token/encoding_aes_key/access_token`
- 如果泄露了：立刻在企业微信后台重置 Secret/Token/AESKey，并同步更新 `/opt/serverowl/config.yaml`，再 `systemctl restart serverowl`

