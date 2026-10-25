# lslan 常用命令

set windows-shell := ["pwsh", "-NoProfile", "-Command"]

# 默认列出所有任务
default:
    @just --list

# 开发模式(热重载)
dev:
    go tool mygo dev

# 全量检查:格式、静态检查、测试、构建
check:
    gofmt -l .
    go vet ./...
    go test ./...

# 构建发布产物到 build/windows-amd64/
build:
    go tool mygo build

# 构建并运行固定路径的测试 exe(路径不变,防火墙授权一次即可,无人值守也点得过去)
# 默认关掉 mDNS:它占一个 UDP 口和组播组,多个任务同时跑会抢;要验发现功能用 just run-mdns
run:
    New-Item -ItemType Directory -Force build/test | Out-Null
    go build -o build/test/lslan-test.exe .
    If (Test-Path build/windows-amd64/ghostty-vt.dll) { Copy-Item build/windows-amd64/ghostty-vt.dll build/test/ -Force }
    $env:LSLAN_MDNS = "0"
    Start-Process build/test/lslan-test.exe

# 同上,但保留 mDNS(只在开发发现/服务识别相关功能时用)
run-mdns:
    New-Item -ItemType Directory -Force build/test | Out-Null
    go build -o build/test/lslan-test.exe .
    If (Test-Path build/windows-amd64/ghostty-vt.dll) { Copy-Item build/windows-amd64/ghostty-vt.dll build/test/ -Force }
    Remove-Item Env:\LSLAN_MDNS -ErrorAction SilentlyContinue
    Start-Process build/test/lslan-test.exe

# 清理构建产物
clean:
    Remove-Item -Recurse -Force build
