取消扫描没有反馈,而且取消后还会开始扫端口

## 现状

- `internal/ui/ui_view.go:151-154` 点「取消扫描」只调 `scanner.Cancel()`,不改任何界面状态
- `internal/scan/scanner.go:92-100` Cancel 不等待、不清 `running`
- `internal/ui/app.go:525-530` 收到完成事件后无条件 `if a.withPorts { a.sweepPorts() }`

## 后果

1. 点「取消扫描」后按钮文字不变、进度条冻结、再点无效(`cancel` 已置 nil),直到引擎走完 `names.wait()` 才有反应——全程没有「正在停止」的反馈
2. 取消产生的完成事件与正常完成无法区分,勾了「顺带扫描端口」时,**取消反而会开始逐台扫端口**

## 方案

1. UI 记录 `cancelled` 标志(或引擎在完成事件里给出 `Cancelled bool`)——前者不需要改契约
2. 点取消立即进入 `stopping` 态:按钮变「正在停止…」且禁用
3. 取消后不跑 `sweepPorts`
4. 结束后状态栏给一句「已取消,扫到 N 个地址」

## 验收

- 取消后同一帧按钮文字变化且不可点
- 取消一次后 `sweepLeft == 0`,没有端口探测 goroutine
- 有可见的「已取消」结果
