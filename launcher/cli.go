package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// cliOut is where command-line output goes. On Windows the GUI build has no
// console, so we try to attach to the parent's console (see console_windows.go).
var cliOut io.Writer = os.Stdout

func runCLI(L *Launcher, args []string) {
	attachConsole()
	out := cliOut
	switch args[0] {
	case "--help", "-h":
		fmt.Fprintf(out, "%s %s\n\n", launcherName, launcherVersion)
		fmt.Fprintln(out, "  LEMVLauncher.exe                    open the launcher window")
		fmt.Fprintln(out, "  LEMVLauncher.exe --list             show every version and whether its jar is present")
		fmt.Fprintln(out, "  LEMVLauncher.exe --play <id> <name> launch a version from the command line")
		return
	case "--list":
		if add, dates := L.ExpandFromManifest(); len(add) > 0 {
			L.AddEntries(add, dates)
			L.Rescan()
		}
		for t := 0; t < TabCount; t++ {
			n, total := L.ReadyCount(t)
			fmt.Fprintf(out, "\n[%s]  %d/%d jars\n", Tabs[t].Name, n, total)
			for _, e := range L.EntriesForTab(t) {
				mark := "  "
				if e.Ready() {
					mark = "OK"
				}
				fmt.Fprintf(out, "  %s  %-22s %-22s %s\n", mark, e.Name, e.JarName(), e.Note)
			}
		}
		fmt.Fprintf(out, "\njars folder: %s\n", L.VersionsDir)
		return
	case "--signin":
		if !msaEnabled {
			fmt.Fprintln(out, "Microsoft sign-in isn't available in this build. Play offline by passing a username.")
			return
		}
		cid := L.Settings.MSAClientID
		if cid == "" {
			cid = "test-client-id"
		}
		dc, err := StartDeviceLogin(cid)
		if err != nil {
			fmt.Fprintln(out, "device code error:", err)
			os.Exit(1)
		}
		fmt.Fprintf(out, "enter code %s at %s\n", dc.UserCode, dc.VerificationURI)
		acc, err := PollDeviceLogin(cid, dc, func(m string) { fmt.Fprintln(out, " ", m) })
		if err != nil {
			fmt.Fprintln(out, "sign-in error:", err)
			os.Exit(1)
		}
		fmt.Fprintf(out, "signed in as %s (uuid %s)\n", acc.Name, acc.UUID)
		L.Settings.Account = acc
		L.SaveSettings()
		return
	case "--cli", "--play":
		if len(args) < 2 {
			fmt.Fprintln(out, "usage: --play <version id> [username]")
			return
		}
		id := args[1]
		// always expand: it fills in the rest of the old eras and lets Mojang
		// serve any archived build it turns out to host
		if add, dates := L.ExpandFromManifest(); len(add) > 0 {
			L.AddEntries(add, dates)
		}
		L.Rescan()
		name := L.Settings.Username
		if len(args) > 2 {
			name = args[2]
		}
		if name == "" {
			name = "Player"
		}
		e := L.FindEntry(id)
		if e == nil {
			fmt.Fprintf(out, "unknown version %q (try --list)\n", id)
			os.Exit(2)
		}
		last := ""
		res, err := L.Launch(e, name, func(msg string, frac float64) {
			if msg != last {
				fmt.Fprintln(out, msg)
				last = msg
			}
		})
		if err != nil {
			fmt.Fprintln(out, "error:", err)
			os.Exit(1)
		}
		fmt.Fprintf(out, "running %s (pid %d), game log: %s\n", res.VersionID, res.Cmd.Process.Pid, res.LogPath)
		code, err := res.Wait()
		if err != nil {
			fmt.Fprintln(out, "wait failed:", err)
			os.Exit(1)
		}
		fmt.Fprintf(out, "%s exited with code %d\n", res.VersionID, code)
		if code != 0 {
			os.Exit(code)
		}
	default:
		fmt.Fprintf(out, "unknown option %s (try --help)\n", strings.Join(args, " "))
	}
}
