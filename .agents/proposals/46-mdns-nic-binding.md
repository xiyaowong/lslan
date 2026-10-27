被动 mDNS 收包落在默认接口上

## 现状

- `internal/scan/nic.go:113-118` 构造 NIC 时**没有设 `Index`**
- `internal/scan/mdns.go:111-117` `if nic.Index > 0 { iface, _ = net.InterfaceByIndex(...) }` —— 永远为假,`iface` 是 nil
- `internal/scan/nic.go:132-137` 「按 Index 排序」实际是稳定排序(全 0)

Go 的 `ListenMulticastUDP` 在 iface 为 nil 时加入默认接口。

## 后果

本机有 Mihomo TUN 作为默认路由时,被动监听会绑到**错误接口**,局域网组播可能收不到,与用户选中的网卡不符——正是 LESSONS 里记录的 TUN 干扰一类。

## 方案

- 从 `ipAdapterAddresses.Union` 的第二个 DWORD 读 IfIndex 填进 `nic.Index`(字段已存在,无契约变更)
- 读不到时按源地址匹配接口,而不是 nil
- 补排序测试

## 验收

- 真实网卡 `Index > 0`,`net.InterfaceByIndex` 能解析
- 多网卡机器上 mDNS 只在所选网卡收包
