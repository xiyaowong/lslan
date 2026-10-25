# LESSONS.md

坑 → 对策。过时即删。

- 沙箱拦 `Remove-Item dir\*` → 对目录本身 `Remove-Item -Recurse -Force dir`
- mygo 文档缺 API → 读模块缓存源码 `ui\`
- Windows raw ICMP 需管理员 → iphlpapi `IcmpSendEcho` syscall 存根
- Go internal 包不能在模块外编译(如 %TEMP% 副本)→ 仓库内构建
- 后台 goroutine 改状态不触发重绘 → 按文档回 UI 线程并加同步
- GetAdaptersAddresses 返回错误码 111/122/206 均为"缓冲区不够,重试"信号 → 循环加大缓冲重试,勿当失败抛出
- 手抄 Windows 结构体必崩 → 先写临时探测程序打印原始字节核对偏移,再定字段;易漏的是 union 后隐藏的成员(如 IP_ADAPTER_UNICAST_ADDRESS 的 DadState)
- 多代理共用一个 checkout 会互相覆盖未提交文件 → 每代理独立 worktree,冲突时代理要立即停写并报告(已发生,损失半天)
- mygo terminal plugin 在 native UI 可用:`terminal.View(c, term)` 即普通 Element,`Conn` 接自定义管道、`term.Feed()` 喂 ANSI 文本;`go test` 里禁用真 `terminal.New`(会触发 DLL 下载)
- worktree 里 `git checkout-index -f -a` 会覆盖未提交修改(已发生,重写整个 scanner);autocrlf 检出的 CRLF 让 `gofmt -l` 全亮,是噪音不致命,勿为它重写工作区
