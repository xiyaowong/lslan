# lslan 常用命令

set windows-shell := ["pwsh", "-NoProfile", "-Command"]

# 默认列出所有任务
default:
    @just --list

# 开发模式(热重载)
dev:
    go tool mygo dev

# 全量检查:格式、静态检查、测试
# just 在非 [script] 配方里只认多行命令合成一份脚本时的退出码,所以一行写完
check:
    $bad = gofmt -l .; if ($bad) { Write-Output $bad; exit 1 }; go vet ./...; if ($LASTEXITCODE) { exit 1 }; go test ./...

# 发版前:tag、mygo.json、CHANGELOG 必须同一版本号,且从 main 打 tag
[script('pwsh', '-NoProfile', '-Command')]
release-check version:
    $v = "{{version}}" -replace '^v', ''
    $json = (Get-Content mygo.json -Raw | ConvertFrom-Json).version
    $top = (Select-String -Path CHANGELOG.md -Pattern '^## (\d+\.\d+\.\d+)' | Select-Object -First 1).Matches.Groups[1].Value
    $bad = $false
    if ($json -ne $v) { Write-Output "mygo.json 是 $json,与 $v 不一致"; $bad = $true }
    if ($top -ne $v) { Write-Output "CHANGELOG 顶部是 $top,与 $v 不一致"; $bad = $true }
    git fetch --quiet origin main
    git merge-base --is-ancestor HEAD origin/main
    if ($LASTEXITCODE -ne 0) { Write-Output "HEAD 不在 main 上,发版应从 main 打 tag"; $bad = $true }
    if ($bad) { exit 1 }
    Write-Output "发布检查通过:$v"

# 构建发布产物到 build/windows-amd64/(含 ghostty-vt.dll,终端插件运行时要挨着 exe)
build:
    go tool mygo build

# 快速试跑:构建到固定路径再启动(防火墙授权按 exe 路径记,固定路径授权一次)
# 传 mdns 保留 mDNS:它占一个 UDP 口和组播组,并发跑会抢
# 多行配方在 just 里合成为同一份脚本,变量可以跨行
[script('pwsh', '-NoProfile', '-Command')]
run mdns='':
    New-Item -ItemType Directory -Force build/test | Out-Null
    $dll = Get-ChildItem "$env:LOCALAPPDATA\mygo\natives\*\ghostty-vt.dll" -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($dll -and -not (Test-Path build/test/ghostty-vt.dll)) { Copy-Item $dll.FullName build/test/ }
    go build -o build/test/lslan-test.exe .
    if ("{{mdns}}" -eq 'mdns') { Remove-Item Env:\LSLAN_MDNS -ErrorAction SilentlyContinue } else { $env:LSLAN_MDNS = '0' }
    Start-Process build/test/lslan-test.exe

# 清理构建产物
clean:
    if (Test-Path build) { Remove-Item -Recurse -Force build }
