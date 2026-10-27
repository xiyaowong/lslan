扫描一开始,之前在跑的工具就失去了所有可见入口

## 现状

- `internal/ui/app.go:469-501` 开始扫描只清空 `hosts`/`ports`/`selIP`,不停止任何工具
- `internal/ui/ui_detail.go:131-157` 停止入口只在选中设备时出现
- `internal/ui/ui_detail.go:53-61` 工具面板按 IP 可见

## 后果

扫描 A 时正在 Ping 或保持 telnet 会话,点「开始扫描」后详情面板关闭、状态栏无任何提示,而:

- ping 仍在每秒发 ICMP
- telnet 连接还开着
- tracert 进程还在跑

要停只能重新选中 A;如果 A 已不在新结果里(设备下线),就完全无法停止。

## 方案

二选一:

1. 开始扫描时统一结束所有工具,并 toast 一句「已停止 Ping / 远程终端 / 路由跟踪」
2. 状态栏常驻一个「正在 Ping 192.168.1.9」+ 停止入口

## 验收

- 扫描启动后 `pingCancel == nil`、`telnetTerm == nil`、`traceCancel == nil`,且用户看到过提示
- 或存在全局停止入口
