package parser

import (
	"os"
	"testing"
)

func mustParse(t *testing.T, path string) *Snapshot {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	snap, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestPlayerInventoryAggregation(t *testing.T) {
	snap := mustParse(t, "testdata/player_only.xml")
	if snap.Items["388"] != 75 { // 50 + 25, (O) prefix stripped, nil slot skipped
		t.Errorf("Wood count = %d, want 75", snap.Items["388"])
	}
	if snap.Items["184"] != 2 {
		t.Errorf("Milk count = %d", snap.Items["184"])
	}
	if snap.Categories["184"] != -6 {
		t.Errorf("Milk category = %d, want -6", snap.Categories["184"])
	}
}

func TestLearnedRecipes(t *testing.T) {
	snap := mustParse(t, "testdata/player_only.xml")
	if !snap.CraftingLearned["Gate"] {
		t.Error("Gate not learned")
	}
	if !snap.CookingLearned["Fried Egg"] {
		t.Error("Fried Egg not learned")
	}
}

func TestBOMStripped(t *testing.T) {
	// fixture is written with a leading EF BB BF; Parse must not error
	mustParse(t, "testdata/player_only.xml")
}

func TestChestsFridgeBuildingsJunimo(t *testing.T) {
	snap := mustParse(t, "testdata/locations.xml")
	if snap.Items["390"] != 150 { // farm chest 100 + shed chest 50
		t.Errorf("Stone = %d, want 150", snap.Items["390"])
	}
	if snap.Items["176"] != 6 {
		t.Errorf("Egg (fridge) = %d, want 6", snap.Items["176"])
	}
	if snap.Items["382"] != 7 {
		t.Errorf("Coal (junimo) = %d, want 7", snap.Items["382"])
	}
	// Placed containers are themselves retrievable items, so they count.
	if snap.Items["130"] != 2 {
		t.Errorf("Chest = %d, want 2", snap.Items["130"])
	}
}

// A machine records the last thing fed into it in <lastInputItem>. That is a
// memory of a consumed item, not stock — counting it inflated the real save by
// 3217 phantom stacks. Shipping-bin contents are already sold, and furniture
// is not a crafting material.
func TestMachineHoldingsAreNotInventory(t *testing.T) {
	snap := mustParse(t, "testdata/machine_holdings.xml")
	if snap.Items["12"] != 1 {
		t.Errorf("the Keg itself should count once, got %d", snap.Items["12"])
	}
	for _, tc := range []struct{ id, why string }{
		{"304", "lastInputItem (already consumed)"},
		{"348", "itemsFromPlayerToSell (shipping bin)"},
		{"1120", "furniture"},
	} {
		if n := snap.Items[tc.id]; n != 0 {
			t.Errorf("item %s counted %d times but is %s", tc.id, n, tc.why)
		}
	}
}

// Finished output sitting in a machine is real, retrievable stock and counts.
func TestHeldObjectCountsAsOwned(t *testing.T) {
	snap := mustParse(t, "testdata/machine_holdings.xml")
	if snap.Items["303"] != 1 {
		t.Errorf("Pale Ale held in the Keg = %d, want 1", snap.Items["303"])
	}
	if snap.Categories["303"] != -26 {
		t.Errorf("held item category = %d, want -26", snap.Categories["303"])
	}
}
