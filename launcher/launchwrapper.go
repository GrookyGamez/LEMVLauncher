package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// Mojang launches every applet-era version (Pre-Classic through 1.5.2) through
// its own net.minecraft.launchwrapper with the AlphaVanillaTweaker. That wrapper
// is broken in ways that matter: Classic and Indev save slots don't work, so
// generating a level runs the generator and then goes nowhere; the game is
// hosted in a Java AWT frame, which breaks mouse input; and skins and sounds
// point at servers that no longer exist. Upstream bug: MCL-1448.
//
// MCPHackers' LaunchWrapper is a drop-in replacement that fixes all of that, so
// LEMV substitutes it whenever a version JSON asks for Mojang's.
const (
	mojangWrapperMain = "net.minecraft.launchwrapper.Launch"
	mcpWrapperMain    = "org.mcphackers.launchwrapper.Launch"
	mcpWrapperLib     = "org.mcphackers:launchwrapper:1.3.0"
)

func launchWrapperMaven() string {
	return envOr("LEMV_LAUNCHWRAPPER_MAVEN", "https://maven.glass-launcher.net/releases/")
}

// needsLaunchWrapper reports whether this version is launched through Mojang's
// applet wrapper, and so is one of the versions the replacement fixes.
func needsLaunchWrapper(v *VersionJSON) bool {
	return v != nil && v.MainClass == mojangWrapperMain
}

// rewriteWrapperArgs converts a legacy argument template for MCPHackers'
// wrapper. Two changes are needed:
//
//   - "--tweakClass <x>" goes away; the replacement has no tweakers.
//   - the leading positional "${auth_player_name} ${auth_session}" becomes
//     "--username ... --session ...". This one is not cosmetic: LaunchConfig
//     collects any token not starting with "--" into a list it never reads, so
//     a positional username is silently dropped and everyone ends up named
//     "Player".
func rewriteWrapperArgs(template string) string {
	toks := strings.Fields(template)
	var out []string
	for i := 0; i < len(toks); i++ {
		if toks[i] == "--tweakClass" {
			i++ // skip its value too
			continue
		}
		out = append(out, toks[i])
	}
	// Promote up to two leading positionals to named parameters.
	named := []string{"--username", "--session"}
	var head []string
	for len(out) > 0 && len(head) < 4 && !strings.HasPrefix(out[0], "--") {
		head = append(head, named[len(head)/2], out[0])
		out = out[1:]
	}
	return strings.TrimSpace(strings.Join(append(head, out...), " "))
}

// applyLaunchWrapper rewrites a resolved version in place so it launches
// through MCPHackers' LaunchWrapper. Safe to call on any version: it does
// nothing unless Mojang's wrapper is what the JSON asked for.
func (L *Launcher) applyLaunchWrapper(v *VersionJSON, p Progress) {
	if !needsLaunchWrapper(v) || L.Settings.DisableLaunchWrapper {
		return
	}
	// Fetch the replacement up front. It lives on a third-party maven, and the
	// whole pre-1.6 catalog would be unplayable if a swap were applied while
	// the jar was unreachable — so on failure keep Mojang's wrapper. The game
	// still starts; only the era-specific fixes are missing.
	wrapper := Library{Name: mcpWrapperLib, URL: launchWrapperMaven()}
	art := libraryArtifact(wrapper)
	if art == nil {
		return
	}
	dest := filepath.Join(L.LibrariesDir, filepath.FromSlash(art.Path))
	if !fileExists(dest) {
		if p != nil {
			p("Fetching LaunchWrapper…", -1)
		}
		if err := downloadFile(art.URL, dest, "", 0); err != nil {
			L.Logf("LaunchWrapper unavailable (%v) — falling back to Mojang's wrapper; "+
				"Classic and Indev level saving will not work in this session", err)
			return
		}
	}
	v.MainClass = mcpWrapperMain

	// Drop Mojang's wrapper: it declares the same net.minecraft.launchwrapper
	// package, so leaving it on the classpath risks loading its classes instead.
	libs := v.Libraries[:0]
	for _, lib := range v.Libraries {
		if strings.HasPrefix(lib.Name, "net.minecraft:launchwrapper:") {
			continue
		}
		libs = append(libs, lib)
	}
	// The replacement goes first so it wins any remaining tie.
	v.Libraries = append([]Library{wrapper}, libs...)

	if v.MinecraftArguments != "" {
		v.MinecraftArguments = rewriteWrapperArgs(v.MinecraftArguments)
	}
	if v.Arguments != nil && len(v.Arguments.Game) > 0 {
		v.Arguments.Game = dropTweakClass(v.Arguments.Game)
	}
	L.Logf("using MCPHackers LaunchWrapper for %s (Mojang's applet wrapper is broken for this era)", v.ID)
}

// dropTweakClass removes a "--tweakClass <x>" pair from a 1.6-style argument
// list. Plain strings only: a rule-guarded argument is never a tweak class.
func dropTweakClass(in []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(in))
	skip := false
	for _, raw := range in {
		if skip {
			skip = false
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil && s == "--tweakClass" {
			skip = true
			continue
		}
		out = append(out, raw)
	}
	return out
}
