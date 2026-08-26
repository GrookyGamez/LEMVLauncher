package main

import (
	"encoding/json"
	"os"
)

// Settings live in launcher-settings.json next to the .exe.
type Settings struct {
	Username       string            `json:"username"`
	MaxMemoryMB    int               `json:"maxMemoryMB"`
	ExtraJVMArgs   []string          `json:"extraJvmArgs"`
	JavaPath       map[string]string `json:"javaPath"`       // component (e.g. "jre-legacy") -> java.exe override
	HideDataFolder bool              `json:"hideDataFolder"` // hide the LEMV folder in Explorer (default true)
	Theme          string            `json:"theme"`          // "modern" (default) or "classic"
	CloseOnLaunch  bool              `json:"closeOnLaunch"`  // quit the launcher once the game is running
	RememberName   bool              `json:"rememberName"`   // prefill the username field from the last launch
	Animations     bool              `json:"animations"`     // slide transitions between views
	AutoImport     bool              `json:"autoImport"`     // copy matching jars from Downloads automatically
	Playtime       map[string]int64  `json:"playtime"`       // seconds played, per version id
	RollCount      int               `json:"rollCount"`      // Surprise Me rolls so far (every 11th is an Alpha)
	DiscordRPC     bool              `json:"discordRpc"`     // publish Rich Presence while a game runs
	DiscordAppID   string            `json:"discordAppId"`   // your Discord application id (see README)
	MSAClientID    string            `json:"msaClientId"`    // your Azure app client id for Microsoft sign-in
	Account        *MSAAccount       `json:"account"`        // signed-in Minecraft account, if any
	LastTab        int               `json:"lastTab"`
	LastVersion    string            `json:"lastVersion"`
	// DisableLaunchWrapper falls back to Mojang's own applet wrapper for
	// legacy versions. Off by default: Mojang's is broken for Classic/Indev.
	DisableLaunchWrapper bool `json:"disableLaunchWrapper"`
}

// defaultDiscordAppID is GrookyGamez's Discord application. Client ids are
// public by design — they travel in every Rich Presence payload.
const defaultDiscordAppID = "1541634679034355712"

func defaultSettings() Settings {
	return Settings{MaxMemoryMB: 2048, ExtraJVMArgs: []string{}, JavaPath: map[string]string{}, HideDataFolder: true, Theme: "modern", Animations: true, AutoImport: true, DiscordRPC: true, DiscordAppID: defaultDiscordAppID}
}

func loadSettings(path string) Settings {
	s := defaultSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	json.Unmarshal(data, &s)
	if s.MaxMemoryMB < 512 {
		s.MaxMemoryMB = 2048
	}
	if s.JavaPath == nil {
		s.JavaPath = map[string]string{}
	}
	if s.ExtraJVMArgs == nil {
		s.ExtraJVMArgs = []string{}
	}
	if s.DiscordAppID == "" {
		s.DiscordAppID = defaultDiscordAppID
		s.DiscordRPC = true
	}
	return s
}

func (L *Launcher) SaveSettings() {
	data, err := json.MarshalIndent(L.Settings, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(L.settingsPath(), data, 0o644)
}
