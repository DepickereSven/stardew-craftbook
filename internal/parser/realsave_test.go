package parser

import (
	"os"
	"testing"
)

// realSavePath is a save copied into the gitignored saved/ directory. The test
// skips when it is absent so CI stays green without it.
const realSavePath = "../../saved/Medow_405190910/Medow_405190910"

func TestRealSave(t *testing.T) {
	if _, err := os.Stat(realSavePath); err != nil {
		t.Skip("real save not present")
	}
	snap, err := ParseFile(realSavePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Items) < 20 {
		t.Errorf("suspiciously few distinct items: %d", len(snap.Items))
	}
	if len(snap.CraftingLearned) < 10 {
		t.Errorf("suspiciously few learned crafting recipes: %d", len(snap.CraftingLearned))
	}
	t.Logf("distinct items: %d, crafting learned: %d, cooking learned: %d",
		len(snap.Items), len(snap.CraftingLearned), len(snap.CookingLearned))
}

// The containers in the real save must actually be walked: a regression that
// silently stopped descending into chests would still leave the player's
// backpack populated, so assert on totals that only chests can explain.
func TestRealSaveFindsContainerContents(t *testing.T) {
	if _, err := os.Stat(realSavePath); err != nil {
		t.Skip("real save not present")
	}
	snap, err := ParseFile(realSavePath)
	if err != nil {
		t.Fatal(err)
	}
	// A backpack holds 36 slots; anything well past that came from containers.
	if len(snap.Items) < 60 {
		t.Errorf("only %d distinct items — containers likely not walked", len(snap.Items))
	}
	total := 0
	for _, n := range snap.Items {
		total += n
	}
	t.Logf("total item count across all storage: %d", total)
	if total < 1000 {
		t.Errorf("total stack count %d is too low for a mid-game save", total)
	}
}

// String item ids arrived in 1.6 and must survive parsing intact.
func TestRealSaveHasStringItemIDs(t *testing.T) {
	if _, err := os.Stat(realSavePath); err != nil {
		t.Skip("real save not present")
	}
	snap, err := ParseFile(realSavePath)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for id := range snap.Items {
		if id != "" && (id[0] < '0' || id[0] > '9') && id[0] != '-' {
			found++
		}
	}
	if found == 0 {
		t.Error("no string item ids found; 1.6 saves should contain some")
	}
	t.Logf("distinct string-id items: %d", found)
}
