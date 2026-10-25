# AGENTS.md

代理必读。坑见 [LESSONS.md](LESSONS.md)。

## 项目

lslan — 家庭局域网 IP 扫描工具(Windows)。需求:[需求.md](需求.md)。

- 技术栈:mygo native UI。文档 https://mygo.egoist.dev/docs/ui ;API 以模块缓存源码为准 `D:\go\pkg\mod\github.com\egoist\mygo@v0.6.4\ui\`
- 运行时零污染:不写注册表/磁盘,数据内嵌。引入写入即违约
- 契约:`internal/model`、`internal/scan` 公开签名冻结,改动须同步 `main.go`
- 备注功能不做
- 界面:始终浅色主题,微信风格,组件大气不拥挤(斟酌 gap/padding/margin),自定义标题栏;用词避免术语化,基础水平用户可循操作;默认窗口尺寸下所有文字完整可见

## 命令

用 justfile:`just dev` / `just check`(fmt+vet+test)/ `just build` / `just clean`

## 结构

- `main.go` — 仅启动窗口
- `internal/ui` — app 状态与视图(工具栏/表格/详情面板)
- `internal/model` — 共享类型
- `internal/scan` — 引擎:网卡、ICMP、ARP、OUI、端口、Ping

## 工作方式

- 开发流程:issues-PR 制——需求点发 GitHub issues(gh),每项工作开分支、PR 合并,留痕防上下文丢失;主分支只经 PR 更新
- 按需查知识,边做边查,不预读通框架;子代理同理
- 并行子代理:主线程先做脚手架 + `go get` + 契约存根(build 通过)再派发;子代理不改 go.mod 与契约
- 测试先评估必要性:只测核心功能,前期优先实现而非 TDD
- 所有文字(文档、教训、注释)用能表达清楚的最少字数,表达方式与内容匹配
