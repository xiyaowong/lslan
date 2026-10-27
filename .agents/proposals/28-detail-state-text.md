详情里看不出设备在不在线,除非会分辨颜色

## 现状

- `internal/ui/ui_detail.go:68-69` 详情头:一个 10px 色点 + IP,没有文字
- 对比 `internal/ui/ui_view.go:495` 表格里有「在线/离线」文字

同一个状态,表格里有字、详情里没字;色弱用户与读屏都无法得知。

## 方案

- IP 旁加 `stateLabel(h.State)` 的小字(12px,`TextMuted`)
- 或给色点加 `.Label(stateLabel(...))`

## 验收

- 详情头出现「在线/离线/未知」文本
- 无障碍树里存在该名称
