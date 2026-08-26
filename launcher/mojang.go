package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Mojang endpoints. The env overrides exist so the launcher can be pointed at
// a local mock server for testing.
var (
	manifestURLs = []string{
		envOr("LEMV_MANIFEST_URL", "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"),
		"https://launchermeta.mojang.com/mc/game/version_manifest_v2.json",
	}
	jreIndexURLs = []string{
		envOr("LEMV_JRE_INDEX_URL", "https://piston-meta.mojang.com/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json"),
		"https://launchermeta.mojang.com/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json",
	}
	resourcesURL = envOr("LEMV_RESOURCES_URL", "https://resources.download.minecraft.net/")
	librariesURL = envOr("LEMV_LIBRARIES_URL", "https://libraries.minecraft.net/")
)

func init() {
	if os.Getenv("LEMV_MANIFEST_URL") != "" {
		manifestURLs = manifestURLs[:1]
	}
	if os.Getenv("LEMV_JRE_INDEX_URL") != "" {
		jreIndexURLs = jreIndexURLs[:1]
	}
}

// ---- version manifest ------------------------------------------------------

type Manifest struct {
	Latest   map[string]string `json:"latest"`
	Versions []ManifestVersion `json:"versions"`
}

type ManifestVersion struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Time        string `json:"time"`
	ReleaseTime string `json:"releaseTime"`
	SHA1        string `json:"sha1"`
}

// ---- version JSON ----------------------------------------------------------

type Download struct {
	ID   string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

type LibraryDownloads struct {
	Artifact    *Download            `json:"artifact"`
	Classifiers map[string]*Download `json:"classifiers"`
}

type Library struct {
	Name      string            `json:"name"`
	Downloads *LibraryDownloads `json:"downloads"`
	Natives   map[string]string `json:"natives"`
	Extract   *struct {
		Exclude []string `json:"exclude"`
	} `json:"extract"`
	Rules []Rule `json:"rules"`
	URL   string `json:"url"`
}

type Arguments struct {
	Game []json.RawMessage `json:"game"`
	JVM  []json.RawMessage `json:"jvm"`
}

type AssetIndexRef struct {
	ID        string `json:"id"`
	SHA1      string `json:"sha1"`
	Size      int64  `json:"size"`
	TotalSize int64  `json:"totalSize"`
	URL       string `json:"url"`
}

type JavaVersionRef struct {
	Component    string `json:"component"`
	MajorVersion int    `json:"majorVersion"`
}

type LoggingClient struct {
	Argument string   `json:"argument"`
	File     Download `json:"file"`
	Type     string   `json:"type"`
}

type VersionJSON struct {
	ID                 string          `json:"id"`
	Type               string          `json:"type"`
	MainClass          string          `json:"mainClass"`
	MinecraftArguments string          `json:"minecraftArguments"`
	Arguments          *Arguments      `json:"arguments"`
	Assets             string          `json:"assets"`
	AssetIndex         *AssetIndexRef  `json:"assetIndex"`
	JavaVersion        *JavaVersionRef `json:"javaVersion"`
	Libraries          []Library       `json:"libraries"`
	Logging            *struct {
		Client *LoggingClient `json:"client"`
	} `json:"logging"`
	Downloads   map[string]Download `json:"downloads"`
	ReleaseTime string              `json:"releaseTime"`

	ResolvedFrom string `json:"-"` // manifest id whose JSON this is
}

// ---- asset index -----------------------------------------------------------

type AssetObject struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

type AssetIndex struct {
	Virtual        bool                   `json:"virtual"`
	MapToResources bool                   `json:"map_to_resources"`
	Objects        map[string]AssetObject `json:"objects"`
}

// ---- java runtimes ---------------------------------------------------------

type RuntimeEntry struct {
	Manifest Download `json:"manifest"`
	Version  struct {
		Name     string `json:"name"`
		Released string `json:"released"`
	} `json:"version"`
}

// platform -> component -> entries
type RuntimeIndex map[string]map[string][]RuntimeEntry

type RuntimeFile struct {
	Type       string              `json:"type"`
	Executable bool                `json:"executable"`
	Target     string              `json:"target"`
	Downloads  map[string]Download `json:"downloads"`
}

type RuntimeManifest struct {
	Files map[string]RuntimeFile `json:"files"`
}

func runtimePlatformKey() string {
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "386":
			return "windows-x86"
		case "arm64":
			return "windows-arm64"
		default:
			return "windows-x64"
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "mac-os-arm64"
		}
		return "mac-os"
	default:
		if runtime.GOARCH == "386" {
			return "linux-i386"
		}
		return "linux"
	}
}

