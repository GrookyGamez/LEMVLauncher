//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW keeps a console window from flashing up if we ever fall
// back to java.exe (javaw.exe is a GUI binary and has no console anyway).
const createNoWindow = 0x08000000

func setProcAttrs(cmd *exec.Cmd) {
	// NOTE: do NOT set HideWindow here. Go turns that into
	// STARTF_USESHOWWINDOW + SW_HIDE in the child's STARTUPINFO, and LWJGL
	// shows the game window with ShowWindow(SW_SHOWDEFAULT), which means
	// "use the value the parent passed in STARTUPINFO". The result is a game
	// that runs perfectly with an invisible window.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
