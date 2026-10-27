深色主题里 toast 没有边界,终端也不是同一套配色

## 现状

- `internal/ui/ui_theme.go:59-60` `Inverse = #2b2b2b`,与深色卡片的 `Surface` 完全相同
- `internal/ui/ui_ping.go:54-60`、`ui_telnet.go:41-47` 用 `terminal.DarkTheme()`/`LightTheme()`,是另一套色值(`#18181b` / `#ffffff`)
- `internal/ui/app.go:299-304` `setDark` 只更新 `a.pingTerm`

## 实测

深色 toast 对卡片 **1.00:1**——唤醒结果、复制提示落在卡片上时只剩一圈阴影,没有边界。浅色下是 12.85:1,正常。

开着 telnet 会话切换深浅色,telnet 终端保持旧外观,一块深色终端留在浅色面板里。

## 方案

1. 深色 `Inverse` 改 `#45484d`(白字对它有 9.18),或给 toast 加 1px 描边
2. 终端配色改成从 `appTheme` 生成(`terminal.Theme{Foreground: t.Text, Background: t.Surface, ...}`)
3. `setDark` 同时更新 ping 与 telnet 终端

## 验收

- 深色下 toast 与卡片之间能看出边界(≥1.5 或有描边)
- 切换深浅后,正在显示的 ping 与 telnet 终端同时换色
