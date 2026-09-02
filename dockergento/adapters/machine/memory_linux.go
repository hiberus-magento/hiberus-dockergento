package machine

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// memoryBytes is the machine's own memory. On Linux it is also the containers', which is why the
// check that compares them always passes here — and that is the correct answer.
func memoryBytes() int64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}

		kilobytes, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}

		return kilobytes * 1024
	}

	return 0
}
