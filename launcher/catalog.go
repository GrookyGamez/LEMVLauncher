package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tabs, in sidebar order.
const (
	TabRelease = iota
	TabBeta
	TabAlpha
	TabInfdev
	TabIndev
	TabClassic
	TabPreClassic
	TabAprilFools
	TabLost
	TabMinor
	TabCount
)

type Tab struct {
	Name string
	Desc string
}

var Tabs = [TabCount]Tab{
	{"Release", "One jar per major version, 1.0 through 26.2"},
	{"Beta", "Every Beta build Mojang still hosts · Dec 2010 - Sep 2011"},
	{"Alpha", "Every Alpha build, hotfixes and all · Jun - Dec 2010"},
	{"Infdev", "Every Infdev build · infinite worlds begin · Feb - Jun 2010"},
	{"Indev", "Every Indev build · Dec 2009 - Feb 2010"},
	{"Classic", "Every Classic and Survival Test build · May - Nov 2009"},
	{"Pre-Classic", "The very first week of Minecraft · May 2009"},
	{"April Fools", "Every joke version Mojang ever shipped, 2013 - 2026"},
	{"Rare Versions", "Recovered rarities, via Omniarchive"},
	{"Minor Updates", "Every point release between the base versions, 1.0 onwards"},
}

// isReleaseID matches a release-era version number: 1.0 onwards, digits and
// dots only. It deliberately rejects snapshots (25w14a), pre-releases
// (1.19-pre1), release candidates and every pre-1.0 era id, so nothing older
// than Release ever lands in Minor Updates.
func isReleaseID(id string) bool {
	parts := strings.Split(id, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// releaseNote turns a manifest timestamp into "Mar 2013".
func releaseNote(ts string) string {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Format("Jan 2006")
	}
	return ""
}

// oldEraTab places an old version id in its era and builds a display name.
// Returns false for ids outside the six pre-release eras.
func oldEraTab(id string) (int, string, bool) {
	switch {
	case strings.HasPrefix(id, "rd-"):
		return TabPreClassic, id, true
	case strings.HasPrefix(id, "inf-"):
		return TabInfdev, "Infdev " + strings.TrimPrefix(id, "inf-"), true
	case strings.HasPrefix(id, "in-"):
		return TabIndev, "Indev " + strings.TrimPrefix(id, "in-"), true
	case strings.HasPrefix(id, "c0."):
		base := strings.TrimPrefix(id, "c")
		if strings.Contains(base, "_st") {
			return TabClassic, "Survival Test " + strings.Replace(base, "_st", "", 1), true
		}
		return TabClassic, "Classic " + base, true
	case strings.HasPrefix(id, "a1."):
		return TabAlpha, "Alpha " + strings.TrimPrefix(id, "a"), true
	case strings.HasPrefix(id, "b1."):
		return TabBeta, "Beta " + strings.TrimPrefix(id, "b"), true
	}
	return 0, "", false
}

func monthYear(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	return t.Format("Jan 2006")
}

// ExpandFromManifest builds a row for every old-era version Mojang lists that
// the curated catalog doesn't already cover. Ids come straight from Mojang, so
// they always match what the download endpoint expects. It also returns every
// manifest release date, used to keep each era in chronological order.
func (L *Launcher) ExpandFromManifest() ([]*Entry, map[string]string) {
	m, err := L.loadManifest(7 * 24 * time.Hour)
	if err != nil {
		return nil, nil // offline: the curated catalog still works
	}
	dates := make(map[string]string, len(m.Versions))
	for i := range m.Versions {
		dates[strings.ToLower(m.Versions[i].ID)] = m.Versions[i].ReleaseTime
	}
	known := map[string]bool{}
	byID := map[string]*Entry{}
	for _, e := range L.Entries {
		known[strings.ToLower(e.ID)] = true
		byID[strings.ToLower(e.ID)] = e
		for _, al := range e.Aliases {
			known[strings.ToLower(al)] = true
		}
	}
	// a build we list from the archive may also be on Mojang's servers; if so
	// let Play fetch it directly instead of sending the user to the vault
	for i := range m.Versions {
		if e := byID[strings.ToLower(m.Versions[i].ID)]; e != nil && e.DropInOnly {
			e.DropInOnly, e.Fallback = false, ""
		}
	}
	var out []*Entry
	for i := range m.Versions {
		mv := &m.Versions[i]
		if mv.Type == "release" {
			// Anything from 1.0 onwards that isn't one of the curated base
			// versions is a point release: 1.0.1, 1.2.5, 1.21.4 and so on.
			if known[strings.ToLower(mv.ID)] || !isReleaseID(mv.ID) {
				continue
			}
			known[strings.ToLower(mv.ID)] = true
			// not Extra: that flag means "discovered drop-in jar", and Rescan
			// drops those on every scan
			out = append(out, &Entry{
				ID: mv.ID, Name: mv.ID, Tab: TabMinor,
				Note: releaseNote(mv.ReleaseTime),
			})
			continue
		}
		if mv.Type != "old_alpha" && mv.Type != "old_beta" {
			continue
		}
		tab, name, ok := oldEraTab(mv.ID)
		if !ok || known[strings.ToLower(mv.ID)] {
			continue
		}
		known[strings.ToLower(mv.ID)] = true
		out = append(out, &Entry{ID: mv.ID, Name: name, Note: monthYear(mv.ReleaseTime), Tab: tab})
	}
	return out, dates
}

// naturalLess compares version ids so numeric runs sort by value rather than
// by character: a1.0.4 before a1.0.13, c0.0.11a before c0.0.21a.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	digit := func(c byte) bool { return c >= '0' && c <= '9' }
	for i < len(a) && j < len(b) {
		if digit(a[i]) && digit(b[j]) {
			si, sj := i, j
			for i < len(a) && digit(a[i]) {
				i++
			}
			for j < len(b) && digit(b[j]) {
				j++
			}
			an, _ := strconv.Atoi(a[si:i])
			bn, _ := strconv.Atoi(b[sj:j])
			if an != bn {
				return an < bn
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a)-i < len(b)-j
}

// AddEntries merges expansion rows in and re-orders the six old eras.
func (L *Launcher) AddEntries(add []*Entry, _ map[string]string) {
	if len(add) == 0 {
		return
	}
	L.Entries = append(L.Entries, add...)
	oldEra := map[int]bool{
		TabBeta: true, TabAlpha: true, TabInfdev: true,
		TabIndev: true, TabClassic: true, TabPreClassic: true,
		// point releases arrive in manifest order (newest first); the rest of
		// the launcher reads oldest-first, so sort them the same way
		TabMinor: true,
	}
	byTab := make([][]*Entry, TabCount)
	for _, e := range L.Entries {
		byTab[e.Tab] = append(byTab[e.Tab], e)
	}
	for t := range byTab {
		if !oldEra[t] {
			continue
		}
		rows := byTab[t]
		// ids in these eras are chronological by construction, and unlike
		// manifest dates every row has one (vault rarities included)
		sort.SliceStable(rows, func(i, j int) bool { return naturalLess(rows[i].ID, rows[j].ID) })
	}
	merged := make([]*Entry, 0, len(L.Entries))
	for t := 0; t < TabCount; t++ {
		merged = append(merged, byTab[t]...)
	}
	L.Entries = merged
}

const vaultBase = "https://vault.omniarchive.uk/archive/java/"

// Entry is one row in the launcher.
type Entry struct {
	ID       string   // exact Mojang manifest id, or the archive's jar base name
	Name     string   // friendly display name
	Note     string   // second line in the UI
	Aliases  []string // alternative jar base names that count as this version
	Tab      int
	Fallback string // manifest id whose JSON to use when ID itself isn't in the manifest
	Extra    bool   // discovered from a jar the user dropped in, not part of the curated list

	// Rare buckets the Rare Versions tab: 0 recovered original, 1 dev/test build, 2 press/event build.
	Rare int

	// DropInOnly marks versions Mojang doesn't host: the user has to supply
	// the jar themselves and it can't be auto-downloaded.
	DropInOnly bool

	// GetURL, when set, is where the archived jar for a DropInOnly version
	// can be downloaded (opened in the browser by "Get this jar").
	GetURL string

	JarPath string // filled in by Rescan
}

func (e *Entry) Ready() bool     { return e.JarPath != "" }
func (e *Entry) JarName() string { return e.ID + ".jar" }

func v(tab int, id, name, note string, aliases ...string) *Entry {
	return &Entry{ID: id, Name: name, Note: note, Aliases: aliases, Tab: tab}
}

// vault marks an entry as archived on Omniarchive's vault (direct .jar link).
func (e *Entry) vault(dir, fallback string) *Entry {
	e.DropInOnly = true
	e.Fallback = fallback
	e.GetURL = vaultBase + dir + "/" + e.ID + ".jar"
	return e
}

// lost marks an entry drop-in only with a custom link (wiki page, archive.org, ...).
func (e *Entry) lost(url, fallback string) *Entry {
	e.DropInOnly = true
	e.Fallback = fallback
	e.GetURL = url
	return e
}

func (e *Entry) fb(id string) *Entry { e.Fallback = id; return e }

func (e *Entry) kind(k int) *Entry { e.Rare = k; return e }

// Rare sub-categories, in card order (chronological eras, then dev builds).
var RareKinds = []struct{ Name, Desc string }{
	{"Classic", "Early Classic rarities · 2009"},
	{"Survival Test", "Where survival mode was born · 2009"},
	{"Indev", "In-development era landmarks · 2009-2010"},
	{"Infdev", "Infinite-world era finds · 2010"},
	{"Alpha", "Seecret-era finds · 2010"},
	{"Beta", "Recovered Beta builds · 2011"},
	{"Release era", "Special builds from the release years"},
	{"Dev builds", "Internal builds that never shipped"},
}

var rareKindNames = [3]string{"Recovered", "Still Lost", "Oddities"}
var rareKindDescs = [3]string{"Found again — downloadable", "Never found — the wanted list", "Not quite Minecraft"}

// catalog is the curated list of base versions across every era.
func catalog() []*Entry {
	R, B, A, IF, IN, C, P, AF, L := TabRelease, TabBeta, TabAlpha, TabInfdev, TabIndev, TabClassic, TabPreClassic, TabAprilFools, TabLost
	list := []*Entry{
		// ---- Release: one per major version ---------------------------------
		v(R, "1.0", "1.0", "Adventure Update · Nov 2011", "1.0.0"),
		v(R, "1.1", "1.1", "Jan 2012"),
		v(R, "1.2.1", "1.2", "First 1.2 release · Mar 2012", "1.2"),
		v(R, "1.3.1", "1.3", "First 1.3 release · Aug 2012", "1.3"),
		v(R, "1.4.2", "1.4", "Pretty Scary Update · Oct 2012", "1.4"),
		v(R, "1.5", "1.5", "Redstone Update · Mar 2013").lost(vaultBase+"client-release/1.5/1.5.jar", "1.5.1"),
		v(R, "1.6.1", "1.6", "Horse Update · Jul 2013", "1.6"),
		v(R, "1.7.2", "1.7", "The Update that Changed the World · Oct 2013", "1.7"),
		v(R, "1.8", "1.8", "Bountiful Update · Sep 2014"),
		v(R, "1.9", "1.9", "Combat Update · Feb 2016"),
		v(R, "1.10", "1.10", "Frostburn Update · Jun 2016"),
		v(R, "1.11", "1.11", "Exploration Update · Nov 2016"),
		v(R, "1.12", "1.12", "World of Color · Jun 2017"),
		v(R, "1.13", "1.13", "Update Aquatic · Jul 2018"),
		v(R, "1.14", "1.14", "Village & Pillage · Apr 2019"),
		v(R, "1.15", "1.15", "Buzzy Bees · Dec 2019"),
		v(R, "1.16", "1.16", "Nether Update · Jun 2020"),
		v(R, "1.17", "1.17", "Caves & Cliffs Part I · Jun 2021"),
		v(R, "1.18", "1.18", "Caves & Cliffs Part II · Nov 2021"),
		v(R, "1.19", "1.19", "The Wild Update · Jun 2022"),
		v(R, "1.20", "1.20", "Trails & Tales · Jun 2023"),
		v(R, "1.21", "1.21", "Tricky Trials · Jun 2024"),
		v(R, "26.1", "26.1", "First year-numbered release · Mar 2026"),
		v(R, "26.2", "26.2", "2026"),

		// ---- Beta -----------------------------------------------------------
		v(B, "b1.0", "Beta 1.0", "Beta begins · Dec 2010"),
		v(B, "b1.1_02", "Beta 1.1", "Dec 2010", "b1.1"),
		v(B, "b1.2", "Beta 1.2", "Jan 2011"),
		v(B, "b1.3_01", "Beta 1.3", "Beds & repeaters · Feb 2011", "b1.3"),
		v(B, "b1.4", "Beta 1.4", "Wolves · Mar 2011"),
		v(B, "b1.5", "Beta 1.5", "Apr 2011"),
		v(B, "b1.6", "Beta 1.6", "Maps & Nether portals in SMP · May 2011"),
		v(B, "b1.7", "Beta 1.7", "Pistons · Jun 2011"),
		v(B, "b1.8", "Beta 1.8", "Adventure Update Part I · Sep 2011"),

		// ---- Alpha: one row every five updates, plus the last ------------
		v(A, "a1.0.1_01", "Alpha 1.0.1", "Earliest surviving Alpha · Seecret Friday 1 · Jun 2010", "a1.0.1").vault("client-alpha", "a1.0.4"),
		v(A, "a1.0.5_01", "Alpha 1.0.5", "Jul 2010", "a1.0.5"),
		v(A, "a1.0.10", "Alpha 1.0.10", "Jul 2010").vault("client-alpha", "a1.0.11"),
		v(A, "a1.0.15", "Alpha 1.0.15", "First multiplayer test · Aug 2010"),
		v(A, "a1.1.0", "Alpha 1.1", "Seecret Friday 9 · compass · Sep 2010", "a1.1"),
		v(A, "a1.2.0", "Alpha 1.2", "Halloween Update · the Nether · Oct 2010", "a1.2"),
		v(A, "a1.2.5", "Alpha 1.2.5", "Dec 2010"),
		v(A, "a1.2.6", "Alpha 1.2.6", "Last Alpha · Dec 2010"),

		// ---- Infdev: every build in Omniarchive's vault --------------------
		v(IF, "inf-20100313", "Infdev 20100313", "Mar 13, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100316", "Infdev 20100316", "Mar 16, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100320", "Infdev 20100320", "Mar 20, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100321-1857", "Infdev 20100321-1857", "Mar 21, 2010 · 18:57").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100325-1640", "Infdev 20100325-1640", "Mar 25, 2010 · 16:40").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100327", "Infdev 20100327", "Mar 27, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100413-1953", "Infdev 20100413-1953", "Apr 13, 2010 · 19:53").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100414", "Infdev 20100414", "Apr 14, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100415", "Infdev 20100415", "Apr 15, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100420", "Infdev 20100420", "Apr 20, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100607", "Infdev 20100607", "Jun 7, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100608", "Infdev 20100608", "Jun 8, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100611", "Infdev 20100611", "Jun 11, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100615", "Infdev 20100615", "Jun 15, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100616-1808", "Infdev 20100616-1808", "Jun 16, 2010 · 18:08").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100617-1205", "Infdev 20100617-1205", "Jun 17, 2010 · 12:05").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100617-1531", "Infdev 20100617-1531", "Jun 17, 2010 · 15:31").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100618", "Infdev 20100618", "The one Infdev build Mojang still hosts").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100624", "Infdev 20100624", "Jun 24, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100625-0922", "Infdev 20100625-0922", "Jun 25, 2010 · 09:22").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100625-1917", "Infdev 20100625-1917", "Jun 25, 2010 · 19:17").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100627", "Infdev 20100627", "Jun 27, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100629", "Infdev 20100629", "Jun 29, 2010").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100630-1340", "Infdev 20100630-1340", "Jun 30, 2010 · 13:40").vault("client-infdev", "inf-20100618"),
		v(IF, "inf-20100630-1835", "Infdev 20100630-1835", "Last Infdev build").vault("client-infdev", "inf-20100618"),

		// ---- Indev: every build in Omniarchive's vault ---------------------
		v(IN, "in-20091231-2255", "Indev 20091231-2255", "Dec 31, 2009 · 22:55").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100104-2258", "Indev 20100104-2258", "Jan 4, 2010 · 22:58").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100110", "Indev 20100110", "Jan 10, 2010").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100124-2310", "Indev 20100124-2310", "Jan 24, 2010 · 23:10").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100125", "Indev 20100125", "Jan 25, 2010").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100128-2304", "Indev 20100128-2304", "Jan 28, 2010 · 23:04").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100129-1452", "Indev 20100129-1452", "Jan 29, 2010 · 14:52").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100130", "Indev 20100130", "Jan 30, 2010").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100131-2244", "Indev 20100131-2244", "Jan 31, 2010 · 22:44").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100201-0025", "Indev 20100201-0025", "Feb 1, 2010 · 00:25").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100201-2227", "Indev 20100201-2227", "Feb 1, 2010 · 22:27").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100202-2330", "Indev 20100202-2330", "Feb 2, 2010 · 23:30").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100206-2103", "Indev 20100206-2103", "The one Indev build Mojang still hosts", "in-20100206").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100207-1101", "Indev 20100207-1101", "Feb 7, 2010 · 11:01").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100207-1703", "Indev 20100207-1703", "Feb 7, 2010 · 17:03").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100212-1210", "Indev 20100212-1210", "Feb 12, 2010 · 12:10").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100212-1622", "Indev 20100212-1622", "Feb 12, 2010 · 16:22").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100213", "Indev 20100213", "Feb 13, 2010").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100214", "Indev 20100214", "Feb 14, 2010").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100218-0016", "Indev 20100218-0016", "Feb 18, 2010 · 00:16").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100219", "Indev 20100219", "Feb 19, 2010").vault("client-indev", "inf-20100618"),
		v(IN, "in-20100223", "Indev 20100223", "Last Indev build").vault("client-indev", "inf-20100618"),

		// ---- Classic --------------------------------------------------------
		v(C, "c0.0.11a", "Classic 0.0.11a", "First public build · May 2009"),
		v(C, "c0.0.16a_02-081047", "Classic 0.0.16a", "Jun 2009", "c0.0.16a_02", "c0.0.16a").vault("client-classic", "c0.0.13a_03"),
		v(C, "c0.0.21a-2008", "Classic 0.0.21a", "Jul 2009", "c0.0.21a").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.23a_01", "Classic 0.0.23a", "Last 0.0.x · Oct 2009", "c0.0.23a").vault("client-classic", "c0.30_01c"),
		v(C, "c0.30_01c", "Classic 0.30", "Classic creative mode · Nov 2009", "c0.30"),
		v(C, "c0.0.13a_03", "Classic 0.0.13a_03", "May 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.14a_08", "Classic 0.0.14a_08", "May 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.15a-05311904", "Classic 0.0.15a-05311904", "Multiplayer Test 1 · May 31, 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.17a-2014", "Classic 0.0.17a-2014", "Jun 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.18a_02", "Classic 0.0.18a_02", "Jun 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.19a_04", "Classic 0.0.19a_04", "Jun 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.19a_06-0137", "Classic 0.0.19a_06-0137", "Jun 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.20a_01", "Classic 0.0.20a_01", "Jun 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.20a_02", "Classic 0.0.20a_02", "Jun 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.0.22a_05", "Classic 0.0.22a_05", "Jul 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.28_01", "Classic 0.28_01", "Nov 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.29", "Classic 0.29", "Nov 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.29_01", "Classic 0.29_01", "Nov 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.29_02", "Classic 0.29_02", "Nov 2009").vault("client-classic", "c0.30_01c"),
		v(C, "c0.30-c-1900-renew", "Classic 0.30-c-1900-renew", "Renewed upload of the Nov 10, 2009 Creative build").vault("client-classic", "c0.30_01c"),

		// ---- Pre-Classic: the first and the last of week one ----------------
		v(P, "rd-132211", "rd-132211", "'Cave Game', the oldest build · May 13, 2009"),
		v(P, "rd-161348", "rd-161348", "First build simply named Minecraft · May 16, 2009"),

		// ---- April Fools ----------------------------------------------------
		v(AF, "2.0_blue", "2.0 (Blue)", "2013 · uses 1.5.1 libraries", "2point0_blue").lost("https://archive.org/download/2point-0-blue/2point0_blue.jar", "1.5.1"),
		v(AF, "2.0_red", "2.0 (Red)", "2013 · uses 1.5.1 libraries", "2point0_red").lost("https://archive.org/download/2point-0-red/2point0_red.jar", "1.5.1"),
		v(AF, "2.0_purple", "2.0 (Purple)", "2013 · uses 1.5.1 libraries", "2point0_purple").lost("https://archive.org/download/2point-0-purple/2point0_purple.jar", "1.5.1"),
		v(AF, "15w14a", "15w14a", "The Love and Hugs Update · 2015"),
		v(AF, "1.RV-Pre1", "1.RV-Pre1", "Trendy Update · 2016"),
		v(AF, "3D Shareware v1.34", "3D Shareware v1.34", "Minecraft 3D · 2019", "3d-shareware-v1.34", "3D-Shareware-v1.34"),
		v(AF, "20w14infinite", "20w14∞", "Ultimate Content Update · 2020", "20w14inf"),
		v(AF, "22w13oneblockatatime", "22w13oneBlockAtATime", "One block at a time · 2022"),
		v(AF, "23w13a_or_b", "23w13a_or_b", "The Vote Update · 2023"),
		v(AF, "24w14potato", "24w14potato", "Poisonous Potato Update · 2024"),
		v(AF, "25w14craftmine", "25w14craftmine", "Craft Mine · 2025"),
		v(AF, "26w14a", "26w14a", "Herdcraft · 2026"),

		// ---- Rare Versions: recovered builds, bucketed by era ---------------
		v(L, "c0.0.12a_03-200018", "Classic 0.0.12a_03", "Recovered early Classic build · May 2009", "c0.0.12a_03").vault("client-classic", "c0.30_01c").kind(0),
		v(L, "c0.30-c-1900", "Classic 0.30 Creative (original)", "The true Nov 2009 Creative build · launcher ships a 2011 recompile", "c0.30-c").vault("client-classic", "c0.30_01c").kind(0),
		v(L, "c0.30-s-1858", "Classic 0.30 Survival", "The Survival twin, released the same evening as Creative · Nov 2009", "c0.30-s").vault("client-classic", "c0.30_01c").kind(0),
		v(L, "c0.24_st_03", "Survival Test 0.24_03", "First Survival Test · Sep 2009", "c0.24-st").vault("client-classic", "c0.30_01c").kind(1),
		v(L, "c0.25_05_st", "Survival Test 0.25_05", "The era survival mode was born · Sep 2009", "c0.25-st").vault("client-classic", "c0.30_01c").kind(1),
		v(L, "c0.27_st", "Survival Test 0.27", "Late Survival Test · Oct 2009", "c0.27-st").vault("client-classic", "c0.30_01c").kind(1),
		v(L, "in-20091223-1459", "Indev 0.31 20091223-1459", "The very first Indev build · Dec 23, 2009", "in-20091223").vault("client-indev", "inf-20100618").kind(2),
		v(L, "inf-20100227-1433", "Infdev 20100227-1433", "The first Infdev build · infinite worlds begin · Feb 27, 2010", "inf-20100227").vault("client-infdev", "inf-20100618").kind(3),
		v(L, "inf-20100330-1611_modified", "Infdev 20100330 (modified)", "Only a tampered copy survives · the clean build has never surfaced", "inf-20100330").vault("client-infdev", "inf-20100618").kind(3),
		v(L, "a1.1.1", "Alpha 1.1.1", "Seecret Saturday · lost for 11 years, found on an old laptop in 2021").vault("client-alpha", "a1.1.2").kind(4),
		v(L, "b1.3-1733", "Original Beta 1.3", "The true b1.3 before the _01 reissue · lost until Dec 2023", "b1.3-original").vault("client-beta/b1.3", "b1.3_01").kind(5),
		v(L, "b1.3-pcgamer", "PC Gamer Demo", "Free 100-minute demo from PC Gamer's DVD · Apr 2011", "pcgamer-demo", "b1.3-demo").vault("client-beta/b1.3/demo", "b1.3_01").kind(5),
		v(L, "1.0.0-tominecon", "1.0.0 MineCon Build", "Made for MINECON 2011 · one auth fix ahead of release · found in 2024", "tominecon").vault("misc", "1.0").kind(6),
		v(L, "2.0-preview", "Minecraft 2.0 (preview)", "The build YouTubers got under NDA a week early · Mar 2013 · found Oct 2025").vault("client-april-fools", "1.5.1").kind(6),
		v(L, "b1.2_02-20110517", "Beta 1.2_02 Builders", "Robot Steve 'Builders' construct a house · made the Xperia Play trailer · May 2011").vault("misc", "b1.2_02").kind(7),
		v(L, "b1.6-tb3", "Beta 1.6 Test Build 3", "Never released publicly · the only archived Beta test build · May 2011").vault("client-beta/b1.6/pre", "b1.6").kind(7),
		v(L, "b1.8-pre2-121559", "Beta 1.8 Pre-release 2 (121559)", "Lost revision of the Adventure Update pre-release · found Dec 2023", "b1.8-pre2").vault("client-beta/b1.8/pre", "b1.8").kind(7),
	}
	return list
}

