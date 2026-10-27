该用等宽数字的地方没用,不该用的用了

## 规则

项目里数字统一用 `FontFeatures("tnum")`:IP、MAC、端口、计数、统计都守了。

## 漏掉的

- `internal/ui/ui_detail.go:96,108` 「上次在线」的 `10-26 15:04` —— 每分钟变化时宽度会跳
- `internal/ui/ui_wake.go:437` 状态栏「正在等 192.168.3.20 上线」里的 IP —— 与表格里的同款 IP 字体不同
- `internal/ui/ui_view.go:184` 「共 N 个地址」

## 用多了的

- `internal/ui/ui_view.go:321` 版本号 `1.2.3` —— 是文本不是量,用了 tnum 反而与相邻说明文不同步

## 方案

- 给上述三处补 `FontFeatures("tnum")`
- 版本号去掉 tnum(或整行都加,二选一)

## 验收

- 检索含数字的文本调用,要么带 tnum,要么注释说明为何不带
- 两帧比较「上次在线」文字宽度不变
