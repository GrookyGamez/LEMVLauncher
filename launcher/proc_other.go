//go:build !windows

package main

import "os/exec"

func setProcAttrs(cmd *exec.Cmd) {}
