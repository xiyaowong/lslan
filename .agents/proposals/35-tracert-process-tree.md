点「停止」之后路由跟踪可能还在跑

## 现状

- `internal/ui/ui_trace.go:25-35` 用 `exec.CommandContext(ctx, "cmd", "/c", "chcp ... & tracert ...")`
- `internal/ui/ui_trace.go:117-123` 取消时的注释说「kills the process」

## 问题

Go 的 `CommandContext` 只杀直接子进程(`cmd.exe`),不会杀它的子进程 `tracert.exe`。点了停止、切换设备或关窗后,tracert 可能继续跑满 15 跳(≥15 秒),期间占着句柄与网络。UI 已认为结束。

## 方案

三选一:

1. 停止时 `taskkill /PID <cmd.Pid> /T /F`(`/T` 杀整棵进程树)
2. 用 Job Object(创建挂起进程 → 分配 → 恢复)
3. `cmd.Wait` 后补一次 `taskkill`(有极短窗口)

## 验收

- 停止后 `Get-Process tracert` 为空
- 有测试用假进程树断言清理调用
