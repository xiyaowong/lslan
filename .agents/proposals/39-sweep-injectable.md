最高风险的代码没有任何测试

## 现状

- `internal/scan/probe_test.go` 整个文件只 `t.Logf`,永远通过
- `internal/scan/scanner.go` 直接调用包级 `probeHost`、`readARPTable`、`startMDNS`,无法注入假实现
- 现有 `-race` 运行只扫了 `127.0.0.1` 一个地址,**没有覆盖并发扫描路径**

## 零覆盖清单

- `nic.go`:结构体偏移/大小、Flags 常量、网关(正是「网关标志」那条 bug)、Virtual 排序
- `icmp.go`:TCP 兜底的三种判定、ICMP 可用性分支
- `scanner.go`:取消、完成、离线回填、已知/未知合并、主机名回调
- `arp.go`:`readARPTable` 本身、重试路径
- `mdns.go`:取消、越界过滤、Additional 解析
- `wol.go`:`SendWake` 的目标与失败路径

## 方案

1. 用编译期布局断言替换 `probe_test.go`:

```go
var _ = [1]struct{}{}[unsafe.Sizeof(ipAdapterAddresses{})-offsetof...]
```

比较 `unsafe.Sizeof`/`Offsetof` 与已知值,布局漂移时测试直接失败
2. 把 `probeHost` / `readARPTable` 收成包内可替换的小接口
3. 用 4 个地址的小范围 + 假探测覆盖取消、完成、离线回填、越界过滤

## 验收

- 布局断言失败即测试失败
- 4 地址假探测的单元测试覆盖上述路径
- `go test -race` 真正跑到并发扫描
