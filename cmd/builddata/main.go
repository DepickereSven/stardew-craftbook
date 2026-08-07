// Command builddata regenerates the committed recipe and machine datasets
// from the Stardew Valley wiki. It is a development-time tool: the app itself
// ships the generated JSON embedded in the binary and never talks to the wiki.
//
// Item ids come out of Modding:Recipe data, but that page carries ids only.
// Display names are recovered by lining each raw recipe up with the named
// ingredient list on that recipe's own wiki page — see resolveNames.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
)

const recipeDataPage = "Modding:Recipe data"

func main() {
	dumpRaw := flag.Bool("dump-raw", false, "print the fetched wikitext and exit")
	out := flag.String("out", "data", "output directory")
	flag.Parse()

	recipeText, err := fetchWikitext(recipeDataPage)
	must(err)
	if *dumpRaw {
		fmt.Println(recipeText)
		return
	}

	cooking, crafting, err := extractRawRecipes(recipeText)
	must(err)
	fmt.Fprintf(os.Stderr, "raw entries: %d cooking, %d crafting\n", len(cooking), len(crafting))

	all := append(append([]rawEntry{}, cooking...), crafting...)
	titles := make([]string, 0, len(all))
	for _, e := range all {
		titles = append(titles, displayName(e.Key))
	}
	pagesByTitle, err := fetchPages(titles)
	must(err)
	pagesByKey := make(map[string]string, len(all))
	for _, e := range all {
		if text, ok := pagesByTitle[displayName(e.Key)]; ok {
			pagesByKey[e.Key] = text
		}
	}
	fmt.Fprintf(os.Stderr, "item pages fetched: %d/%d\n", len(pagesByKey), len(all))

	names := resolveNames(all, pagesByKey)
	fmt.Fprintf(os.Stderr, "item names resolved: %d\n", len(names))

	var recipes []Recipe
	for _, group := range []struct {
		typ     string
		entries []rawEntry
	}{{"cooking", cooking}, {"crafting", crafting}} {
		for _, e := range group.entries {
			r, err := parseRecipeLine(e.Key, e.Raw, group.typ, names)
			must(err)
			recipes = append(recipes, r)
		}
	}
	sort.Slice(recipes, func(i, j int) bool { return recipes[i].Key < recipes[j].Key })

	machines := machineConversions()
	if problems := validate(recipes, machines); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "INVALID:", p)
		}
		os.Exit(1)
	}
	writeJSON(*out+"/recipes.json", recipes)
	writeJSON(*out+"/machines.json", machines)
	fmt.Printf("wrote %d recipes, %d machine conversions\n", len(recipes), len(machines))
}

func writeJSON(path string, v any) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	must(enc.Encode(v))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
