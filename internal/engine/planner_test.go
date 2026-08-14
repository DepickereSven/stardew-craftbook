package engine

import "testing"

var furnaceIridium = Machine{Machine: "Furnace",
	Inputs: []Ingredient{{ID: "386", Name: "Iridium Ore", Qty: 5}, {ID: "382", Name: "Coal", Qty: 1}},
	Output: Ingredient{ID: "337", Name: "Iridium Bar", Qty: 1}, Minutes: 480}

var kiln = Machine{Machine: "Charcoal Kiln",
	Inputs: []Ingredient{{ID: "388", Name: "Wood", Qty: 10}},
	Output: Ingredient{ID: "382", Name: "Coal", Qty: 1}, Minutes: 30}

var scarecrow = Recipe{Key: "Deluxe Scarecrow", Name: "Deluxe Scarecrow", Type: "crafting", OutputQty: 1,
	Ingredients: []Ingredient{
		{ID: "388", Name: "Wood", Qty: 50},
		{ID: "337", Name: "Iridium Bar", Qty: 1},
	}}

func TestPlanSingleStep(t *testing.T) {
	snap := snapWith(map[string]int{"388": 60, "386": 5, "382": 1}, nil)
	res, err := PlanRecipe(snap, []Recipe{scarecrow}, []Machine{furnaceIridium}, "Deluxe Scarecrow")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible {
		t.Fatalf("not feasible: %+v", res)
	}
	if len(res.Steps) != 1 || res.Steps[0].Machine != "Furnace" || res.Steps[0].Runs != 1 {
		t.Errorf("steps: %+v", res.Steps)
	}
}

func TestPlanTwoLevelChain(t *testing.T) {
	// no coal, but wood for the kiln: wood -> coal -> (with ore) -> bar
	snap := snapWith(map[string]int{"388": 70, "386": 5}, nil)
	res, err := PlanRecipe(snap, []Recipe{scarecrow}, []Machine{furnaceIridium, kiln}, "Deluxe Scarecrow")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible || len(res.Steps) != 2 {
		t.Fatalf("want 2-step feasible plan, got %+v", res)
	}
	if res.Steps[0].Machine != "Charcoal Kiln" { // dependency ordered before consumer
		t.Errorf("step order: %+v", res.Steps)
	}
}

func TestPlanBudgetsSharedInputs(t *testing.T) {
	// scarecrow needs 50 wood directly; kiln needs 10 more for coal -> 60
	// total, only 55 owned
	snap := snapWith(map[string]int{"388": 55, "386": 5}, nil)
	res, _ := PlanRecipe(snap, []Recipe{scarecrow}, []Machine{furnaceIridium, kiln}, "Deluxe Scarecrow")
	if res.Feasible {
		t.Errorf("should be infeasible — wood double-spent: %+v", res)
	}
	if len(res.StillMissing) == 0 {
		t.Error("expected StillMissing entries")
	}
}

func TestPlanCycleProtection(t *testing.T) {
	// pathological: A->B and B->A machines must not loop forever
	ab := Machine{Machine: "M1", Inputs: []Ingredient{{ID: "A", Name: "A", Qty: 1}}, Output: Ingredient{ID: "B", Name: "B", Qty: 1}, Minutes: 1}
	ba := Machine{Machine: "M2", Inputs: []Ingredient{{ID: "B", Name: "B", Qty: 1}}, Output: Ingredient{ID: "A", Name: "A", Qty: 1}, Minutes: 1}
	rec := Recipe{Key: "R", Name: "R", Type: "crafting", OutputQty: 1, Ingredients: []Ingredient{{ID: "B", Name: "B", Qty: 1}}}
	snap := snapWith(map[string]int{}, nil)
	res, err := PlanRecipe(snap, []Recipe{rec}, []Machine{ab, ba}, "R")
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Errorf("nothing owned — must be infeasible, got %+v", res)
	}
}

func TestPlanUnknownKey(t *testing.T) {
	if _, err := PlanRecipe(snapWith(map[string]int{}, nil), nil, nil, "Nope"); err == nil {
		t.Error("expected error")
	}
}

func TestEvaluateWithPlannerUpgradesFarOff(t *testing.T) {
	// nothing satisfied directly, but iridium bar producible -> partial
	bar := Recipe{Key: "BarOnly", Name: "BarOnly", Type: "crafting", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "337", Name: "Iridium Bar", Qty: 1}}}
	snap := snapWith(map[string]int{"386": 5, "382": 1}, nil)
	av := EvaluateWithPlanner(snap, []Recipe{bar}, []Machine{furnaceIridium})
	if av[0].State != Partial {
		t.Errorf("state = %s, want partial", av[0].State)
	}
}

