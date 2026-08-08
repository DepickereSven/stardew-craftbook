package engine

import (
	"os"
	"testing"

	"github.com/svendep/stardew-craftbook/internal/parser"
)

const realSavePath = "../../saved/Medow_405190910/Medow_405190910"

func loadReal(t *testing.T) (*parser.Snapshot, []Recipe, []Machine) {
	t.Helper()
	if _, err := os.Stat(realSavePath); err != nil {
		t.Skip("real save not present")
	}
	snap, err := parser.ParseFile(realSavePath)
	if err != nil {
		t.Fatal(err)
	}
	recipes, machines, err := LoadData()
	if err != nil {
		t.Fatal(err)
	}
	return snap, recipes, machines
}

// The whole pipeline against real data: every state bucket should be
// populated. An empty bucket means something upstream is silently broken.
func TestRealSaveEvaluation(t *testing.T) {
	snap, recipes, machines := loadReal(t)
	avs := EvaluateWithPlanner(snap, recipes, machines)
	counts := map[State]int{}
	learned := 0
	for _, av := range avs {
		counts[av.State]++
		if av.Learned {
			learned++
		}
	}
	t.Logf("craftable=%d partial=%d far_off=%d (of %d recipes, %d learned)",
		counts[Craftable], counts[Partial], counts[FarOff], len(avs), learned)
	for _, s := range []State{Craftable, Partial, FarOff} {
		if counts[s] == 0 {
			t.Errorf("no recipes in state %q — suspicious", s)
		}
	}
	if learned == 0 {
		t.Error("no recipes matched the learned sets; key mismatch between dataset and save?")
	}
}

// Recipe keys in the dataset must line up with the keys the save uses, or the
// learned flag is meaningless. A few unmatched save keys are expected (recipes
// removed between versions), but most must match.
func TestRealSaveRecipeKeysMatchDataset(t *testing.T) {
	snap, recipes, _ := loadReal(t)
	known := map[string]bool{}
	for _, r := range recipes {
		known[r.Key] = true
	}
	var unmatched []string
	for key := range snap.CraftingLearned {
		if !known[key] {
			unmatched = append(unmatched, key)
		}
	}
	for key := range snap.CookingLearned {
		if !known[key] {
			unmatched = append(unmatched, key)
		}
	}
	total := len(snap.CraftingLearned) + len(snap.CookingLearned)
	t.Logf("learned keys in save: %d, not found in dataset: %d %v", total, len(unmatched), unmatched)
	if len(unmatched)*4 > total {
		t.Errorf("%d of %d learned keys are unknown to the dataset", len(unmatched), total)
	}
}

// Plans built from real data must be well formed: positive runs, a named
// machine, and no step claiming to produce nothing.
func TestRealSavePlansAreWellFormed(t *testing.T) {
	snap, recipes, machines := loadReal(t)
	avs := EvaluateWithPlanner(snap, recipes, machines)
	planned, withSteps := 0, 0
	for _, av := range avs {
		if av.State == Craftable {
			continue
		}
		res, err := PlanRecipe(snap, recipes, machines, av.Recipe.Key)
		if err != nil {
			t.Fatalf("%s: %v", av.Recipe.Key, err)
		}
		planned++
		if len(res.Steps) > 0 {
			withSteps++
		}
		for _, s := range res.Steps {
			if s.Machine == "" || s.Runs < 1 || s.Output.ID == "" || s.Minutes < 1 {
				t.Errorf("%s: malformed step %+v", av.Recipe.Key, s)
			}
		}
		if res.Feasible && len(res.StillMissing) != 0 {
			t.Errorf("%s: feasible but still missing %+v", av.Recipe.Key, res.StillMissing)
		}
		if !res.Feasible && len(res.StillMissing) == 0 {
			t.Errorf("%s: infeasible but nothing reported missing", av.Recipe.Key)
		}
	}
	t.Logf("planned %d non-craftable recipes, %d produced machine steps", planned, withSteps)
}

// Saved item prices are the source of truth for variable artisan goods and
// quality variants, so a representative save should expose prices for most of
// its distinct stacks.
func TestRealSaveInventoryPrices(t *testing.T) {
	snap, recipes, _ := loadReal(t)
	items, err := LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	inventory := BuildInventory(snap, NewItemIndex(items), recipes)
	priced := 0
	for _, item := range inventory {
		if item.SellPrice != nil {
			priced++
		}
	}
	t.Logf("priced inventory stacks: %d/%d", priced, len(inventory))
	if priced*10 < len(inventory)*9 {
		t.Errorf("only %d of %d inventory stacks have prices", priced, len(inventory))
	}
}
