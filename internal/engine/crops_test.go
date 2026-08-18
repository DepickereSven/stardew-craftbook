package engine

import (
	"os"
	"testing"

	"github.com/svendep/stardew-craftbook/internal/parser"
)

// The counts here are read straight off the game's own rule: a watered day
// advances dayOfCurrentPhase, and the phase rolls over once it reaches
// phaseDays[currentPhase]. The last phaseDays entry is the 99999 sentinel, so
// reaching that index means harvestable.
func TestDaysUntilHarvest(t *testing.T) {
	cases := []struct {
		name  string
		crop  parser.CropPlant
		days  int
		ready bool
	}{
		{
			name: "mid phase sums the rest of this phase and every later one",
			crop: parser.CropPlant{PhaseDays: []int{1, 2, 1, 1, 2, 99999}, CurrentPhase: 0, DayOfCurrentPhase: 0},
			days: 7,
		},
		{
			name: "part way through the last growth phase",
			crop: parser.CropPlant{PhaseDays: []int{1, 2, 1, 1, 2, 99999}, CurrentPhase: 4, DayOfCurrentPhase: 1},
			days: 1,
		},
		{
			name:  "reaching the sentinel phase is harvestable",
			crop:  parser.CropPlant{PhaseDays: []int{1, 2, 1, 1, 2, 99999}, CurrentPhase: 5, DayOfCurrentPhase: 0},
			ready: true,
		},
		{
			name: "a regrowing crop counts its day counter back down",
			crop: parser.CropPlant{PhaseDays: []int{2, 3, 3, 3, 3, 99999}, CurrentPhase: 5, DayOfCurrentPhase: 3, FullyGrown: true},
			days: 3,
		},
		{
			name:  "a regrown crop at zero is ready again",
			crop:  parser.CropPlant{PhaseDays: []int{2, 3, 3, 3, 3, 99999}, CurrentPhase: 5, DayOfCurrentPhase: 0, FullyGrown: true},
			ready: true,
		},
		{
			name:  "a forage crop has no schedule and is always grown",
			crop:  parser.CropPlant{Forage: true},
			ready: true,
		},
		{
			name: "zero-length phases cost nothing",
			crop: parser.CropPlant{PhaseDays: []int{0, 0, 3, 99999}, CurrentPhase: 0, DayOfCurrentPhase: 0},
			days: 3,
		},
		{
			name: "a day counter past its phase still needs one more day",
			crop: parser.CropPlant{PhaseDays: []int{1, 1, 99999}, CurrentPhase: 1, DayOfCurrentPhase: 4},
			days: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			days, ready := daysUntilHarvest(tc.crop)
			if ready != tc.ready {
				t.Fatalf("ready = %v, want %v", ready, tc.ready)
			}
			if !ready && days != tc.days {
				t.Errorf("days = %d, want %d", days, tc.days)
			}
		})
	}
}

func TestGameDateAddDaysRollsSeasonsAndYears(t *testing.T) {
	start := newGameDate("fall", 27, 3)
	cases := []struct {
		add   int
		label string
	}{
		{1, "Fall 28, Year 3"},
		{2, "Winter 1, Year 3"},
		{29, "Winter 28, Year 3"},
		{30, "Spring 1, Year 4"},
	}
	for _, tc := range cases {
		if got := start.AddDays(tc.add).Label; got != tc.label {
			t.Errorf("AddDays(%d) = %q, want %q", tc.add, got, tc.label)
		}
	}
}

func cropsFixture(t *testing.T) map[string]CropData {
	t.Helper()
	return map[string]CropData{
		"282": {ID: "282", Name: "Cranberries", Seasons: []string{"fall"}, RegrowDays: ptr(5), WikiURL: "https://example/Cranberries"},
		"276": {ID: "276", Name: "Pumpkin", Seasons: []string{"fall"}},
		"400": {ID: "400", Name: "Strawberry", Seasons: []string{"spring"}, RegrowDays: ptr(4)},
		"613": {ID: "613", Name: "Apple", Seasons: []string{"fall"}, GrowthDays: ptr(28)},
		"637": {ID: "637", Name: "Pomegranate", Seasons: []string{"fall"}, GrowthDays: ptr(28)},
		"91":  {ID: "91", Name: "Banana", Seasons: append([]string(nil), parser.Seasons...), GrowthDays: ptr(28)},
	}
}

func snapshotFixture(crops ...parser.CropPlant) *parser.Snapshot {
	return &parser.Snapshot{Date: parser.GameDate{Season: "fall", Day: 7, Year: 3}, Crops: crops}
}

