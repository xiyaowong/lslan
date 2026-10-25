package scan

import (
	"os"
	"testing"
	"time"
	"unsafe"
)

func TestListNICsProbe(t *testing.T) {
	wd, _ := os.Getwd()
	t.Logf("cwd=%q", wd)
	flags := uintptr(gaaIncludePrefix | gaaIncludeGateways)
	var size uint32
	buf := []byte{}
	for attempt := 0; attempt < 5; attempt++ {
		var bufPtr uintptr
		if len(buf) > 0 {
			bufPtr = uintptr(unsafe.Pointer(&buf[0]))
		}
		sizePtr := uintptr(unsafe.Pointer(&size))
		ret, _, _ := procGetAdaptersAddresses.Call(afInet, flags, 0, bufPtr, sizePtr)
		t.Logf("attempt=%d ret=%d size=%d buflen=%d", attempt, ret, size, len(buf))
		if ret == 0 {
			return
		}
		if size == 0 {
			size = 15000
		}
		buf = make([]byte, size)
		time.Sleep(50 * time.Millisecond)
	}
}
