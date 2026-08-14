package main

import (
	"regexp"
	"strconv"
	"strings"
)

// objectSpritesPage is the wiki's springobjects sprite sheet written out as a
// grid: a row of item images followed by a row of the ids those images sit at.
// It is the only place on the wiki that carries object ids in bulk, and crop
// pages carry none of their own, so it is how a crop page becomes an id.
const objectSpritesPage = "Modding:Objects/Object sprites"

// Crop is the growing-side data for one plantable crop. crops.json is keyed by
// the id a planted crop writes as its indexOfHarvest, because that is the only
// crop identity a save reliably carries — the seed a crop grew from is recorded
// too, but several seeds can yield the same crop (Grape comes from both Summer
// Seeds and a Grape Starter).
//
// Only Seasons and RegrowDays are strictly needed by the app: everything else
// about a planted crop's schedule is written into the save itself. They are the
// two facts the save cannot answer, since a crop that has never been harvested
// looks identical whether or not it will regrow.
type Crop struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Seasons    []string `json:"seasons,omitempty"`
	GrowthDays *int     `json:"growth_days,omitempty"`
	RegrowDays *int     `json:"regrow_days,omitempty"`
	SeedIDs    []string `json:"seed_ids,omitempty"`
	SeedNames  []string `json:"seed_names,omitempty"`
	WikiURL    string   `json:"wiki_url"`
}

var (
	seasonTemplateRe = regexp.MustCompile(`\{\{Season\|([A-Za-z]+)`)
	// The per-crop Stages table ends with an "After-Harvest" column whose cell
	// reads "Regrowth: 5 Days". The infobox has no field for it.
	regrowthRe   = regexp.MustCompile(`(?i)Regrowth:\s*([0-9]+)\s*Days?`)
	growthDaysRe = regexp.MustCompile(`([0-9]+)\s*days?`)
	spriteFileRe = regexp.MustCompile(`^\[\[File:(.+?)\.png\b`)
	// Sprites with no name of their own are filed under their own index.
	placeholderSpriteRe = regexp.MustCompile(`(?i)^springobjects[0-9]+$`)
	seasonWordRe        = map[string]*regexp.Regexp{}
)

var seasonOrder = []string{"spring", "summer", "fall", "winter"}

func init() {
	for _, season := range seasonOrder {
		seasonWordRe[season] = regexp.MustCompile(`\b` + season + `\b`)
	}
}

// objectIDsByName reads the sprite grid into item name → object id. Rows come
// in pairs — images then ids — so a row of images is held until the row of ids
// beneath it arrives and the two are zipped together.
func objectIDsByName(page string) map[string]string {
	ids := map[string]string{}
	var names []string
	for _, row := range strings.Split(page, "\n|-") {
		cells := tableCells(row)
		if len(cells) == 0 {
			continue
		}
		if m := spriteFileRe.FindStringSubmatch(cells[0]); m != nil {
			names = names[:0]
			for _, cell := range cells {
				match := spriteFileRe.FindStringSubmatch(cell)
				if match == nil {
					names = append(names, "")
					continue
				}
				names = append(names, match[1])
			}
			continue
		}
		if len(names) == 0 {
			continue
		}
		for i, cell := range cells {
			if i >= len(names) || names[i] == "" || placeholderSpriteRe.MatchString(names[i]) {
				continue
			}
			n, err := strconv.Atoi(cell) // ids are zero-padded in the table
			if err != nil {
				continue
			}
			// First writer wins: a handful of names appear twice in the sheet
			// and the lower id is the real item.
			if _, seen := ids[names[i]]; !seen {
				ids[names[i]] = strconv.Itoa(n)
			}
		}
		names = names[:0]
	}
	return ids
}

func tableCells(row string) []string {
	var cells []string
	for _, line := range strings.Split(row, "\n") {
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|+") {
			continue
		}
		cell := strings.TrimSpace(strings.TrimPrefix(line, "|"))
		if cell == "" {
			continue
		}
		cells = append(cells, cell)
	}
	return cells
}

// parseSeasons reads the infobox season field, which is written three ways:
// as {{Season|…}} templates joined by bullets, as a bare season name, or as
// "All" for the crops with no season at all.
func parseSeasons(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	found := map[string]bool{}
	for _, m := range seasonTemplateRe.FindAllStringSubmatch(trimmed, -1) {
		found[strings.ToLower(m[1])] = true
	}
	if found["all"] || strings.EqualFold(trimmed, "All") {
		return append([]string(nil), seasonOrder...)
	}
	if len(found) == 0 {
		lower := strings.ToLower(trimmed)
		for _, season := range seasonOrder {
			if seasonWordRe[season].MatchString(lower) {
				found[season] = true
			}
		}
	}
	var out []string
	for _, season := range seasonOrder {
		if found[season] {
			out = append(out, season)
		}
	}
	return out
}

// parseGrowthDays takes the first day count in the growth field. Crops reachable
// from two different seeds list both ("7 days (Summer Seeds)<br />10 days
// (Grape Starter)"); this is only a headline figure, since the real schedule for
// a planted crop comes out of the save.
func parseGrowthDays(raw string) *int {
	m := growthDaysRe.FindStringSubmatch(raw)
	if m == nil {
		return nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	return &n
}

// objectID resolves an item name to the id a save would write. Items added in
// 1.6 are absent from the sprite sheet because they are keyed by a string id
// instead, which the game forms by removing the spaces from the English name.
func objectID(name string, objectIDs map[string]string) string {
	if id, ok := objectIDs[name]; ok {
		return id
	}
	return strings.ReplaceAll(name, " ", "")
}

func cropFromPage(title, page string, objectIDs map[string]string) (Crop, bool) {
	seedField := infoboxField(page, "seed")
	if seedField == "" {
		return Crop{}, false
	}
	crop := Crop{ID: objectID(title, objectIDs), Name: title, WikiURL: wikiURL(title)}
	crop.Seasons = parseSeasons(infoboxField(page, "season"))
	crop.GrowthDays = parseGrowthDays(infoboxField(page, "growth"))
	if m := regrowthRe.FindStringSubmatch(page); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			crop.RegrowDays = &n
		}
	}
	seen := map[string]bool{}
	for _, fields := range nameTemplateFields(seedField) {
		name := strings.TrimSpace(fields[0])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		crop.SeedNames = append(crop.SeedNames, name)
		crop.SeedIDs = append(crop.SeedIDs, objectID(name, objectIDs))
	}
	return crop, true
}

// collectCrops picks the plantable things out of the already-fetched item
// pages. The filter is "the infobox names a seed", which also admits fruit
// trees; that is harmless, because a lookup only ever comes from a crop planted
// in tilled soil, and a tree is never one of those.
func collectCrops(pages map[string]string, objectIDs map[string]string) map[string]Crop {
	crops := map[string]Crop{}
	for title, page := range pages {
		crop, ok := cropFromPage(title, page, objectIDs)
		if !ok || crop.ID == "" {
			continue
		}
		crops[crop.ID] = crop
	}
	return crops
}
