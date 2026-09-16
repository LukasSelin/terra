package sysmem

import (
	"syscall"
	"unsafe"
)

var globalMemoryStatusEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

// memoryStatusEx is MEMORYSTATUSEX.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

// Free is how much a program may still allocate on this machine: what
// is left to commit, which is where an allocation fails, but never more than
// the machine has in it - a world made in the page file is not finished.
func Free() (uint64, bool) {
	var m memoryStatusEx
	m.length = uint32(unsafe.Sizeof(m))
	if ok, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); ok == 0 {
		return 0, false
	}
	return min(m.availPageFile, m.totalPhys), true
}
