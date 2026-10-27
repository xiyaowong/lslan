仓库里有不该在的东西

## 清单(已核实)

| 项 | 证据 | 处置 |
|---|---|---|
| `lslan.exe`(根目录,20MB) | 被 `.gitignore` 忽略的旧产物 | 删 |
| `build/.old-*\`、`build/windows-amd64\` | 0.1.0 的产物,mygo.json 已是 0.1.5 | 删 |
| `.gitignore:5` `/tmp/` | 仓库没有 tmp 目录 | 删 |
| `release.yml:13-14` `concurrency` | 无 `cancel-in-progress`, tag 触发本身不会并发 | 补 `cancel-in-progress: true` 或删 |
| `justfile:25` `run *args` | 只认 `mdns`,其他参数被静默忽略 | 改成显式参数 |

## 验收

- 仓库内无 >1MB 的非必要产物
- `just run foo` 会报错而不是静默忽略
