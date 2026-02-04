#Requires -RunAsAdministrator
#
# Owl Agent Windows 安装脚本
# 用法: iwr -useb http://your-master:19528/install.ps1 | iex
# 或者: .\install-agent.ps1 -MasterUrl "http://master:19527"
#

param(
    [Parameter(Mandatory=$true)]
    [string]$MasterUrl,

    [string]$AgentName = $env:COMPUTERNAME,

    [string]$Version = "latest",

    [string]$Tags = "",

    [string]$InstallDir = "C:\Program Files\Owl",

    [string]$ConfigDir = "C:\ProgramData\Owl",

    [string]$LogDir = "C:\ProgramData\Owl\logs"
)

$ErrorActionPreference = "Stop"

function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $color = switch ($Level) {
        "INFO"  { "Green" }
        "WARN"  { "Yellow" }
        "ERROR" { "Red" }
        default { "White" }
    }
    Write-Host "[$Level] $Message" -ForegroundColor $color
}

function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# 检查管理员权限
if (-not (Test-Administrator)) {
    Write-Log "请以管理员身份运行此脚本" "ERROR"
    exit 1
}

Write-Log "开始安装 Owl Agent..."
Write-Log "Master 地址: $MasterUrl"
Write-Log "Agent 名称: $AgentName"

# 创建目录
function New-Directories {
    Write-Log "创建目录..."

    $dirs = @($InstallDir, $ConfigDir, $LogDir)
    foreach ($dir in $dirs) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }
    }
}

# 下载二进制文件
function Get-Binary {
    Write-Log "下载 Owl Agent..."

    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    $downloadUrl = "$MasterUrl/download/owl-agent-windows-$arch.exe"
    $targetPath = Join-Path $InstallDir "owl-agent.exe"

    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $targetPath -UseBasicParsing
    } catch {
        Write-Log "从 Master 下载失败，尝试从 GitHub 下载..." "WARN"
        $githubUrl = "https://github.com/your-org/owl/releases/$Version/download/owl-agent-windows-$arch.exe"
        Invoke-WebRequest -Uri $githubUrl -OutFile $targetPath -UseBasicParsing
    }

    Write-Log "二进制文件已安装到 $targetPath"
}

# 生成配置文件
function New-Config {
    Write-Log "生成配置文件..."

    $tagsYaml = if ($Tags) {
        $tagList = $Tags -split ',' | ForEach-Object { "  - $_" }
        "tags:`n" + ($tagList -join "`n")
    } else {
        "tags: []"
    }

    $config = @"
# Owl Agent 配置文件
# 由安装脚本自动生成

agent:
  name: "$AgentName"
  $tagsYaml

master:
  url: "$MasterUrl"
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
  file: "$($LogDir -replace '\\', '/')/agent.log"
  max_size: 100
  max_backups: 3
  max_age: 7
"@

    $configPath = Join-Path $ConfigDir "agent.yaml"
    $config | Out-File -FilePath $configPath -Encoding UTF8
    Write-Log "配置文件已生成: $configPath"
}

# 安装 Windows 服务
function Install-Service {
    Write-Log "安装 Windows 服务..."

    $serviceName = "OwlAgent"
    $exePath = Join-Path $InstallDir "owl-agent.exe"
    $configPath = Join-Path $ConfigDir "agent.yaml"

    # 如果服务已存在，先停止并删除
    $existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Log "停止并删除现有服务..." "WARN"
        Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $serviceName | Out-Null
        Start-Sleep -Seconds 2
    }

    # 创建服务
    $binPath = "`"$exePath`" -config `"$configPath`""

    New-Service -Name $serviceName `
        -DisplayName "Owl Agent" `
        -Description "Owl Server Monitoring Agent" `
        -BinaryPathName $binPath `
        -StartupType Automatic | Out-Null

    # 配置服务恢复选项
    sc.exe failure $serviceName reset= 86400 actions= restart/5000/restart/5000/restart/5000 | Out-Null

    Write-Log "服务已安装: $serviceName"
}

# 启动服务
function Start-OwlService {
    Write-Log "启动 Owl Agent..."

    Start-Service -Name "OwlAgent"
    Start-Sleep -Seconds 2

    $service = Get-Service -Name "OwlAgent"
    if ($service.Status -eq "Running") {
        Write-Log "Owl Agent 启动成功!"
    } else {
        Write-Log "Owl Agent 启动失败，请检查日志" "ERROR"
        exit 1
    }
}

# 添加防火墙规则
function Add-FirewallRule {
    Write-Log "配置防火墙规则..."

    $ruleName = "Owl Agent"
    $exePath = Join-Path $InstallDir "owl-agent.exe"

    # 删除现有规则
    Remove-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue

    # 添加新规则
    New-NetFirewallRule -DisplayName $ruleName `
        -Direction Outbound `
        -Program $exePath `
        -Action Allow `
        -Profile Any | Out-Null

    Write-Log "防火墙规则已添加"
}

# 显示状态
function Show-Status {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "Owl Agent 安装完成!" -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "安装目录: $InstallDir"
    Write-Host "配置文件: $ConfigDir\agent.yaml"
    Write-Host "日志目录: $LogDir"
    Write-Host ""
    Write-Host "常用命令:"
    Write-Host "  启动: Start-Service OwlAgent"
    Write-Host "  停止: Stop-Service OwlAgent"
    Write-Host "  重启: Restart-Service OwlAgent"
    Write-Host "  状态: Get-Service OwlAgent"
    Write-Host ""
}

# 主流程
try {
    New-Directories
    Get-Binary
    New-Config
    Install-Service
    Add-FirewallRule
    Start-OwlService
    Show-Status
} catch {
    Write-Log "安装失败: $_" "ERROR"
    exit 1
}
