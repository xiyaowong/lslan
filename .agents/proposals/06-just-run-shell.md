just run 的跨行变量全部失效

## 现状

`justfile:25-31` 的 `run` 任务分多行写,但 just 每行启动**独立 shell**,变量不跨行:

```
$dll = ...                          ← 下一行就没了
Copy-Item $dll.FullName ...         ← 永远不执行
$env:LSLAN_MDNS = "0"               ← 设完即随进程退出
Start-Process build/test/...        ← 继承不到环境变量
```

已实测确认(最小 justfile:上一行的 `$x` 下一行读到空;`$env:` 同样不跨行)。

## 后果

- `build/test/ghostty-vt.dll` **从未被复制过**(实际检查:文件不存在,而缓存里有一份)
- `just run mdns` 与 `just run` 行为完全相同,`AGENTS.md:20` 承诺的「默认关 mDNS」不成立

## 方案

把整段合成一条 `[script]` 行(用 here-string 或 `;` 串联),或给这个 recipe 指定 pwsh 脚本解释器(`set script-interpreter := ["pwsh","-NoProfile"]`),并在注释里写明「just 每行独立 shell,变量不跨行」。

## 验收

- 执行 `just run` 后 `build/test/ghostty-vt.dll` 存在
- 直接运行该 exe 时进程内 `LSLAN_MDNS` 为 `0`;`just run mdns` 时不设置
- `LESSONS.md` 记一条这个坑
