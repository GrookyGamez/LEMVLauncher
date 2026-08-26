//go:build !windows

package main

// Hidden attributes are a Windows concept; elsewhere these are no-ops.
func hideFolder(path string) error   { return nil }
func unhideFolder(path string) error { return nil }
