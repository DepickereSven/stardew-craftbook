package main

import "testing"

func TestValidateCatchesProblems(t *testing.T) {
	bad := []Recipe{
		{Key: "A", Name: "A", Type: "crafting"}, // no ingredients
		{Key: "B", Name: "B", Type: "cooking", Ingredients: []Ingredient{{ID: "1", Name: "X", Qty: 0}}, OutputQty: 1},  // qty 0
		{Key: "B", Name: "B2", Type: "cooking", Ingredients: []Ingredient{{ID: "1", Name: "X", Qty: 1}}, OutputQty: 1}, // dup key
		{Key: "C", Name: "", Type: "cooking", Ingredients: []Ingredient{{ID: "1", Name: "X", Qty: 1}}, OutputQty: 1},   // empty name
	}
	problems := validate(bad, machineConversions())
	if len(problems) < 4 {
		t.Errorf("expected >=4 problems, got %d: %v", len(problems), problems)
	}
}

func TestValidateFlagsUnnamedIngredient(t *testing.T) {
	r := []Recipe{{Key: "A", Name: "A", Type: "crafting", OutputQty: 1,
		Ingredients: []Ingredient{{ID: "999", Qty: 1}}}}
	if p := validate(r, nil); len(p) != 1 {
		t.Errorf("expected the unnamed ingredient to be flagged, got %v", p)
	}
}

func TestValidateMachinesClean(t *testing.T) {
	if p := validate(nil, machineConversions()); len(p) != 0 {
		t.Errorf("machine list invalid: %v", p)
	}
}