// classify guesses the tab for a jar the user dropped in that isn't curated.
var aprilFoolsPattern = regexp.MustCompile(`^\d{2}w\d{2}[a-z_∞][a-z_0-9∞]+`)

var aprilFoolsIDs = map[string]bool{
	"2.0_blue": true, "2.0_red": true, "2.0_purple": true, "15w14a": true, "1.rv-pre1": true,
	"3d shareware v1.34": true, "20w14infinite": true, "22w13oneblockatatime": true,
	"23w13a_or_b": true, "24w14potato": true, "25w14craftmine": true, "26w14a": true,
}

func classify(id string) int {
	l := strings.ToLower(id)
	switch {
	case strings.HasPrefix(l, "rd-"):
		return TabPreClassic
	case strings.HasPrefix(l, "c0.") || strings.HasPrefix(l, "0.3"):
		return TabClassic
	case strings.HasPrefix(l, "in-"):
		return TabIndev
	case strings.HasPrefix(l, "inf-"):
		return TabInfdev
	case strings.HasPrefix(l, "a1.") || strings.HasPrefix(l, "a0."):
		return TabAlpha
	case strings.HasPrefix(l, "b1.") || strings.HasPrefix(l, "b0."):
		return TabBeta
	case aprilFoolsIDs[l], aprilFoolsPattern.MatchString(l), strings.HasPrefix(l, "2.0"), strings.HasPrefix(l, "1.rv"):
		return TabAprilFools
	default:
		return TabRelease
	}
}

