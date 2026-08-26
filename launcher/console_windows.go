//go:build windows

package main

import (
	"os"
	"syscall"
)

// attachConsole re-attaches the GUI-subsystem exe to the console it was
// started from so --list / --play can print there.
func attachConsole() {
	k := syscall.NewLazyDLL("kernel32.dll")
	attach := k.NewProc("AttachConsole")
	r, _, _ := attach.Call(^uintptr(0)) // ATTACH_PARENT_PROCESS
	if r == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		cliOut = f
	}
}
