# LEMV Launcher

**L**iterally **E**very **M**inecraft **V**ersion — a single portable `.exe` that
launches Minecraft: Java Edition from the very first week of its existence
through to the current release.

139 versions are curated in the catalog and more are pulled from Mojang's own
version manifest at runtime, so the list grows on its own as new versions ship.
No installer, no dependencies, no accounts required. Game jars download straight
from Mojang's servers (sha1-verified), so nothing copyrighted ships inside the
executable.

## Features

- **Every era.** Release (base versions and every point release), Beta, Alpha,
  Infdev, Indev, Classic, Pre-Classic, every April Fools joke version, and a
  Rare Versions tab of recovered builds.
- **Legacy versions that actually work.** Pre-1.6 versions launch through
  [MCPHackers LaunchWrapper](https://github.com/MCPHackers/LaunchWrapper)
  instead of Mojang's applet wrapper, which fixes Classic and Indev save slots,
  mouse input, and dead skin/sound servers.
- **Archived versions.** Releases Mojang no longer hosts are pulled from
  [Omniarchive](https://omniarchive.uk)'s version manifest and launch through
  the same path as any other version.
- **Portable.** Everything lives in one folder next to the exe — versions,
  worlds, assets, Java runtimes. Nothing touches `%APPDATA%`, nothing is
  installed, and moving the folder moves the whole setup.
- **Offline or Microsoft sign-in.** Type a username and play, or sign in with a
  Microsoft account via the device-code flow.
- **Per-version instances.** Each version gets its own worlds, options and
  screenshots, so a Beta 1.7.3 world can't collide with a 1.21 one.
- **Drop-in jars.** Put a jar in `versions/` named after an official version id
  and it's picked up automatically — useful for builds no server hosts.
- **Surprise Me.** Rolls a random version when you can't decide.

## Version states

- **READY** — the jar is on disk; Play launches immediately.
- **ON MOJANG** — no jar yet; Play fetches it automatically.
- **DROP-IN ONLY** — nobody hosts a direct download; the launcher links to an
  archive page and picks the jar up once it lands in `versions/`.

## Building

Requires **Go 1.22+** (no cgo, no external modules). Building the icon and
manifest resource additionally needs `binutils-mingw-w64-x86-64`, though the
generated `.syso` is committed, so this step is only needed if you edit `res/`.

### Cross-compiling from Linux or macOS

```sh
# optional: regenerate the icon/manifest resource after editing res/
cd res
x86_64-w64-mingw32-windres --preprocessor=/usr/bin/cpp \
    -i app.rc -O coff -o ../launcher/rsrc_windows_amd64.syso

# build the exe
cd ../launcher
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
    -trimpath -ldflags="-s -w -H windowsgui" -o ../LEMVLauncher.exe .
```

### Building on Windows

```powershell
cd launcher
go build -trimpath -ldflags="-s -w -H windowsgui" -o ..\LEMVLauncher.exe .
```

`-H windowsgui` suppresses the console window. Drop it if you want to see
stdout while debugging.

> **Always build with `GOOS=windows`.** A plain `go build` on Linux skips every
> `_windows.go` file — including the entire UI — so it will compile happily
> while telling you nothing about whether the interface still works.

## Command line

The launcher is a GUI app, but a few flags are useful for scripting and
debugging:

```sh
LEMVLauncher.exe --list                 # print the catalog and which jars are present
LEMVLauncher.exe --play <version> <name>  # launch a version headlessly
LEMVLauncher.exe --signin               # Microsoft device-code sign-in
```

## Project layout

| Path         | What it is                                                        |
|--------------|-------------------------------------------------------------------|
| `launcher/`  | The launcher itself. `_windows.go` files hold the Win32 UI.        |
| `res/`       | Icon, application manifest, and the resource script.               |
| `mock/`      | A fake Mojang + Microsoft + archive server for offline testing.    |
| `fakejava/`  | A `java.exe` stand-in that records its arguments instead of running the game. |

Inside `launcher/`, the interesting files are `catalog.go` (the version list and
tab structure), `mojang.go` (manifest and version JSON resolution), `launch.go`
(building the command line), `launchwrapper.go` (the legacy-version fix), and
`ui_windows.go` (the entire interface).

## Testing without Windows or internet

`mock/` serves a complete fake Mojang: version manifest, version JSONs, client
jars, libraries, assets and Java runtimes, plus a stand-in Microsoft sign-in
chain and archive manifest. Combined with `fakejava/`, the whole download and
launch pipeline can be exercised on Linux.

```sh
cd mock && go build -o ../mockserver .
cd ../fakejava && GOOS=windows GOARCH=amd64 go build -o ../fakejava.exe .
cd ..
./mockserver -addr 127.0.0.1:8765 -javaexe ./fakejava.exe &

. ./testenv.sh          # points every LEMV_* endpoint at the mock
cd launcher && go build -o ../lemv-linux . && cd ..
./lemv-linux --list
./lemv-linux --play 1.21 Steve
```

Restart the mock server after rebuilding `fakejava.exe` — it reads the binary
into memory at startup and will otherwise serve a stale copy.

The UI can be checked under Wine with `gui.sh` (launch and screenshot) and
`ui.sh` (click, hover, scroll). Both need `Xvfb`, `xdotool` and `imagemagick`.

### Endpoint overrides

Every remote endpoint can be redirected, which is what the test harness uses:

| Variable                     | Default                                            |
|------------------------------|----------------------------------------------------|
| `LEMV_ROOT`                  | the `LEMV` folder beside the exe                   |
| `LEMV_MANIFEST_URL`          | Mojang's `version_manifest_v2.json`                |
| `LEMV_OMNIARCHIVE_MANIFEST`  | `https://meta.omniarchive.uk/v1/manifest.json`     |
| `LEMV_LAUNCHWRAPPER_MAVEN`   | `https://maven.glass-launcher.net/releases/`       |
| `LEMV_JRE_INDEX_URL`         | Mojang's Java runtime index                        |
| `LEMV_LIBRARIES_URL`         | Mojang's library host                              |
| `LEMV_RESOURCES_URL`         | Mojang's legacy resource host                      |
| `LEMV_MSA_URL` / `LEMV_XBL_URL` / `LEMV_XSTS_URL` / `LEMV_MCSVC_URL` | the Microsoft sign-in chain |

## Credits

- [Omniarchive](https://omniarchive.uk) — preservation of versions no longer
  distributed, and the version manifest this launcher reads.
- [MCPHackers LaunchWrapper](https://github.com/MCPHackers/LaunchWrapper) — the
  replacement wrapper that makes the pre-1.6 eras playable.

## License

No license file is included yet. Without one, the default is
all-rights-reserved, which means nobody can legally fork or redistribute this —
worth picking a license (MIT and Apache-2.0 are the usual choices) before making
the repository public.

## Disclaimer

Not affiliated with, endorsed by, or associated with Mojang Studios, Microsoft
or Oracle. Minecraft is a trademark of Mojang Synergies AB. This launcher
downloads game files from their official servers and redistributes none of them
— you need to own the game.
