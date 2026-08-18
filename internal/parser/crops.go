package parser

import "strings"

// GameDate is the in-game day the save was written on. Season is the lowercase
// name the save uses ("spring", "summer", "fall", "winter").
type GameDate struct {
	Season string
	Day    int
	Year   int
}

// DaysPerSeason is fixed by the game and is what turns a count of remaining
// growth days back into a calendar date.
const DaysPerSeason = 28

// Seasons in the order the year runs.
var Seasons = []string{"spring", "summer", "fall", "winter"}

// CropPlant is one crop or fruit tree growing on one tile, exactly as the save
// records it. Nothing here is interpreted — the arithmetic that turns it into
// "three days left" lives in the engine, so this stays a faithful reading of
// the file.
type CropPlant struct {
	Location string
	X, Y     int
	// InPot marks a crop growing in a garden pot rather than tilled ground.
	InPot   bool
	Watered bool
	// Outdoors and Greenhouse come from the location and together decide
	// whether the season applies to this crop at all.
	Outdoors   bool
	Greenhouse bool

	SeedID    string
	HarvestID string
	// PhaseDays is the crop's own growth schedule: days needed in each phase.
	// The game appends a 99999 sentinel for the harvestable phase, which is
	// kept as written so CurrentPhase indexes into it the way the game does.
	PhaseDays         []int
	CurrentPhase      int
	DayOfCurrentPhase int
	// FullyGrown is set by the game only on a crop that has been harvested at
	// least once and is regrowing, so it doubles as proof that a crop regrows.
	FullyGrown bool
	Dead       bool
	// Forage marks the wild-seed crops, which carry no growth schedule at all.
	Forage bool

	// Fruit trees are terrain features too, but carry a calendar-day maturity
	// countdown and a list of fruit instead of a crop phase schedule.
	FruitTree       bool
	TreeID          string
	DaysUntilMature int
	FruitCount      int
}

type vector2XML struct {
	X int `xml:"X"`
	Y int `xml:"Y"`
}

type terrainEntryXML struct {
	Key   vector2XML `xml:"key>Vector2"`
	Value terrainXML `xml:"value>TerrainFeature"`
}

// terrainXML covers every terrain feature; only HoeDirt carries a crop, and
// the rest decode to a zero value with a nil Crop, which is skipped.
type terrainXML struct {
	Type            string    `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	State           int       `xml:"state"`
	Crop            *cropXML  `xml:"crop"`
	GrowthStage     int       `xml:"growthStage"`
	TreeID          string    `xml:"treeId"`
	DaysUntilMature int       `xml:"daysUntilMature"`
	Fruit           []itemXML `xml:"fruit"`
	Stump           bool      `xml:"stump"`
}

type hoeDirtXML struct {
	State int      `xml:"state"`
	Crop  *cropXML `xml:"crop"`
}

type cropXML struct {
	PhaseDays         []int  `xml:"phaseDays>int"`
	CurrentPhase      int    `xml:"currentPhase"`
	DayOfCurrentPhase int    `xml:"dayOfCurrentPhase"`
	IndexOfHarvest    string `xml:"indexOfHarvest"`
	SeedIndex         string `xml:"seedIndex"`
	FullyGrown        bool   `xml:"fullGrown"`
	Dead              bool   `xml:"dead"`
	ForageCrop        bool   `xml:"forageCrop"`
}

// wateredState is HoeDirt.state when the tile has been watered today; 0 is dry.
const wateredState = 1

func addCrops(snap *Snapshot, loc *locationXML, name string) {
	for _, entry := range loc.TerrainFeatures {
		if entry.Value.Crop != nil {
			snap.Crops = append(snap.Crops, cropPlant(
				entry.Value.Crop, loc, name, entry.Key.X, entry.Key.Y,
				entry.Value.State == wateredState, false))
			continue
		}
		if entry.Value.Type == "FruitTree" {
			snap.Crops = append(snap.Crops, fruitTreePlant(
				&entry.Value, loc, name, entry.Key.X, entry.Key.Y))
		}
	}
	for _, entry := range loc.Objects {
		dirt := entry.Object.HoeDirt
		if dirt == nil || dirt.Crop == nil {
			continue
		}
		// A pot's own tile is the key in the object list; the soil inside it
		// has no coordinates of its own.
		snap.Crops = append(snap.Crops, cropPlant(
			dirt.Crop, loc, name, entry.Key.X, entry.Key.Y,
			dirt.State == wateredState, true))
	}
}

func fruitTreePlant(tree *terrainXML, loc *locationXML, location string, x, y int) CropPlant {
	harvestID := ""
	if len(tree.Fruit) > 0 {
		harvestID = strings.TrimPrefix(tree.Fruit[0].ItemID, "(O)")
	}
	return CropPlant{
		Location:        location,
		X:               x,
		Y:               y,
		Watered:         true,
		Outdoors:        loc.IsOutdoors,
		Greenhouse:      loc.IsGreenhouse,
		HarvestID:       harvestID,
		CurrentPhase:    tree.GrowthStage,
		Dead:            tree.Stump,
		FruitTree:       true,
		TreeID:          strings.TrimPrefix(tree.TreeID, "(O)"),
		DaysUntilMature: tree.DaysUntilMature,
		FruitCount:      len(tree.Fruit),
	}
}

func cropPlant(c *cropXML, loc *locationXML, location string, x, y int, watered, inPot bool) CropPlant {
	return CropPlant{
		Location:          location,
		X:                 x,
		Y:                 y,
		InPot:             inPot,
		Watered:           watered,
		Outdoors:          loc.IsOutdoors,
		Greenhouse:        loc.IsGreenhouse,
		SeedID:            strings.TrimPrefix(c.SeedIndex, "(O)"),
		HarvestID:         strings.TrimPrefix(c.IndexOfHarvest, "(O)"),
		PhaseDays:         c.PhaseDays,
		CurrentPhase:      c.CurrentPhase,
		DayOfCurrentPhase: c.DayOfCurrentPhase,
		FullyGrown:        c.FullyGrown,
		Dead:              c.Dead,
		Forage:            c.ForageCrop,
	}
}
