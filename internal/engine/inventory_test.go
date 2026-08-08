package engine

import (
	"strconv"
	"testing"

	"github.com/svendep/stardew-craftbook/internal/parser"
)

func ptr(n int) *int { return &n }

// showInt renders a nullable gold figure for failure messages, where "none" and
// a value have to be told apart at a glance.
func showInt(p *int) string {
	if p == nil {
		return "none"
	}
	return strconv.Itoa(*p)
}

func testIndex() *ItemIndex {
	return NewItemIndex(map[string]Item{
		"131":        {ID: "131", Name: "Sardine", SellPrice: ptr(75)},
		"246":        {ID: "246", Name: "Wheat Flour", SellPrice: ptr(50)},
		"709":        {ID: "709", Name: "Hardwood", SellPrice: ptr(15)},
		"388":        {ID: "388", Name: "Wood", SellPrice: ptr(2)},
		"390":        {ID: "390", Name: "Stone"}, // in the dataset, no price
		"BobberItem": {ID: "BobberItem", Name: "Cork Bobber", SellPrice: ptr(250)},
		"FenceItem":  {ID: "FenceItem", Name: "Hardwood Fence", SellPrice: ptr(10)},
		"PressItem":  {ID: "PressItem", Name: "Cheese Press", SellPriceNote: "Cannot be sold"},
		"CaskItem":   {ID: "CaskItem", Name: "Cask", SellPriceNote: "Cannot Be Sold"},
		"TemplItem":  {ID: "TemplItem", Name: "Templated", SellPriceNote: "{{Price|50}}"},
		"NAItem":     {ID: "NAItem", Name: "Unrecorded", SellPriceNote: "N/A"},
	})
}

// A bare id alone is not a safe key: the dataset says 246 is Wheat Flour while
// the save may hold a Coffee Maker under the same number.
func TestLookupRejectsNameMismatch(t *testing.T) {
	idx := testIndex()
	if _, ok := idx.Lookup("246", "Coffee Maker"); ok {
		t.Error("Coffee Maker matched Wheat Flour's dataset entry")
	}
	if it, ok := idx.Lookup("246", "Wheat Flour"); !ok || *it.SellPrice != 50 {
		t.Error("Wheat Flour did not resolve to its own entry")
	}
}

// Big craftables are keyed bare in the dataset but qualified in a snapshot, so
// the bare fallback has to work — still under the name guard.
func TestLookupFallsBackToBareIDWhenNameAgrees(t *testing.T) {
	idx := NewItemIndex(map[string]Item{"146": {ID: "146", Name: "Campfire", SellPrice: ptr(0)}})
	if _, ok := idx.Lookup("(BC)146", "Campfire"); !ok {
		t.Error("(BC)146 did not fall back to bare 146")
	}
	if _, ok := idx.Lookup("(BC)146", "Something Else"); ok {
		t.Error("bare fallback ignored the name guard")
	}
}

func TestLookupUsesIDlessMetadataAfterAnIDCollision(t *testing.T) {
	idx := NewItemIndex(map[string]Item{
		"147":          {ID: "147", Name: "Stump Brazier"},
		"name:Herring": {ID: "name:Herring", Name: "Herring", SellPrice: ptr(30)},
	})
	it, ok := idx.Lookup("147", "Herring")
	if !ok || it.SellPrice == nil || *it.SellPrice != 30 {
		t.Fatalf("Herring did not resolve through name metadata: %+v", it)
	}
	if _, ok := idx.Lookup("147", "Stump Brazier"); !ok {
		t.Error("Stump Brazier no longer resolves by its direct ID")
	}
}

