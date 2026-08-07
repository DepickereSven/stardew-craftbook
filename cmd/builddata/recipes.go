package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Ingredient struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Qty      int    `json:"qty"`
	Category bool   `json:"category,omitempty"`
}

type Recipe struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Ingredients []Ingredient `json:"ingredients"`
	OutputQty   int          `json:"output_qty"`
	Unlock      string       `json:"unlock"`
	WikiURL     string       `json:"wiki_url"`
}

// Save-file recipe keys whose display name differs. The raw data's trailing
// display-name field is empty for every vanilla recipe, so these are resolved
// against the wiki's Cooking and Crafting recipe listings by hand.
var keyToDisplay = map[string]string{
	// Cooking — every key on Modding:Recipe data that has no matching row
	// on the wiki's Cooking page.
	"Cheese Cauli.":   "Cheese Cauliflower",
	"Cookies":         "Cookie",
	"Cran. Sauce":     "Cranberry Sauce",
	"Dish o' The Sea": "Dish O' The Sea",
	"Eggplant Parm.":  "Eggplant Parmesan",
	"Vegetable Stew":  "Vegetable Medley",
	// Crafting.
	"Oil Of Garlic":   "Oil of Garlic",
	"Wild Seeds (Sp)": "Spring Seeds",
	"Wild Seeds (Su)": "Summer Seeds",
	"Wild Seeds (Fa)": "Fall Seeds",
	"Wild Seeds (Wi)": "Winter Seeds",
}

// The Transmute recipes are the only ones with no wiki page of their own —
// they are listed on the Crafting page instead.
var keyToWikiPage = map[string]string{
	"Transmute (Fe)": "Crafting",
	"Transmute (Au)": "Crafting",
}

// displayName is the human-facing name for a save-file recipe key, and the
// title of that recipe's wiki page.
func displayName(key string) string {
	if d, ok := keyToDisplay[key]; ok {
		return d
	}
	return key
}

// wikiPage is the wiki page title to link a recipe to.
func wikiPage(key string) string {
	if p, ok := keyToWikiPage[key]; ok {
		return p
	}
	return displayName(key)
}

func parseRecipeLine(key, raw, typ string, names map[string]string) (Recipe, error) {
	fields := strings.Split(raw, "/")
	minFields := 4 // cooking: ingredients/unused/output/unlock
	unlockIdx := 3
	if typ == "crafting" {
		minFields = 5 // + isBigCraftable before unlock
		unlockIdx = 4
	}
	if len(fields) < minFields {
		return Recipe{}, fmt.Errorf("recipe %q: %d fields, want >= %d", key, len(fields), minFields)
	}
	ings, err := parseIngredients(fields[0], names)
	if err != nil {
		return Recipe{}, fmt.Errorf("recipe %q: %w", key, err)
	}
	outParts := strings.Fields(fields[2])
	outQty := 1
	if len(outParts) == 2 {
		if q, err := strconv.Atoi(outParts[1]); err == nil {
			outQty = q
		}
	}
	name := displayName(key)
	return Recipe{
		Key:         key,
		Name:        name,
		Type:        typ,
		Ingredients: ings,
		OutputQty:   outQty,
		Unlock:      translateUnlock(strings.TrimSpace(fields[unlockIdx])),
		WikiURL:     "https://stardewvalleywiki.com/" + strings.ReplaceAll(wikiPage(key), " ", "_"),
	}, nil
}

func parseIngredients(s string, names map[string]string) ([]Ingredient, error) {
	parts := strings.Fields(s)
	if len(parts) == 0 || len(parts)%2 != 0 {
		return nil, fmt.Errorf("bad ingredient list %q", s)
	}
	var out []Ingredient
	for i := 0; i < len(parts); i += 2 {
		qty, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return nil, fmt.Errorf("bad qty %q", parts[i+1])
		}
		id := parts[i]
		out = append(out, Ingredient{
			ID:       id,
			Name:     names[id],
			Qty:      qty,
			Category: strings.HasPrefix(id, "-"),
		})
	}
	return out, nil
}

func translateUnlock(raw string) string {
	f := strings.Fields(raw)
	switch {
	case raw == "default", raw == "l 0", raw == "":
		return "Starter"
	case len(f) == 3 && f[0] == "s":
		return fmt.Sprintf("%s Level %s", f[1], f[2])
	case len(f) == 3 && f[0] == "f":
		return fmt.Sprintf("%s (%s hearts)", f[1], f[2])
	case len(f) == 2 && f[0] == "l":
		return "Level " + f[1]
	default:
		return "Special (see wiki)"
	}
}
