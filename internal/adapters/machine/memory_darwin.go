package machine

import "golang.org/x/sys/unix"

// memoryBytes is the machine's own memory. On macOS this is emphatically not the containers':
// the daemon reports whatever the virtual machine was given, and the gap between the two is what
// the diagnosis exists to say out loud.
func memoryBytes() int64 {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}

	return int64(total)
}
