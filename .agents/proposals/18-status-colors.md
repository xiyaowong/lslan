状态的颜色语言自相矛盾

## 现状

- `internal/ui/ui_view.go:495` 主行状态「在线」文字用 `TextMuted`(灰)
- `internal/ui/ui_view.go:492` 端口行「开放」文字用 `Success`(绿)
- `internal/ui/ui_view.go:275` 状态栏「在线」的绿点恒为 `true`,在线数为 0 时也是绿的
- `internal/ui/ui_view.go:501-509` 未知状态的点用 `Border`(1.24:1,等于不存在)
- 三处状态圆点尺寸分别是 7 / 8 / 10

用户的感受:「在线」是灰的,「开放」是绿的,但左边那个点是绿的——两种状态语言。

## 方案

- 状态列文字统一:在线用 `Success`,离线用 `TextMuted`;或文字一律 `Text`,只用点表达
- 状态栏绿点改为 `onlineCount() > 0 ? Success : TextMuted`,或干脆去掉
- 未知状态的点改 `TextMuted`
- 圆点尺寸统一为 8(状态栏 7 可保留,但要注释说明)

## 验收

- 同一个列里「在线/离线/开放」三态截图,文字颜色只为两类语义
- 在线数为 0 时状态栏不再显示绿点
