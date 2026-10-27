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
	waited := make(chan error, 1)
	go func() { waited <- wait() }()
	time.Sleep(700 * time.Millisecond) // let the child start
	cancel()
	select {
	case err := <-waited:
		// The process was killed, so the wait must say so rather than
		// report a clean exit.
		if err == nil {
			t.Error("a killed trace returned no error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the trace never returned after cancel")
	}

	// The tree kill runs alongside cmd.Wait, so tracert may outlive the
	// shell by a moment: poll rather than assume it is already gone.
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, _ := exec.Command("tasklist", "/FI", "IMAGENAME eq tracert.exe").Output()
		if !strings.Contains(string(out), "tracert.exe") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("tracert.exe survived the cancel")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
