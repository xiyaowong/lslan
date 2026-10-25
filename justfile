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
run:
    New-Item -ItemType Directory -Force build/test | Out-Null
    go build -o build/test/lslan-test.exe .
    If (Test-Path build/windows-amd64/ghostty-vt.dll) { Copy-Item build/windows-amd64/ghostty-vt.dll build/test/ -Force }
    Start-Process build/test/lslan-test.exe

# 清理构建产物
clean:
    Remove-Item -Recurse -Force build
