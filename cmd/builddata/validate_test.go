package main

import "testing"

func TestValidateCatchesProblems(t *testing.T) {
	bad := []Recipe{
		{Key: "A", Name: "A", Type: "crafting"}, // no ingredients
		{Key: "B", Name: "B", Type: "cooking", Ingredients: []Ingredient{{ID: "1", Name: "X", Qty: 0}}, OutputQty: 1},  // qty 0
		{Key: "B", Name: "B2", Type: "cooking", Ingredients: []Ingredient{{ID: "1", Name: "X", Qty: 1}}, OutputQty: 1}, // dup key
		{Key: "C", Name: "", Type: "cooking", Ingredients: []Ingredient{{ID: "1", Name: "X", Qty: 1}}, OutputQty: 1},   // empty name
	}
	problems := validate(bad, machineConversions(), nil)
	if len(problems) < 4 {
		t.Errorf("expected >=4 problems, got %d: %v", len(problems), problems)
	}
}

func TestValidateFlagsUnnamedIngredient(t *testing.T) {
	r := []Recipe{{Key: "A", Name: "A", Type: "crafting", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "999", Qty: 1}}}}
	if p := validate(r, nil, nil); len(p) != 1 {
		t.Errorf("expected the unnamed ingredient to be flagged, got %v", p)
	}
}

func TestValidateMachinesClean(t *testing.T) {
	if p := validate(nil, machineConversions(), nil); len(p) != 0 {
		t.Errorf("machine list invalid: %v", p)
	}
}

func TestValidateItems(t *testing.T) {
	negative := -1
	zero := 0
	items := map[string]Item{
		"1": {ID: "1", SellPrice: &negative},
		"2": {ID: "wrong", Name: "Tea", ProcessingMinutes: &zero},
	}
	if p := validate(nil, nil, items); len(p) != 4 {
		t.Errorf("expected four item problems, got %v", p)
	}
}

func TestVariableMachineInputsUseGameCategoryIDs(t *testing.T) {
	want := map[string]string{"Flower (Any)": "-80", "Fruit (Any)": "-79", "Vegetable (Any)": "-75"}
	for _, machine := range machineConversions() {
		for _, input := range machine.Inputs {
			if id, ok := want[input.Name]; ok && input.ID != id {
				t.Errorf("%s input %s has ID %q, want %q", machine.Machine, input.Name, input.ID, id)
			}
		}
	}
}