func TestInventoryUsesSavedPriceAndQualityForVariants(t *testing.T) {
	baseBlueberry, driedBlueberries, driedStrawberries := 50, 400, 475
	snap := &parser.Snapshot{Stacks: map[string]parser.ItemStack{
		"258":                                   {Key: "258", ID: "258", Name: "Blueberry", Count: 2, Price: &baseBlueberry, Quality: 2},
		"DriedFruit#Dried Blueberries#p400#q0":  {Key: "DriedFruit#Dried Blueberries#p400#q0", ID: "DriedFruit", Name: "Dried Blueberries", Count: 3, Price: &driedBlueberries},
		"DriedFruit#Dried Strawberries#p475#q0": {Key: "DriedFruit#Dried Strawberries#p475#q0", ID: "DriedFruit", Name: "Dried Strawberries", Count: 2, Price: &driedStrawberries},
	}}
	idx := NewItemIndex(map[string]Item{
		"258": {ID: "258", Name: "Blueberry", SellPrice: ptr(50)},
		"635": {ID: "635", Name: "Dried Fruit", SellPriceNote: "7.5 × Fruit Base Price + 25"},
	})
	got := map[string]InventoryItem{}
	for _, item := range BuildInventory(snap, idx, nil) {
		got[item.ID] = item
	}
	for key, want := range map[string]int{
		"258":                                   75,
		"DriedFruit#Dried Blueberries#p400#q0":  400,
		"DriedFruit#Dried Strawberries#p475#q0": 475,
	} {
		item := got[key]
		if item.SellPrice == nil || *item.SellPrice != want {
			t.Errorf("%s sell price = %v, want %d", key, showInt(item.SellPrice), want)
		}
	}
}

func TestInventoryUsesSavedPriceWithoutMetadata(t *testing.T) {
	price := 42
	snap := &parser.Snapshot{Stacks: map[string]parser.ItemStack{
		"ModItem": {Key: "ModItem", ID: "ModItem", Name: "Uncatalogued Item", Count: 2, Price: &price},
	}}
	got := BuildInventory(snap, NewItemIndex(nil), nil)
	if len(got) != 1 || got[0].SellPrice == nil || *got[0].SellPrice != 42 || got[0].StackValue == nil || *got[0].StackValue != 84 {
		t.Errorf("inventory = %+v, want saved price 42 and stack value 84", got)
	}
}

func TestPriceTemplateRecovered(t *testing.T) {
	idx := testIndex()
	it, ok := idx.Lookup("TemplItem", "Templated")
	if !ok || it.SellPrice == nil || *it.SellPrice != 50 {
		t.Fatalf("unparsed {{Price|50}} not recovered: %+v", it)
	}
	if it.SellPriceNote != "" {
		t.Errorf("note kept after recovery: %q", it.SellPriceNote)
	}
}

func TestNotForSale(t *testing.T) {
	idx := testIndex()
	for _, name := range []string{"Cheese Press", "Cask"} {
		it, _ := idx.ByName(name)
		if !notForSale(it) {
			t.Errorf("%s should be not-for-sale", name)
		}
	}
	// "N/A" records absence of information, not a rule against selling.
	it, _ := idx.ByName("Unrecorded")
	if notForSale(it) {
		t.Error("N/A treated as cannot-be-sold")
	}
}

func snapshot() *parser.Snapshot {
	return &parser.Snapshot{
		Items:           map[string]int{"709": 327, "388": 100, "390": 50, "(BC)246": 1},
		Names:           map[string]string{"709": "Hardwood", "388": "Wood", "390": "Stone", "(BC)246": "Coffee Maker"},
		Categories:      map[string]int{"709": -16, "388": -16, "390": -16, "(BC)246": -9},
		CraftingLearned: map[string]bool{"Cork Bobber": true, "Hardwood Fence": true, "Cheese Press": true},
		CookingLearned:  map[string]bool{},
	}
}

func testRecipes() []Recipe {
	return []Recipe{
		{Key: "Cork Bobber", Name: "Cork Bobber", Type: "crafting", OutputQty: 1,
			Ingredients: []Ingredient{{ID: "709", Name: "Hardwood", Qty: 5}, {ID: "388", Name: "Wood", Qty: 10}}},
		{Key: "Hardwood Fence", Name: "Hardwood Fence", Type: "crafting", OutputQty: 1,
			Ingredients: []Ingredient{{ID: "709", Name: "Hardwood", Qty: 1}}},
		{Key: "Cheese Press", Name: "Cheese Press", Type: "crafting", OutputQty: 1,
			Ingredients: []Ingredient{{ID: "709", Name: "Hardwood", Qty: 10}}},
		{Key: "Stone Thing", Name: "Stone Thing", Type: "crafting", OutputQty: 1,
			Ingredients: []Ingredient{{ID: "709", Name: "Hardwood", Qty: 1}, {ID: "390", Name: "Stone", Qty: 1}}},
	}
}

func detailFor(t *testing.T, id string) ItemDetail {
	t.Helper()
	snap, idx, recipes := snapshot(), testIndex(), testRecipes()
	d, ok := BuildItemDetail(snap, idx, recipes, Evaluate(snap, recipes), id)
	if !ok {
		t.Fatalf("item %s not found", id)
	}
	return d
}

