崩溃一:扫单地址时局域网有 mDNS 设备就闪退

## 现状

被动 mDNS 监听会收到局域网上**任何**设备的组播,而合并回调不检查来源地址是否在本次扫描范围内,直接写进主机表:

- `internal/scan/mdns.go:185-207` 被动 socket 收包,只过滤端口,不判断源 IP
- `internal/scan/scanner.go:140-150` 合并回调无条件 `hosts[key] = h`
- `internal/scan/scanner.go:357` 离线回填按 `hi-lo+1-len(known)` 预分配,bing `known` 包含越界主机
- `internal/scan/scanner.go:284-296` 生成 `known`

## 复现

扫描范围设成单个地址(界面允许起始=结束),同时局域网里有 ≥2 台会发 mDNS 的设备(打印机、电视、音箱)。越界主机进入 `hosts` 后 `len(known)` 大于范围内地址数,容量为负:

```
panic: runtime error: makeslice: cap out of range
```

panic 发生在扫描 goroutine 里,没有 recover,**整个应用退出**。

即使不崩,越界设备也会出现在表格里,并让「在线」「已发现」计数超过「总数」。

## 方案

1. 合并回调按 `key < lo || key > hi` 丢弃,与 ARP 一致性回填(`scanner.go:250-257`)保持一致
2. `offlineBackfill` 容量取 `max(0, ...)`,不预分配负数
3. sweep 加 `defer recover()`,异常时发出错误事件而不是让进程退出

## 验收

- 单地址范围 + 3 台越界 mDNS 主机的单元测试不 panic、不发出越界主机
- 「在线 ≤ 总数」恒成立
- 人为制造 sweep panic 时应用存活并给出提示
