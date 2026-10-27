NetBIOS 名会串到别的设备

## 现状

`netbiosState.name` 是单个字符串,渲染时完全不看当前选中的是哪台设备:

- `internal/ui/ui_netbios.go:150-153` `name string` / `failed bool`
- `internal/ui/ui_netbios.go:201-209` `netbiosFact` 直接渲染 `a.netbios.name`
- `internal/ui/ui_netbios.go:158-195` 每次询问都覆盖同一个字段

## 复现

已用测试证实:先问 A(记录 `asked[A]`),切到 B 询问,切回 A 时 A 不再询问,随后 B 的答案落到 `netbios.name`,A 的详情就显示 **B 的 NetBIOS 名**并长期保持。

```
A 的详情显示 "DEVICE-B"  ← 实际属于 B
```

相关问题:
- `busy` 会让快速切换时第二个设备的询问被静默丢弃
- `asked` 永不复位,重新扫描后设备改名也不会再问

## 方案

1. 名字按地址存:`map[netip.Addr]string`(或 `nameIP + name`),渲染时按当前 IP 取
2. `asked` 在每次新扫描时清空
3. `busy` 改为按地址排队,或允许并发少量询问

## 验收

- A/B 交替选中,各自显示自己的名字
- 未问到的地址显示「—」
- 新扫描后 `asked` 为空,会重新询问
