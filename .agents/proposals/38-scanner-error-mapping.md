用户分不清是范围写反、正在扫描,还是地址非法

## 现状

- `internal/ui/app.go:487-490` 三种不同原因都变成「无法开始扫描,请检查网卡后重试」
- `internal/scan/scanner.go:68-88` 返回的是 `errInvalidAddress`、`errors.New("a sweep is already running")`、`"start address is after end address"`
- `internal/scan/scan.go:20` 定义了错误事件类型,但**全仓库没有任何地方发出它**
- `internal/ui/app.go:531` 消费错误事件的分支永远不可达

## 方案

- 导出哨兵错误(`ErrBusy`、`ErrRangeReversed`),UI 用 `errors.Is` 映射成三句不同的、可行动的文案
- 扫描内部 panic 时发出错误事件(配合「崩溃一」的 recover)
- `ListNICs` 失败按错误码区分(拒绝访问 / 其他)

## 验收

- 三种启动失败在界面上显示三条不同的、告诉用户下一步的文案
- 人为让扫描 panic 时界面显示错误且进程存活
