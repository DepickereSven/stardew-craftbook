package engine

import (
	"testing"

	"github.com/svendep/stardew-craftbook/internal/parser"
)

func snapWith(items map[string]int, cats map[string]int, learned ...string) *parser.Snapshot {
	s := &parser.Snapshot{
		Items: items, Categories: cats,
		CraftingLearned: map[string]bool{}, CookingLearned: map[string]bool{},
	}
	for _, l := range learned {
		s.CraftingLearned[l] = true
	}
	return s
}

var gate = Recipe{Key: "Gate", Name: "Gate", Type: "crafting", OutputQty: 1,
	Ingredients: []Ingredient{{ID: "388", Name: "Wood", Qty: 10}}}

var cheesePress = Recipe{Key: "Cheese Press", Name: "Cheese Press", Type: "crafting", OutputQty: 1,
	Ingredients: []Ingredient{
		{ID: "388", Name: "Wood", Qty: 45},
		{ID: "390", Name: "Stone", Qty: 45},
		{ID: "709", Name: "Hardwood", Qty: 10},
	}}

var omelet = Recipe{Key: "Omelet", Name: "Omelet", Type: "cooking", OutputQty: 1,
	Ingredients: []Ingredient{
		{ID: "-5", Name: "Egg (Any)", Qty: 1, Category: true},
		{ID: "-6", Name: "Milk (Any)", Qty: 1, Category: true},
	}}

func TestEvaluateCraftable(t *testing.T) {
	snap := snapWith(map[string]int{"388": 50}, nil, "Gate")
	av := Evaluate(snap, []Recipe{gate})
	if av[0].State != Craftable || !av[0].Learned || len(av[0].Missing) != 0 {
		t.Errorf("got %+v", av[0])
	}
}

func TestEvaluatePartialWithMissing(t *testing.T) {
	snap := snapWith(map[string]int{"388": 50, "390": 10}, nil)
	av := Evaluate(snap, []Recipe{cheesePress})
	if av[0].State != Partial {
		t.Errorf("state = %s", av[0].State)
	}
	if len(av[0].Missing) != 2 {
		t.Fatalf("missing = %+v", av[0].Missing)
	}
	stone := av[0].Missing[0]
	if stone.ID != "390" || stone.Need != 45 || stone.Have != 10 {
		t.Errorf("stone missing entry: %+v", stone)
	}
}

func TestEvaluateFarOff(t *testing.T) {
	snap := snapWith(map[string]int{}, nil)
	av := Evaluate(snap, []Recipe{gate})
	if av[0].State != FarOff {
		t.Errorf("state = %s", av[0].State)
	}
}

func TestCategoryMatching(t *testing.T) {
	snap := snapWith(map[string]int{"186": 1, "176": 2}, map[string]int{"186": -6, "176": -5})
	av := Evaluate(snap, []Recipe{omelet})
	if av[0].State != Craftable {
		t.Errorf("category matching failed: %+v", av[0])
	}
}

// Learned status is tracked per recipe type; a cooking recipe must not be
// looked up in the crafting set.
func TestLearnedUsesTheRightRecipeSet(t *testing.T) {
	snap := snapWith(map[string]int{"186": 1, "176": 1}, map[string]int{"186": -6, "176": -5})
	snap.CookingLearned["Omelet"] = true
	av := Evaluate(snap, []Recipe{omelet})
	if !av[0].Learned {
		t.Error("cooking recipe not reported as learned")
	}
}

// The generated dataset contains machine inputs the wiki described in prose,
// which carry no id. Those must not be mistaken for a real category and must
// never panic.
func TestCountOfToleratesUnresolvedIDs(t *testing.T) {
	snap := snapWith(map[string]int{"388": 5}, map[string]int{"388": -16})
	if n := countOf(snap, Ingredient{ID: "", Name: "Flower (Any)", Qty: 1, Category: true}); n != 0 {
		t.Errorf("unresolved category counted %d, want 0", n)
	}
	if n := countOf(snap, Ingredient{ID: "", Name: "Pine Tree", Qty: 1}); n != 0 {
		t.Errorf("unresolved item counted %d, want 0", n)
	}
}

