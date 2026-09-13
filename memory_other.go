//go:build !windows && !linux

package terra

// freeMemory cannot be read here, so only the program's memory limit is
// held against a world.
func freeMemory() (uint64, bool) { return 0, false }