func TestBuildCropsSummaryAndTimeline(t *testing.T) {
	snap := snapshotFixture(
		// Ready now.
		parser.CropPlant{Location: "Farm", Outdoors: true, X: 1, Y: 1, HarvestID: "282", PhaseDays: []int{1, 2, 1, 1, 2, 99999}, CurrentPhase: 5, Watered: true},
		// One day out, and dry.
		parser.CropPlant{Location: "Farm", Outdoors: true, X: 2, Y: 1, HarvestID: "282", PhaseDays: []int{1, 2, 1, 1, 2, 99999}, CurrentPhase: 4, DayOfCurrentPhase: 1},
		// Three days out.
		parser.CropPlant{Location: "Farm", Outdoors: true, X: 3, Y: 1, HarvestID: "276", PhaseDays: []int{1, 2, 99999}, CurrentPhase: 0, Watered: true},
	)
	view := BuildCrops(snap, cropsFixture(t))

	if view.Date.Label != "Fall 7, Year 3" {
		t.Errorf("date = %q", view.Date.Label)
	}
	want := CropsSummary{TotalGrowing: 3, ReadyNow: 1, Unwatered: 1, NextHarvestDays: ptr(1)}
	got := view.Summary
	if got.TotalGrowing != want.TotalGrowing || got.ReadyNow != want.ReadyNow || got.Unwatered != want.Unwatered {
		t.Errorf("summary = %+v, want %+v", got, want)
	}
	if got.NextHarvestDays == nil || *got.NextHarvestDays != 1 {
		t.Fatalf("next harvest days = %v, want 1", got.NextHarvestDays)
	}
	if got.NextHarvestDate == nil || got.NextHarvestDate.Label != "Fall 8, Year 3" {
		t.Errorf("next harvest date = %+v", got.NextHarvestDate)
	}

	if len(view.Timeline) != 3 {
		t.Fatalf("timeline buckets = %d, want 3", len(view.Timeline))
	}
	if view.Timeline[0].Days != 0 || view.Timeline[0].Label != "Ready now" || view.Timeline[0].Count != 1 {
		t.Errorf("first bucket = %+v", view.Timeline[0])
	}
	if view.Timeline[1].Label != "1 day" || view.Timeline[2].Label != "3 days" {
		t.Errorf("bucket labels = %q, %q", view.Timeline[1].Label, view.Timeline[2].Label)
	}
	if view.Timeline[2].Date == nil || view.Timeline[2].Date.Label != "Fall 10, Year 3" {
		t.Errorf("third bucket date = %+v", view.Timeline[2].Date)
	}
}

func TestFruitTreesShareTheHarvestTimeline(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "Farm", Outdoors: true, X: 1, Y: 1, FruitTree: true, TreeID: "632", DaysUntilMature: -12, FruitCount: 2},
		parser.CropPlant{Location: "Greenhouse", Greenhouse: true, X: 2, Y: 2, FruitTree: true, TreeID: "633", DaysUntilMature: 20},
	)
	view := BuildCrops(snap, cropsFixture(t))

	if view.Summary.TotalGrowing != 2 || view.Summary.ReadyNow != 1 || view.Summary.Unwatered != 0 {
		t.Errorf("summary = %+v", view.Summary)
	}
	if len(view.Timeline) != 2 || view.Timeline[0].Days != 0 || view.Timeline[1].Days != 20 {
		t.Fatalf("timeline = %+v", view.Timeline)
	}
	if view.Timeline[1].Date == nil || view.Timeline[1].Date.Label != "Fall 27, Year 3" {
		t.Errorf("tree maturity date = %+v", view.Timeline[1].Date)
	}

	ready := view.Crops[0]
	if !ready.FruitTree || ready.Name != "Pomegranate tree" || !ready.Ready {
		t.Errorf("ready tree = %+v", ready)
	}
	if ready.Regrows == nil || !*ready.Regrows || ready.RegrowDays == nil || *ready.RegrowDays != 1 {
		t.Errorf("tree production cadence = %+v", ready)
	}
	if !view.Groups[0].FruitTree {
		t.Errorf("fruit-tree marker missing from group: %+v", view.Groups[0])
	}
}