// matchesSearch reports whether the entry matches a lowercase search query.
func (e *Entry) matchesSearch(q string) bool {
	if strings.Contains(strings.ToLower(e.ID), q) || strings.Contains(strings.ToLower(e.Name), q) || strings.Contains(strings.ToLower(e.Note), q) {
		return true
	}
	for _, a := range e.Aliases {
		if strings.Contains(strings.ToLower(a), q) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(Tabs[e.Tab].Name), q)
}

// Search returns all entries (curated + extras) matching the query, in tab order.
func (L *Launcher) Search(query string) []*Entry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []*Entry
	for tab := 0; tab < TabCount; tab++ {
		for _, e := range L.EntriesForTab(tab) {
			if e.matchesSearch(q) {
				out = append(out, e)
			}
		}
	}
	return out
}

// scanJars finds every jar the user dropped into the versions folder.
// Accepted layouts:
//
//	versions/<id>.jar
//	versions/<id>/<id>.jar
//	versions/<id>/minecraft.jar   (or client.jar, or any single .jar)
//
// The result maps a lower-cased version id to the jar's path.
func scanJars(dir string) map[string]string {
	found := map[string]string{}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return found
	}
	for _, de := range ents {
		name := de.Name()
		if de.IsDir() {
			sub := filepath.Join(dir, name)
			for _, cand := range []string{name + ".jar", "minecraft.jar", "client.jar"} {
				if st, err := os.Stat(filepath.Join(sub, cand)); err == nil && st.Mode().IsRegular() {
					found[strings.ToLower(name)] = filepath.Join(sub, cand)
					break
				}
			}
			if _, ok := found[strings.ToLower(name)]; !ok {
				jars, _ := filepath.Glob(filepath.Join(sub, "*.jar"))
				if len(jars) == 1 {
					found[strings.ToLower(name)] = jars[0]
				}
			}
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), ".jar") {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		found[strings.ToLower(base)] = filepath.Join(dir, name)
	}
	return found
}

