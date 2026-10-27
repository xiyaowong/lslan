很多端口显示英文服务名,而且没有任何下一步

## 现状

- `internal/scan/ports.go:44-104` 服务名表里是 `printer`、`netbios-ssn`、`adb`、`dvr` 这类英文
- `internal/ui/ui_ports.go:92-110` `portLabel` 未命中时直接返回英文服务名
- 未映射端口没有动作(`portActions` 返回空),导致「选中设备就崩」那条 panic

## 方案

- 常见项补中文:打印服务、文件共享、数据库、音视频
- 未知显示「其他服务」,原始服务名放 tooltip
- 无动作时给「复制地址」(`http://ip:port`)

## 风险

- 过度翻译会掩盖信息 → 原文放 tooltip 即可

## 验收

- 9100 显示「打印服务」,悬停能看到 `printer`
- 任一未映射端口都有至少一个动作
