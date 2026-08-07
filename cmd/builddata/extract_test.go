package main

import "testing"

const sampleRecipePage = `
==Raw data==
===Cooking recipes===
blah blah
{{collapse|Data|content=<syntaxhighlight lang="json">
{
   "Fried Egg":"-5 1/10 10/194/default/",
   "Omelet":"-5 1 -6 1/1 10/195/l 10/"
}
</syntaxhighlight>}}
===Crafting recipes===
more blah
{{collapse|Data|content=<syntaxhighlight lang="json">
{
   "Wood Fence":"388 2/Field/322/false/default/",
   "Gate":"388 10/Home/325/false/default/"
}
</syntaxhighlight>}}
==Format==
`

func TestExtractRawRecipes(t *testing.T) {
	cooking, crafting, err := extractRawRecipes(sampleRecipePage)
	if err != nil {
		t.Fatal(err)
	}
	if len(cooking) != 2 || len(crafting) != 2 {
		t.Fatalf("got %d cooking, %d crafting", len(cooking), len(crafting))
	}
	if cooking[0].Key != "Fried Egg" || cooking[0].Raw != "-5 1/10 10/194/default/" {
		t.Errorf("cooking[0] = %+v", cooking[0])
	}
	if crafting[1].Key != "Gate" || crafting[1].Raw != "388 10/Home/325/false/default/" {
		t.Errorf("crafting[1] = %+v", crafting[1])
	}
}

func TestExtractRawRecipesNeedsTwoBlocks(t *testing.T) {
	if _, _, err := extractRawRecipes("no json here"); err == nil {
		t.Error("expected error when the data blocks are missing")
	}
}

func TestExtractIngredientNames(t *testing.T) {
	page := `<onlyinclude>{{{{{1|Infobox cooking}}}
|name        = Omelet
|ingredients = {{Name|Egg|1}}{{Name|Milk|1}}
|sellprice   = 125
}}</onlyinclude>
Omelet is used with {{Name|Shirt021|link=Tailoring{{!}}Yellow Shirt|class=inline}}.`
	got := extractIngredientNames(page)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Name != "Egg" || got[0].Qty != 1 || got[1].Name != "Milk" {
		t.Errorf("got %+v", got)
	}
}

func TestExtractIngredientNamesMissingField(t *testing.T) {
	if got := extractIngredientNames("no infobox"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestResolveNamesSeedsCategories(t *testing.T) {
	names := resolveNames(nil, nil)
	if names["-6"] != "Milk (Any)" {
		t.Errorf("category seed missing: %q", names["-6"])
	}
	if names["-5"] != "Egg (Any)" || names["-4"] != "Fish (Any)" || names["-777"] != "Wild Seeds (Any)" {
		t.Errorf("category seeds incomplete: %v", names)
	}
}

// The wiki lists ingredients in its own order; distinct quantities pin each
// id to exactly one name regardless of that order.
func TestResolveNamesByDistinctQuantities(t *testing.T) {
	entries := []rawEntry{{Key: "Furnace", Raw: "378 20 390 25/Home/13/true/l 2/"}}
	pages := map[string]string{"Furnace": "|ingredients = {{Name|Stone|25}}{{Name|Copper Ore|20}}"}
	names := resolveNames(entries, pages)
	if names["378"] != "Copper Ore" || names["390"] != "Stone" {
		t.Errorf("got 378=%q 390=%q", names["378"], names["390"])
	}
}

// When quantities repeat there is nothing to pin against, so fall back to the
// wiki's listing order.
func TestResolveNamesPositionalFallback(t *testing.T) {
	entries := []rawEntry{{Key: "Salad", Raw: "20 1 22 1/25 5/196/f Emily 3/"}}
	pages := map[string]string{"Salad": "|ingredients = {{Name|Leek|1}}{{Name|Dandelion|1}}"}
	names := resolveNames(entries, pages)
	if names["20"] != "Leek" || names["22"] != "Dandelion" {
		t.Errorf("got 20=%q 22=%q", names["20"], names["22"])
	}
}

// Category ingredients never appear in the wiki's ingredient list, so they
// must be dropped before the two lists are compared.
func TestResolveNamesIgnoresCategoryIngredients(t *testing.T) {
	entries := []rawEntry{{Key: "Banana Pudding", Raw: "91 1 -6 1 245 1/2 2/904/f Leo 3/"}}
	pages := map[string]string{"Banana Pudding": "|ingredients = {{Name|Banana|1}}{{Name|Sugar|1}}"}
	names := resolveNames(entries, pages)
	if names["91"] != "Banana" || names["245"] != "Sugar" {
		t.Errorf("got 91=%q 245=%q", names["91"], names["245"])
	}
	if names["-6"] != "Milk (Any)" {
		t.Errorf("category seed clobbered: %q", names["-6"])
	}
}

// Length or quantity mismatches mean the wiki row does not correspond to the
// raw entry; guessing from it would poison the map.
func TestResolveNamesSkipsMismatchedRows(t *testing.T) {
	entries := []rawEntry{{Key: "Bogus", Raw: "999 3/x/1/false/default/"}}
	pages := map[string]string{"Bogus": "|ingredients = {{Name|Something|7}}"}
	if names := resolveNames(entries, pages); names["999"] != "" {
		t.Errorf("resolved from a mismatched row: %q", names["999"])
	}
}

func TestResolveNamesFallbackMap(t *testing.T) {
	names := resolveNames(nil, nil)
	if names["194"] != "Fried Egg" {
		t.Errorf("curated fallback missing: %q", names["194"])
	}
}
