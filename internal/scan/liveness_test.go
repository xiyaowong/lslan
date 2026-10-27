package scan

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
)

// TestRefusedIsAlive: a host that refuses a connection is up — it just has
// nothing listening. Windows reports WSAECONNREFUSED (10061), which does
// not match syscall.ECONNREFUSED, so the check needs the Windows spelling.
func TestRefusedIsAlive(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close() // nothing listens now: the host refuses

	src := netip.MustParseAddr("127.0.0.1")
	c, err := dialFrom(t.Context(), src, src, port, time.Second)
	if c != nil {
		c.Close()
		t.Fatal("nothing should be listening")
	}
	if !refused(err) {
		t.Errorf("a refused connection was not recognised: %v", err)
	}
}

// TestTimeoutIsNotAlive: silence is not proof of life.
func TestTimeoutIsNotAlive(t *testing.T) {
	if refused(context.DeadlineExceeded) {
		t.Error("a timeout counts as refused")
	}
	if refused(nil) {
		t.Error("a nil error counts as refused")
	}
}
