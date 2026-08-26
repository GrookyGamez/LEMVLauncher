# LEMV Launcher — handover notes (v3.4.0)

## What this build is

Back to the original shape: **pick a version, hit Play.** The profile system,
mod loaders, NeoForge and the Modded view were all removed on purpose in
v3.0.0. This is a deliberate scope decision, not an unfinished state.

## What was removed

- `profiles.go` and `neoforge.go` deleted outright.
- The Profiles rail entry, `viewProfiles` / `viewLoader` / `viewLoaderMC` /
  `viewLoaderBuild` / `viewModded`, and all their drawing, hit-testing, layout
  rects, click handlers and state.
- The `--neoforge-list`, `--neoforge-install` and `--play-profile` CLI commands.
- `inheritsFrom` support in `mojang.go` (`resolveInherits`, `mergeInherited`,
  `LoadLocalVersionJSON`, `libKey`) — that existed only to merge a loader's
  version JSON onto its vanilla parent.
- `EntryForVersion` in `catalog.go` — it existed so modded profiles could pin
  exact patch releases like 1.21.1, which the curated base-version catalog
  deliberately does not carry.
- The NeoForge maven from `mock/`, and the whole `mockinstaller/` Java project.
  `fakejava` is a plain argument-recording stub again.

**All of that still exists in the v2.3.0-rc source zip.** If loaders are ever
wanted back, start there rather than rewriting — the merge layer was generic
and Fabric was close to free on top of it.

## What was restored

- **Version rows are selectable and playable again.** Browse-only was a
  profiles-era change (`hitRow` used to fire only while creating a profile).
- The top-bar pill, the play bar and LAUNCH follow the **selected version**
  again instead of a "current profile".
- The play bar is drawn on the version list again, and the list ends above it.
  Without this the username box floated orphaned in the middle of the list.
- `onPlay` — the original version-launch path was still present, parked as
  `onPlayLegacy` when profiles took it over. Restored, `onPlayLegacy` gone.
- Home is **CONTINUE + VERSIONS**. Continue runs off `Settings.LastVersion`,
  which survived untouched from the pre-profiles design along with the
  per-version `Playtime` map.
- The rail closed up to one browse entry, evenly spaced: Home, Versions,
  Open jars folder, Your worlds, Logs, About. `l.railVers` is now an unused
  zero rect; Versions occupies the `l.railCats` slot.

## Verified under Wine

Version list renders with READY / ON MOJANG / DROP-IN ONLY badges, Download all
and Copy links. Clicking a row moves the selection, the pill, and the play bar.
Launching 1.6 downloaded the jar from (mock) Mojang, ran, exited, and flipped
the row to green READY with `1.6.1.jar` on disk. Continue then showed "1.6 —
Play it again" after a restart and drove straight to launch.

## Not restored

**Surprise Me** was removed back in v2.1.0-rc and was not brought back.
`Settings.RollCount` is still in the struct if it is ever wanted.

## Versions grid (v3.1.0)

The top level is four cards: **Release, Pre-Release, April Fools, Rare
Versions**. The six pre-release eras sit one level down behind Pre-Release,
which opens `viewPreCats` — the same shape as the existing Rare Versions
sub-grid.

- `topTabs` is the top-level grid; `preTabs` is what sits behind Pre-Release,
  newest first (Beta, Alpha, Infdev, Indev, Classic, Pre-Classic).
- `tabPreRelease` is a pseudo-tab (-1). Pre-Release is a *group*, not a catalog
  tab, so it has no `Tabs[]` entry and no jars of its own — its count is the sum
  across `preTabs` via `preReleaseCount()`.
- Both grids share `drawCatCard` and reuse the `l.catCards` rects; each just
  draws fewer than the nine available.
- Back from an era list returns to Pre-Release (`isPreTab`), not to the top
  grid — mirroring how a rare-filtered list returns to Rare Versions.

Nothing changed in the catalog itself: `Tabs`, `TabCount`, `ReadyCount` and
`EntriesForTab` are untouched, so this is purely a navigation regrouping.

