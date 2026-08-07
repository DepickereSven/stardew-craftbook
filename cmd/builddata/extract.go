package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// rawEntry is one recipe as the game stores it: the save-file key plus the
// slash-separated data string.
type rawEntry struct {
	Key string
	Raw string
}

var (
	// Modding:Recipe data embeds the cooking and crafting tables (in that
	// order) as JSON inside syntaxhighlight blocks.
	dataBlockRe = regexp.MustCompile(`(?s)<syntaxhighlight lang="json">\s*\{(.*?)\}\s*</syntaxhighlight>`)
	entryRe     = regexp.MustCompile(`"([^"]+)"\s*:\s*"([^"]*)"`)

	// Item pages carry their recipe's ingredients by display name in an
	// infobox field, e.g. `|ingredients = {{Name|Egg|1}}{{Name|Milk|1}}`.
	ingredientsFieldRe = regexp.MustCompile(`\|\s*ingredients\s*=\s*(.*)`)
	nameTemplateRe     = regexp.MustCompile(`\{\{Name\|([^}|]+)\|(\d+)(?:\|[^}]*)?\}\}`)
	// Some pages set off the "any milk"/"any egg" style entries with extra
	// template parameters, so a bare list is a second reading of the same row.
	plainNameTemplateRe = regexp.MustCompile(`\{\{Name\|([^}|]+)\|(\d+)\}\}`)
)

func extractRawRecipes(wikitext string) (cooking, crafting []rawEntry, err error) {
	blocks := dataBlockRe.FindAllStringSubmatch(wikitext, -1)
	if len(blocks) < 2 {
		return nil, nil, fmt.Errorf("found %d JSON data blocks on the recipe page, want 2", len(blocks))
	}
	return parseEntries(blocks[0][1]), parseEntries(blocks[1][1]), nil
}

func parseEntries(block string) []rawEntry {
	var out []rawEntry
	for _, m := range entryRe.FindAllStringSubmatch(block, -1) {
		out = append(out, rawEntry{Key: m[1], Raw: m[2]})
	}
	return out
}

// extractIngredientNames pulls the named ingredient list out of an item page.
func extractIngredientNames(pageWikitext string) []Ingredient {
	return ingredientList(pageWikitext, nameTemplateRe)
}

// extractPlainIngredientNames is extractIngredientNames restricted to entries
// written without extra template parameters.
func extractPlainIngredientNames(pageWikitext string) []Ingredient {
	return ingredientList(pageWikitext, plainNameTemplateRe)
}

func ingredientList(pageWikitext string, re *regexp.Regexp) []Ingredient {
	m := ingredientsFieldRe.FindStringSubmatch(pageWikitext)
	if m == nil {
		return nil
	}
	var out []Ingredient
	for _, t := range re.FindAllStringSubmatch(m[1], -1) {
		qty, err := strconv.Atoi(t[2])
		if err != nil {
			continue
		}
		out = append(out, Ingredient{Name: strings.TrimSpace(t[1]), Qty: qty})
	}
	return out
}

// Category ids never appear on item pages — the wiki writes "Egg", not
// "any egg" — so they are seeded explicitly.
var categoryNames = map[string]string{
	"-4": "Fish (Any)", "-5": "Egg (Any)", "-6": "Milk (Any)", "-777": "Wild Seeds (Any)",
}

// fallbackNames covers ids the resolver cannot reach because every recipe
// using them is ambiguous (repeated quantities, all-unknown ingredients) or
// because the item has no wiki page of its own. Hand-checked against the wiki.
var fallbackNames = map[string]string{
	"194": "Fried Egg",
	"211": "Pancakes",
	"372": "Clam",
	"715": "Lobster",
	"717": "Crab",
}

// resolveNames builds an item id → display name map by lining each raw recipe
// up with the named ingredient list on its wiki page. Quantities are the
// anchor: within a recipe an id whose quantity is unique in the list can only
// be the one wiki ingredient with that quantity, whatever order the wiki
// chose. Recipes are swept repeatedly, so names learned in one pass pin down
// neighbours in the next; only once nothing is left to pin does the sweep fall
// back to the wiki's listing order.
//
// pagesByKey maps a recipe key to the wikitext of that recipe's item page.
func resolveNames(entries []rawEntry, pagesByKey map[string]string) map[string]string {
	names := map[string]string{}
	for id, n := range categoryNames {
		names[id] = n
	}
	for id, n := range fallbackNames {
		names[id] = n
	}

	type pair struct {
		raw  []Ingredient // categories dropped
		wiki []Ingredient
	}
	var pairs []pair
	for _, e := range entries {
		page := pagesByKey[e.Key]
		raw, err := parseIngredients(strings.Split(e.Raw, "/")[0], nil)
		if err != nil {
			continue
		}
		var kept []Ingredient
		for _, ing := range raw {
			if !ing.Category {
				kept = append(kept, ing)
			}
		}
		if len(kept) == 0 {
			continue
		}
		for _, wiki := range [][]Ingredient{extractIngredientNames(page), extractPlainIngredientNames(page)} {
			if len(kept) == len(wiki) && sameQuantities(kept, wiki) {
				pairs = append(pairs, pair{raw: kept, wiki: wiki})
				break
			}
		}
	}

	for {
		progress := false
		for _, p := range pairs {
			pool := append([]Ingredient(nil), p.wiki...)
			var rest []Ingredient
			for _, ing := range p.raw {
				if known, ok := names[ing.ID]; ok {
					if i := indexOf(pool, known, ing.Qty); i >= 0 {
						pool = append(pool[:i], pool[i+1:]...)
						continue
					}
				}
				rest = append(rest, ing)
			}
			// Assign every id whose quantity is unique among what is left,
			// repeating because each assignment can make another unique.
			for changed := true; changed; {
				changed = false
				for i := 0; i < len(rest); i++ {
					cand := candidatesWithQty(pool, rest[i].Qty)
					if len(cand) != 1 {
						continue
					}
					if learn(names, rest[i].ID, pool[cand[0]].Name) {
						progress = true
					}
					pool = append(pool[:cand[0]], pool[cand[0]+1:]...)
					rest = append(rest[:i], rest[i+1:]...)
					changed = true
					i--
				}
			}
			if len(rest) > 0 && len(rest) == len(pool) && sameQuantities(rest, pool) {
				for i := range rest {
					if learn(names, rest[i].ID, pool[i].Name) {
						progress = true
					}
				}
			}
		}
		if !progress {
			return names
		}
	}
}

// learn records a name for an id, and reports whether that was new. The first
// name wins: seeded categories and curated fallbacks are never overwritten,
// and neither is anything an earlier, better-pinned recipe established.
func learn(names map[string]string, id, name string) bool {
	if _, ok := names[id]; ok {
		return false
	}
	names[id] = name
	return true
}

func sameQuantities(a, b []Ingredient) bool {
	counts := map[int]int{}
	for _, x := range a {
		counts[x.Qty]++
	}
	for _, x := range b {
		counts[x.Qty]--
	}
	for _, n := range counts {
		if n != 0 {
			return false
		}
	}
	return true
}

func indexOf(pool []Ingredient, name string, qty int) int {
	for i, p := range pool {
		if p.Name == name && p.Qty == qty {
			return i
		}
	}
	return -1
}

func candidatesWithQty(pool []Ingredient, qty int) []int {
	var out []int
	for i, p := range pool {
		if p.Qty == qty {
			out = append(out, i)
		}
	}
	return out
}
