有一类测试在保护「已经不存在的东西」

## 规则

某处从显示 A 改成显示 B 后,测试被写成「不能有 A 且必须有 B」。这是冗余:代码库里已经没有 A 了,`HasText("A") == false` 只在防回退,真正有价值的是断言 B。

## 具体清单

**删掉(纯冗余或恒真):**

| 位置 | 问题 |
|---|---|
| `internal/scan/probe_test.go` 整个文件 | 只 `t.Logf`,永不失败,是调试残留 |
| `internal/ui/netbios_test.go:161` | 断言三个生产中**根本不存在的字符串**(「正在询问/没有报出/没有问到」),永远为真 |
| `internal/ui/sweep_ports_test.go:44` | 第 47 行已正向断言端口出现 |
| `internal/ui/sweep_ports_test.go:117` | 111/114 已正向断言 |
| `internal/ui/view_test.go:133-137` | 断言「没有右键菜单」,但代码里根本没有菜单实现 |
| `internal/ui/trace_test.go:143,152` | 与 155-161 的直接状态断言重复 |
| `internal/ui/sort_test.go:32` | 测一个 `"ports"` 列,而 `sortHosts` 没有该分支 |

**改断言高度(不必全删):**

- `internal/ui/sweep_ports_test.go:81-83,180`、`:108` 直接读 `a.ports` 缓存键
- `internal/ui/view_test.go:140,199,202,237` 直接写 `a.selRow`/`a.selIP`,依赖行序,改为点 IP 文本
- `internal/ui/ping_hide_test.go:25-28` 手工构造「已停止的 ping」,应由 start + stop 生成
- `internal/ui/sweep_ports_test.go:154-171` 遍历 `a.tree.Rows()/Item`

**保留(不是冗余):**

- `ping_hide_test.go:54`、`trace_test.go:87` 的「运行中没有关闭按钮」——这是状态规格,A 仍在代码里
- `ping_hide_test.go:40,66` 关闭后消失——消失就是该功能的正面行为

## 顺带

15 处 `time.Sleep` 轮询,建议统一 `waitFor(t, cond)` 帮助函数;`update_test.go:37-40` 只 `runPending` 一次就断言,依赖调度,有偶发风险。

## 验收

- `grep HasText` 中不再出现「仅否定且被正向覆盖」的断言
- `probe_test.go` 删除
- `go test ./...` 与 `go test -race ./internal/ui` 通过
