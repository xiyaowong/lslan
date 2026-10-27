发布流程没有门禁:不跑测试、不校验版本一致

## 现状

`.github/workflows/release.yml` 只有三步:checkout → setup-go → `mygo build -upload` → 无条件 publish。

- 不跑 `just check` / `go test`
- 不校验 tag 与 `mygo.json` 版本、`CHANGELOG.md` 首个章节三者一致
- 不校验 tag 打在 main 上
- publish 无条件执行

`RELEASE.md:14` 自己描述了后果:「tag 与 mygo.json 版本必须一致,不一致时 feed 解析不到,已装用户收不到更新」——但没有任何机制拦截。更糟的是坏代码会直接对全体用户发布。

## 方案

1. workflow 加一步 `just check`(在 build 之前)
2. 新增 `just release-check`,比较:
   - `git describe --tags` / `GITHUB_REF_NAME` 去掉 `v`
   - `mygo.json` 的 `version`
   - `CHANGELOG.md` 第一个 `## ` 标题
   三者必须相同,否则非零退出
3. 校验 tag 是 main 的祖先:`git merge-base --is-ancestor <tag> origin/main`
4. publish 步骤依赖产物存在(或显式判断)

## 验收

- 故意把 `mygo.json` 版本改成旧值时,CI 在 build 前失败
- 本地可跑 `just release-check` 并返回非零
