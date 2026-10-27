文档里有已经不对的说法

## 逐条

| 位置 | 问题 |
|---|---|
| `AGENTS.md:26` | 说 internal/ui 是「工具栏/表格/详情面板」,实际叫 command bar,且已有 9 个文件 |
| `AGENTS.md:9` | 硬编码 `D:\go\pkg\mod\github.com\egoist\mygo@v0.6.4\ui\`,mygo 一升级就失效 |
| `AGENTS.md:20` | 声称「just run 默认关 mDNS」,因 shell 问题不成立 |
| `AGENTS.md:13` | 「运行时从缓存加载」不够准:裸 exe 会现场下载 |
| `LESSONS.md:39` | 说三个工具各有自己的 `ui.ListState`,telnet 并没有降级列表,trace 的终端字段是死的 |
| `RELEASE.md:5` | 「没这节构建直接失败」——只在签名构建时成立 |
| `release.yml:4-5` vs `:32-36` vs `RELEASE.md:9` | 三处对「是否自动 publish」说法不一 |
| `AGENTS.md:11` | 说公开签名冻结,但 `ScanPorts`、`Host.Ports`、`Host.LastOctet`、`Progress.Online` 已是死 API |

## 方案

逐条改到与代码一致;`AGENTS.md:9` 改成 `go list -m -f '{{.Dir}}' github.com/egoist/mygo`(仓库自带 skill 已是这么写的)。

## 验收

- 上述每条与代码一致
- 新增 `just release-check` 后可机器校验版本一致性
