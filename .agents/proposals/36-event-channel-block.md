大范围扫描时事件通道写满,扫描再也退不出来

## 现状

- `internal/scan/scanner.go:25` `eventBuf = 1024`
- `internal/scan/scanner.go:373-378` 普通事件写阻塞时可被 ctx 解开
- `internal/scan/scanner.go:381-385` **终止事件是裸写**,没有 ctx 也没有超时
- `internal/ui/app.go:505-514` 事件泵只启动一次

## 问题

/16 这类范围,每个地址一条进度、在线主机 2–3 条,通道很快写满。如果 UI 主线程卡住,或窗口正在销毁导致事件泵停止消费:

- 终止事件永久阻塞
- 扫描 goroutine 与 mDNS / 主机名查询 goroutine 都退不掉
- 取消能解开普通事件,但解不开终止事件

用户看到的就是「卡死且永远不会恢复」。

## 方案

- 终止事件也带 ctx,或 `select` + 超时丢弃
- 提供 `Stop()` 或关闭 `Events()`,让事件泵能退出
- 大范围时合并进度事件(见「事件风暴」条)

## 验收

- 通道填满且无消费者时,取消后扫描 goroutine 在 1 秒内退出
- `go test -race` 下 goroutine 泄漏断言通过
