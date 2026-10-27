同一件事写了六遍

## 现状

**六个同构的代际计数器**:`pingGen`、`telnetGen`、`traceGen`、`portGen`、`waitGen`、`netbios.gen`

**三份「可见/停止/关闭/面板」逻辑**:`ui_detail.go:50-61`、`:184-218`、`:221-253`、`ui_trace.go:100-177`

**重复的停止调用**:

- `internal/ui/app.go:641-644` `startTelnet` 先 `stopPing()`(内部已调 `stopTrace`)又调 `stopTrace()`
- `internal/ui/ui_trace.go:42-43` `traceRoute` 先 `stopPing()` 又 `stopTelnet()`

**终端实现有两份**:`ui_ping.go:46-71` 与 `ui_telnet.go:39-47` 各有一套 `setDark/view/close/feed`;`ui_ping.go` 这个文件名实际是终端基建,不是 ping 面板。

## 方案

- 引入 `toolRun{gen, cancel, term}` + `stopAll()` / `startOne()`
- 面板统一成一个 `pane(title, visible, stop, close)` 构造
- 终端合并成一个实现,删 `telnetTermIface`

## 验收

- 新增或切换工具只改一处
- `grep -c 'Gen' app.go` 从 6 降到 1–2
- 现有工具切换测试仍通过
