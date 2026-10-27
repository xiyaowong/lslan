崩溃二:选中 Chromecast/Apple TV/打印机就闪退

## 现状

详情面板「网络服务」区对每个自报服务渲染一个动作按钮,但直接取动作列表的第 0 项:

- `internal/ui/ui_services.go:69-71` `acts := portActions(...)` 后 `acts[0].Label`
- `internal/ui/ui_ports.go:34-59` `portActions` 只覆盖 80/443/554/445/139/21/22/23/3389,其余端口返回 nil

## 复现

`_googlecast._tcp`(8009)、`_airplay._tcp`(5000/7000)、`_printer._tcp`(9100)、`_ipp._tcp`(631)等服务的端口都不在映射表里,`acts` 为空切片。选中这类设备:

```
panic: runtime error: index out of range [0] with length 0
lslan/internal/ui.(*app).serviceSection.func1.1()
	G:/lslan/internal/ui/ui_services.go:70
```

Chromecast、Apple TV、AirPlay 音箱、网络打印机都在其中,正是家庭环境最常见的设备。

## 方案

1. `len(acts) == 0` 时不画按钮,只显示服务名与端口号
2. 可选:给未映射端口一个通用动作「复制地址」(`http://ip:port`),与端口行的兜底语言一致
3. 补一个 `serviceSection` 的帧级测试(目前完全没有测试)

## 验收

- 构造 `Services: [{_googlecast._tcp, 8009}]` 与 `[{_printer._tcp, 9100}]` 后渲染不 panic,行内只有服务名和端口
- 上述用例进测试
