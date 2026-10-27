TCP 兜底把「拒绝连接」当成不在线,防火墙设备被永久漏报

## 现状

`internal/scan/icmp.go:93-100`:

```go
if c, err := dialFrom(...); err == nil {   // 只有成功才算活
```

`scanner.go:273` 的一致性回填复用同一逻辑。

## 问题

Windows 默认防火墙会拦入站 ICMP,而 80/443/445 通常也关着。此时设备会回 TCP RST(`ECONNREFUSED`)——**这是主机活着的确定性证据**,但代码把它当失败,继续试下一个端口,最终判离线。

结果:开了防火墙、又不跑 Web/文件共享的电脑和 NAS **永远显示离线**。这类设备在家庭网络里很常见。

## 方案

新增内部判定函数(不改公开签名):

```go
// dialVerdict: nil 或 ECONNREFUSED/WSAECONNRESET 都说明主机在
func dialVerdict(err error) bool
```

`tcpAliveFrom` 在第一个端口得到「拒绝」时即可返回 true,无需再试其余端口——顺带把一致性回填的探测成本降到 1/3。

## 验收

- 对一个监听后关闭的端口,`tcpAliveFrom` 返回 true
- 单元测试覆盖 refused / timeout / unreachable 三种情形
