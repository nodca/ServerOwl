#Requires -RunAsAdministrator
#
# Owl Agent Windows 卸载脚本
#

param(
    [switch]$RemoveConfig = $false,
    [switch]$Force = $false
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

$serviceName = "OwlAgent"
$installDir = "C:\Program Files\Owl"
$configDir = "C:\ProgramData\Owl"

Write-Log "开始卸载 Owl Agent..."

# 停止服务
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($service) {
    if ($service.Status -eq "Running") {
        Write-Log "停止服务..."
        Stop-Service -Name $serviceName -Force
        Start-Sleep -Seconds 2
    }

    Write-Log "删除服务..."
    sc.exe delete $serviceName | Out-Null
    Start-Sleep -Seconds 2
}

# 删除二进制文件
if (Test-Path $installDir) {
    Write-Log "删除程序文件..."
    Remove-Item -Path $installDir -Recurse -Force
}

# 删除防火墙规则
Write-Log "删除防火墙规则..."
Remove-NetFirewallRule -DisplayName "Owl Agent" -ErrorAction SilentlyContinue

# 删除配置和日志
if ($RemoveConfig -or $Force) {
    if (Test-Path $configDir) {
        Write-Log "删除配置和日志..."
        Remove-Item -Path $configDir -Recurse -Force
    }
} else {
    $response = Read-Host "是否删除配置文件和日志? [y/N]"
    if ($response -eq "y" -or $response -eq "Y") {
        if (Test-Path $configDir) {
            Write-Log "删除配置和日志..."
            Remove-Item -Path $configDir -Recurse -Force
        }
    }
}

Write-Host ""
Write-Host "Owl Agent 已卸载完成" -ForegroundColor Green