// A recipe already craftable needs no plan at all.
func TestPlanOnCraftableRecipeIsEmpty(t *testing.T) {
	snap := snapWith(map[string]int{"388": 100, "337": 5}, nil)
	res, err := PlanRecipe(snap, []Recipe{scarecrow}, []Machine{furnaceIridium}, "Deluxe Scarecrow")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible || len(res.Steps) != 0 {
		t.Errorf("owning everything should need no steps: %+v", res)
	}
}

// Several runs are needed when one machine cycle yields less than required.
func TestPlanMultipleRuns(t *testing.T) {
	rec := Recipe{Key: "NeedsThree", Name: "NeedsThree", Type: "crafting", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "337", Name: "Iridium Bar", Qty: 3}}}
	snap := snapWith(map[string]int{"386": 15, "382": 3}, nil)
	res, err := PlanRecipe(snap, []Recipe{rec}, []Machine{furnaceIridium}, "NeedsThree")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible || len(res.Steps) != 1 || res.Steps[0].Runs != 3 {
		t.Errorf("want one 3-run furnace step, got %+v", res)
	}
}

// A machine whose output covers more than needed must not report a shortfall,
// and the leftover stays available to later ingredients.
func TestPlanRoundsRunsUpAndKeepsSurplus(t *testing.T) {
	fireQuartz := Machine{Machine: "Furnace",
		Inputs: []Ingredient{{ID: "82", Name: "Fire Quartz", Qty: 1}, {ID: "382", Name: "Coal", Qty: 1}},
		Output: Ingredient{ID: "338", Name: "Refined Quartz", Qty: 3}, Minutes: 90}
	rec := Recipe{Key: "NeedsTwo", Name: "NeedsTwo", Type: "crafting", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "338", Name: "Refined Quartz", Qty: 2}}}
	snap := snapWith(map[string]int{"82": 1, "382": 1}, nil)
	res, err := PlanRecipe(snap, []Recipe{rec}, []Machine{fireQuartz}, "NeedsTwo")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible || len(res.Steps) != 1 || res.Steps[0].Runs != 1 {
		t.Errorf("one run yields 3, covering 2: %+v", res)
	}
}

// Depth is capped, so a chain longer than the cap is reported infeasible
// rather than explored forever.
func TestPlanDepthCap(t *testing.T) {
	var machines []Machine
	// E->D->C->B->A, four conversions deep, with only E owned.
	for _, s := range []struct{ in, out string }{{"E", "D"}, {"D", "C"}, {"C", "B"}, {"B", "A"}} {
		machines = append(machines, Machine{Machine: "M" + s.out,
			Inputs: []Ingredient{{ID: s.in, Name: s.in, Qty: 1}},
			Output: Ingredient{ID: s.out, Name: s.out, Qty: 1}, Minutes: 1})
	}
	rec := Recipe{Key: "Deep", Name: "Deep", Type: "crafting", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "A", Name: "A", Qty: 1}}}
	snap := snapWith(map[string]int{"E": 1}, nil)
	res, err := PlanRecipe(snap, []Recipe{rec}, machines, "Deep")
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Errorf("chain is deeper than the cap, want infeasible: %+v", res)
	}
}

// Category ingredients are consumed from whatever matching items are owned,
// and an unresolvable category must not silently eat category-0 items.
func TestPlanCategoryConsumption(t *testing.T) {
	rec := Recipe{Key: "Omelet", Name: "Omelet", Type: "cooking", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "-6", Name: "Milk (Any)", Qty: 1, Category: true}}}
	snap := snapWith(map[string]int{"186": 2}, map[string]int{"186": -6})
	res, err := PlanRecipe(snap, []Recipe{rec}, nil, "Omelet")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible {
		t.Errorf("owned large milk should satisfy the milk category: %+v", res)
	}

	bad := Recipe{Key: "Prose", Name: "Prose", Type: "crafting", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "", Name: "Flower (Any)", Qty: 1, Category: true}}}
	zeroCat := snapWith(map[string]int{"999": 5}, map[string]int{"999": 0})
	res, err = PlanRecipe(zeroCat, []Recipe{bad}, nil, "Prose")
	if err != nil {
		t.Fatal(err)
	}
	if res.Feasible {
		t.Errorf("an id-less category must not match category-0 items: %+v", res)
	}
}