// Rescan re-reads the versions folder and updates every entry's JarPath.
// Jars that don't match a curated entry are added as extra entries.
// ImportFromDownloads copies jars the user downloaded (via the Download
// buttons) from their Downloads folder into the versions folder. Only exact
// filename matches for missing drop-in versions are touched, and files are
// copied, never moved. Returns the names imported.
func (L *Launcher) ImportFromDownloads() []string {
	if !L.Settings.AutoImport {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dl := filepath.Join(home, "Downloads")
	var got []string
	for _, e := range L.Entries {
		if !e.DropInOnly || e.Ready() {
			continue
		}
		src := filepath.Join(dl, e.JarName())
		if fi, err := os.Stat(src); err != nil || fi.IsDir() || fi.Size() == 0 {
			continue
		}
		os.MkdirAll(L.VersionsDir, 0o755)
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		if os.WriteFile(filepath.Join(L.VersionsDir, e.JarName()), data, 0o644) == nil {
			got = append(got, e.JarName())
		}
	}
	return got
}

// importFromDownloads moves jars the user downloaded for missing entries
// into the versions folder: copy, verify, then delete the original.
func (L *Launcher) importFromDownloads() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dl := filepath.Join(home, "Downloads")
	wanted := map[string]string{} // lowercase filename -> canonical jar name
	for _, e := range L.Entries {
		if e.Ready() || !e.DropInOnly {
			continue
		}
		wanted[strings.ToLower(e.JarName())] = e.JarName()
		for _, al := range e.Aliases {
			wanted[strings.ToLower(al)+".jar"] = e.JarName()
		}
	}
	items, err := os.ReadDir(dl)
	if err != nil {
		return nil
	}
	var moved []string
	for _, it := range items {
		if it.IsDir() {
			continue
		}
		canon, ok := wanted[strings.ToLower(it.Name())]
		if !ok {
			continue
		}
		src := filepath.Join(dl, it.Name())
		dst := filepath.Join(L.VersionsDir, canon)
		data, err := os.ReadFile(src)
		if err != nil || len(data) == 0 {
			continue
		}
		os.MkdirAll(L.VersionsDir, 0o755)
		if err := os.WriteFile(dst+".part", data, 0o644); err != nil {
			continue
		}
		if err := os.Rename(dst+".part", dst); err != nil {
			os.Remove(dst + ".part")
			continue
		}
		if w, err := os.Stat(dst); err == nil && w.Size() == int64(len(data)) {
			os.Remove(src) // the copy is verified; clear it out of Downloads
		}
		moved = append(moved, canon)
		delete(wanted, strings.ToLower(it.Name()))
	}
	return moved
}

