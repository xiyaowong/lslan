同类图标尺寸、线宽、交互形态都不统一

## 现状

尺寸/线宽:

- 标题栏窗口按钮 10px / stroke 1(`ui_titlebar.go:10,12`)
- 太阳月亮 24 viewBox / stroke 2,显示 14(`ui_titlebar.go:18,66`)
- `drawClose` stroke 1.3(`ui_controls.go:56`)、`drawCopy` 1.2(`:67`)
- 详情关闭 18、Ping 关闭 16、Tracert 关闭 16、复制 18

交互形态:

- Ping / Tracert 用右上角 ✕ 关面板
- telnet 用左上文字按钮「断开」关同一类面板
- `关闭详情` 的 tooltip 是「关闭」,与两个 pane 的 tooltip 完全相同,悬停时无法预知关的是哪一层

## 方案

- 图标统一 16,内部图形按 24 viewBox 的 1.5 线宽等比;窗口按钮单独保留 10/1
- 三个 docked pane 统一用 ✕,tooltip 分别为「关闭 Ping 结果」「关闭远程终端」「关闭路由跟踪」
- 补 `复制 MAC 地址` 中间的空格(`ui_detail.go:112`)

## 验收

- 图标尺寸枚举 ≤ 3、线宽枚举 ≤ 3
- 三个 pane 的关闭控件形态一致,tooltip 互不相同
