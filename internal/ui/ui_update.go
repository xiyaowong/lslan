package ui

import (
	"context"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// updateIface stands in for mygo's *Update in tests.
type updateIface interface {
	version() string
	install(ctx context.Context, progress func(downloaded, total int64)) error
}

type realUpdate struct{ u *mygo.Update }

func (r realUpdate) version() string { return r.u.Version }
func (r realUpdate) install(ctx context.Context, progress func(int64, int64)) error {
	return r.u.Install(ctx, progress)
}

// updaterIface is the check entry, so tests fake the feed.
type updaterIface interface {
	check(ctx context.Context) (updateIface, error)
}

type realUpdater struct{}

func (realUpdater) check(ctx context.Context) (updateIface, error) {
	u, err := mygo.Updater.Check(ctx)
	if u == nil {
		return nil, err
	}
	return realUpdate{u}, err
}

// update state lives on the app: one check at a time, the found version
// waiting for the user's click, the install carrying its progress. The
// feed is asked off the interface thread, like the dial of the telnet
// test; states change only through dispatch.
type updateState struct {
	checkFn    updaterIface
	busy       bool // a check is in flight
	found      updateIface
	installing bool    // an install is in flight
	progress   float64 // downloaded / total while installing
	note       string  // the outcome of the last check or install
	failed     bool    // note is a failure
}

// checkForUpdate asks the update feed; "已是最新版本" lands in note, a
// newer version waits in found for its install row.
func (a *app) checkForUpdate() {
	u := &a.updater
	if u.busy || u.installing {
		return
	}
	if u.checkFn == nil {
		u.checkFn = realUpdater{}
	}
	u.busy, u.note, u.failed = true, "", false
	go func() {
		upd, err := u.checkFn.check(context.Background())
		a.dispatch(func() {
			u.busy = false
			switch {
			case err != nil:
				u.note, u.failed = "检查失败，网络可能不通", true
			case upd != nil:
				u.found = upd
				u.note = ""
			default:
				u.note, u.failed = "已是最新版本", false
			}
		})
	}()
}

// installUpdate swaps the found version in place, reporting progress; the
// restart question goes to a native dialog once the install lands, since
// quitting ends this process either way.
func (a *app) installUpdate() {
	u := &a.updater
	if u.found == nil || u.installing {
		return
	}
	u.installing, u.note, u.failed, u.progress = true, "", false, 0
	upd := u.found
	go func() {
		err := upd.install(context.Background(), func(downloaded, total int64) {
			a.dispatch(func() {
				if total > 0 {
					u.progress = float64(downloaded) / float64(total)
				}
			})
		})
		a.dispatch(func() {
			u.installing = false
			if err != nil {
				u.note, u.failed = "安装没有完成，可稍后再试", true
				return
			}
			u.found = nil
			u.note, u.failed = "已更新到 "+upd.version()+"，重启后生效", false
			if a.win != nil {
				go mygo.Dialog.Message(mygo.MessageOptions{
					Parent:  a.win,
					Type:    mygo.MessageInfo,
					Title:   "更新完成",
					Message: "新版本已就绪，现在重启吗？",
					Buttons: []string{"现在重启", "下次启动生效"},
				})
			}
		})
	}()
}

// updateRow is the 关于 line's second half: the check button, then the
// state of the check or install it led to. Dev builds cannot update
// themselves (mygo.Updater says so), so the row only shows there.
func (a *app) updateRow(c *ui.Context, t *ui.Theme) {
	u := &a.updater
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Box(c).Grow(1)
		switch {
		case u.installing:
			ui.Progress(c, u.progress).Width(120).Height(3).Shrink(0)
			ui.Text(c, "正在下载更新…").FontSize(12).TextColor(t.TextMuted)
		case u.busy:
			ui.Spinner(c)
			ui.Text(c, "正在检查更新…").FontSize(12).TextColor(t.TextMuted)
		case u.found != nil:
			if flatButton(c, "安装 "+u.found.version()).Clicked() {
				a.installUpdate()
			}
			if u.note != "" {
				ui.Text(c, u.note).FontSize(12).TextColor(t.TextMuted)
			}
		default:
			if u.note != "" {
				color := t.TextMuted
				if u.failed {
					color = t.Danger
				}
				ui.Text(c, u.note).FontSize(12).TextColor(color)
			}
			if flatButton(c, "检查更新").Clicked() {
				a.checkForUpdate()
			}
		}
	})
}
