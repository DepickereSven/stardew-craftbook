# Stardew Craftbook

A self-hosted companion for Stardew Valley 1.6. It runs on the machine that holds your save, reads it, and serves 
a small web app over your local network, so you can check it on your phone while you play. The save is opened read-only and never written to.

It answers four questions:

1. **What can I craft or cook right now?**
2. **What is missing for recipe X?**
3. **How do I get what's missing**, including making intermediates from what I already own: *"smelt 5 Iridium Ore + 1 Coal → 1 Iridium Bar, ×5 → craft Deluxe Scarecrow"*.
4. **What is growing, and when does it come in?** — every planted crop and fruit tree, grouped by location and type, on one harvest timeline.

One static binary, no dependencies, no installer, no accounts, no telemetry, nothing leaves your network. The recipe data and the web UI are compiled into the executable.

## Install a release

Download the archive for your platform from the [GitHub Releases page](https://github.com/DepickereSven/stardew-craftbook/releases), extract it, and run the included binary. 
Each release includes Linux (x86_64 and ARM64), macOS (Intel and Apple Silicon), and Windows (x86_64) builds, plus `checksums.txt` for verification.

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

If no save is found the server still starts and the web app explains what paths it tried, use `--save-path` to point it directly at one.

### Flags

| Flag          | Default         | What it does                                                    |
|---------------|-----------------|-----------------------------------------------------------------|
| `--save-path` | *(auto-detect)* | Path to a save **file** (not the folder). Overrides detection.  |
| `--save-name` | *(most recent)* | Pick a save folder by name within the detected saves directory. |
| `--port`      | `8375`          | HTTP port.                                                      |
| `--poll`      | `3`             | How often, in seconds, to check the save file for changes.      |

The save is re-read automatically whenever the game writes it, sleep in-game and the page updates on its own.

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

Steam **Launch Options** for Stardew Valley, starts the server alongside the game:

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

Windows Firewall will prompt on first run, allow it on **private networks** so your phone can connect. 
Denying it still leaves `http://localhost:8375` working on the PC itself.

## Regenerating the data after a game update

Recipe, machine and item data are generated from the [Stardew Valley Wiki](https://stardewvalleywiki.com) and committed to the repo, so the app never needs the internet. After a game update:

```sh
go run ./cmd/builddata
```

This rewrites `data/recipes.json`, `data/machines.json`, `data/items.json` and `data/crops.json`, validating as it goes and refusing to write anything if the data looks wrong. Rebuild afterwards to embed the new data.

## Status

The app is complete: an Items view (what you own, what it's worth raw vs. processed), a Recipes view (what's craftable, what's missing, how to get there) and a Crops view (what's in the ground and when it's due), all in a single self-contained page at `/`.

The JSON API behind it:

| Endpoint              | Returns                                                               |
|-----------------------|-----------------------------------------------------------------------|
| `GET /api/version`    | Snapshot counter, for cheap change polling                            |
| `GET /api/state`      | Every recipe: craftability, what's missing, sale economics            |
| `GET /api/plan/{key}` | Step chain to produce a recipe's missing intermediates                |
| `GET /api/items`      | Item reference: sell price, buffs, processing time                    |
| `GET /api/inventory`  | Everything you own, most valuable stack first                         |
| `GET /api/item/{id}`  | One item, the recipes it feeds, and whether they pay                  |
| `GET /api/crops`      | Every planted crop and fruit tree, with groups and a harvest timeline |

```sh
curl -s localhost:8375/api/plan/Anvil
curl -s localhost:8375/api/inventory
curl -s localhost:8375/api/item/709          # Hardwood: what it makes, and the margins
curl -s localhost:8375/api/crops             # what's growing and when it's due
```

### What "days until harvest" means

A crop only advances on a day it is **watered** — that is the rule the game itself
uses, and it is the rule the countdown follows. So `days_until_harvest` is a count
of remaining watered growth days, not of calendar days. Miss a watering, plant into
a season the crop cannot survive, or let it wilt, and the real date slips. The dates
the API and the UI print are therefore the *earliest* possible ones, and both say so.

Fruit-tree countdowns are calendar days instead. They account for the tree's
remaining maturity time and bearing season; trees in the Greenhouse or on Ginger
Island bear year-round. A blocked sapling can still delay its predicted date.

Not modelled, by design: foraging, fishing and buying. 

## Development

```sh
go test ./...
gofmt -l .
go vet ./...
```

Tests that need a real save look in `saved/` (gitignored) and skip when it's absent, so a clean checkout stays green.

## License

MIT — see [LICENSE](LICENSE).
