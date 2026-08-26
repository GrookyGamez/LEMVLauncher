//go:build !windows

package main

// The graphical launcher is Windows-only; other platforms get the CLI.
func runUI(L *Launcher) {
	runCLI(L, []string{"--help"})
	runCLI(L, []string{"--list"})
}
