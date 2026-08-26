// mock serves a fake copy of Mojang's metadata/download endpoints so the
// launcher can be tested without internet access.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

var (
	base  string
	files = map[string][]byte{} // path -> bytes
)

type dl struct {
	ID   string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

func put(path string, data []byte) dl {
	files[path] = data
	h := sha1.Sum(data)
	return dl{Path: strings.TrimPrefix(path, "/lib/"), SHA1: hex.EncodeToString(h[:]), Size: int64(len(data)), URL: base + path}
}

func jar(entries map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		f, _ := w.Create(name)
		f.Write([]byte(entries[name]))
	}
	w.Close()
	return buf.Bytes()
}

func lib(name string, rules []any, natives map[string]string, extract bool) map[string]any {
	parts := strings.Split(name, ":")
	g, a, v := strings.ReplaceAll(parts[0], ".", "/"), parts[1], parts[2]
	m := map[string]any{"name": name}
	downloads := map[string]any{}
	{
		suffix := ""
		if len(parts) > 3 {
			suffix = "-" + parts[3]
		}
		p := fmt.Sprintf("%s/%s/%s/%s-%s%s.jar", g, a, v, a, v, suffix)
		downloads["artifact"] = put("/lib/"+p, jar(map[string]string{"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\n", a + suffix + ".class": "fake " + name}))
	}
	if natives != nil {
		cls := map[string]any{}
		for _, c := range []string{"natives-windows", "natives-windows-32", "natives-windows-64", "natives-linux", "natives-osx"} {
			p := fmt.Sprintf("%s/%s/%s/%s-%s-%s.jar", g, a, v, a, v, c)
			cls[c] = put("/lib/"+p, jar(map[string]string{"META-INF/MANIFEST.MF": "x\n", a + "-" + c + ".dll": "fake dll " + c, a + "-" + c + ".so": "fake so"}))
		}
		if natives["_noArtifact"] == "1" {
			delete(downloads, "artifact")
			delete(natives, "_noArtifact")
		}
		downloads["classifiers"] = cls
		m["natives"] = natives
		if extract {
			m["extract"] = map[string]any{"exclude": []string{"META-INF/"}}
		}
	}
	m["downloads"] = downloads
	if rules != nil {
		m["rules"] = rules
	}
	return m
}

func assetIndex(id string, virtual, mapRes bool, names []string) dl {
	objs := map[string]any{}
	for _, n := range names {
		data := []byte("asset " + id + " " + n)
		h := sha1.Sum(data)
		hs := hex.EncodeToString(h[:])
		files["/res/"+hs[:2]+"/"+hs] = data
		objs[n] = map[string]any{"hash": hs, "size": len(data)}
	}
	idx := map[string]any{"objects": objs}
	if virtual {
		idx["virtual"] = true
	}
	if mapRes {
		idx["map_to_resources"] = true
	}
	data, _ := json.Marshal(idx)
	d := put("/assets/"+id+".json", data)
	d.ID = id
	d.Path = ""
	return d
}

