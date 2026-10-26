# LESSONS.md

坑 → 对策。过时即删。

## 工具链与协作

- 多代理共用一个 checkout 会互相覆盖未提交文件 → 每代理独立 worktree,冲突时代理要立即停写并报告(已发生,损失半天);多个会话并发时同理,主 checkout 里做的改动会被别的会话一起提交或回退
- worktree 里 `git checkout-index -f -a` 会覆盖未提交修改(已发生,重写整个 scanner)
- 提交时 `git add -A` 会带上别的会话在同一目录的未提交改动 → 只 add 自己改的文件,或干脆另开 worktree 干活
- 沙箱拦 `Remove-Item dir\*` → 对目录本身 `Remove-Item -Recurse -Force dir`
- autocrlf 检出的 CRLF 让 `gofmt -l` 全亮,是噪音不致命,勿为它重写工作区
- Go internal 包不能在模块外编译(如 %TEMP% 副本)→ 仓库内构建
- Go 1.27 删了 `UDPConn.JoinGroup` → 组播监听用 `net.ListenMulticastUDP`(Windows 上已自带 SO_REUSEADDR,可直接与系统 mDNS 共存 5353)
- just 默认用 `sh`,Windows 上 `New-Item` 之类会 not found → justfile 顶部 `set windows-shell := ["pwsh", "-NoProfile", "-Command"]`
- 防火墙/UAC 授权按 exe 路径记,`go build` 每次换临时目录就重新弹窗 → 用固定路径 `build/test/lslan-test.exe`(`just run`)
- `Remove-Item Env:\VAR` 在变量不存在时仍报错退出 → 先 `Test-Path Env:\VAR` 再删
- 终端插件的 ghostty-vt.dll 加载顺序:exe 旁 → 用户缓存 `<LocalAppData>/mygo/natives/`(`go test`/`go run` 会自动下载到这,`mygo build` 放进产物)→ 裸 exe 现场下载;justfile 手动构建时要自己从缓存补到 exe 旁

## Windows API

- Windows raw ICMP 需管理员 → iphlpapi `IcmpSendEcho` syscall 存根
- GetAdaptersAddresses 返回错误码 111/122/206 均为"缓冲区不够,重试"信号 → 循环加大缓冲重试,勿当失败抛出
- 手抄 Windows 结构体必崩 → 先写临时探测程序打印原始字节核对偏移,再定字段;易漏的是 union 后隐藏的成员(如 IP_ADAPTER_UNICAST_ADDRESS 的 DadState)

## mygo / UI

- mygo 文档缺 API → 读模块缓存源码 `ui\`
- 后台 goroutine 改状态不触发重绘 → 按文档回 UI 线程并加同步
- mygo terminal plugin 在 native UI 可用:`terminal.View(c, term)` 即普通 Element,`Conn` 接自定义管道、`term.Feed()` 喂 ANSI 文本;`go test` 里禁用真 `terminal.New`(会触发 DLL 下载);终端配色自带 `UseTheme(Light/DarkTheme)`
- 状态栏被挤掉:窗口根列里 `Fill()` 的容器会和状态栏抢满高度 → 主内容用 `Grow(1).Shrink(1)`
- 窗口标题栏按钮随系统深色模式变白 → 现为自绘标题栏(HideWindowButtons),颜色全部出自 ui.Theme,不再受系统影响;主题深浅切换用 `mygo.Theme.SetSource/IsDark/OnUpdated`,窗口底色 `BackgroundColor` 支持 `light-dark(浅,深)` 自动跟随
- mygo 自绘标题栏只要还有最小化按钮就把三个全画出来(DisableMaximize 只是画成禁用态,不会消失)→ 要去掉某个按钮只能 `HideWindowButtons` 整个自绘,视图里调 `win.Minimize()/Close()`

## 引擎与数据流

- 引擎每次上报 host 都会整体覆盖 `model.Host`,它不知道端口扫描 → 端口结果单独缓存(`a.ports[ip]`),只在开始扫描时清空
- 离线回填把扫描范围里每个没应答的地址都填成 `StateOffline`,所以"离线"不等于"这台有设备"(一个 C 段能有 240 多个空地址)→ 要挑真实设备时以有 MAC 为准,别拿离线当条件
- 装了代理/ VPN 的 TUN 网卡时,不绑定源地址的 TCP 连接会被 TUN 接走并"秒成功"(源地址是 198.18.x.x),于是任何地址的任何端口都显示开放 → 探测局域网要 `LocalAddr` 绑到所选网卡自己的地址(`scan.dialFrom`);ICMP 走 IP 层不受影响
- 底部交互区(Ping/Telnet/Tracert)的降级列表各有自己的 `ui.ListState`——两个面板同帧共享一个 state 直接崩;启动任一工具时把其他工具的面板整个关掉
- 控制台子进程(tracert 等)要 `SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}`(CREATE_NO_WINDOW),否则闪一个黑框到前台
- 帧级 mock 编排(多 fake 会话/流配对喂样本)成本很快超过保护价值 → 状态断言能过的就别全帧断言
