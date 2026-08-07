# Stardew Craftbook

A self-hosted companion for Stardew Valley 1.6. It runs on the machine that holds your save, reads it, and serves a small web app over your local network — so you can check it on your phone while you play. The save is opened read-only and never written to.

It answers three questions:

1. **What can I craft or cook right now?**
2. **What is missing for recipe X?**
3. **How do I get what's missing**, including making intermediates from what I already own — *"smelt 5 Iridium Ore + 1 Coal → 1 Iridium Bar, ×5 → craft Deluxe Scarecrow"*.

One static binary, no dependencies, no installer, no accounts, no telemetry, nothing leaves your network. The recipe data and the web UI are compiled into the executable.

## Install a release

Download the archive for your platform from the [GitHub Releases page](https://github.com/DepickereSven/stardew-craftbook/releases), extract it, and run the included binary. Each release includes Linux (x86_64 and ARM64), macOS (Intel and Apple Silicon), and Windows (x86_64) builds, plus `checksums.txt` for verification.

On Linux or macOS:

```sh
tar -xzf stardew-craftbook-<platform>.tar.gz
chmod +x stardew-craftbook
./stardew-craftbook
```

On Windows, extract the `.zip` and double-click `stardew-craftbook.exe` (or run it from PowerShell).

## Quick start

```sh
go build ./cmd/stardew-craftbook
./stardew-craftbook
```

It finds your most recently played save automatically, then prints:

```
using save: /home/you/.config/StardewValley/Saves/Medow_405190910/Medow_405190910
serving on http://0.0.0.0:8375 (open from your phone via this machine's LAN IP)
```

Open <http://localhost:8375>.

If no save is found the server still starts and the web app explains what paths it tried — use `--save-path` to point it directly at one.

### Flags

| Flag          | Default         | What it does                                                    |
|---------------|-----------------|-----------------------------------------------------------------|
| `--save-path` | *(auto-detect)* | Path to a save **file** (not the folder). Overrides detection.  |
| `--save-name` | *(most recent)* | Pick a save folder by name within the detected saves directory. |
| `--port`      | `8375`          | HTTP port.                                                      |
| `--poll`      | `3`             | How often, in seconds, to check the save file for changes.      |

The save is re-read automatically whenever the game writes it — sleep in-game and the page updates on its own.

## Reading it on your phone

Both devices must be on the same network. Find this machine's LAN address:

```sh
hostname -I | awk '{print $1}'        # Linux / Steam Deck
ipconfig getifaddr en0                # macOS
ipconfig                              # Windows — look for IPv4 Address
```

Then open `http://<that-address>:8375` on your phone, e.g. `http://192.168.50.83:8375`.

If it doesn't load, it's almost always a firewall on the host or client isolation on the Wi-Fi access point (common on guest networks).

## Steam Deck

Switch to **Desktop Mode** first. Either build from source (as above) or drop a `linux-amd64` binary at `/home/deck/stardew-craftbook` and `chmod +x` it.

Saves are auto-detected in both of the usual places:

- Native / Flatpak: `~/.config/StardewValley/Saves` and `~/.var/app/com.valvesoftware.Steam/.config/StardewValley/Saves`
- Proton: `~/.local/share/Steam/steamapps/compatdata/413150/pfx/drive_c/users/steamuser/AppData/Roaming/StardewValley/Saves`

> The Proton path is implemented from the documented prefix layout but has **not** been confirmed on a physical Deck. If detection fails there, pass `--save-path` and please report the real path.

### Start it automatically with the game

Steam **Launch Options** for Stardew Valley — starts the server alongside the game:

```
/home/deck/stardew-craftbook & %command%
```

Or run it permanently as a user service, `~/.config/systemd/user/stardew-craftbook.service`:

```ini
[Unit]
Description=Stardew Craftbook
After=network.target

[Service]
ExecStart=/home/deck/stardew-craftbook
Restart=on-failure

[Install]
WantedBy=default.target
```

```sh
systemctl --user enable --now stardew-craftbook
systemctl --user status stardew-craftbook
```

## Windows

Cross-compile from any platform:

```sh
GOOS=windows GOARCH=amd64 go build ./cmd/stardew-craftbook
```

Run `stardew-craftbook.exe`. Saves are detected at `%AppData%\StardewValley\Saves`.

Windows Firewall will prompt on first run — allow it on **private networks** so your phone can connect. Denying it still leaves `http://localhost:8375` working on the PC itself.

## Regenerating the data after a game update

Recipe, machine and item data are generated from the [Stardew Valley Wiki](https://stardewvalleywiki.com) and committed to the repo, so the app never needs the internet. After a game update:

```sh
go run ./cmd/builddata
```

This rewrites `data/recipes.json`, `data/machines.json` and `data/items.json`, validating as it goes and refusing to write anything if the data looks wrong. Rebuild afterwards to embed the new data.

## Status

The API and the engine are complete. The web page at `/` is currently a **placeholder** that only proves static serving works.

The JSON API is stable and usable on its own:

| Endpoint              | Returns                                                |
|-----------------------|--------------------------------------------------------|
| `GET /api/version`    | Snapshot counter, for cheap change polling             |
| `GET /api/state`      | Every recipe with its craftability and what's missing  |
| `GET /api/plan/{key}` | Step chain to produce a recipe's missing intermediates |
| `GET /api/items`      | Item reference: sell price, buffs, processing time     |
| `GET /api/inventory`  | Everything you own, most valuable stack first          |
| `GET /api/item/{id}`  | One item, the recipes it feeds, and whether they pay   |

```sh
curl -s localhost:8375/api/plan/Anvil
curl -s localhost:8375/api/inventory
curl -s localhost:8375/api/item/709          # Hardwood: what it makes, and the margins
```

Not modelled, by design: growing, foraging, fishing and buying. When a plan needs a Banana, it says so and links to the wiki rather than trying to explain farming.

## Development

```sh
go test ./...
gofmt -l .
go vet ./...
```

Tests that need a real save look in `saved/` (gitignored) and skip when it's absent, so a clean checkout stays green.

## License

MIT — see [LICENSE](LICENSE).