func version(id, typ string, body map[string]any) map[string]any {
	body["id"] = id
	body["type"] = typ
	if _, ok := body["downloads"]; !ok {
		client := put("/client/"+id+".jar", jar(map[string]string{"META-INF/MANIFEST.MF": "Main-Class: none\n", "client-marker.txt": "official client " + id}))
		client.Path = ""
		body["downloads"] = map[string]any{"client": client}
	}
	data, _ := json.MarshalIndent(body, "", " ")
	d := put("/v/"+id+".json", data)
	return map[string]any{"id": id, "type": typ, "url": d.URL, "sha1": d.SHA1, "time": "2020-01-01T00:00:00+00:00", "releaseTime": body["releaseTime"]}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	fakeJavaExe := flag.String("javaexe", "", "path to a fake java.exe to serve in the runtime")
	flag.Parse()
	base = "http://" + *addr

	winOnly := []any{map[string]any{"action": "allow", "os": map[string]any{"name": "windows"}}}
	notOSX := []any{map[string]any{"action": "allow"}, map[string]any{"action": "disallow", "os": map[string]any{"name": "osx"}}}
	osxOnly := []any{map[string]any{"action": "allow", "os": map[string]any{"name": "osx"}}}
	linuxOnly := []any{map[string]any{"action": "allow", "os": map[string]any{"name": "linux"}}}

	nat := func() map[string]string {
		return map[string]string{"linux": "natives-linux", "osx": "natives-osx", "windows": "natives-windows"}
	}
	natArch := func() map[string]string {
		return map[string]string{"linux": "natives-linux", "osx": "natives-osx", "windows": "natives-windows-${arch}", "_noArtifact": "1"}
	}

	idxModern := assetIndex("19", false, false, []string{"minecraft/sounds/a.ogg", "minecraft/lang/en_us.json", "icons/icon_16x16.png"})
	idx18 := assetIndex("1.8", false, false, []string{"minecraft/sounds/b.ogg", "minecraft/lang/en_US.lang"})
	idxLegacy := assetIndex("legacy", true, false, []string{"sound/step/grass1.ogg", "lang/en_US.lang", "icons/icon_16x16.png"})
	idxPre16 := assetIndex("pre-1.6", false, true, []string{"sound/step/grass1.ogg", "music/calm1.ogg", "newsound/random/bow.ogg"})

	logCfg := put("/log/client-1.12.xml", []byte("<Configuration/>"))
	logCfg.ID = "client-1.12.xml"
	logCfg.Path = ""

	javaDelta := map[string]any{"component": "java-runtime-delta", "majorVersion": 21}
	javaLegacy := map[string]any{"component": "jre-legacy", "majorVersion": 8}

	modern := version("1.21", "release", map[string]any{
		"releaseTime": "2024-06-13T08:24:03+00:00",
		"mainClass":   "net.minecraft.client.main.Main",
		"assets":      "19", "assetIndex": idxModern, "javaVersion": javaDelta,
		"logging": map[string]any{"client": map[string]any{"argument": "-Dlog4j.configurationFile=${path}", "file": logCfg, "type": "log4j2-xml"}},
		"arguments": map[string]any{
			"game": []any{"--username", "${auth_player_name}", "--version", "${version_name}", "--gameDir", "${game_directory}", "--assetsDir", "${assets_root}", "--assetIndex", "${assets_index_name}", "--uuid", "${auth_uuid}", "--accessToken", "${auth_access_token}", "--clientId", "${clientid}", "--xuid", "${auth_xuid}", "--userType", "${user_type}", "--versionType", "${version_type}",
				map[string]any{"rules": []any{map[string]any{"action": "allow", "features": map[string]any{"is_demo_user": true}}}, "value": "--demo"},
				map[string]any{"rules": []any{map[string]any{"action": "allow", "features": map[string]any{"has_custom_resolution": true}}}, "value": []string{"--width", "${resolution_width}", "--height", "${resolution_height}"}},
				map[string]any{"rules": []any{map[string]any{"action": "allow", "features": map[string]any{"has_quick_plays_support": true}}}, "value": []string{"--quickPlayPath", "${quickPlayPath}"}},
			},
			"jvm": []any{
				map[string]any{"rules": osxOnly, "value": []string{"-XstartOnFirstThread"}},
				map[string]any{"rules": winOnly, "value": "-XX:HeapDumpPath=MojangTricksIntelDriversForPerformance_javaw.exe_minecraft.exe.heapdump"},
				map[string]any{"rules": []any{map[string]any{"action": "allow", "os": map[string]any{"arch": "x86"}}}, "value": "-Xss1M"},
				"-Djava.library.path=${natives_directory}", "-Djna.tmpdir=${natives_directory}", "-Dorg.lwjgl.system.SharedLibraryExtractPath=${natives_directory}", "-Dio.netty.native.workdir=${natives_directory}", "-Dminecraft.launcher.brand=${launcher_name}", "-Dminecraft.launcher.version=${launcher_version}", "-cp", "${classpath}",
			},
		},
		"libraries": []any{
			lib("com.github.oshi:oshi-core:6.4.10", nil, nil, false),
			lib("org.lwjgl:lwjgl:3.3.3", nil, nil, false),
			lib("org.lwjgl:lwjgl:3.3.3:natives-windows", winOnly, nil, false),
			lib("org.lwjgl:lwjgl:3.3.3:natives-linux", linuxOnly, nil, false),
			lib("org.lwjgl:lwjgl:3.3.3:natives-macos", osxOnly, nil, false),
		},
	})
	// a patch release: present on Mojang, absent from LEMV's curated base list
	modernPatch := version("1.21.1", "release", map[string]any{
		"releaseTime": "2024-08-08T12:24:45+00:00",
		"mainClass":   "net.minecraft.client.main.Main",
		"assets":      "19", "assetIndex": idxModern, "javaVersion": javaDelta,
		"logging": map[string]any{"client": map[string]any{"argument": "-Dlog4j.configurationFile=${path}", "file": logCfg, "type": "log4j2-xml"}},
		"arguments": map[string]any{
			"game": []any{"--username", "${auth_player_name}", "--version", "${version_name}", "--gameDir", "${game_directory}", "--assetsDir", "${assets_root}", "--assetIndex", "${assets_index_name}", "--uuid", "${auth_uuid}", "--accessToken", "${auth_access_token}", "--userType", "${user_type}", "--versionType", "${version_type}"},
			"jvm": []any{
				map[string]any{"rules": winOnly, "value": "-XX:HeapDumpPath=MojangTricksIntelDriversForPerformance_javaw.exe_minecraft.exe.heapdump"},
				"-Djava.library.path=${natives_directory}", "-Dminecraft.launcher.brand=${launcher_name}", "-Dminecraft.launcher.version=${launcher_version}", "-cp", "${classpath}",
			},
		},
		"libraries": []any{
			lib("com.github.oshi:oshi-core:6.4.10", nil, nil, false),
			lib("org.lwjgl:lwjgl:3.3.3", nil, nil, false),
			lib("org.lwjgl:lwjgl:3.3.3:natives-windows", winOnly, nil, false),
			lib("org.lwjgl:lwjgl:3.3.3:natives-linux", linuxOnly, nil, false),
		},
	})
	v18 := version("1.8", "release", map[string]any{
		"releaseTime":        "2014-09-02T08:42:07+00:00",
		"mainClass":          "net.minecraft.client.main.Main",
		"minecraftArguments": "--username ${auth_player_name} --version ${version_name} --gameDir ${game_directory} --assetsDir ${assets_root} --assetIndex ${assets_index_name} --uuid ${auth_uuid} --accessToken ${auth_access_token} --userProperties ${user_properties} --userType ${user_type}",
		"assets":             "1.8", "assetIndex": idx18, "javaVersion": javaLegacy,
		"libraries": []any{
			lib("com.mojang:netty:1.6", nil, nil, false),
			lib("org.lwjgl.lwjgl:lwjgl:2.9.4-nightly-20150209", nil, nil, false),
			lib("org.lwjgl.lwjgl:lwjgl-platform:2.9.4-nightly-20150209", notOSX, natArch(), true),
			lib("net.java.jinput:jinput-platform:2.0.5", nil, nat(), true),
			lib("tv.twelvemonkeys:ignored:1.0", osxOnly, nil, false),
		},
	})
	v151 := version("1.5.1", "release", map[string]any{
		"releaseTime":        "2013-03-20T10:00:00+00:00",
		"mainClass":          "net.minecraft.client.Minecraft",
		"minecraftArguments": "${auth_player_name} ${auth_session} --gameDir ${game_directory} --assetsDir ${game_assets}",
		"assets":             "pre-1.6", "assetIndex": idxPre16, "javaVersion": javaLegacy,
		"libraries": []any{
			lib("org.lwjgl.lwjgl:lwjgl:2.9.0", nil, nil, false),
			lib("org.lwjgl.lwjgl:lwjgl_util:2.9.0", nil, nil, false),
			lib("org.lwjgl.lwjgl:lwjgl-platform:2.9.0", nil, nat(), true),
		},
	})
	v161 := version("1.6.1", "release", map[string]any{
		"releaseTime":        "2013-06-28T15:00:00+00:00",
		"mainClass":          "net.minecraft.client.main.Main",
		"minecraftArguments": "--username ${auth_player_name} --session ${auth_session} --version ${version_name} --gameDir ${game_directory} --assetsDir ${game_assets}",
		"assets":             "legacy", "assetIndex": idxLegacy, "javaVersion": javaLegacy,
		"libraries": []any{
			lib("net.sf.jopt-simple:jopt-simple:4.5", nil, nil, false),
			lib("org.lwjgl.lwjgl:lwjgl-platform:2.9.0", nil, nat(), true),
		},
	})
	b173 := version("b1.7.3", "old_beta", map[string]any{
		"releaseTime":        "2011-07-08T22:00:00+00:00",
		"mainClass":          "net.minecraft.launchwrapper.Launch",
		"minecraftArguments": "${auth_player_name} ${auth_session} --gameDir ${game_directory} --assetsDir ${game_assets} --tweakClass net.minecraft.launchwrapper.VanillaTweaker",
		"assets":             "pre-1.6", "assetIndex": idxPre16, "javaVersion": javaLegacy,
		"libraries": []any{
			lib("net.minecraft:launchwrapper:1.5", nil, nil, false),
			lib("org.lwjgl.lwjgl:lwjgl-platform:2.9.0", nil, nat(), true),
		},
	})
	a104 := version("a1.0.4", "old_alpha", map[string]any{
		"releaseTime":        "2010-07-02T22:00:00+00:00",
		"mainClass":          "net.minecraft.launchwrapper.Launch",
		"minecraftArguments": "${auth_player_name} ${auth_session} --gameDir ${game_directory} --assetsDir ${game_assets} --tweakClass net.minecraft.launchwrapper.AlphaVanillaTweaker",
		"assets":             "pre-1.6", "assetIndex": idxPre16,
		"libraries": []any{
			lib("net.minecraft:launchwrapper:1.5", nil, nil, false),
		},
	})
	var alphaClones []any
	cloneDates := map[string]string{
		"rd-132211": "2009-05-13T20:11:00+00:00", "rd-132328": "2009-05-13T23:28:00+00:00",
		"rd-20090515": "2009-05-15T00:00:00+00:00", "rd-160052": "2009-05-16T00:52:00+00:00",
		"rd-161348": "2009-05-16T13:48:00+00:00", "c0.0.13a_03": "2009-05-22T00:00:00+00:00",
		"c0.30_01c": "2009-11-10T00:00:00+00:00", "inf-20100618": "2010-06-18T00:00:00+00:00",
		"a1.1.0": "2010-09-13T00:00:00+00:00", "a1.1.2": "2010-09-19T00:00:00+00:00",
		"a1.2.0": "2010-10-30T00:00:00+00:00", "b1.2_02": "2011-01-13T00:00:00+00:00",
		"b1.3_01": "2011-02-23T00:00:00+00:00", "b1.6": "2011-05-25T00:00:00+00:00",
	}
	for _, id := range []string{"a1.1.0", "a1.2.0", "inf-20100618", "c0.30_01c", "rd-132211", "a1.1.2", "b1.2_02", "b1.3_01", "c0.0.13a_03", "b1.6", "rd-132328", "rd-20090515", "rd-160052", "rd-161348"} {
		alphaClones = append(alphaClones, version(id, "old_alpha", map[string]any{
			"releaseTime":        cloneDates[id],
			"mainClass":          "net.minecraft.launchwrapper.Launch",
			"minecraftArguments": "${auth_player_name} ${auth_session} --gameDir ${game_directory} --assetsDir ${game_assets} --tweakClass net.minecraft.launchwrapper.AlphaVanillaTweaker",
			"assets":             "pre-1.6", "assetIndex": idxPre16,
			"libraries": []any{
				lib("net.minecraft:launchwrapper:1.5", nil, nil, false),
			},
		}))
	}

	// extra old-era ids so the launcher's manifest expansion can be exercised
	for _, tv := range []struct{ id, date string }{
		{"c0.0.11a", "2009-05-18T00:00:00+00:00"},
		{"c0.24_st_03", "2009-09-18T00:00:00+00:00"},
		{"c0.27_st", "2009-10-13T00:00:00+00:00"},
		{"in-20100107", "2010-01-07T00:00:00+00:00"},
		{"in-20100206", "2010-02-06T00:00:00+00:00"},
		{"in-20100223", "2010-02-23T00:00:00+00:00"},
		{"inf-20100227-1433", "2010-02-27T14:33:00+00:00"},
		{"inf-20100413", "2010-04-13T00:00:00+00:00"},
		{"a1.0.13_01", "2010-08-03T00:00:00+00:00"},
		{"a1.0.17_04", "2010-09-16T00:00:00+00:00"},
		{"a1.2.6", "2010-12-03T00:00:00+00:00"},
		{"b1.1_02", "2010-12-22T00:00:00+00:00"},
		{"b1.6.6", "2011-05-31T00:00:00+00:00"},
		{"b1.8.1", "2011-09-19T00:00:00+00:00"},
	} {
		typ := "old_alpha"
		if strings.HasPrefix(tv.id, "b1.") {
			typ = "old_beta"
		}
		alphaClones = append(alphaClones, version(tv.id, typ, map[string]any{
			"releaseTime":        tv.date,
			"mainClass":          "net.minecraft.launchwrapper.Launch",
			"minecraftArguments": "${auth_player_name} ${auth_session} --gameDir ${game_directory} --assetsDir ${game_assets} --tweakClass net.minecraft.launchwrapper.AlphaVanillaTweaker",
			"assets":             "pre-1.6", "assetIndex": idxPre16,
			"libraries": []any{
				lib("net.minecraft:launchwrapper:1.5", nil, nil, false),
			},
		}))
	}

	noLibDownloads := version("x-nodl", "snapshot", map[string]any{
		"releaseTime":        "2015-01-01T00:00:00+00:00",
		"mainClass":          "net.minecraft.client.main.Main",
		"minecraftArguments": "--username ${auth_player_name}",
		"assets":             "1.8", "assetIndex": idx18, "javaVersion": javaLegacy,
		"libraries": []any{
			map[string]any{"name": "com.example:bare:1.0"},
		},
	})
	_ = put("/lib/com/example/bare/1.0/bare-1.0.jar", jar(map[string]string{"a": "b"}))

	manifest := map[string]any{
		"latest":   map[string]string{"release": "1.21.1", "snapshot": "1.21.1"},
		"versions": append([]any{modernPatch, modern, v18, v161, v151, b173, a104, noLibDownloads}, alphaClones...),
	}
	mdata, _ := json.MarshalIndent(manifest, "", " ")
	files["/manifest.json"] = mdata

	// ---- java runtimes ---------------------------------------------------
	script := "#!/bin/sh\n# fake java\n: > \"$PWD/java-args.txt\"\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> \"$PWD/java-args.txt\"; done\necho \"fake java ran with $# args\"\nexit 0\n"
	for _, comp := range []string{"jre-legacy", "java-runtime-delta"} {
		rf := map[string]any{
			"bin":           map[string]any{"type": "directory"},
			"lib":           map[string]any{"type": "directory"},
			"bin/java":      map[string]any{"type": "file", "executable": true, "downloads": map[string]any{"raw": put("/jre/"+comp+"/bin/java", []byte(script))}},
			"lib/dummy.txt": map[string]any{"type": "file", "executable": false, "downloads": map[string]any{"raw": put("/jre/"+comp+"/lib/dummy.txt", []byte("dummy "+comp))}},
			"lib/link":      map[string]any{"type": "link", "target": "dummy.txt"},
		}
		if *fakeJavaExe != "" {
			data, err := os.ReadFile(*fakeJavaExe)
			if err != nil {
				log.Fatal(err)
			}
			rf["bin/java.exe"] = map[string]any{"type": "file", "executable": true, "downloads": map[string]any{"raw": put("/jre/"+comp+"/bin/java.exe", data)}}
			rf["bin/javaw.exe"] = map[string]any{"type": "file", "executable": true, "downloads": map[string]any{"raw": put("/jre/"+comp+"/bin/javaw.exe", data)}}
		}
		man, _ := json.Marshal(map[string]any{"files": rf})
		md := put("/jre/"+comp+"/manifest.json", man)
		md.Path = ""
		entry := []any{map[string]any{"availability": map[string]any{"group": 1, "progress": 100}, "manifest": md, "version": map[string]any{"name": map[string]string{"jre-legacy": "8u202", "java-runtime-delta": "21.0.3"}[comp], "released": "2024-01-01T00:00:00+00:00"}}}
		files["/jre/entry/"+comp] = []byte{} // placeholder
		_ = entry
	}
	all := map[string]any{}
	for _, plat := range []string{"windows-x64", "linux", "mac-os"} {
		comps := map[string]any{}
		for _, comp := range []string{"jre-legacy", "java-runtime-delta"} {
			md := dl{URL: base + "/jre/" + comp + "/manifest.json"}
			h := sha1.Sum(files["/jre/"+comp+"/manifest.json"])
			md.SHA1 = hex.EncodeToString(h[:])
			md.Size = int64(len(files["/jre/"+comp+"/manifest.json"]))
			comps[comp] = []any{map[string]any{"availability": map[string]any{"group": 1, "progress": 100}, "manifest": md, "version": map[string]any{"name": map[string]string{"jre-legacy": "8u202", "java-runtime-delta": "21.0.3"}[comp], "released": "2024-01-01T00:00:00+00:00"}}}
		}
		comps["java-runtime-gamma"] = []any{}
		all[plat] = comps
	}
	adata, _ := json.MarshalIndent(all, "", " ")
	files["/jre/all.json"] = adata

	// ---- fake Omniarchive manifest -----------------------------------------
	// 1.0.1 is a real release Mojang dropped from its manifest; Omniarchive
	// still has it. It must show up in Minor Updates alongside Mojang's.
	omniOnly := version("1.0.1", "release", map[string]any{
		"releaseTime": "2011-11-24T14:00:00+00:00",
		"mainClass":   "net.minecraft.client.main.Main",
		"assets":      "pre-1.6", "assetIndex": idx18, "javaVersion": javaLegacy,
		"minecraftArguments": "${auth_player_name} ${auth_session}",
		"libraries":          []any{lib("org.lwjgl.lwjgl:lwjgl:2.9.0", nil, nil, false)},
	})
	omniManifest, _ := json.MarshalIndent(map[string]any{
		"latest":   map[string]string{"release": "1.0.1", "snapshot": "1.0.1"},
		"versions": []any{omniOnly},
	}, "", " ")
	files["/omni/v1/manifest.json"] = omniManifest

	// ---- fake MCPHackers maven (LaunchWrapper) -----------------------------
	files["/lwmaven/org/mcphackers/launchwrapper/1.3.0/launchwrapper-1.3.0.jar"] = jar(map[string]string{
		"org/mcphackers/launchwrapper/Launch.class": "mock launchwrapper",
	})

	// ---- fake Microsoft sign-in chain (device-code -> XBL -> XSTS -> MC) ----
	var pollCount int
	http.HandleFunc("/msa/oauth2/v2.0/devicecode", func(w http.ResponseWriter, r *http.Request) {
		pollCount = 0
		json.NewEncoder(w).Encode(map[string]any{
			"device_code": "DEV-CODE", "user_code": "WXYZ-1234",
			"verification_uri": base + "/msa/activate", "expires_in": 300, "interval": 1,
		})
	})
	http.HandleFunc("/msa/oauth2/v2.0/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("grant_type") == "refresh_token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "MSA-ACCESS", "refresh_token": "MSA-REFRESH"})
			return
		}
		pollCount++
		if pollCount < 2 { // first poll: still waiting, then approved
			json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "MSA-ACCESS", "refresh_token": "MSA-REFRESH"})
	})
	http.HandleFunc("/xbl/user/authenticate", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"Token":         "XBL-TOKEN",
			"DisplayClaims": map[string]any{"xui": []any{map[string]any{"uhs": "USERHASH"}}},
		})
	})
	http.HandleFunc("/xsts/xsts/authorize", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"Token": "XSTS-TOKEN"})
	})
	http.HandleFunc("/mcsvc/authentication/login_with_xbox", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"access_token": "MC-TOKEN", "expires_in": 86400})
	})
	http.HandleFunc("/mcsvc/minecraft/profile", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "00000000000040008000000000000001", "name": "GrookyTest"})
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[r.URL.Path]
		if !ok {
			log.Printf("404 %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	})
	log.Printf("mock Mojang serving %d files on %s", len(files), base)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
