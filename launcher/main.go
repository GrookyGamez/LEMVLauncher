package main

import (
	"os"
	"path/filepath"
)

func exeDir() string {
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		return filepath.Dir(exe)
	}
	wd, _ := os.Getwd()
	return wd
}

// migrateOldLayout moves the old flat folder set (v1.2 and earlier dropped
// eight folders right next to the exe) into the single LEMV data folder.
// Only runs when there is clear proof the old layout belongs to this launcher.
func migrateOldLayout(oldRoot, dataDir string) bool {
	if oldRoot == dataDir {
		return false
	}
	ours := fileExists(filepath.Join(oldRoot, "launcher-settings.json")) ||
		fileExists(filepath.Join(oldRoot, "logs", "launcher.log"))
	if !ours {
		return false
	}
	os.MkdirAll(dataDir, 0o755)
	moved := false
	for _, item := range []string{"versions", "libraries", "assets", "natives", "runtimes", "instances", "meta", "logs", "launcher-settings.json"} {
		src := filepath.Join(oldRoot, item)
		dst := filepath.Join(dataDir, item)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			continue // never clobber anything already in the new spot
		}
		if os.Rename(src, dst) == nil {
			moved = true
		}
	}
	return moved
}

func main() {
	base := exeDir()
	root := filepath.Join(base, "LEMV")
	migrated := false
	if v := os.Getenv("LEMV_ROOT"); v != "" {
		root = v // test override: use the given folder directly
	} else {
		migrated = migrateOldLayout(base, root)
	}
	L := newLauncher(root)
	if migrated {
		L.Logf("moved the old flat folder layout into %s", root)
	}
	if L.Settings.HideDataFolder {
		if err := hideFolder(root); err != nil {
			L.Logf("couldn't hide %s: %v", root, err)
		}
	} else {
		unhideFolder(root)
	}
	defer func() {
		if L.logFile != nil {
			L.logFile.Close()
		}
	}()

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "--cli", "--play", "--list", "--help", "-h", "--signin":
			runCLI(L, args)
			return
		}
	}
	runUI(L)
}
