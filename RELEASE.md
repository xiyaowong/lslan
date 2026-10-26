# 发布新版本

以 0.2.0 为例。

1. **CHANGELOG.md** 顶部加一节,标题就是版本号(`## 0.2.0`),写用户能懂的变化;`mygo build` 从这里取 release notes,没这节构建直接失败
2. **mygo.json** 的 `version` 改成同一版本号
3. `just check` 过了再提交
4. 打 tag 推送:`git tag v0.2.0 && git push origin v0.2.0`
5. workflow 自动构建并上传签名归档、delta、`update-windows-amd64.json` 到 **draft** release,然后自动 publish。产物清单即应用内自动更新的 feed,release 一发布,已装用户就能收到更新弹窗
6. 检查 release 页面产物齐全(Setup exe、归档、delta、update 清单)

## 约定

- tag 与 mygo.json 版本必须一致(去掉 `v` 前缀);不一致时 feed 解析不到,已装用户收不到更新
- **签名私钥**只在 GitHub Actions secret `MYGO_UPDATER_PRIVATE_KEY` 里,公钥在 mygo.json;丢了只能换钥匙重发新版,存量用户只能手动装
- 版本号语义化:换版本号用户才会收到更新(feed 只认更新的版本)
- tag 打在 main 上;`--latest=false` 恒定,latest 指向应用 release 而非仓库其他发布

## 换签名钥匙(除非私钥泄露,不做)

`go tool mygo keygen` 生成新对,公钥写进 mygo.json 的 `updates.publicKey`,私钥更新 secret;从新版起旧钥匙作废,存量用户手动装一次新版后恢复自动更新。