## LaunchWrapper for legacy versions (v3.2.0)

Mojang launches every applet-era version (Pre-Classic through 1.5.2) through its
own `net.minecraft.launchwrapper` with the AlphaVanillaTweaker. That wrapper is
broken: Classic and Indev save slots don't work, so generating a level runs the
generator and then goes nowhere; the game sits in a Java AWT frame, which breaks
mouse input; and skins and sounds point at dead servers. Upstream: **MCL-1448**.

LEMV now substitutes **MCPHackers' LaunchWrapper** (`org.mcphackers:launchwrapper`,
from `maven.glass-launcher.net`, overridable via `LEMV_LAUNCHWRAPPER_MAVEN`).

`applyLaunchWrapper` triggers purely on `mainClass ==
net.minecraft.launchwrapper.Launch`, so it follows Mojang's own JSONs rather
than a hardcoded version list, and modern versions are untouched.

It does four things:
1. Swaps `mainClass` to `org.mcphackers.launchwrapper.Launch`.
2. Drops `net.minecraft:launchwrapper:*` from the classpath — it declares the
   same package, so leaving it risks loading the wrong classes.
3. Removes `--tweakClass` and its value; the replacement has no tweakers.
4. **Promotes the leading positional args to named ones.** This is the
   non-obvious part: `LaunchConfig` collects any token not starting with `--`
   into a list it never reads, so the old JSON's positional
   `${auth_player_name} ${auth_session}` is silently discarded and every player
   ends up named "Player". They become `--username` / `--session`.

**Fallback:** the jar is fetched *before* any rewrite. If the maven is
unreachable, nothing is swapped and Mojang's wrapper is used — the game still
starts, only the era fixes are missing. Without this the whole pre-1.6 catalog
would be unplayable whenever a third-party host went down. Verified both ways.

`disableLaunchWrapper: true` in launcher-settings.json forces Mojang's wrapper.

Not yet wired: `--assetIndex` (would fix sounds) and `--levelsDir`. The wrapper
defaults `levelsDir` to `gameDir/levels`, which is already per-version, so
Indev saves land in the right place without passing it.

## Exit codes (v3.2.1)

Exit codes **1 and -1 are no longer reported**. The game just shows the normal
"closed after Xs" message. Old versions routinely exit 1 on a clean quit, and -1
only means the exit code couldn't be read, so neither was ever worth surfacing.
Other non-zero codes still report as before. `launcher.log` still records every
exit code regardless.

`fakejava` honours `LEMV_FAKE_EXIT=<n>` so this path can be tested; it is not
set in `gui.sh` by default.

## Surprise Me (v3.3.0)

Back as a third home card. A roll picks a random version, selects it, jumps to
its list and says "Rolled X — hit LAUNCH to play it." It reveals rather than
auto-launches, since a launch needs a username first.

**Hard exclusions.** `rollTabs` is an allowlist, not a blocklist:

    var rollTabs = []int{TabRelease, TabBeta, TabAlpha, TabInfdev}

April Fools, Pre-Classic, Classic, Indev and Rare Versions can never come up —
not because they're filtered out, but because they're never in the pool. Rare
entries all carry `Tab == TabLost`, so they're excluded by the same rule.
Audited with 200,000 rolls against the real catalog: zero hits on any banned
era. If a tab is ever added, it stays excluded until deliberately listed here.

Note **Infdev is allowed** and **Indev is not** — they're one letter apart, so
check this line if that ever looks wrong.

The card subtitle has no room to spare at higher DPI — keep it short.

Drop-in-only versions with no jar on disk are skipped: a roll you can't play is
a wasted roll. That means the pool grows as versions get downloaded and as
`ExpandFromManifest` fills in the old eras.

Every 11th roll is a guaranteed Alpha (`Settings.RollCount`, persisted across
restarts). The card previews this on roll 10 with "Next roll is a guaranteed
Alpha". If no Alpha is playable, it falls back to a normal roll rather than
failing.

## Rail hover fix (v3.3.2)

