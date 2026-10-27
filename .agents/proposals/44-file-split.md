单个文件承担了太多职责

## 现状

`internal/ui/app.go` **868 行**,同时装着:测试替身与状态、扫描与事件、IP 段解析与裁剪、Ping、Telnet、端口、排序与计数。

`internal/ui/ui_view.go` **557 行**:整窗视图、命令栏、IP 控件、表格、状态栏、主题注入。

`internal/ui/ui_detail.go` **375 行**:详情 + ping / telnet / trace 三个面板正文。

纯函数归属也分裂:「范围」相关在 `range_guard.go`,「范围解析」却在 `app.go:343-466`。

## 最长的非测试函数

```
scanner.go:102-313  sweep          212 行
ui_wake.go:297-364  wakeDialog      68 行
ui_detail.go:129-182 detailActions  54 行
mdns.go:211-263     handle          53 行
ui_theme.go:10-62   appTheme        53 行
ui_view.go:450-498  treeCell        49 行
```

## 方案

- `scanner.sweep` 按注释里已有的四段拆成 `probePass / consistencyPass / backfillPass / finalPass`
- `app.go` 拆成 `scan_state.go` / `range.go`(并入 `range_guard.go`)/ `ping.go` / `telnet.go` / `ports.go` / `host_sort.go`
- `ui_view.go` 拆成 `ui_range.go` / `ui_table.go` / `ui_status.go`
- `ui_detail.go` 里三个面板移回各自文件

## 验收

- 单文件 < 400 行
- `app.go` 不再包含 `startPing` / `startTelnet` / `sweepPorts`
- 同一概念的纯函数只有一处归属
