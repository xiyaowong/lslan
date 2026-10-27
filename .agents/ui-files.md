## internal/ui 的文件划分

包名已经是 `ui`,文件名不再重复它。去掉 `ui_` 前缀,并让每个文件的名字说清它装什么。

| 文件 | 内容 |
|---|---|
| `app.go` | `app` 状态、启动、窗口与主题、网卡 |
| `view.go` | 整窗构建、命令栏、表格、状态栏 |
| `detail.go` | 详情面板与三个工具面板的正文 |
| `range.go` | 八格地址输入:解析、锁定、裁剪、上限 |
| `scan.go` | 扫描生命周期、事件折叠、取消、变化提示 |
| `hosts.go` | 主机排序、计数、复制清单、本机判定 |
| `ping.go` / `telnet.go` / `trace.go` | 三个工具各自的状态机 |
| `ports.go` / `portactions.go` | 端口扫描 / 端口到下一步动作的映射 |
| `terminal.go` | 终端插件封装(ping 与 telnet 共用) |
| `netbios.go` / `kind.go` / `wake.go` / `update.go` | 各自一项能力 |
| `theme.go` / `controls.go` / `titlebar.go` | 外观、通用控件、标题栏 |

判断归属时只看一件事:这个函数属于哪个领域。`app` 的方法可以分散在任何文件里,不需要和类型定义放在一起。