When the Profiles entry was removed, Versions was moved into the freed
`l.railCats` slot to close the gap — but hit-testing that rect still returned
`hitRailCats` while the rail item was declared with `hitRailVersions`.
`drawRail` highlights by comparing kinds, so the hover never matched and the
entry looked dead. Clicking still worked only because the handler had been
merged to accept both kinds, which hid the bug.

`hitRailCats`, `l.railVers` and the old 2x2-grid Profiles icon are now gone:
one rect, one kind. **If a rail entry is ever moved between slots again, move
the hit-test branch with it** — a mismatch here is silent apart from a
non-highlighting icon.

## Release split + Omniarchive (v3.4.0)

Release is now a group card opening two sub-cards, same shape as Pre-Release:

- **Base Updates** — the curated `TabRelease` list, one jar per major version.
- **Minor Updates** — `TabMinor`, every point release in between.

`TabMinor` is a real tab (it has entries), unlike the `tabPreRelease` /
`tabReleaseGroup` pseudo-tabs (-1 / -2) which are grouping cards only.

**Population.** `ExpandFromManifest` gained a `release` branch: any manifest
entry of type `release` that isn't already a curated base version (by id or
alias) becomes a Minor row. `isReleaseID` accepts only digits-and-dots with 2-3
parts, so snapshots (25w14a), pre-releases (1.19-pre1) and every pre-1.0 era id
are rejected — **nothing older than Release can reach Minor Updates.**

**Omniarchive.** `mergeOmniarchive` folds
`https://meta.omniarchive.uk/v1/manifest.json` into the version list inside
`loadManifest`. It's published in Mojang's own schema as a superset, so every
downstream consumer works unchanged — a vault-hosted version resolves and
launches through the normal path. Override with `LEMV_OMNIARCHIVE_MANIFEST`.

Only `release`-type entries are taken from it. Their archive also holds Alpha,
Beta and older, and pulling those in would drag pre-1.0 builds into Minor
Updates. Merge failure is non-fatal and logged: the archive being down must
never stop Mojang's own versions from listing.

**Gotcha that cost time:** manifest-derived entries must NOT set `Extra: true`.
That flag means "discovered drop-in jar", and `Rescan` deletes every `Extra`
entry on each scan — so the rows appeared and then silently vanished.

Minor Updates sorts oldest-first via the `oldEra` sort set in `AddEntries`;
manifest order is newest-first, which read backwards next to every other list.

**Surprise Me is unchanged** — `rollTabs` still lists `TabRelease`, not
`TabMinor`, so rolls stay on base versions. Add `TabMinor` there if point
releases should be rollable.

## Building

```
cd res && x86_64-w64-mingw32-windres --preprocessor=/usr/bin/cpp \
    -i app.rc -O coff -o ../launcher/rsrc_windows_amd64.syso
cd ../launcher && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
    -trimpath -ldflags="-s -w -H windowsgui" -o ../LEMVLauncher.exe .
```

Always build with `GOOS=windows` — a bare `go build` skips every `_windows.go`
file, so UI breakage will not show up.

## Testing offline

`mock/` is a fake Mojang + Microsoft. `fakejava/` is a java.exe stand-in.

```
cd mock && go build -o ../mockserver .
cd ../fakejava && GOOS=windows GOARCH=amd64 go build -o ../fakejava.exe .
(setsid nohup ./mockserver -addr 127.0.0.1:8765 -javaexe ./fakejava.exe &)
. ./testenv.sh
./lemv-linux --list
./lemv-linux --play 1.21 Steve
```

Restart the mock after rebuilding `fakejava.exe` — it reads the binary into
memory at startup and will otherwise serve a stale copy.

`./gui.sh <shot> <seconds>` launches under Wine and screenshots;
`./ui.sh click <x> <y> <shot>` drives it.

**Harness note:** background jobs must be detached as `(setsid nohup ... &)` in
a subshell. A bare `setsid ... &` takes a SIGHUP when the shell exits, which
kills Xvfb between commands and produces black screenshots with a live window —
the exact "worked fine 20 minutes ago" failure from earlier sessions.
