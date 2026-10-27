自绘控件缺少可读名称,点击目标偏小

## 现状

- `internal/ui/ui_titlebar.go:43-44,51-66,75-87` 用 `ui.Button(c, "")` 再塞一个 `ui.Icon(...).Label(...)` 子节点
- mygo 的 Button 是 leaf 节点,名称取 `innerText()`(只收 Text),**图标的 Label 被丢弃**
- 主题按钮只剩 tooltip 的描述;最小化、关闭既无 Label 也无 Tooltip,读屏读出空按钮
- 关闭按钮还进入 Tab 序列,Enter 即关窗
- `iconButton` 尺寸 16/18、`flatButton` 高度约 24,都低于 24×24 的点击目标建议

## 方案

- Label 直接设在 Button 上:`b.Label("最小化")`;caption 按钮补 Tooltip
- `iconButton` 最小 24×24;`flatButton` 垂直内边距提到 ≥6
- 关闭按钮移出默认 Tab 序列,或加确认

## 验收

- 无障碍树里三个按钮名称分别为「切换深浅颜色」「最小化」「关闭」
- 所有图标按钮边界 ≥24×24
