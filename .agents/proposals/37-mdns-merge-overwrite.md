同一 IP 的多次 mDNS 响应会让主机名来回翻转

## 现状

- `internal/scan/scanner.go:152-155` 任何非空 mDNS 名都覆盖现有名
- `internal/scan/mdns.go:299-309` `appendService` 按名字去重,端口只补 0,从不更新
- `internal/scan/mdns.go:238-262` 只读 `msg.Answer`,忽略 `Additional`

## 问题

1. 同一 IP 上有两台响应者报不同名字时(双栈、旧记录、虚拟化),名称会在每次响应之间**来回翻转**
2. 设备更正端口后(如 8080→80),已有条目永不更新
3. 按 DNS-SD 惯例,SRV/PTR 常放在 Additional 段,只读 Answer 会丢掉大量真实服务

注意:「mDNS 名优先于反向 DNS」是刻意的(反向 DNS 不覆盖已有名),但「mDNS 之间无条件覆盖」不是。

## 方案

- 名字只在为空时写入,与反向 DNS 的处理对称
- `appendService` 在端口不同且新端口非 0 时更新
- `msg.Extra` 一并纳入 PTR/SRV 解析

## 验收

- 两次不同名字的合并后,保留首个非空值
- 端口更新有测试
- Additional 段样本能被解析