func TestOutdoorFruitTreeWaitsForItsBearingSeason(t *testing.T) {
	snap := snapshotFixture(
		// This apple matures in winter, then waits until Fall 1 to bear fruit.
		parser.CropPlant{Location: "Farm", Outdoors: true, FruitTree: true, TreeID: "633", DaysUntilMature: 25},
		// Banana metadata from the crop scraper says "all seasons" because it is
		// describing Ginger Island; the actual tree bears in summer on the farm.
		parser.CropPlant{Location: "Farm", Outdoors: true, FruitTree: true, TreeID: "69", DaysUntilMature: -10},
	)
	view := BuildCrops(snap, cropsFixture(t))

	if got := view.Crops[0].DaysUntilHarvest; got == nil || *got != 106 {
		t.Errorf("apple next harvest = %v days, want 106", got)
	}
	if got := view.Crops[1].DaysUntilHarvest; got == nil || *got != 78 {
		t.Errorf("banana next harvest = %v days, want 78", got)
	}
	if view.Summary.OutOfSeason != 0 {
		t.Errorf("fruit trees should not be counted as crops that will wither: %+v", view.Summary)
	}
}

func TestBuildCropsGroupsByLocationAndCrop(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "282", PhaseDays: []int{1, 99999}, CurrentPhase: 1, Watered: true},
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "282", PhaseDays: []int{1, 99999}, CurrentPhase: 0},
		parser.CropPlant{Location: "Greenhouse", Greenhouse: true, HarvestID: "282", PhaseDays: []int{1, 99999}, CurrentPhase: 1, Watered: true},
	)
	view := BuildCrops(snap, cropsFixture(t))
	if len(view.Groups) != 2 {
		t.Fatalf("groups = %d, want one per location", len(view.Groups))
	}
	farm, greenhouse := view.Groups[0], view.Groups[1]
	if farm.LocationLabel != "Farm" || greenhouse.LocationLabel != "Greenhouse" {
		t.Fatalf("groups out of order: %q, %q", farm.LocationLabel, greenhouse.LocationLabel)
	}
	if farm.Count != 2 || farm.Ready != 1 || farm.Unwatered != 1 {
		t.Errorf("farm group = %+v", farm)
	}
	if farm.EarliestDays == nil || *farm.EarliestDays != 0 {
		t.Errorf("farm earliest = %v, want 0 (something is ready)", farm.EarliestDays)
	}
	if len(farm.Buckets) != 2 || farm.Buckets[0].Days != 0 || farm.Buckets[1].Days != 1 {
		t.Errorf("farm buckets = %+v", farm.Buckets)
	}
	if farm.Regrows == nil || !*farm.Regrows || farm.RegrowDays == nil || *farm.RegrowDays != 5 {
		t.Errorf("regrow not carried onto the group: %+v", farm)
	}
}

// Each bucket dates itself: a group whose earliest crop is one day out still
// has to say Fall 11 for the crops in it that are four days out.
func TestGroupBucketsCarryTheirOwnDate(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "282", PhaseDays: []int{1, 99999}, CurrentPhase: 0, Watered: true},
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "282", PhaseDays: []int{4, 99999}, CurrentPhase: 0, Watered: true},
	)
	view := BuildCrops(snap, cropsFixture(t))
	buckets := view.Groups[0].Buckets
	if len(buckets) != 2 {
		t.Fatalf("buckets = %+v", buckets)
	}
	for _, want := range []struct {
		days  int
		label string
	}{{1, "Fall 8, Year 3"}, {4, "Fall 11, Year 3"}} {
		var found *CropBucket
		for i := range buckets {
			if buckets[i].Days == want.days {
				found = &buckets[i]
			}
		}
		if found == nil || found.Date == nil {
			t.Fatalf("no dated bucket for %d days: %+v", want.days, buckets)
		}
		if found.Date.Label != want.label {
			t.Errorf("%d-day bucket = %q, want %q", want.days, found.Date.Label, want.label)
		}
	}
}

// Wild-seed forage crops are stored permanently "fully grown", so that flag is
// no evidence of regrowth the way it is for a real crop.
func TestForageCropDoesNotClaimRegrowth(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "IslandNorth", Outdoors: true, Forage: true, FullyGrown: true},
	)
	view := BuildCrops(snap, cropsFixture(t))
	c := view.Crops[0]
	if c.Regrows != nil {
		t.Errorf("regrows = %v, want unknown", *c.Regrows)
	}
	if !c.Ready {
		t.Error("a forage crop with no schedule is grown")
	}
}

