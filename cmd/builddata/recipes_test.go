package main

import "testing"

var testNames = map[string]string{
	"388": "Wood", "390": "Stone", "382": "Coal", "335": "Iron Bar",
	"186": "Large Milk", "-6": "Milk (Any)",
}

func TestParseCraftingLine(t *testing.T) {
	r, err := parseRecipeLine("Gate", "388 10/Home/325/false/l 0", "crafting", testNames)
	if err != nil {
		t.Fatal(err)
	}
	if r.Key != "Gate" || r.Name != "Gate" || r.Type != "crafting" {
		t.Errorf("header wrong: %+v", r)
	}
	if len(r.Ingredients) != 1 || r.Ingredients[0].ID != "388" || r.Ingredients[0].Qty != 10 || r.Ingredients[0].Name != "Wood" {
		t.Errorf("ingredients wrong: %+v", r.Ingredients)
	}
	if r.OutputQty != 1 {
		t.Errorf("output qty: %d", r.OutputQty)
	}
	if r.WikiURL != "https://stardewvalleywiki.com/Gate" {
		t.Errorf("wiki url: %s", r.WikiURL)
	}
}

func TestParseCookingLineWithCategoryAndKeyMismatch(t *testing.T) {
	r, err := parseRecipeLine("Cheese Cauli.", "190 1 424 1/70 1/197/f Pam 3", "cooking", map[string]string{"190": "Cauliflower", "424": "Cheese"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "Cheese Cauliflower" {
		t.Errorf("display-name mapping failed: %q", r.Name)
	}
	if r.WikiURL != "https://stardewvalleywiki.com/Cheese_Cauliflower" {
		t.Errorf("wiki url: %s", r.WikiURL)
	}
}

func TestCategoryIngredient(t *testing.T) {
	r, err := parseRecipeLine("Test", "-6 1/x/197/default", "cooking", testNames)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Ingredients[0].Category || r.Ingredients[0].ID != "-6" {
		t.Errorf("category flag not set: %+v", r.Ingredients[0])
	}
}

func TestParseOutputQty(t *testing.T) {
	r, err := parseRecipeLine("Wood Fence", "388 2/Home/322 1/false/l 0", "crafting", testNames)
	if err != nil {
		t.Fatal(err)
	}
	if r.OutputQty != 1 {
		t.Errorf("explicit qty: %d", r.OutputQty)
	}
}

func TestTranslateUnlock(t *testing.T) {
	cases := map[string]string{
		"default":     "Starter",
		"l 0":         "Starter",
		"l 100":       "Special (see wiki)",
		"s Farming 2": "Farming Level 2",
		"f Pam 3":     "Pam (3 hearts)",
		"null":        "Special (see wiki)",
	}
	for raw, want := range cases {
		if got := translateUnlock(raw); got != want {
			t.Errorf("translateUnlock(%q) = %q, want %q", raw, got, want)
		}
	}
}
