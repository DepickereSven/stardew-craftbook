package parser

import (
	"os"
	"testing"
)

func cropAt(t *testing.T, snap *Snapshot, x, y int) CropPlant {
	t.Helper()
	for _, c := range snap.Crops {
		if c.X == x && c.Y == y {
			return c
		}
	}
	t.Fatalf("no crop at (%d,%d); have %+v", x, y, snap.Crops)
	return CropPlant{}
}

func TestParsesGameDate(t *testing.T) {
	snap := mustParse(t, "testdata/crops.xml")
	want := GameDate{Season: "fall", Day: 7, Year: 3}
	if snap.Date != want {
		t.Errorf("Date = %+v, want %+v", snap.Date, want)
	}
}

// Mail flags are how the game records world progress, and the only reason to
// read them here is that some of them gate whole locations.
func TestParsesMailFlags(t *testing.T) {
	snap := mustParse(t, "testdata/crops.xml")
	if !snap.MailReceived["ccPantry"] || !snap.MailReceived["Willy_married"] {
		t.Errorf("mail flags = %v", snap.MailReceived)
	}
	if snap.MailReceived["willyBoatFixed"] {
		t.Error("a flag the save does not carry read as set")
	}
}

func TestParsesPlantedCrops(t *testing.T) {
	snap := mustParse(t, "testdata/crops.xml")
	if len(snap.Crops) != 5 {
		t.Fatalf("crops = %d, want 5 (three in soil, one in a pot, one in the greenhouse)", len(snap.Crops))
	}

	cranberry := cropAt(t, snap, 50, 31)
	if cranberry.HarvestID != "282" || cranberry.SeedID != "493" {
		t.Errorf("cranberry ids = %q/%q", cranberry.HarvestID, cranberry.SeedID)
	}
	if !cranberry.Watered {
		t.Error("state 1 should read as watered")
	}
	if got, want := cranberry.PhaseDays, []int{1, 2, 1, 1, 2, 99999}; len(got) != len(want) {
		t.Errorf("phaseDays = %v, want %v", got, want)
	}
	if cranberry.CurrentPhase != 4 || cranberry.DayOfCurrentPhase != 1 {
		t.Errorf("phase = %d day %d", cranberry.CurrentPhase, cranberry.DayOfCurrentPhase)
	}
	if cranberry.InPot || !cranberry.Outdoors || cranberry.Greenhouse {
		t.Errorf("farm soil misread: %+v", cranberry)
	}

	corn := cropAt(t, snap, 12, 7)
	if !corn.FullyGrown {
		t.Error("regrowing corn should be marked fully grown")
	}
	if corn.Watered {
		t.Error("state 0 should read as dry")
	}
}

// A terrain feature that is not HoeDirt — grass, a tree — carries no crop and
// must not turn into an empty one.
func TestNonCropTerrainFeaturesAreSkipped(t *testing.T) {
	snap := mustParse(t, "testdata/crops.xml")
	for _, c := range snap.Crops {
		if c.HarvestID == "" {
			t.Errorf("crop with no harvest id parsed from a non-crop feature: %+v", c)
		}
	}
}

func TestParsesGardenPotCrop(t *testing.T) {
	snap := mustParse(t, "testdata/crops.xml")
	pot := cropAt(t, snap, 8, 9)
	if !pot.InPot {
		t.Error("crop in a garden pot not marked as potted")
	}
	if pot.HarvestID != "24" || pot.Location != "Farm" {
		t.Errorf("pot crop = %+v", pot)
	}
}

// Building interiors are locations in their own right and their soil counts.
func TestParsesCropsInsideBuildings(t *testing.T) {
	snap := mustParse(t, "testdata/crops.xml")
	greenhouse := cropAt(t, snap, 2, 4)
	if greenhouse.Location != "Greenhouse" {
		t.Errorf("location = %q, want Greenhouse", greenhouse.Location)
	}
	if !greenhouse.Greenhouse || greenhouse.Outdoors {
		t.Errorf("greenhouse flags = %+v", greenhouse)
	}
}

func TestRealSaveCrops(t *testing.T) {
	if _, err := os.Stat(realSavePath); err != nil {
		t.Skip("real save not present")
	}
	snap, err := ParseFile(realSavePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Crops) == 0 {
		t.Fatal("no crops read from the real save")
	}
	if snap.Date.Season == "" || snap.Date.Day == 0 {
		t.Errorf("date not read: %+v", snap.Date)
	}
	locations := map[string]int{}
	withSchedule := 0
	for _, c := range snap.Crops {
		locations[c.Location]++
		if c.Location == "" {
			t.Fatalf("crop with no location: %+v", c)
		}
		if len(c.PhaseDays) > 0 {
			withSchedule++
			if c.CurrentPhase >= len(c.PhaseDays) {
				t.Errorf("phase %d out of range for %v", c.CurrentPhase, c.PhaseDays)
			}
		}
	}
	if withSchedule == 0 {
		t.Error("no crop carried a growth schedule")
	}
	t.Logf("crops: %d across %d locations on %+v", len(snap.Crops), len(locations), snap.Date)
}
