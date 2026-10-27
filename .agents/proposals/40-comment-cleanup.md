注释过多,而且有一部分是错的

## 现状

注释行占比(纯 `//` 行 / 总行数):

```
app.go       123 / 868
ui_view.go    91 / 557
ui_wake.go    71 / 439
scanner.go    59 / 385
ui_detail.go  48 / 375
mdns.go       34 / 309
```

其中相当一部分是复述签名或代码,例如:

- `app.go:38` `// pingStats is the running tally…`(类型名已说明)
- `app.go:257` `// NewApp creates the app state…`
- `ui_view.go:15-16`、`:48-49`、`:95-97` 等整段签名复述
- `ui_detail.go:50-53`、`ui_ports.go:91`、`ui_controls.go:50,59` 同类

另有**错误或过期**的注释(优先级更高):

| 位置 | 问题 |
|---|---|
| `internal/ui/ui_trace.go:87-88` | 说 trace「reuses the ping pane's terminal」,但该字段从无构造,是死代码 |
| `internal/scan/range.go:35` | 说 `be32` 返回 host-order,实际是 BigEndian |
| `internal/scan/mdns.go:61-64` | 说值以 "n" 开头也算关闭,代码只认 `no` |
| `internal/ui/ui_ports.go:34` | 注释说「generic last」,通用动作早已删除 |
| `internal/scan/scan.go:5-8`、`winapi.go:12-15` | 说「golang.org/x/sys 不在模块图里」,实际 go.mod 有(间接依赖) |
| `internal/scan/mdns.go:107-108` | 说「joins it on the sweep's NIC」,实际 `nic.Index` 从不赋值 |
| `internal/ui/ui_view.go:105-106` | 病句,且与 `cellLocked` 重复 |

## 应当保留的注释(这条最重要)

只有这几类值得留:

1. **不显然的取舍与坑**:排序并列时按 IP 防跳动、选中行用 tint 绕过泛洪、终端无自高、TUN 源地址绑定
2. **平台/API 坑**:Windows 结构体 union、错误码 111/122/206、`CREATE_NO_WINDOW`、ghostty-vt.dll、`SO_REUSEADDR`
3. **契约与调用约束**:事件必须持续排空、仅主线程、代际丢弃、取消才杀进程
4. **线格式偏移**:NBNS、telnet、魔术包
5. **用户可见行为的原因**:能一句话说清的那些

## 验收

- 注释行 / 代码行 < 10%
- 上表错误注释全部修正或删除
- 保留清单里的注释一条不减
