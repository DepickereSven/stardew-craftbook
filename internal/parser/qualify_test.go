package parser

import "testing"

// Bare numeric ids overlap between item types. Counting a chair as a fish makes
// it satisfy recipes calling for fish, so the two must never share a key.
func TestCollidingIDsCountedApart(t *testing.T) {
	snap := mustParse(t, "testdata/qualified.xml")

	if got := snap.Items["131"]; got != 5 {
		t.Errorf("Sardine (object 131) = %d, want 5", got)
	}
	if got := snap.Items["(F)131"]; got != 1 {
		t.Errorf("Crystal Chair (furniture 131) = %d, want 1", got)
	}
	if got := snap.Items["246"]; got != 12 {
		t.Errorf("Wheat Flour (object 246) = %d, want 12", got)
	}
	if got := snap.Items["(BC)246"]; got != 1 {
		t.Errorf("Coffee Maker (big craftable 246) = %d, want 1", got)
	}
}

func TestNonObjectsAreQualified(t *testing.T) {
	snap := mustParse(t, "testdata/qualified.xml")
	for _, tc := range []struct{ id, name string }{
		{"(W)4", "Galaxy Sword"},
		{"(T)GoldAxe", "Gold Axe"},
	} {
		if snap.Items[tc.id] != 1 {
			t.Errorf("%s not counted under %s; keys are %v", tc.name, tc.id, keys(snap))
		}
	}
	// The bare ids they would otherwise have taken must stay clear, so a recipe
	// asking for object 4 is never satisfied by a sword.
	if _, taken := snap.Items["4"]; taken {
		t.Error("Galaxy Sword leaked into the object id space as 4")
	}
}

func TestNamesRetained(t *testing.T) {
	snap := mustParse(t, "testdata/qualified.xml")
	if got := snap.Names["131"]; got != "Sardine" {
		t.Errorf("Names[131] = %q, want Sardine", got)
	}
	if got := snap.Names["(F)131"]; got != "Crystal Chair" {
		t.Errorf("Names[(F)131] = %q, want Crystal Chair", got)
	}
	for id := range snap.Items {
		if snap.Names[id] == "" {
			t.Errorf("item %q has no name", id)
		}
	}
}

// One id can hold several differently-named items. The count must be the sum
// and the name one of them, not a merge artefact.
func TestSharedIDKeepsFirstNameAndSumsCount(t *testing.T) {
	snap := mustParse(t, "testdata/qualified.xml")
	if got := snap.Items["DriedFruit"]; got != 18 {
		t.Errorf("DriedFruit = %d, want 18", got)
	}
	if got := snap.Names["DriedFruit"]; got != "Dried Blueberries" {
		t.Errorf("Names[DriedFruit] = %q, want Dried Blueberries", got)
	}
}

func TestVariantsKeepSavedPriceAndQuality(t *testing.T) {
	snap := mustParse(t, "testdata/qualified.xml")
	for key, want := range map[string]struct {
		name    string
		price   int
		quality int
		count   int
	}{
		"DriedFruit#Dried Blueberries#p400#q0":  {"Dried Blueberries", 400, 0, 10},
		"DriedFruit#Dried Strawberries#p475#q0": {"Dried Strawberries", 475, 0, 8},
		"258":                                   {"Blueberry", 50, 2, 2},
	} {
		stack, ok := snap.Stacks[key]
		if !ok || stack.Name != want.name || stack.Price == nil || *stack.Price != want.price || stack.Quality != want.quality || stack.Count != want.count {
			t.Errorf("Stacks[%q] = %+v, want %+v", key, stack, want)
		}
	}
}

func TestQualifiedIDInSaveLeftAlone(t *testing.T) {
	// (O)388 must land on 388, not "((O)388".
	snap := mustParse(t, "testdata/player_only.xml")
	if snap.Items["388"] != 75 {
		t.Errorf("Wood = %d, want 75", snap.Items["388"])
	}
}

func keys(s *Snapshot) []string {
	out := make([]string, 0, len(s.Items))
	for k := range s.Items {
		out = append(out, k)
	}
	return out
}
