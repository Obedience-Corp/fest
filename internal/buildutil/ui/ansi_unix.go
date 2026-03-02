//go:build unix

package ui

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// TIOCGWINSZ is the ioctl command to get window size (macOS/Darwin)
const TIOCGWINSZ = 0x40087468

// TermWidth returns the terminal width
func TermWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var width int
		fmt.Sscanf(cols, "%d", &width)
		if width > 0 {
			return width
		}
	}

	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	ws := &winsize{}

	_, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(os.Stdout.Fd()),
		uintptr(TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)

	if (err != 0 || ws.Col == 0) && isatty() {
		_, _, err = syscall.Syscall(syscall.SYS_IOCTL,
			uintptr(os.Stderr.Fd()),
			uintptr(TIOCGWINSZ),
			uintptr(unsafe.Pointer(ws)),
		)
	}

	if err != 0 || ws.Col == 0 {
		return 80
	}
	return int(ws.Col)
}
