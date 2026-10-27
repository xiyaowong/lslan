一批符号没有读者,或永远不会发生

## 已核实(全部自行 grep 确认过)

| 符号 | 证据 | 处置 |
|---|---|---|
| `app.presetNames` | `app.go:170,707-709` 只写不读 | 删 |
| `app.traceDone` | `app.go:156`、`ui_trace.go:136` 只写不读 | 删 |
| `app.traceTerm` | `app.go:157`、`ui_trace.go:109-139` **只有 `= nil`,从无构造** | 删字段、4 处分支、165-170 死分支、87-88 错误注释 |
| `model.Host.LastOctet` | `model.go:58-66` 全仓 0 调用,注释引用的「主机号」列不存在 | 删(契约变更,需同步 AGENTS) |
| `model.Host.Ports` | `model.go:51` 无读写 | 删或注明保留原因 |
| `model.Progress.Online` | 由 `scanner.go:370` 生产,UI 用 `onlineCount()` 重算 | 删或让 UI 使用 |
| `scan.EventError` | `scan.go:20`,**全仓无生产者**;`app.go:531` 分支不可达 | 要么真正发出(见错误映射条),要么删 |
| `NIC.Index` | `model.go:12`,从未赋值;`mdns.go:113` 的 `Index > 0` 恒假 | 见「mDNS 未绑定网卡」条 |
| `Preset.Description` | `ports.go:21,29-40` 无读取 | 删 |
| `netbiosState.failed` | 除测试外无读者 | 删 |
| `app.parseRange` | `app.go:414-419` 纯转发 `fullRange` | 删,调用点直接用 `fullRange` |
| `ui_controls.go:10-11,15` | `b := ...; return b` 无意义局部变量 | 内联 |

## 注意

`nic.go:28` 的 `Description *uint16` 是 ABI 占位字段,**删了布局会崩**,不是死代码。

## 验收

- 上表符号 grep 后无读取点
- `go vet` 与 `go test ./...` 通过
- 契约变更在 AGENTS.md 里注明