// Missing entries drive the UI's "needs 35x Stone" text, so they must carry a
// usable name and link even when the count is zero.
func TestMissingEntriesCarryNameAndLink(t *testing.T) {
	snap := snapWith(map[string]int{}, nil)
	av := Evaluate(snap, []Recipe{gate})
	m := av[0].Missing[0]
	if m.Name != "Wood" || m.Need != 10 || m.Have != 0 {
		t.Errorf("missing entry: %+v", m)
	}
	if m.WikiURL != "https://stardewvalleywiki.com/Wood" {
		t.Errorf("wiki url: %s", m.WikiURL)
	}
}

func TestLoadData(t *testing.T) {
	recipes, machines, err := LoadData()
	if err != nil {
		t.Fatal(err)
	}
	if len(recipes) < 150 || len(machines) < 10 {
		t.Errorf("embedded data thin: %d recipes, %d machines", len(recipes), len(machines))
	}
}

// The engine re-declares the dataset structs, so a tag drift in cmd/builddata
// would decode to zero values rather than erroring. Assert the fields the
// engine actually depends on survive a real decode.
func TestLoadDataDecodesFieldsNotJustCounts(t *testing.T) {
	recipes, machines, err := LoadData()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recipes {
		if r.Key == "" || r.Name == "" || r.Type == "" || r.OutputQty < 1 || len(r.Ingredients) == 0 {
			t.Fatalf("recipe decoded with empty fields: %+v", r)
		}
	}
	for _, m := range machines {
		if m.Machine == "" || m.Output.ID == "" || m.Minutes < 1 {
			t.Fatalf("machine decoded with empty fields: %+v", m)
		}
	}
}

func TestLoadDataIncludesVariableMachineConversions(t *testing.T) {
	_, machines, err := LoadData()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		output string
		input  string
	}{
		"Dehydrator":  {output: "DriedFruit", input: "-79"},
		"Fish Smoker": {output: "SmokedFish", input: "-4"},
		"Keg":         {output: "348", input: "-79"},
	}
	for machine, expected := range want {
		found := false
		for _, got := range machines {
			if got.Machine != machine || got.Output.ID != expected.output {
				continue
			}
			for _, input := range got.Inputs {
				if input.ID == expected.input && input.Category {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s conversion to %s with category %s missing", machine, expected.output, expected.input)
		}
	}
}

func TestLoadItems(t *testing.T) {
	items, err := LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 300 {
		t.Errorf("embedded items thin: %d", len(items))
	}
	for id, it := range items {
		if it.ID != id || it.Name == "" || it.WikiURL == "" {
			t.Fatalf("item %q decoded with empty fields: %+v", id, it)
		}
	}
}

// The metadata the API is meant to surface: processing time, sell price and
// buffs. Green Tea carries all three.
func TestLoadItemsDecodesMetadata(t *testing.T) {
	items, err := LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	tea, ok := items["614"]
	if !ok {
		t.Fatal("Green Tea (614) missing from items")
	}
	if tea.Name != "Green Tea" {
		t.Errorf("name = %q", tea.Name)
	}
	if tea.SellPrice == nil || *tea.SellPrice != 100 {
		t.Errorf("sell_price = %v, want 100", tea.SellPrice)
	}
	if tea.ProcessingMinutes == nil || *tea.ProcessingMinutes != 180 {
		t.Errorf("processing_minutes = %v, want 180", tea.ProcessingMinutes)
	}
	if len(tea.Buffs) != 2 || tea.Buffs[0].Name != "Max Energy" || tea.Buffs[0].Value != "+30" {
		t.Errorf("buffs = %+v", tea.Buffs)
	}
	if tea.BuffDuration != "4m 12s" {
		t.Errorf("buff_duration = %q", tea.BuffDuration)
	}
}

// Sell price is genuinely absent for some items and non-numeric for others, so
// it must stay distinguishable from zero.
func TestLoadItemsSellPriceIsOptional(t *testing.T) {
	items, err := LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	wine, ok := items["348"]
	if !ok {
		t.Fatal("Wine (348) missing")
	}
	if wine.SellPrice != nil {
		t.Errorf("Wine has no fixed price, want nil, got %v", *wine.SellPrice)
	}
	if wine.SellPriceNote == "" {
		t.Error("Wine should carry a sell_price_note explaining the variable price")
	}
}
