package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// CandidateRoots lists the save directories that actually exist on this
// machine, in the order they should be searched: native install first, then
// Flatpak, then the Proton prefix.
func CandidateRoots() []string {
	home, _ := os.UserHomeDir()
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = append(candidates, filepath.Join(os.Getenv("APPDATA"), "StardewValley", "Saves"))
	} else {
		candidates = append(candidates,
			filepath.Join(home, ".config", "StardewValley", "Saves"),
			filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".config", "StardewValley", "Saves"),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "compatdata", "413150",
				"pfx", "drive_c", "users", "steamuser", "AppData", "Roaming", "StardewValley", "Saves"),
		)
	}
	var existing []string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			existing = append(existing, c)
		}
	}
	return existing
}

// FindSave returns the path to a save file under root. A save folder is valid
// when it contains a file with the same name as the folder. An empty saveName
// picks the most recently modified valid save.
func FindSave(root, saveName string) (string, error) {
	if saveName != "" {
		p := filepath.Join(root, saveName, saveName)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("save %q not found under %s: %w", saveName, root, err)
		}
		return p, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var best string
	var bestTime int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name(), e.Name())
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if mt := fi.ModTime().UnixNano(); mt > bestTime {
			best, bestTime = p, mt
		}
	}
	if best == "" {
		return "", fmt.Errorf("no save folders found under %s", root)
	}
	return best, nil
}