// ---- fetching --------------------------------------------------------------

const cacheMaxAge = 12 * time.Hour

// cachedJSON returns the cached file when it is younger than maxAge, otherwise
// downloads from the first mirror that answers (falling back to a stale cache
// when offline).
func cachedJSON(path string, urls []string, maxAge time.Duration, v any) error {
	if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < maxAge {
		if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, v) == nil {
			return nil
		}
	}
	data, err := fetchBytesAny(urls)
	if err == nil {
		if jerr := json.Unmarshal(data, v); jerr == nil {
			os.MkdirAll(filepath.Dir(path), 0o755)
			os.WriteFile(path, data, 0o644)
			return nil
		}
		err = fmt.Errorf("unexpected response from %s", urls[0])
	}
	if data, rerr := os.ReadFile(path); rerr == nil && json.Unmarshal(data, v) == nil {
		return nil
	}
	return err
}

// Omniarchive publishes a superset of Mojang's manifest in the same format.
const omniarchiveManifestURL = "https://meta.omniarchive.uk/v1/manifest.json"

func (L *Launcher) loadManifest(maxAge time.Duration) (*Manifest, error) {
	var m Manifest
	err := cachedJSON(filepath.Join(L.MetaDir, "version_manifest_v2.json"), manifestURLs, maxAge, &m)
	if err != nil {
		return nil, fmt.Errorf("couldn't download Mojang's version list (are you online?): %v", err)
	}
	if len(m.Versions) == 0 {
		return nil, fmt.Errorf("Mojang's version list came back empty")
	}
	L.mergeOmniarchive(&m, maxAge)
	return &m, nil
}

// mergeOmniarchive folds in Omniarchive's version list, which is published in
// the same shape as Mojang's and carries releases Mojang no longer hosts.
// Only release-era entries are taken: the pre-1.0 eras are already covered by
// the curated catalog, and pulling them in here would drag Alpha and Beta
// builds into Minor Updates.
//
// Best effort by design — the archive being unreachable must never stop the
// launcher from listing Mojang's own versions.
func (L *Launcher) mergeOmniarchive(m *Manifest, maxAge time.Duration) {
	url := envOr("LEMV_OMNIARCHIVE_MANIFEST", omniarchiveManifestURL)
	if url == "" {
		return
	}
	var om Manifest
	if err := cachedJSON(filepath.Join(L.MetaDir, "omniarchive_manifest.json"), []string{url}, maxAge, &om); err != nil {
		L.Logf("Omniarchive version list unavailable (%v) — continuing with Mojang's alone", err)
		return
	}
	have := make(map[string]bool, len(m.Versions))
	for i := range m.Versions {
		have[strings.ToLower(m.Versions[i].ID)] = true
	}
	added := 0
	for i := range om.Versions {
		v := om.Versions[i]
		if v.Type != "release" || v.URL == "" || have[strings.ToLower(v.ID)] {
			continue
		}
		have[strings.ToLower(v.ID)] = true
		m.Versions = append(m.Versions, v)
		added++
	}
	if added > 0 {
		L.Logf("Omniarchive added %d release versions Mojang no longer hosts", added)
	}
}

func findVersion(m *Manifest, id string) *ManifestVersion {
	for i := range m.Versions {
		if m.Versions[i].ID == id {
			return &m.Versions[i]
		}
	}
	for i := range m.Versions {
		if strings.EqualFold(m.Versions[i].ID, id) {
			return &m.Versions[i]
		}
	}
	return nil
}

