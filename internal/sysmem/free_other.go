//go:build !windows && !linux

package sysmem

// freeMemory cannot be read here, so only the program's memory limit is
// held against a world.
func Free() (uint64, bool) { return 0, false }
