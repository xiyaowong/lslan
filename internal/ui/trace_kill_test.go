package ui

import (
	"context"
	"net/netip"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestTraceTreeKilledLive: stopping a route trace takes the whole process
// tree with it. Cancelling the context killed only cmd.exe, leaving
// tracert.exe to run out its 15 hops on its own.
func TestTraceTreeKilledLive(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a real tracert")
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, wait, err := realTrace(ctx, netip.MustParseAddr("192.0.2.1")) // never answers
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = wait() }()
	time.Sleep(700 * time.Millisecond) // let the child start
	cancel()
	time.Sleep(1200 * time.Millisecond) // and the tree kill land

	out, _ := exec.Command("tasklist", "/FI", "IMAGENAME eq tracert.exe").Output()
	if strings.Contains(string(out), "tracert.exe") {
		t.Error("tracert.exe survived the cancel")
	}
}
