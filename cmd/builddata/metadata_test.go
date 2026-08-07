package main

import "testing"

func TestParseCraftMinutes(t *testing.T) {
	cases := map[string]int{
		"30m":                 30,
		"5h":                  300,
		"2h":                  120,
		"1.5h":                90,
		"200min (3.3h)":       200,
		"180m (3h)":           180,
		"1750m (&#8776;29h)":  1750,
		"2250m (37.5h)":       2250,
		"10,000m (&#8776;7d)": 10000,
	}
	for raw, want := range cases {
		got, err := parseCraftMinutes(raw)
		if err != nil || got != want {
			t.Errorf("parseCraftMinutes(%q) = %d, %v; want %d", raw, got, err, want)
		}
	}
}

func TestParseMetadataHandlesVariablePriceCategoryAndNestedBuff(t *testing.T) {
	page := `{{Infobox
|id = 348
|sellprice = 3 × Base [[Fruits|Fruit]] Price
|edibility = 25
|buff = {{Name|Tipsy|link=Buffs#Tipsy{{!}}Tipsy}}{{Name|Speed|−1}}
|buffduration = 30s
|ingredients = Any [[Fruits|Fruit]] (1)
|craftingstation = {{Name|Keg}}
|crafttime = 10,000m (&#8776;7d)
}}`
	item, err := parseItemPage("Wine", page)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "348" || item.SellPrice != nil || item.SellPriceNote == "" || item.Edibility == nil || *item.Edibility != 25 {
		t.Errorf("item = %+v", item)
	}
	if len(item.Buffs) != 2 || item.Buffs[0] != (Buff{Name: "Tipsy"}) || item.Buffs[1] != (Buff{Name: "Speed", Value: "-1"}) {
		t.Errorf("buffs = %+v", item.Buffs)
	}
	m, ok, err := machineFromPage(item, page, map[string]string{})
	if err != nil || !ok {
		t.Fatalf("machineFromPage = %+v, %v, %v", m, ok, err)
	}
	if m.Minutes != 10000 || len(m.Inputs) != 1 || !m.Inputs[0].Category || m.Inputs[0].Name != "Fruit (Any)" {
		t.Errorf("machine = %+v", m)
	}
}

func TestParseMetadataParsesPriceTemplate(t *testing.T) {
	page := `{{Infobox
|id = 131
|price = {{Price|75}}
}}`
	item, err := parseItemPage("Sardine", page)
	if err != nil {
		t.Fatal(err)
	}
	if item.SellPrice == nil || *item.SellPrice != 75 {
		t.Errorf("sell price = %v, want 75", item.SellPrice)
	}
	if item.SellPriceNote != "" {
		t.Errorf("sell price note = %q, want empty", item.SellPriceNote)
	}
}

func TestCollectMetadataKeepsIDlessItemInfoboxes(t *testing.T) {
	pages := map[string]string{
		"Herring": `{{Infobox fish
|price = 30
}}`,
		"Leather Boots": `{{Infobox clothing
|value = {{Price|100}}
}}`,
		"The Desert": `{{Infobox location
|name = The Desert
}}`,
	}
	_, items, _, err := collectMetadata(pages, nil)
	if err != nil {
		t.Fatal(err)
	}
	for key, wantPrice := range map[string]int{"name:Herring": 30, "name:Leather Boots": 100} {
		item, ok := items[key]
		if !ok || item.SellPrice == nil || *item.SellPrice != wantPrice {
			t.Errorf("items[%q] = %+v, want sell price %d", key, item, wantPrice)
		}
	}
	if _, ok := items["name:The Desert"]; ok {
		t.Error("location without an item ID was included")
	}
}

func TestMachineMergeKeepsBaselineIDs(t *testing.T) {
	baseline := Machine{Machine: "Keg", Inputs: []Ingredient{{ID: "433", Name: "Coffee Bean", Qty: 5}}, Output: Ingredient{ID: "395", Name: "Coffee", Qty: 1}, Minutes: 120}
	scraped := Machine{Machine: "Keg", Inputs: []Ingredient{{Name: "Coffee Bean", Qty: 5}}, Output: Ingredient{ID: "395", Name: "Coffee", Qty: 1}, Minutes: 120}
	got := appendMachineUnique([]Machine{baseline}, scraped)
	if len(got) != 1 || got[0].Inputs[0].ID != "433" {
		t.Errorf("baseline conversion was not retained: %+v", got)
	}
}

func TestParseMachineIngredientsNormalizesDehydratorFruit(t *testing.T) {
	got := parseMachineIngredients(`Any [[Fruit]] except [[Grape]]s (5)`, nil)
	if len(got) != 1 || got[0].Name != "Fruit (Any)" || !got[0].Category || got[0].Qty != 5 {
		t.Errorf("ingredients = %+v", got)
	}
}
