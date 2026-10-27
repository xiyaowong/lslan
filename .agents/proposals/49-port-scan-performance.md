局域网延迟通常不到 2ms,却在等 500ms

## 现状

- `internal/scan/ports.go:106-109` `portTimeout = 500ms`、`portWorkers = 32`
- `internal/scan/ports.go:131-151` 每个端口先起 goroutine 再抢信号量
- `internal/ui/app.go:738-765` 每台主机 4 并发

## 量级

- 单台 18 个端口:约 0.5 秒(合理)
- 勾了「顺带扫描端口」时:20 台在线约 2.5 秒,100 台约 12.5 秒,**254 台约 32 秒**

## 方案

- 超时降到 200–250ms
- 改成固定 worker 池(32 个 goroutine 消费端口 channel),内存常数化——现在若传入 6 万端口会一次创建 6 万个 goroutine

## 验收

- 100 台在线主机的端口扫描 < 6 秒
- 6 万端口列表的 goroutine 峰值 ≈ 32
