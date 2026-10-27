端口扫描没有代际控制,换设备后既不补扫也不解释

## 现状

- `internal/ui/app.go:772-798` 手动端口扫描没有 cancel 出口,也没有 gen
- `internal/ui/ui_detail.go:143` 扫描中按钮只置灰,没有原因说明
- `internal/ui/ui_view.go:419-430` `portScanning` 为真时 `autoPortScan` 直接 return

## 后果

A 的端口扫描在跑时选中 B:

- `autoPortScan(B)` 立即返回且不再重试,B 显示「还没扫描过」
- B 的「端口扫描」按钮是禁用态,却没有任何文字解释
- A 扫完后结果仍写入 `a.ports`,此时若已开始新扫描,结果会被清空

## 方案

- 手动扫描加 `portScanGen` 或按 IP 的在飞集合
- 换设备时记录一次待扫,B 在 A 完成后自动补上
- 按钮禁用时给 Tooltip「正在扫描上一台,稍等」
- 提供取消入口

## 验收

- 切到 B 后 A 完成时自动补扫 B
- 新扫描启动后旧的端口结果被丢弃
- 扫描中可从界面结束