func TestSeasonMismatchIsFlaggedOnlyWhereItApplies(t *testing.T) {
	snap := snapshotFixture(
		// Strawberry is a spring crop; in fall soil it will wither.
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "400", PhaseDays: []int{1, 99999}, CurrentPhase: 0},
		// The same crop in the greenhouse is fine.
		parser.CropPlant{Location: "Greenhouse", Greenhouse: true, HarvestID: "400", PhaseDays: []int{1, 99999}, CurrentPhase: 0},
		// And so is one in an indoor pot.
		parser.CropPlant{Location: "Shed", InPot: true, HarvestID: "400", PhaseDays: []int{1, 99999}, CurrentPhase: 0},
	)
	view := BuildCrops(snap, cropsFixture(t))
	if view.Summary.OutOfSeason != 1 {
		t.Errorf("out of season = %d, want 1", view.Summary.OutOfSeason)
	}
	for _, c := range view.Crops {
		if c.InSeason == nil {
			t.Fatalf("season not decided for %+v", c)
		}
		wantIn := c.Location != "Farm"
		if *c.InSeason != wantIn {
			t.Errorf("%s in_season = %v, want %v", c.Location, *c.InSeason, wantIn)
		}
		if c.SeasonExempt != wantIn {
			t.Errorf("%s season_exempt = %v", c.Location, c.SeasonExempt)
		}
	}
}

// An outdoor pot is still at the mercy of the season, unlike an indoor one.
func TestOutdoorPotIsNotSeasonExempt(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "Farm", Outdoors: true, InPot: true, HarvestID: "400", PhaseDays: []int{1, 99999}},
	)
	view := BuildCrops(snap, cropsFixture(t))
	c := view.Crops[0]
	if c.SeasonExempt {
		t.Error("outdoor pot treated as season exempt")
	}
	if c.InSeason == nil || *c.InSeason {
		t.Errorf("in_season = %v, want false for a spring crop in fall", c.InSeason)
	}
}

// A crop the dataset has never heard of still reports everything the save knows,
// and says "unknown" rather than "no" about regrowth.
func TestUnknownCropStillReportsSaveFacts(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "Zzz_Modded", PhaseDays: []int{2, 99999}, CurrentPhase: 0, Watered: true},
	)
	view := BuildCrops(snap, nil)
	c := view.Crops[0]
	if c.Regrows != nil {
		t.Errorf("regrows = %v, want unknown", *c.Regrows)
	}
	if c.InSeason != nil {
		t.Errorf("in_season = %v, want unknown", *c.InSeason)
	}
	if c.DaysUntilHarvest == nil || *c.DaysUntilHarvest != 2 {
		t.Errorf("days = %v, want 2", c.DaysUntilHarvest)
	}
	if c.Name != "Crop Zzz_Modded" {
		t.Errorf("name = %q", c.Name)
	}
}

// Having regrown once is proof enough, whatever the dataset says.
func TestRegrowthProvenByTheSave(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "Zzz_Modded", PhaseDays: []int{2, 99999}, CurrentPhase: 1, DayOfCurrentPhase: 2, FullyGrown: true},
	)
	view := BuildCrops(snap, nil)
	c := view.Crops[0]
	if c.Regrows == nil || !*c.Regrows {
		t.Errorf("regrows = %v, want true", c.Regrows)
	}
	if c.DaysUntilHarvest == nil || *c.DaysUntilHarvest != 2 {
		t.Errorf("days = %v, want 2", c.DaysUntilHarvest)
	}
}

// A dead crop is never coming in, so it must not sit in a harvest bucket
// pretending otherwise.
func TestDeadCropsAreExcludedFromHarvestCounts(t *testing.T) {
	snap := snapshotFixture(
		parser.CropPlant{Location: "Farm", Outdoors: true, HarvestID: "276", PhaseDays: []int{1, 99999}, CurrentPhase: 1, Dead: true},
	)
	view := BuildCrops(snap, cropsFixture(t))
	if view.Summary.Dead != 1 || view.Summary.ReadyNow != 0 {
		t.Errorf("summary = %+v", view.Summary)
	}
	if len(view.Timeline) != 0 {
		t.Errorf("timeline = %+v, want empty", view.Timeline)
	}
	if view.Groups[0].EarliestDays != nil {
		t.Errorf("earliest = %v, want none", view.Groups[0].EarliestDays)
	}
}

