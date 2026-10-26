package ui

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"
)

// fakeUpdate records one install.
type fakeUpdate struct {
	ver       string
	installed bool
	progress  []float64
}

func (f *fakeUpdate) version() string { return f.ver }
func (f *fakeUpdate) install(_ context.Context, progress func(int64, int64)) error {
	f.installed = true
	if progress != nil {
		progress(1, 4)
		progress(4, 4)
	}
	return nil
}

// fakeUpdater feeds checkForUpdate a scripted answer.
type fakeUpdater struct {
	upd updateIface
	err error
}

func (f *fakeUpdater) check(context.Context) (updateIface, error) { return f.upd, f.err }

// drainUpdate waits for the check goroutine to report back.
func drainUpdate(t *testing.T, a *app) {
	t.Helper()
	a.runPending()
}

// TestAboutPopover: the about popover shows the version and the check, and
// the update flow walks check → found → install → note.
func TestAboutPopover(t *testing.T) {
	a := newApp()
	upd := &fakeUpdate{ver: "0.2.0"}
	a.updater.checkFn = &fakeUpdater{upd: upd}
	tt := ui.NewTester(a.View, 1040, 720)

	if err := tt.Click("关于"); err != nil {
		t.Fatalf("open about: %v", err)
	}
	tt.Frame()
	if !tt.HasText("检查更新") {
		t.Errorf("check button missing, texts %q", tt.Texts())
	}
	if err := tt.Click("检查更新"); err != nil {
		t.Fatalf("click check: %v", err)
	}
	drainUpdate(t, a)
	tt.Frame()
	if !tt.HasText("安装 0.2.0") {
		t.Errorf("install button missing, texts %q", tt.Texts())
	}
	if err := tt.Click("安装 0.2.0"); err != nil {
		t.Fatalf("click install: %v", err)
	}
	drainUpdate(t, a)
	tt.Frame()
	if !upd.installed {
		t.Error("update not installed")
	}
	if !tt.HasText("已更新到 0.2.0") {
		t.Errorf("install note missing, texts %q", tt.Texts())
	}

	// Up to date lands its note.
	a2 := newApp()
	a2.updater.checkFn = &fakeUpdater{}
	tt2 := ui.NewTester(a2.View, 1040, 720)
	if err := tt2.Click("关于"); err != nil {
		t.Fatalf("open about: %v", err)
	}
	if err := tt2.Click("检查更新"); err != nil {
		t.Fatalf("click check: %v", err)
	}
	drainUpdate(t, a2)
	tt2.Frame()
	if !tt2.HasText("已是最新版本") {
		t.Errorf("up-to-date note missing, texts %q", tt2.Texts())
	}

	// A failed check says so.
	a3 := newApp()
	a3.updater.checkFn = &fakeUpdater{err: errors.New("offline")}
	tt3 := ui.NewTester(a3.View, 1040, 720)
	if err := tt3.Click("关于"); err != nil {
		t.Fatalf("open about: %v", err)
	}
	if err := tt3.Click("检查更新"); err != nil {
		t.Fatalf("click check: %v", err)
	}
	drainUpdate(t, a3)
	tt3.Frame()
	if !tt3.HasText("检查失败") {
		t.Errorf("failure note missing, texts %q", tt3.Texts())
	}
}
