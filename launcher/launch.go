package main

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	launcherName    = "LEMVLauncher"
	launcherVersion = "3.5.0"
)

// Launcher owns the folder layout next to the .exe.
type Launcher struct {
	Root         string
	VersionsDir  string // user-supplied jars
	LibrariesDir string
	AssetsDir    string
	NativesDir   string
	RuntimesDir  string
	InstancesDir string
	MetaDir      string
	LogsDir      string
	LastImported []string // jars moved in from Downloads by the last Rescan

	Settings Settings
	Entries  []*Entry
	logger   *log.Logger
	logFile  *os.File
}

func newLauncher(root string) *Launcher {
	L := &Launcher{
		Root:         root,
		VersionsDir:  filepath.Join(root, "versions"),
		LibrariesDir: filepath.Join(root, "libraries"),
		AssetsDir:    filepath.Join(root, "assets"),
		NativesDir:   filepath.Join(root, "natives"),
		RuntimesDir:  filepath.Join(root, "runtimes"),
		InstancesDir: filepath.Join(root, "instances"),
		MetaDir:      filepath.Join(root, "meta"),
		LogsDir:      filepath.Join(root, "logs"),
	}
	for _, d := range []string{L.VersionsDir, L.LibrariesDir, L.AssetsDir, L.NativesDir, L.RuntimesDir, L.InstancesDir, L.MetaDir, L.LogsDir} {
		os.MkdirAll(d, 0o755)
	}
	if f, err := os.OpenFile(filepath.Join(L.LogsDir, "launcher.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		L.logFile = f
		L.logger = log.New(f, "", log.LstdFlags)
	} else {
		L.logger = log.New(io.Discard, "", 0)
	}
	L.Settings = loadSettings(L.settingsPath())
	L.Entries = catalog()
	L.Rescan()
	L.Logf("---- %s %s started (%s/%s) root=%s", launcherName, launcherVersion, runtime.GOOS, runtime.GOARCH, root)
	return L
}

func (L *Launcher) settingsPath() string { return filepath.Join(L.Root, "launcher-settings.json") }

func (L *Launcher) Logf(format string, args ...any) { L.logger.Printf(format, args...) }

// Progress receives status text and a completion fraction (negative = unknown).
type Progress func(msg string, frac float64)

// LaunchResult describes a started game process.
type LaunchResult struct {
	Cmd       *exec.Cmd
	VersionID string
	JavaPath  string
	GameDir   string
	LogPath   string
	Args      []string

	done     chan struct{}
	exitCode int
	waitErr  error
}

// Wait blocks until the game exits and returns its exit code.
func (r *LaunchResult) Wait() (int, error) {
	<-r.done
	return r.exitCode, r.waitErr
}

// ensureClientJar returns the path of the jar to launch. When the user hasn't
// dropped one in, the official client jar is downloaded straight from Mojang's
// servers (the URL comes from the version JSON, verified by sha1).
func (L *Launcher) ensureClientJar(e *Entry, v *VersionJSON, p Progress) (string, error) {
	jarPath := e.JarPath // snapshot: a rescan on the UI thread may update the entry meanwhile
	if jarPath != "" && fileExists(jarPath) {
		return jarPath, nil
	}
	if e.DropInOnly {
		return "", fmt.Errorf("Mojang doesn't host %s, so it can't be auto-downloaded. Put your own %s in the versions folder", e.Name, e.JarName())
	}
	if !strings.EqualFold(v.ResolvedFrom, e.ID) {
		return "", fmt.Errorf("Mojang's version list doesn't have %q (closest is %s), so its jar can't be auto-downloaded. Put your own %s in the versions folder", e.ID, v.ResolvedFrom, e.JarName())
	}
	cl, ok := v.Downloads["client"]
	if !ok || cl.URL == "" {
		return "", fmt.Errorf("Mojang's info for %s has no client download. Put your own %s in the versions folder", e.ID, e.JarName())
	}
	dest := filepath.Join(L.VersionsDir, sanitizeName(e.ID)+".jar")
	L.Logf("downloading client jar for %s from %s", e.ID, cl.URL)
	err := downloadFileProgress(cl.URL, dest, cl.SHA1, cl.Size, func(done, total int64) {
		frac := -1.0
		if total > 0 {
			frac = float64(done) / float64(total)
		}
		p(fmt.Sprintf("Downloading %s from Mojang (%s / %s)…", e.Name, mib(done), mib(total)), frac)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't download %s's jar from Mojang: %v", e.Name, err)
	}
	return dest, nil
}

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`)

func validUsername(s string) bool { return usernameRe.MatchString(s) }

// offlineUUID matches what vanilla servers generate for offline players.
func offlineUUID(name string) string {
	h := md5.Sum([]byte("OfflinePlayer:" + name))
	h[6] = (h[6] & 0x0f) | 0x30
	h[8] = (h[8] & 0x3f) | 0x80
	return hex.EncodeToString(h[:])
}

type nativeJar struct {
	path    string
	exclude []string
}

// Launch prepares everything a version needs and starts the game.
// Launch runs a version in its own per-version game folder.
func (L *Launcher) Launch(e *Entry, username string, p Progress) (*LaunchResult, error) {
	if p == nil {
		p = func(string, float64) {}
	}
	if !validUsername(username) {
		return nil, fmt.Errorf("usernames are 1-16 letters, numbers or underscores")
	}
	p("Reading version info for "+e.Name+"…", -1)
	v, err := L.resolveVersion(e)
	if err != nil {
		return nil, err
	}
	L.applyLaunchWrapper(v, p)
	jarPath, err := L.ensureClientJar(e, v, p)
	if err != nil {
		return nil, err
	}
	L.Logf("launching %s (jar %s) as %s", e.ID, jarPath, username)
	safeID := sanitizeName(e.ID)
	gameDir := filepath.Join(L.InstancesDir, safeID)
	libDir := L.LibrariesDir
	nativesDir := filepath.Join(L.NativesDir, safeID)
	versionName := e.ID
	os.MkdirAll(gameDir, 0o755)
	os.MkdirAll(nativesDir, 0o755)

	// ---- libraries ------------------------------------------------------
	var classpath []string
	var natives []nativeJar
	var tasks []func() error
	seen := map[string]bool{}
	for _, lib := range v.Libraries {
		if !rulesAllow(lib.Rules) {
			continue
		}
		if art := libraryArtifact(lib); art != nil {
			dest := filepath.Join(libDir, filepath.FromSlash(art.Path))
			if !seen[dest] {
				seen[dest] = true
				classpath = append(classpath, dest)
				a := *art
				tasks = append(tasks, func() error { return downloadFile(a.URL, dest, a.SHA1, a.Size) })
			}
		}
		if lib.Natives != nil {
			cls := lib.Natives[currentOSName()]
			if cls == "" {
				continue
			}
			bits := "64"
			if currentArch() == "x86" {
				bits = "32"
			}
			cls = strings.ReplaceAll(cls, "${arch}", bits)
			if lib.Downloads == nil || lib.Downloads.Classifiers == nil || lib.Downloads.Classifiers[cls] == nil {
				L.Logf("library %s has no %s natives for this platform", lib.Name, cls)
				continue
			}
			art := lib.Downloads.Classifiers[cls]
			dest := filepath.Join(libDir, filepath.FromSlash(art.Path))
			var excl []string
			if lib.Extract != nil {
				excl = lib.Extract.Exclude
			}
			natives = append(natives, nativeJar{path: dest, exclude: excl})
			if !seen[dest] {
				seen[dest] = true
				a := *art
				tasks = append(tasks, func() error { return downloadFile(a.URL, dest, a.SHA1, a.Size) })
			}
		}
	}
	total := len(tasks)
	p(fmt.Sprintf("Checking libraries (0/%d)…", total), 0)
	if err := runParallel(8, tasks, func(done, total int) {
		p(fmt.Sprintf("Downloading libraries (%d/%d)…", done, total), float64(done)/float64(total))
	}); err != nil {
		return nil, fmt.Errorf("library download failed: %v", err)
	}
	classpath = append(classpath, jarPath)

	// ---- natives --------------------------------------------------------
	if len(natives) > 0 {
		p("Unpacking natives…", -1)
		for _, n := range natives {
			if err := extractNatives(n.path, nativesDir, n.exclude); err != nil {
				return nil, fmt.Errorf("couldn't unpack %s: %v", filepath.Base(n.path), err)
			}
		}
	}

	// ---- assets ---------------------------------------------------------
	assetsRoot, indexID, gameAssets, err := L.prepareAssets(v, gameDir, p)
	if err != nil {
		return nil, err
	}

	// ---- java -----------------------------------------------------------
	javaPath, err := L.ensureRuntime(v, p)
	if err != nil {
		return nil, err
	}

	// ---- logging config -------------------------------------------------
	var logArg string
	if v.Logging != nil && v.Logging.Client != nil && v.Logging.Client.File.URL != "" {
		lc := v.Logging.Client
		name := lc.File.ID
		if name == "" {
			name = "client.xml"
		}
		dest := filepath.Join(L.AssetsDir, "log_configs", sanitizeName(name))
		if err := downloadFile(lc.File.URL, dest, lc.File.SHA1, lc.File.Size); err == nil {
			logArg = strings.ReplaceAll(lc.Argument, "${path}", dest)
		} else {
			L.Logf("logging config download failed (continuing without it): %v", err)
		}
	}

	// ---- command line ---------------------------------------------------
	p("Starting "+e.Name+"…", -1)
	sep := string(os.PathListSeparator)
	if acc := L.Settings.Account; acc != nil && !acc.Valid() && acc.RefreshToken != "" && L.Settings.MSAClientID != "" {
		p("Refreshing your Microsoft sign-in…", -1)
		if fresh, err := RefreshLogin(L.Settings.MSAClientID, acc, nil); err == nil {
			L.Settings.Account = fresh
			L.SaveSettings()
		} else {
			return nil, fmt.Errorf("your Microsoft sign-in expired and couldn't refresh (%v) — open Settings and sign in again, or sign out to play offline", err)
		}
	}
	authName, authUUID, authToken, authType, authSession := username, offlineUUID(username), "0", "legacy", "-"
	if acc := L.Settings.Account; acc != nil && acc.AccessToken != "" {
		authName, authUUID, authToken, authType = acc.Name, acc.UUID, acc.AccessToken, "msa"
		authSession = "token:" + acc.AccessToken + ":" + acc.UUID
		L.Logf("launching as Microsoft account %s", acc.Name)
	}
	vars := map[string]string{
		"auth_player_name":    authName,
		"auth_session":        authSession,
		"auth_uuid":           authUUID,
		"auth_access_token":   authToken,
		"user_type":           authType,
		"user_properties":     "{}",
		"clientid":            "",
		"auth_xuid":           "",
		"version_name":        versionName,
		"version_type":        v.Type,
		"game_directory":      gameDir,
		"assets_root":         assetsRoot,
		"assets_index_name":   indexID,
		"game_assets":         gameAssets,
		"natives_directory":   nativesDir,
		"launcher_name":       launcherName,
		"launcher_version":    launcherVersion,
		"classpath":           strings.Join(classpath, sep),
		"library_directory":   libDir,
		"classpath_separator": sep,
		"resolution_width":    "854",
		"resolution_height":   "480",
	}

	mem := L.Settings.MaxMemoryMB
	if mem < 512 {
		mem = 2048
	}
	args := []string{fmt.Sprintf("-Xmx%dM", mem)}
	args = append(args, L.Settings.ExtraJVMArgs...)
	if logArg != "" {
		args = append(args, logArg)
	}
	if v.Arguments != nil && len(v.Arguments.JVM) > 0 {
		args = append(args, expandArguments(v.Arguments.JVM, vars)...)
	} else {
		args = append(args,
			"-Djava.library.path="+nativesDir,
			"-Dminecraft.launcher.brand="+launcherName,
			"-Dminecraft.launcher.version="+launcherVersion,
			"-Dminecraft.applet.TargetDirectory="+gameDir, // pre-1.6 clients read this
		)
	}
	if !hasArg(args, "-cp") && !hasArg(args, "-classpath") {
		args = append(args, "-cp", vars["classpath"])
	}
	mainClass := v.MainClass
	if mainClass == "" {
		mainClass = "net.minecraft.client.main.Main"
	}
	args = append(args, mainClass)
	if v.Arguments != nil && len(v.Arguments.Game) > 0 {
		args = append(args, expandArguments(v.Arguments.Game, vars)...)
	} else if v.MinecraftArguments != "" {
		args = append(args, expandLegacyArguments(v.MinecraftArguments, vars)...)
	} else {
		args = append(args, username, "-")
	}
	for _, a := range args {
		if strings.Contains(a, "${") {
			L.Logf("warning: unresolved placeholder in argument %q", a)
		}
	}

	// ---- start ----------------------------------------------------------
	logPath := filepath.Join(L.LogsDir, safeID+".log")
	gameLog, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't create %s: %v", logPath, err)
	}
	fmt.Fprintf(gameLog, "# %s %s - %s launched at %s\n# java: %s\n# cwd: %s\n# args:\n", launcherName, launcherVersion, e.ID, time.Now().Format(time.RFC3339), javaPath, gameDir)
	for _, a := range args {
		fmt.Fprintf(gameLog, "#   %s\n", a)
	}
	fmt.Fprintln(gameLog, "#")

	cmd := exec.Command(javaPath, args...)
	cmd.Dir = gameDir
	cmd.Stdout = gameLog
	cmd.Stderr = gameLog
	setProcAttrs(cmd)
	if err := cmd.Start(); err != nil {
		gameLog.Close()
		return nil, fmt.Errorf("couldn't start Java: %v", err)
	}
	L.Logf("started %s with pid %d (log %s)", e.ID, cmd.Process.Pid, logPath)
	res := &LaunchResult{Cmd: cmd, VersionID: e.ID, JavaPath: javaPath, GameDir: gameDir, LogPath: logPath, Args: args, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		res.exitCode = -1
		if cmd.ProcessState != nil {
			res.exitCode = cmd.ProcessState.ExitCode()
		}
		if _, isExit := err.(*exec.ExitError); err != nil && !isExit {
			res.waitErr = err
		}
		gameLog.Close()
		L.Logf("%s exited with code %d", e.ID, res.exitCode)
		close(res.done)
	}()
	return res, nil
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// libraryArtifact returns the jar that goes on the classpath for a library,
// deriving the Maven path when the JSON doesn't spell it out.
func libraryArtifact(lib Library) *Download {
	if lib.Downloads != nil && lib.Downloads.Artifact != nil {
		return lib.Downloads.Artifact
	}
	if lib.Downloads != nil && lib.Downloads.Artifact == nil && lib.Downloads.Classifiers != nil {
		return nil // natives-only library
	}
	parts := strings.Split(lib.Name, ":")
	if len(parts) < 3 {
		return nil
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	file := artifact + "-" + version
	if len(parts) > 3 {
		file += "-" + parts[3]
	}
	file += ".jar"
	path := strings.ReplaceAll(group, ".", "/") + "/" + artifact + "/" + version + "/" + file
	base := lib.URL
	if base == "" {
		base = librariesURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return &Download{Path: path, URL: base + path}
}

// extractNatives unpacks a natives jar into dir, skipping excluded prefixes.
func extractNatives(jar, dir string, exclude []string) error {
	r, err := zip.OpenReader(jar)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		skip := false
		for _, ex := range exclude {
			if strings.HasPrefix(f.Name, ex) {
				skip = true
				break
			}
		}
		if skip || strings.Contains(f.Name, "..") {
			continue
		}
		dest := filepath.Join(dir, filepath.FromSlash(f.Name))
		if st, err := os.Stat(dest); err == nil && st.Size() == int64(f.UncompressedSize64) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// prepareAssets downloads the asset index and objects, and rebuilds the
// "virtual" or "resources" folders that old versions read directly.
func (L *Launcher) prepareAssets(v *VersionJSON, gameDir string, p Progress) (assetsRoot, indexID, gameAssets string, err error) {
	p("Checking assets…", -1)
	idx, id, err := L.loadAssetIndex(v)
	if err != nil {
		return "", "", "", err
	}
	objectsDir := filepath.Join(L.AssetsDir, "objects")

	names := make([]string, 0, len(idx.Objects))
	for n := range idx.Objects {
		names = append(names, n)
	}
	sort.Strings(names)

	var tasks []func() error
	for _, n := range names {
		obj := idx.Objects[n]
		if len(obj.Hash) < 2 {
			continue
		}
		h := strings.ToLower(obj.Hash)
		dest := filepath.Join(objectsDir, h[:2], h)
		if fileExists(dest) {
			if st, _ := os.Stat(dest); st != nil && (obj.Size <= 0 || st.Size() == obj.Size) {
				continue
			}
		}
		url := resourcesURL + h[:2] + "/" + h
		size := obj.Size
		tasks = append(tasks, func() error { return downloadFile(url, dest, h, size) })
	}
	if len(tasks) > 0 {
		p(fmt.Sprintf("Downloading assets (0/%d)…", len(tasks)), 0)
		if err := runParallel(16, tasks, func(done, total int) {
			p(fmt.Sprintf("Downloading assets (%d/%d)…", done, total), float64(done)/float64(total))
		}); err != nil {
			return "", "", "", fmt.Errorf("asset download failed: %v", err)
		}
	}

	gameAssets = L.AssetsDir
	target := ""
	switch {
	case idx.MapToResources:
		target = filepath.Join(gameDir, "resources")
	case idx.Virtual:
		target = filepath.Join(L.AssetsDir, "virtual", sanitizeName(id))
	}
	if target != "" {
		p("Preparing resources…", -1)
		var copies []func() error
		for _, n := range names {
			obj := idx.Objects[n]
			if len(obj.Hash) < 2 || strings.Contains(n, "..") {
				continue
			}
			h := strings.ToLower(obj.Hash)
			src := filepath.Join(objectsDir, h[:2], h)
			dst := filepath.Join(target, filepath.FromSlash(n))
			copies = append(copies, func() error { return copyFile(src, dst) })
		}
		if err := runParallel(8, copies, func(done, total int) {
			if done%200 == 0 || done == total {
				p(fmt.Sprintf("Preparing resources (%d/%d)…", done, total), float64(done)/float64(total))
			}
		}); err != nil {
			return "", "", "", fmt.Errorf("couldn't prepare resources: %v", err)
		}
		gameAssets = target
	}
	return L.AssetsDir, id, gameAssets, nil
}

func findJavaExe(dir string) string {
	var cands []string
	if runtime.GOOS == "windows" {
		cands = []string{"bin/javaw.exe", "bin/java.exe"}
	} else if runtime.GOOS == "darwin" {
		cands = []string{"jre.bundle/Contents/Home/bin/java", "bin/java"}
	} else {
		cands = []string{"bin/java"}
	}
	for _, c := range cands {
		p := filepath.Join(dir, filepath.FromSlash(c))
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// ensureRuntime downloads the Java runtime Mojang specifies for a version.
func (L *Launcher) ensureRuntime(v *VersionJSON, p Progress) (string, error) {
	component, major := "jre-legacy", 8
	if v.JavaVersion != nil && v.JavaVersion.Component != "" {
		component = v.JavaVersion.Component
		major = v.JavaVersion.MajorVersion
	}
	if override := L.Settings.JavaPath[component]; override != "" && fileExists(override) {
		return override, nil
	}
	dir := filepath.Join(L.RuntimesDir, sanitizeName(component))
	marker := filepath.Join(dir, ".lemv-complete")
	if exe := findJavaExe(dir); exe != "" && fileExists(marker) {
		return exe, nil
	}

	p(fmt.Sprintf("Looking up Java %d (%s)…", major, component), -1)
	idx, err := L.loadRuntimeIndex()
	if err != nil {
		return "", err
	}
	plat := runtimePlatformKey()
	entries := idx[plat][component]
	if len(entries) == 0 {
		// fall back to any component that ships the right major version
		want := fmt.Sprint(major)
		for comp, es := range idx[plat] {
			for _, en := range es {
				n := en.Version.Name
				if n == want || strings.HasPrefix(n, want+".") || strings.HasPrefix(n, want+"u") {
					L.Logf("java component %s missing for %s; using %s (%s)", component, plat, comp, n)
					entries = es
					break
				}
			}
			if len(entries) > 0 {
				break
			}
		}
	}
	if len(entries) == 0 || entries[0].Manifest.URL == "" {
		return "", fmt.Errorf("Mojang doesn't offer Java %d (%s) for %s. Install it yourself and set javaPath in launcher-settings.json", major, component, plat)
	}
	var man RuntimeManifest
	if err := fetchJSON(entries[0].Manifest.URL, &man); err != nil {
		return "", fmt.Errorf("couldn't download the Java %d file list: %v", major, err)
	}

	var tasks []func() error
	var execs []string
	var links [][2]string
	paths := make([]string, 0, len(man.Files))
	for path := range man.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		f := man.Files[path]
		if strings.Contains(path, "..") {
			continue
		}
		dest := filepath.Join(dir, filepath.FromSlash(path))
		switch f.Type {
		case "directory":
			os.MkdirAll(dest, 0o755)
		case "link":
			links = append(links, [2]string{dest, f.Target})
		case "file":
			d := f.Downloads["raw"]
			if d.URL == "" {
				continue
			}
			if f.Executable {
				execs = append(execs, dest)
			}
			tasks = append(tasks, func() error { return downloadFile(d.URL, dest, d.SHA1, d.Size) })
		}
	}
	p(fmt.Sprintf("Downloading Java %d (0/%d)…", major, len(tasks)), 0)
	if err := runParallel(12, tasks, func(done, total int) {
		p(fmt.Sprintf("Downloading Java %d (%d/%d)…", major, done, total), float64(done)/float64(total))
	}); err != nil {
		return "", fmt.Errorf("Java %d download failed: %v", major, err)
	}
	if runtime.GOOS != "windows" {
		for _, e := range execs {
			os.Chmod(e, 0o755)
		}
		for _, l := range links {
			os.Remove(l[0])
			os.Symlink(l[1], l[0])
		}
	}
	exe := findJavaExe(dir)
	if exe == "" {
		return "", fmt.Errorf("Java %d downloaded but no java executable was found in %s", major, dir)
	}
	os.WriteFile(marker, []byte(entries[0].Version.Name+"\n"), 0o644)
	return exe, nil
}
