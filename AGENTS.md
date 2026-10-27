# AGENTS.md

代理必读。坑见 [LESSONS.md](LESSONS.md)。

## 项目

lslan — 家庭局域网 IP 扫描工具(Windows)。

- 技术栈:mygo native UI。文档 https://mygo.egoist.dev/docs/ui ;API 以模块缓存源码为准,定位用 `go list -m -f '{{.Dir}}' github.com/egoist/mygo`(不要写死版本号)
- 运行时零污染:不写注册表/磁盘,数据内嵌。引入写入即违约
- 契约:`internal/model`、`internal/scan` 公开类型与导出函数被 UI 直接依赖,改动须全仓同步(含 `main.go`)
- 备注功能不做
- 终端插件靠 ghostty-vt.dll(Ghostty 的终端仿真,运行时从缓存加载):`go build` 出的裸 exe 找不到它会自己下载,`just run`/`just build` 都会摆好;只看源码可不管它
- 界面:Fluent 2 质感,深浅主题切换(默认跟随系统),组件大气不拥挤(斟酌 gap/padding/margin),自定义标题栏;用词避免术语化,基础水平用户可循操作;窗口固定尺寸下所有文字完整可见

## 命令

用 justfile:`just dev` / `just check`(fmt+vet+test) / `just build` / `just run`(可选 `mdns` 参数) / `just clean` / `just release-check <版本>`(发版前核对版本一致)

`just run` 构建到固定路径 `build/test/lslan-test.exe` 再启动(防火墙授权按 exe 路径记,固定路径授权一次),默认关 mDNS(并发跑会抢 UDP 口),`just run mdns` 保留;顺便把 ghostty-vt.dll 从缓存复制到 exe 旁

## 结构

- `main.go` — 仅启动窗口
- `internal/ui` — app 状态与视图(命令栏/表格/详情面板)
- `internal/model` — 共享类型
- `internal/scan` — 引擎:网卡、ICMP、ARP、OUI、端口、Ping

## 工作方式

- 开发流程:issues-PR 制——需求点发 GitHub issues(gh),每项工作开分支、PR 合并,留痕防上下文丢失;主分支只经 PR 更新
- 审查提案带 `proposal` 标签,发布后**只监听、不执行**:issue 上有一个 👍 或评论区明确批准才动手(见 `.agents/proposal-workflow.md`)
- 不重复造轮子:优先成熟第三方库,除非引入收益不大或包维护有风险
- 按需查知识,边做边查,不预读通框架;子代理同理
- 并行子代理:主线程先做脚手架 + `go get` + 契约存根(build 通过)再派发;子代理不改 go.mod 与契约;**每个代理必须用独立 worktree(EnterWorktree/isolation: worktree),绝不多代理共用一个 checkout**——两个代理同写一个工作区会互相覆盖文件(已发生一次,损失半天)
- 测试先评估必要性:只测核心功能,前期优先实现而非 TDD
- 所有文字(文档、教训、注释)用能表达清楚的最少字数,表达方式与内容匹配
