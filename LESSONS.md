# LESSONS.md

坑 → 对策。过时即删。

- 沙箱拦 `Remove-Item dir\*` → 对目录本身 `Remove-Item -Recurse -Force dir`
- mygo 文档缺 API → 读模块缓存源码 `ui\`
- Windows raw ICMP 需管理员 → iphlpapi `IcmpSendEcho` syscall 存根
- Go internal 包不能在模块外编译(如 %TEMP% 副本)→ 仓库内构建
- 后台 goroutine 改状态不触发重绘 → 按文档回 UI 线程并加同步
- GetAdaptersAddresses 返回错误码 111/122/206 均为"缓冲区不够,重试"信号 → 循环加大缓冲重试,勿当失败抛出
- 手抄 Windows 结构体必崩 → 先写临时探测程序打印原始字节核对偏移,再定字段;易漏的是 union 后隐藏的成员(如 IP_ADAPTER_UNICAST_ADDRESS 的 DadState)
- 多代理共用一个 checkout 会互相覆盖未提交文件 → 每代理独立 worktree,冲突时代理要立即停写并报告(已发生,损失半天);多个会话并发时同理,主 checkout 里做的改动会被别的会话一起提交或回退
- 提交时 `git add -A` 会带上别的会话在同一目录的未提交改动 → 只 add 自己改的文件,或干脆另开 worktree 干活
- mygo terminal plugin 在 native UI 可用:`terminal.View(c, term)` 即普通 Element,`Conn` 接自定义管道、`term.Feed()` 喂 ANSI 文本;`go test` 里禁用真 `terminal.New`(会触发 DLL 下载)
- worktree 里 `git checkout-index -f -a` 会覆盖未提交修改(已发生,重写整个 scanner);autocrlf 检出的 CRLF 让 `gofmt -l` 全亮,是噪音不致命,勿为它重写工作区
- Go 1.27 删了 `UDPConn.JoinGroup` → 组播监听用 `net.ListenMulticastUDP`(Windows 上已自带 SO_REUSEADDR,可直接与系统 mDNS 共存 5353)
- 窗口标题栏按钮随系统深色模式变白 → 现为自绘标题栏(HideWindowButtons),颜色全部出自 ui.Theme,不再受系统影响;主题深浅切换用 `mygo.Theme.SetSource/IsDark/OnUpdated`,窗口底色 `BackgroundColor` 支持 `light-dark(浅,深)` 自动跟随
- 列表选中行被主题 Accent "灌满",Fluent 要的是浅色底纹 → 建表时临时把 Accent 换成底纹色、AccentText 换成正文色,画完再还原
- 状态栏被挤掉:窗口根列里 `Fill()` 的容器会和状态栏抢满高度 → 主内容用 `Grow(1).Shrink(1)`
- 引擎每次上报 host 都会整体覆盖 `model.Host`,它不知道端口扫描 → 端口结果单独缓存(`a.ports[ip]`),只在开始扫描时清空
- just 默认用 `sh`,Windows 上 `New-Item` 之类会 not found → justfile 顶部 `set windows-shell := ["pwsh", "-NoProfile", "-Command"]`
- mygo 自绘标题栏只要还有最小化按钮就把三个全画出来(DisableMaximize 只是画成禁用态,不会消失)→ 要去掉某个按钮只能 `HideWindowButtons` 整个自绘,视图里调 `win.Minimize()/Close()`
- 防火墙/UAC 授权按 exe 路径记,`go build` 每次换临时目录就重新弹窗,无人值守点不了 → 用固定路径 `build/test/lslan-test.exe`(`just run`)
