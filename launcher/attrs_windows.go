//go:build windows

package main

import "syscall"

// hideFolder sets the Windows "hidden" attribute on a folder, the same way
// %APPDATA%\.minecraft-style data folders stay out of sight.
func hideFolder(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		return err
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		return nil
	}
	return syscall.SetFileAttributes(p, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}

// unhideFolder clears the hidden attribute (used when the setting is off).
func unhideFolder(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		return err
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN == 0 {
		return nil
	}
	return syscall.SetFileAttributes(p, attrs&^uint32(syscall.FILE_ATTRIBUTE_HIDDEN))
}
