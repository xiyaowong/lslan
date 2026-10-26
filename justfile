# lslan 常用命令

set windows-shell := ["pwsh", "-NoProfile", "-Command"]

# 默认列出所有任务
default:
    @just --list

# 开发模式(热重载)
dev:
    go tool mygo dev

# 全量检查:格式、静态检查、测试
check:
    gofmt -l .
    go vet ./...
    go test ./...

# 构建发布产物到 build/windows-amd64/(含 ghostty-vt.dll,终端插件运行时要挨着 exe)
build:
    go tool mygo build

# 快速试跑:构建到固定路径再启动(防火墙授权按 exe 路径记,固定路径授权一次)
# 传 run=mdns 保留 mDNS:它占一个 UDP 口和组播组,并发跑会抢
run *args='':
    New-Item -ItemType Directory -Force build/test | Out-Null
    try { $dll = Get-ChildItem "$env:LOCALAPPDATA\mygo\natives\*\ghostty-vt.dll" | Select-Object -First 1 } catch {}
    if ($dll -and -not (Test-Path build/test/ghostty-vt.dll)) { Copy-Item $dll.FullName build/test/ }
    go build -o build/test/lslan-test.exe .
    if ("{{args}}" -eq "mdns") { if (Test-Path Env:\LSLAN_MDNS) { Remove-Item Env:\LSLAN_MDNS } } else { $env:LSLAN_MDNS = "0" }
    Start-Process build/test/lslan-test.exe

# 清理构建产物
clean:
    Remove-Item -Recurse -Force build
