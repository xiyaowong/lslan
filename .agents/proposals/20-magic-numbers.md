同一个数值在多处硬编码,改一处就会不一致

## 现状

- 终端面板高 `140` 出现 4 次(`ui_detail.go:204,213`、`ui_trace.go:169,174`)
- 工具面板高 `180` 一次(`ui_detail.go:248`)
- 标题栏高 `44` 在 `main.go:42` 与 `ui_titlebar.go:34` 各写一次
- 状态栏 `32`(`ui_view.go:274`)、表格/服务行 `34`(`ui_detail.go:308,339`、`ui_services.go:64`)
- 窄卡阈值 `600`(`ui_view.go:385`)、面板最小宽 `380`(`ui_view.go:93`)、卡宽 `470/400/760`(`ui_view.go:65-67`)
- `pingLines` 上限 400 / 保留 300(`app.go:624-625`)
- 端口并发 `4`(`app.go:738`,而引擎里是 `32`)
- telnet 拨号 `3s`(`app.go:215`)
- 范围裁剪的 `256 / 1 / 254`(`app.go:446-455`)

## 方案

提取到一处:

```go
const (
    terminalPaneHeight = 140
    toolPaneHeight     = 180
    titleBarHeight     = 44 // main.go 引用这个
    rowHeight          = 34
    statusBarHeight    = 32
    cardNarrowWidth    = 600
    detailMinWidth     = 380
    telnetDialTimeout  = 3 * time.Second
)
```

`pingLines` 的 400/300 关系写成 `maxLines / keepLast` 并注释原因。

## 验收

- 上列数字只在常量定义处出现一次
- 改标题栏高度只需改一个地方
