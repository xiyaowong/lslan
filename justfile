# lslan 常用命令

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

# 清理构建产物
clean:
    Remove-Item -Recurse -Force build
