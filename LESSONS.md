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
- just 的非 `[script]` 配方里,pwsh 的 `exit 1` 传不出去(实测多行与单行都是 0)→ 要么写成 `[script('pwsh','-NoProfile','-Command')]`,要么把多行用 `;` 串成一行并在最后一条命令带出退出码;`#!/usr/bin/env pwsh` 这种 shebang 在 Windows 上还要 cygpath,别用
- 多行配方在 just 里是合成为同一份脚本的,变量可以跨行;但**逐行分句**执行时不能——`just run` 曾因此从不复制 DLL、mDNS 开关从不生效
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
- 表格为涂抹选中行会把主题的 `Accent` 覆写成行底色(`chosenRow`)→ **表内任何前景色都不能用 `t.Accent`**,否则在选中行上就是同色写字(浅色下 1:1 直接隐形);独立 token 见 `kindInk`/`chosenMuted`
- 主题对比度测试要成对覆盖「卡片上」和「选中行上」,且只测 muted 不够:tint 改写了哪些字段,就得有对应断言,否则配色失误没有测试能发现
- 模态框还开着时 `c.Toast` 会落在遮罩后面 → 在 `ui.Modal(...)` 之后发,或把话带出来让调用处发

## 引擎与数据流

- 引擎每次上报 host 都会整体覆盖 `model.Host`,它不知道端口扫描 → 端口结果单独缓存(`a.ports[ip]`),只在开始扫描时清空
- 离线回填把扫描范围里每个没应答的地址都填成 `StateOffline`,所以"离线"不等于"这台有设备"(一个 C 段能有 240 多个空地址)→ 要挑真实设备时以有 MAC 为准,别拿离线当条件
- 装了代理/ VPN 的 TUN 网卡时,不绑定源地址的 TCP 连接会被 TUN 接走并"秒成功"(源地址是 198.18.x.x),于是任何地址的任何端口都显示开放 → 探测局域网要 `LocalAddr` 绑到所选网卡自己的地址(`scan.dialFrom`);ICMP 走 IP 层不受影响
- 底部交互区(Ping/Tracert)的列表各有自己的 `ui.ListState`——两个面板同帧共享一个 state 直接崩;启动任一工具时把其他工具的面板整个关掉
- 控制台子进程(tracert 等)要 `SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}`(CREATE_NO_WINDOW),否则闪一个黑框到前台
- 国内设备自报的是内部型号(`MBC31`、`WS7200`、`TL-XDR3010`),不含产品词,靠关键词猜类型覆盖率很低 → 唯一靠谱的是 UPnP 的 `friendlyName`;`AX3000`/`BE3600` 是 Wi-Fi 代际+速率,多家共用,只能说"是路由器"
- 帧级 mock 编排(多 fake 会话/流配对喂样本)成本很快超过保护价值 → 状态断言能过的就别全帧断言
- 扫描判在线有三条路(ICMP、ARP 回填的 TCP 探测、mDNS),而唤醒等待只看 ICMP → 只答 TCP 的设备表格写"在线"、等待却永远过不去;任何"等某设备上线"的判定都要和扫描同标准(`probeHostFallback`)
- 对已在线设备"等它上线"没有可点亮的行,结果只会是它本来就那样 → 发信号前先看状态,已在线就直说,别让用户等 30 秒再无声消失
