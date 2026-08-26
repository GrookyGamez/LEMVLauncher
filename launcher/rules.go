package main

import (
	"encoding/json"
	"regexp"
	"runtime"
	"strings"
)

// Rule is the allow/disallow rule format used in version JSONs for
// libraries and (since 1.13) for arguments.
type Rule struct {
	Action   string          `json:"action"`
	OS       *RuleOS         `json:"os,omitempty"`
	Features map[string]bool `json:"features,omitempty"`
}

type RuleOS struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

func currentOSName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	default:
		return "linux"
	}
}

// currentArch uses the same vocabulary as Mojang's rules ("x86" means 32-bit).
func currentArch() string {
	switch runtime.GOARCH {
	case "386":
		return "x86"
	case "amd64":
		return "x64"
	default:
		return runtime.GOARCH
	}
}

func currentOSVersion() string {
	if runtime.GOOS == "windows" {
		return "10.0" // Windows 10 and 11 both report 10.0
	}
	return ""
}

// Features the launcher supports. Everything is off: no demo mode, no custom
// resolution, no quick play.
var launchFeatures = map[string]bool{
	"is_demo_user":               false,
	"has_custom_resolution":      false,
	"has_quick_plays_support":    false,
	"is_quick_play_singleplayer": false,
	"is_quick_play_multiplayer":  false,
	"is_quick_play_realms":       false,
}

func (r Rule) matches() bool {
	if r.OS != nil {
		if r.OS.Name != "" && r.OS.Name != currentOSName() {
			return false
		}
		if r.OS.Arch != "" && r.OS.Arch != currentArch() {
			return false
		}
		if r.OS.Version != "" {
			re, err := regexp.Compile(r.OS.Version)
			if err != nil || !re.MatchString(currentOSVersion()) {
				return false
			}
		}
	}
	for k, v := range r.Features {
		if launchFeatures[k] != v {
			return false
		}
	}
	return true
}

// rulesAllow evaluates a rule list the way the official launcher does:
// no rules means allowed; otherwise the last matching rule wins, and the
// default when nothing matches is disallowed.
func rulesAllow(rules []Rule) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, r := range rules {
		if r.matches() {
			allowed = r.Action == "allow"
		}
	}
	return allowed
}

var placeholderRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// substitute replaces ${placeholders}. Unknown placeholders are left as-is
// so problems show up in the log instead of silently vanishing.
func substitute(s string, vars map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := vars[m[2:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

type argEntry struct {
	Rules []Rule          `json:"rules"`
	Value json.RawMessage `json:"value"`
}

// expandArguments handles the 1.13+ "arguments" format, where each entry is
// either a plain string or {rules, value} with value a string or a list.
func expandArguments(raw []json.RawMessage, vars map[string]string) []string {
	var out []string
	for _, r := range raw {
		var s string
		if err := json.Unmarshal(r, &s); err == nil {
			out = append(out, substitute(s, vars))
			continue
		}
		var e argEntry
		if err := json.Unmarshal(r, &e); err != nil {
			continue
		}
		if !rulesAllow(e.Rules) {
			continue
		}
		var vs string
		if err := json.Unmarshal(e.Value, &vs); err == nil {
			out = append(out, substitute(vs, vars))
			continue
		}
		var vl []string
		if err := json.Unmarshal(e.Value, &vl); err == nil {
			for _, x := range vl {
				out = append(out, substitute(x, vars))
			}
		}
	}
	return out
}

// expandLegacyArguments handles the pre-1.13 "minecraftArguments" string.
func expandLegacyArguments(s string, vars map[string]string) []string {
	var out []string
	for _, tok := range strings.Fields(s) {
		out = append(out, substitute(tok, vars))
	}
	return out
}