// fuzzyVersion finds the earliest manifest version whose id extends the
// requested one: "1.5" -> "1.5.1", "b1.3" -> "b1.3_01", "a1.0" -> "a1.0.4".
// Digits are not allowed right after the prefix, so "1.1" never matches "1.10".
func fuzzyVersion(m *Manifest, id string) *ManifestVersion {
	want := strings.ToLower(id)
	var best *ManifestVersion
	for i := range m.Versions {
		v := &m.Versions[i]
		have := strings.ToLower(v.ID)
		if !strings.HasPrefix(have, want) || have == want {
			continue
		}
		c := have[len(want)]
		if c != '.' && c != '_' && c != '-' && !(c >= 'a' && c <= 'z') {
			continue
		}
		if best == nil || v.ReleaseTime < best.ReleaseTime {
			best = v
		}
	}
	return best
}

// resolveVersion finds the version JSON for an entry: exact id first, then
// the entry's explicit fallback, then the closest id in the same family.
func (L *Launcher) resolveVersion(e *Entry) (*VersionJSON, error) {
	m, err := L.loadManifest(cacheMaxAge)
	if err != nil {
		return nil, err
	}
	mv := findVersion(m, e.ID)
	if mv == nil {
		// maybe the list is stale (new release); re-check, but not more than every few minutes
		if m2, err := L.loadManifest(5 * time.Minute); err == nil {
			m = m2
			mv = findVersion(m, e.ID)
		}
	}
	if mv == nil && e.Fallback != "" {
		mv = findVersion(m, e.Fallback)
	}
	if mv == nil {
		mv = fuzzyVersion(m, e.ID)
	}
	if mv == nil {
		return nil, fmt.Errorf("%q isn't in Mojang's version list, so there's nothing to tell me which libraries it needs. Name the jar with an official version id (like 1.2.1, b1.7.3 or a1.2.6)", e.ID)
	}
	if mv.ID != e.ID {
		L.Logf("version %s is not in the manifest; using the JSON of %s", e.ID, mv.ID)
	}
	return L.fetchVersionJSON(mv)
}

// fetchVersionJSON downloads and parses one manifest entry's version JSON.
// It does not follow inheritsFrom — callers use resolveInherits for that.
func (L *Launcher) fetchVersionJSON(mv *ManifestVersion) (*VersionJSON, error) {
	path := filepath.Join(L.MetaDir, "versions", sanitizeName(mv.ID)+".json")
	if err := downloadFile(mv.URL, path, mv.SHA1, 0); err != nil {
		return nil, fmt.Errorf("couldn't download the version info for %s: %v", mv.ID, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v VersionJSON
	if err := json.Unmarshal(data, &v); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("version info for %s is malformed: %v", mv.ID, err)
	}
	if v.ID == "" {
		v.ID = mv.ID
	}
	if v.Type == "" {
		v.Type = mv.Type
	}
	v.ResolvedFrom = mv.ID
	return &v, nil
}

func (L *Launcher) loadAssetIndex(v *VersionJSON) (*AssetIndex, string, error) {
	id := v.Assets
	if v.AssetIndex != nil && v.AssetIndex.ID != "" {
		id = v.AssetIndex.ID
	}
	if id == "" {
		id = "legacy"
	}
	path := filepath.Join(L.AssetsDir, "indexes", sanitizeName(id)+".json")
	if v.AssetIndex != nil && v.AssetIndex.URL != "" {
		if err := downloadFile(v.AssetIndex.URL, path, v.AssetIndex.SHA1, v.AssetIndex.Size); err != nil {
			return nil, id, fmt.Errorf("couldn't download the asset index %s: %v", id, err)
		}
	} else if !fileExists(path) {
		return nil, id, fmt.Errorf("version info has no asset index information")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, id, err
	}
	var idx AssetIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		os.Remove(path)
		return nil, id, fmt.Errorf("asset index %s is malformed: %v", id, err)
	}
	return &idx, id, nil
}

func (L *Launcher) loadRuntimeIndex() (RuntimeIndex, error) {
	var idx RuntimeIndex
	err := cachedJSON(filepath.Join(L.MetaDir, "java_runtimes.json"), jreIndexURLs, cacheMaxAge, &idx)
	if err != nil {
		return nil, fmt.Errorf("couldn't download Mojang's Java runtime list: %v", err)
	}
	return idx, nil
}

// sanitizeName makes a version id safe to use as a file or folder name.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "unnamed"
	}
	return out
}
