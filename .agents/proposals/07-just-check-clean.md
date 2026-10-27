just check 不校验格式,just clean 无产物时报错

## 现状

实测两处:

- `justfile:15` `gofmt -l .` —— gofmt 只列出文件,**退出码恒为 0**。故意放一个未格式化文件,`just check` 依然成功
- `justfile:35` `Remove-Item -Recurse -Force build` —— `build/` 不存在时退出 1

`AGENTS.md:18` 说 check 是「fmt+vet+test」,与事实有差距。

## 方案

```
check:
    $files = gofmt -l .
    if ($files) { $files; exit 1 }
    go vet ./...
    go test ./...

clean:
    if (Test-Path build) { Remove-Item -Recurse -Force build }
```

## 验收

- 留一个未格式化文件时 `just check` 失败并列出该文件
- 没有 `build/` 时 `just clean` 成功