func TestPlanFishSmokerUsesFishCategoryAndCoal(t *testing.T) {
	smoker := Machine{Machine: "Fish Smoker",
		Inputs: []Ingredient{{ID: "-4", Name: "Fish (Any)", Qty: 1, Category: true}, {ID: "382", Name: "Coal", Qty: 1}},
		Output: Ingredient{ID: "SmokedFish", Name: "Smoked Fish", Qty: 1}, Minutes: 50}
	recipe := Recipe{Key: "Smoked Fish Dish", Name: "Smoked Fish Dish", Type: "cooking", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "SmokedFish", Name: "Smoked Fish", Qty: 1}}}
	snap := snapWith(map[string]int{"147": 1, "382": 1}, map[string]int{"147": -4})
	res, err := PlanRecipe(snap, []Recipe{recipe}, []Machine{smoker}, recipe.Key)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Feasible || len(res.Steps) != 1 || res.Steps[0].Machine != "Fish Smoker" {
		t.Errorf("fish smoker plan = %+v", res)
	}
}

func TestAvailableMachinesIncludesCategoryConversions(t *testing.T) {
	machines := []Machine{
		{Machine: "Fish Smoker", Inputs: []Ingredient{{ID: "-4", Name: "Fish (Any)", Qty: 1, Category: true}, {ID: "382", Name: "Coal", Qty: 1}}, Output: Ingredient{ID: "SmokedFish", Name: "Smoked Fish", Qty: 1}},
		{Machine: "Keg", Inputs: []Ingredient{{ID: "-79", Name: "Fruit (Any)", Qty: 1, Category: true}}, Output: Ingredient{ID: "348", Name: "Wine", Qty: 1}},
		{Machine: "Dehydrator", Inputs: []Ingredient{{ID: "-79", Name: "Fruit (Any)", Qty: 5, Category: true}}, Output: Ingredient{ID: "DriedFruit", Name: "Dried Fruit", Qty: 1}},
	}
	snap := snapWith(map[string]int{"147": 3, "258": 12, "382": 2}, map[string]int{"147": -4, "258": -79})
	got := AvailableMachines(snap, machines)
	if len(got) != 3 {
		t.Fatalf("available machines = %+v, want three", got)
	}
	byName := map[string]int{}
	for _, machine := range got {
		byName[machine.Machine.Machine] = machine.MaxRuns
	}
	for name, want := range map[string]int{"Fish Smoker": 2, "Keg": 12, "Dehydrator": 2} {
		if got := byName[name]; got != want {
			t.Errorf("%s max runs = %d, want %d", name, got, want)
		}
	}
}

// Item ids are learned from recipe data, so wheat — which no recipe uses — has
// none, and a keg input naming it carries an empty id. Matching on the name is
// the only way that wheat in the chest can ever become beer.
func TestMachineInputWithNoIDMatchesByName(t *testing.T) {
	keg := []Machine{{Machine: "Keg", Inputs: []Ingredient{{Name: "Wheat", Qty: 1}}, Output: Ingredient{ID: "346", Name: "Beer", Qty: 1}}}
	snap := snapWith(map[string]int{"262": 20}, nil)
	snap.Names = map[string]string{"262": "Wheat"}
	got := AvailableMachines(snap, keg)
	if len(got) != 1 || got[0].MaxRuns != 20 {
		t.Errorf("available = %+v, want the keg at 20 runs", got)
	}
}

// A conversion the player cannot supply yet still has to be findable — beer is
// a keg recipe whether or not there is wheat in the chest, and a search that
// only covers what is already possible cannot answer "how do I make beer".
func TestAllMachinesKeepsConversionsWithNothingToFeedThem(t *testing.T) {
	machines := []Machine{
		{Machine: "Keg", Inputs: []Ingredient{{ID: "-79", Name: "Fruit (Any)", Qty: 1, Category: true}}, Output: Ingredient{ID: "348", Name: "Wine", Qty: 1}},
		{Machine: "Keg", Inputs: []Ingredient{{ID: "262", Name: "Wheat", Qty: 1}}, Output: Ingredient{ID: "346", Name: "Beer", Qty: 1}},
	}
	snap := snapWith(map[string]int{"258": 4}, map[string]int{"258": -79})
	got := AllMachines(snap, machines)
	if len(got) != 2 {
		t.Fatalf("all machines = %+v, want both conversions", got)
	}
	var beer *MachineAvailability
	for i, machine := range got {
		if machine.Output.Name == "Beer" {
			beer = &got[i]
		}
	}
	if beer == nil {
		t.Fatal("beer conversion missing")
	}
	if beer.MaxRuns != 0 {
		t.Errorf("beer max runs = %d, want 0", beer.MaxRuns)
	}
	if len(beer.Missing) != 1 || beer.Missing[0].Name != "Wheat" || beer.Missing[0].Need != 1 || beer.Missing[0].Have != 0 {
		t.Errorf("beer missing = %+v, want one wheat short", beer.Missing)
	}
}