// Every save ships the Ginger Island locations with wild ginger already in
// them, whether or not the boat has been repaired, so an unreached island must
// not turn up as a planting.
func TestIslandCropsAreLockedUntilTheBoatIsFixed(t *testing.T) {
	plants := []parser.CropPlant{
		{Location: "Farm", Outdoors: true, HarvestID: "276", PhaseDays: []int{1, 99999}, CurrentPhase: 1, Watered: true},
		{Location: "IslandWest", Outdoors: true, Forage: true, FullyGrown: true},
		{Location: "IslandNorth", Outdoors: true, Forage: true, FullyGrown: true},
	}
	locked := snapshotFixture(plants...)
	locked.MailReceived = map[string]bool{}
	view := BuildCrops(locked, cropsFixture(t))
	if view.Summary.Locked != 2 {
		t.Errorf("locked = %d, want 2", view.Summary.Locked)
	}
	for _, c := range view.Crops {
		want := c.Location == "Farm"
		if c.Accessible != want {
			t.Errorf("%s accessible = %v, want %v", c.Location, c.Accessible, want)
		}
	}
	// Reachable places sort ahead of locked ones whatever their counts.
	if len(view.Locations) != 3 || view.Locations[0].Name != "Farm" || view.Locations[0].Accessible != true {
		t.Fatalf("locations = %+v", view.Locations)
	}
	for _, l := range view.Locations[1:] {
		if l.Accessible {
			t.Errorf("%s should be locked", l.Name)
		}
	}

	// Repair the boat and the same crops become the player's business.
	open := snapshotFixture(plants...)
	open.MailReceived = map[string]bool{boatFixedMail: true}
	view = BuildCrops(open, cropsFixture(t))
	if view.Summary.Locked != 0 {
		t.Errorf("locked = %d after the boat is fixed, want 0", view.Summary.Locked)
	}
	for _, g := range view.Groups {
		if !g.Accessible {
			t.Errorf("group %s still locked", g.Location)
		}
	}
}

func TestLocationAccessible(t *testing.T) {
	fixed := map[string]bool{boatFixedMail: true}
	for _, tc := range []struct {
		name string
		mail map[string]bool
		want bool
	}{
		{"Farm", nil, true},
		{"Greenhouse", nil, true},
		{"IslandWest", nil, false},
		{"IslandNorth", nil, false},
		{"IslandWest", fixed, true},
		{"Farm", fixed, true},
	} {
		if got := locationAccessible(tc.name, tc.mail); got != tc.want {
			t.Errorf("locationAccessible(%q, boat=%v) = %v, want %v", tc.name, tc.mail != nil, got, tc.want)
		}
	}
}

func TestLocationLabels(t *testing.T) {
	for name, want := range map[string]string{
		"Farm":            "Farm",
		"IslandWest":      "Ginger Island Farm",
		"Greenhouse":      "Greenhouse",
		"IslandSouthEast": "Ginger Island Southeast",
		"CustomModShed":   "Custom Mod Shed",
		"":                "Unknown location",
	} {
		if got := locationLabel(name); got != want {
			t.Errorf("locationLabel(%q) = %q, want %q", name, got, want)
		}
	}
}

// Every crop and fruit tree the real save has planted must resolve to an entry
// in crops.json; a plant the dataset cannot name is one the dashboard cannot
// explain.
func TestRealSaveCropsAreAllKnown(t *testing.T) {
	if _, err := os.Stat("../../saved/Medow_405190910/Medow_405190910"); err != nil {
		t.Skip("real save not present")
	}
	snap, err := parser.ParseFile("../../saved/Medow_405190910/Medow_405190910")
	if err != nil {
		t.Fatal(err)
	}
	crops, err := LoadCrops()
	if err != nil {
		t.Fatal(err)
	}
	unknown := map[string]int{}
	for _, c := range snap.Crops {
		if c.Forage {
			continue // wild seeds carry no crop id at all
		}
		id := c.HarvestID
		if c.FruitTree {
			if spec, ok := fruitTreeSpecs[c.TreeID]; ok {
				id = spec.HarvestID
			}
		}
		if _, ok := crops[id]; !ok {
			unknown[id]++
		}
	}
	if len(unknown) > 0 {
		t.Errorf("crop ids missing from crops.json: %v", unknown)
	}

	view := BuildCrops(snap, crops)
	if view.Summary.TotalGrowing != len(snap.Crops) {
		t.Errorf("total = %d, want %d", view.Summary.TotalGrowing, len(snap.Crops))
	}
	counted := 0
	for _, b := range view.Timeline {
		counted += b.Count
	}
	if counted+view.Summary.Dead != view.Summary.TotalGrowing {
		t.Errorf("timeline covers %d of %d crops", counted, view.Summary.TotalGrowing)
	}
	// This save has never repaired the boat, so its island forage must be
	// reported as out of reach rather than as something to go and harvest.
	if snap.MailReceived[boatFixedMail] {
		t.Log("save has the boat repaired; island crops are legitimately reachable")
	} else if view.Summary.Locked == 0 {
		t.Error("island crops not flagged as locked in a save without a repaired boat")
	}

	next := "nothing pending"
	if view.Summary.NextHarvestDate != nil {
		next = view.Summary.NextHarvestDate.Label
	}
	t.Logf("%d crops on %s, %d ready, next %s", view.Summary.TotalGrowing, view.Date.Label, view.Summary.ReadyNow, next)
}