func TestVerdicts(t *testing.T) {
	byName := map[string]UsedIn{}
	for _, u := range detailFor(t, "709").UsedIn {
		byName[u.Name] = u
	}

	// 250 output − (5 Hardwood × 15 + 10 Wood × 2) = 155
	if got := byName["Cork Bobber"]; got.Verdict != VerdictProfit || got.Delta == nil || *got.Delta != 155 {
		t.Errorf("Cork Bobber = %s delta %s, want profit 155", got.Verdict, showInt(got.Delta))
	}
	if got := byName["Hardwood Fence"]; got.Verdict != VerdictLoss || got.Delta == nil || *got.Delta != -5 {
		t.Errorf("Hardwood Fence = %s delta %s, want loss -5", got.Verdict, showInt(got.Delta))
	}
	if got := byName["Cheese Press"]; got.Verdict != VerdictNotForSale {
		t.Errorf("Cheese Press = %s, want not_for_sale", got.Verdict)
	}
	// An unsellable output must carry no cost either — a visible input cost
	// invites a subtraction with no answer.
	if got := byName["Cheese Press"]; got.InputCost != nil || got.Delta != nil {
		t.Errorf("Cheese Press carried cost %s delta %s", showInt(got.InputCost), showInt(got.Delta))
	}
	// Stone is priceless in the dataset, so the total is unknowable — and must
	// not be reported as the cost of the Hardwood alone.
	if got := byName["Stone Thing"]; got.Verdict != VerdictUnknown || got.InputCost != nil {
		t.Errorf("Stone Thing = %s cost %s, want unknown and no cost", got.Verdict, showInt(got.InputCost))
	}
}

func TestQtyHereAndMaxMakeable(t *testing.T) {
	byName := map[string]UsedIn{}
	for _, u := range detailFor(t, "709").UsedIn {
		byName[u.Name] = u
	}
	// 327 Hardwood / 5 = 65, but only 100 Wood / 10 = 10 to go round.
	if got := byName["Cork Bobber"]; got.QtyHere != 5 || got.MaxMakeable != 10 {
		t.Errorf("Cork Bobber qty_here=%d max=%d, want 5 and 10", got.QtyHere, got.MaxMakeable)
	}
	if got := byName["Hardwood Fence"]; got.MaxMakeable != 327 {
		t.Errorf("Hardwood Fence max = %d, want 327", got.MaxMakeable)
	}
}

// A qualified id feeds no recipe, which is the whole point of qualifying it.
func TestBigCraftableHasNoRecipes(t *testing.T) {
	d := detailFor(t, "(BC)246")
	if d.Name != "Coffee Maker" {
		t.Errorf("name = %q", d.Name)
	}
	if len(d.UsedIn) != 0 {
		t.Errorf("Coffee Maker used_in = %d rows, want 0", len(d.UsedIn))
	}
	if d.SellPrice != nil {
		t.Errorf("Coffee Maker inherited a price: %d", *d.SellPrice)
	}
}

func TestBuildItemDetailUnknownItem(t *testing.T) {
	snap, idx, recipes := snapshot(), testIndex(), testRecipes()
	if _, ok := BuildItemDetail(snap, idx, recipes, nil, "nope"); ok {
		t.Error("unowned item reported as found")
	}
}

func TestInventorySortedByStackValue(t *testing.T) {
	inv := BuildInventory(snapshot(), testIndex(), testRecipes())
	if len(inv) != 4 {
		t.Fatalf("inventory has %d entries, want 4", len(inv))
	}
	if inv[0].Name != "Hardwood" || inv[0].StackValue == nil || *inv[0].StackValue != 4905 {
		t.Errorf("first entry = %+v, want Hardwood at 4905", inv[0])
	}
	if inv[1].Name != "Wood" {
		t.Errorf("second entry = %s, want Wood", inv[1].Name)
	}
	// Unpriced items sort last, and must carry no value at all rather than 0.
	last := inv[len(inv)-1]
	if last.SellPrice != nil || last.StackValue != nil {
		t.Errorf("unpriced item carries a value: %+v", last)
	}
	for _, it := range inv {
		if it.Name == "Hardwood" && it.RecipeCount != 4 {
			t.Errorf("Hardwood recipe_count = %d, want 4", it.RecipeCount)
		}
		if it.Name == "Coffee Maker" && it.RecipeCount != 0 {
			t.Errorf("Coffee Maker recipe_count = %d, want 0", it.RecipeCount)
		}
	}
}
