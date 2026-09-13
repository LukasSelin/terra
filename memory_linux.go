package terra

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// freeMemory is MemAvailable: what the kernel reckons can be handed out
// without swapping.
func freeMemory() (uint64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		rest, ok := strings.CutPrefix(s.Text(), "MemAvailable:")
		if !ok {
			continue
		}
		kb, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}