func (L *Launcher) Rescan() {
	if L.Settings.AutoImport {
		L.LastImported = L.importFromDownloads()
	} else {
		L.LastImported = nil
	}
	found := scanJars(L.VersionsDir)
	used := map[string]bool{}

	// keep curated entries, drop previously discovered extras
	var entries []*Entry
	for _, e := range L.Entries {
		if !e.Extra {
			entries = append(entries, e)
		}
	}
	for _, e := range entries {
		e.JarPath = ""
		for _, n := range append([]string{e.ID}, e.Aliases...) {
			if p, ok := found[strings.ToLower(n)]; ok {
				e.JarPath = p
				used[strings.ToLower(n)] = true
				break
			}
		}
	}
	var extras []*Entry
	for key, p := range found {
		if used[key] {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if fi, err := os.Stat(filepath.Dir(p)); err == nil && fi.IsDir() && filepath.Dir(p) != L.VersionsDir {
			id = filepath.Base(filepath.Dir(p))
		}
		extras = append(extras, &Entry{ID: id, Name: id, Note: "Extra jar you added", Tab: classify(id), Extra: true, JarPath: p})
	}
	sort.Slice(extras, func(i, j int) bool { return extras[i].ID < extras[j].ID })
	L.Entries = append(entries, extras...)
}

// EntriesForTab returns the entries shown on one tab, in display order.
func (L *Launcher) EntriesForTab(tab int) []*Entry {
	var out []*Entry
	for _, e := range L.Entries {
		if e.Tab == tab {
			out = append(out, e)
		}
	}
	return out
}

// FindEntry looks an entry up by id or alias (case-insensitive).
func (L *Launcher) FindEntry(id string) *Entry {
	for _, e := range L.Entries {
		if strings.EqualFold(e.ID, id) {
			return e
		}
		for _, a := range e.Aliases {
			if strings.EqualFold(a, id) {
				return e
			}
		}
	}
	return nil
}

// ReadyCount returns (jars found, total) for a tab.
func (L *Launcher) ReadyCount(tab int) (int, int) {
	n, total := 0, 0
	for _, e := range L.Entries {
		if e.Tab == tab {
			total++
			if e.Ready() {
				n++
			}
		}
	}
	return n, total
}
