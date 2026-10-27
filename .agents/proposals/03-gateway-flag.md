网关永远读不到:标志位写成了 WINS

## 现状

`internal/scan/winapi.go:34`:

```go
gaaIncludeGateways = 0x40
```

对照 `golang.org/x/sys/windows/types_windows.go:2062-2069`:

```
GAA_FLAG_INCLUDE_PREFIX   = 0x10
GAA_FLAG_INCLUDE_WINS_INFO = 0x40   ← 现在实际请求的是这个
GAA_FLAG_INCLUDE_GATEWAYS  = 0x80   ← 应该是这个
```

MSDN 明确 `FirstGatewayAddress` 只在设置 `GAA_FLAG_INCLUDE_GATEWAYS` 时有效,所以 `nic.go:124-127` 读到的永远是 NULL。

## 影响

本机实测(有默认网关 192.168.3.1):

```
WLAN    192.168.3.205  gateway=invalid IP
Mihomo  198.18.0.1     gateway=invalid IP
```

`model.NIC.Gateway` 恒为空,`Host.Gateway` 恒为 false,详情面板和表格里的「网关」徽章**从上线起就没出现过**。`scanner.go:112-115` 的网关标记逻辑整段是死代码。

## 方案

1. 常量改成 `0x80`
2. 加一个常量断言测试,防止再抄错(见「测试补强:Windows ABI 布局断言」)
3. `ListNICs` 的 live 测试补断言:在有默认网关的网卡上 `Gateway.IsValid()`

## 验收

- 实机 `ListNICs` 返回的 WLAN 网卡 `Gateway` 为 `192.168.3.1`
- 扫描结果里该网段路由器一行出现「网关」徽章
