package engine

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/svendep/stardew-craftbook/data"
	"github.com/svendep/stardew-craftbook/internal/parser"
)

// CropData mirrors one entry of crops.json — see cmd/builddata/crops.go. It
// supplies the two things a save cannot: whether a crop regrows after harvest,
// and which seasons it survives.
type CropData struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Seasons    []string `json:"seasons,omitempty"`
	GrowthDays *int     `json:"growth_days,omitempty"`
	RegrowDays *int     `json:"regrow_days,omitempty"`
	SeedIDs    []string `json:"seed_ids,omitempty"`
	SeedNames  []string `json:"seed_names,omitempty"`
	WikiURL    string   `json:"wiki_url"`
}

func LoadCrops() (map[string]CropData, error) {
	var crops map[string]CropData
	if err := json.Unmarshal(data.CropsJSON, &crops); err != nil {
		return nil, err
	}
	return crops, nil
}

// GameDate is a day in the calendar, with a label the UI can print as-is.
type GameDate struct {
	Season string `json:"season"`
	Day    int    `json:"day"`
	Year   int    `json:"year"`
	Label  string `json:"label"`
}

// CropInstance is one planted tile: what it is, where it is, and how far it has
// left to go. DaysUntilHarvest counts *watered growth days*, not calendar days
// — see daysUntilHarvest.
type CropInstance struct {
	CropID        string `json:"crop_id"`
	SeedID        string `json:"seed_id,omitempty"`
	Name          string `json:"name"`
	Location      string `json:"location"`
	LocationLabel string `json:"location_label"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	InPot         bool   `json:"in_pot,omitempty"`

	Phase         int   `json:"phase"`
	PhaseCount    int   `json:"phase_count"`
	DayInPhase    int   `json:"day_in_phase"`
	PhaseSchedule []int `json:"phase_schedule,omitempty"`

	Watered    bool `json:"watered"`
	Dead       bool `json:"dead,omitempty"`
	Ready      bool `json:"ready"`
	FullyGrown bool `json:"fully_grown"`
	// DaysUntilHarvest is nil for a crop that is ready now, and for a dead one
	// that will never be ready.
	DaysUntilHarvest *int `json:"days_until_harvest"`
	// Progress is how much of this crop's growth is done, 0–1. It is nil when
	// the total is unknown, which never happens for a normal planted crop.
	Progress *float64 `json:"progress,omitempty"`

	// Regrows is nil when the dataset has never heard of this crop and the save
	// has not yet proved the answer by regrowing it once.
	Regrows    *bool    `json:"regrows"`
	RegrowDays *int     `json:"regrow_days,omitempty"`
	Seasons    []string `json:"seasons,omitempty"`
	// InSeason is nil when the crop's seasons are unknown. SeasonExempt marks
	// soil the season does not apply to, where InSeason is always true.
	InSeason     *bool  `json:"in_season"`
	SeasonExempt bool   `json:"season_exempt,omitempty"`
	WikiURL      string `json:"wiki_url,omitempty"`
	// Accessible is false for a crop in a place this save cannot reach yet —
	// see locationAccessible.
	Accessible bool `json:"accessible"`
}

// CropBucket is one "these many are this many days out" slice, used both for a
// group's progress bar and for the harvest timeline.
type CropBucket struct {
	Days  int    `json:"days"`
	Label string `json:"label"`
	Count int    `json:"count"`
	// Crops names what is in this bucket, most numerous first. It is only
	// populated on the timeline, where a bucket spans several crop types.
	Crops []CropCount `json:"crops,omitempty"`
	// Date is the calendar day this bucket lands on if every crop in it is
	// watered every day from now on.
	Date *GameDate `json:"date,omitempty"`
}

type CropCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CropGroup is one crop type in one location.
type CropGroup struct {
	Key           string `json:"key"`
	CropID        string `json:"crop_id"`
	Name          string `json:"name"`
	Location      string `json:"location"`
	LocationLabel string `json:"location_label"`

	Count     int `json:"count"`
	Ready     int `json:"ready"`
	Watered   int `json:"watered"`
	Unwatered int `json:"unwatered"`
	Dead      int `json:"dead,omitempty"`
	// OutOfSeason counts the crops here that will not grow in the current
	// season and are heading for a wilting rather than a harvest.
	OutOfSeason int `json:"out_of_season,omitempty"`

	// EarliestDays is 0 when something is ready now and nil when nothing in the
	// group can be harvested at all.
	EarliestDays *int      `json:"earliest_days"`
	EarliestDate *GameDate `json:"earliest_date,omitempty"`

	Regrows    *bool        `json:"regrows"`
	RegrowDays *int         `json:"regrow_days,omitempty"`
	Seasons    []string     `json:"seasons,omitempty"`
	WikiURL    string       `json:"wiki_url,omitempty"`
	Accessible bool         `json:"accessible"`
	Buckets    []CropBucket `json:"buckets"`
}

type CropsSummary struct {
	TotalGrowing int `json:"total_growing"`
	ReadyNow     int `json:"ready_now"`
	Unwatered    int `json:"unwatered"`
	Dead         int `json:"dead"`
	OutOfSeason  int `json:"out_of_season"`
	// Locked counts the crops standing in places this save cannot reach. They
	// are reported so a client can offer them, not so it must show them.
	Locked int `json:"locked"`
	// NextHarvestDays is the soonest a crop that is not already ready can come
	// in, and nil when nothing is still growing.
	NextHarvestDays *int      `json:"next_harvest_days"`
	NextHarvestDate *GameDate `json:"next_harvest_date,omitempty"`
}

type CropsView struct {
	Date      GameDate       `json:"date"`
	Summary   CropsSummary   `json:"summary"`
	Locations []CropLocation `json:"locations"`
	Crops     []CropInstance `json:"crops"`
	Groups    []CropGroup    `json:"groups"`
	Timeline  []CropBucket   `json:"timeline"`
}

type CropLocation struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Count      int    `json:"count"`
	Accessible bool   `json:"accessible"`
}

// boatFixedMail is the flag the game itself checks before letting the boat sail
// to Ginger Island.
const boatFixedMail = "willyBoatFixed"

// locationAccessible reports whether the save can actually reach a place.
//
// Every save carries the Ginger Island locations from the day it is created,
// wild ginger already growing in them, whether or not the boat has ever been
// repaired. Listing that alongside a player's own plantings tells them about
// crops they cannot walk to, so the island is gated on the same flag the game
// gates the boat on.
func locationAccessible(name string, mail map[string]bool) bool {
	if strings.HasPrefix(name, "Island") {
		return mail[boatFixedMail]
	}
	return true
}

// daysUntilHarvest reports how many *successful growth days* the crop still
// needs, and whether it is harvestable right now.
//
// A growth day is a day the crop is watered (or is raised, or is rained on).
// The game only advances a crop on such a day, so this is emphatically not a
// countdown of calendar days: skip the watering can and the number does not
// move. Nothing here can predict that, which is why the UI says so.
//
// The arithmetic mirrors Crop.newDay. A growing crop spends phaseDays[i] days
// in phase i and moves on; the final entry of phaseDays is a 99999 sentinel
// meaning "harvestable", so reaching that index is the finish line. A crop that
// has already been harvested once and regrows instead counts dayOfCurrentPhase
// back down to zero.
func daysUntilHarvest(p parser.CropPlant) (days int, ready bool) {
	final := len(p.PhaseDays) - 1
	// Forage crops (wild seeds) carry no schedule at all; the save only ever
	// holds them fully grown.
	if final < 0 {
		return 0, true
	}
	if p.CurrentPhase >= final {
		if p.FullyGrown && p.DayOfCurrentPhase > 0 {
			return p.DayOfCurrentPhase, false
		}
		return 0, true
	}
	remaining := -p.DayOfCurrentPhase
	for i := p.CurrentPhase; i < final; i++ {
		remaining += p.PhaseDays[i]
	}
	if remaining < 1 {
		// The save was written mid-phase-advance, or the schedule has a zero
		// entry the game would have skipped. Either way one more day settles it.
		remaining = 1
	}
	return remaining, false
}

// totalGrowthDays is the length of the run daysUntilHarvest is counting down,
// so the two together give a progress fraction.
func totalGrowthDays(p parser.CropPlant, data *CropData) int {
	if p.FullyGrown {
		if data != nil && data.RegrowDays != nil {
			return *data.RegrowDays
		}
		return 0
	}
	total := 0
	for i := 0; i < len(p.PhaseDays)-1; i++ {
		total += p.PhaseDays[i]
	}
	return total
}

// AddDays walks the calendar forward. Seasons are 28 days and the year rolls
// over after winter.
func (d GameDate) AddDays(n int) GameDate {
	index := 0
	for i, season := range parser.Seasons {
		if season == d.Season {
			index = i
			break
		}
	}
	day := d.Day + n
	year := d.Year
	for day > parser.DaysPerSeason {
		day -= parser.DaysPerSeason
		index++
		if index >= len(parser.Seasons) {
			index = 0
			year++
		}
	}
	return newGameDate(parser.Seasons[index], day, year)
}

func newGameDate(season string, day, year int) GameDate {
	return GameDate{Season: season, Day: day, Year: year, Label: dateLabel(season, day, year)}
}

func dateLabel(season string, day, year int) string {
	if season == "" || day == 0 {
		return ""
	}
	title := strings.ToUpper(season[:1]) + season[1:]
	return title + " " + strconv.Itoa(day) + ", Year " + strconv.Itoa(year)
}

// locationLabels renames the save's internal location keys to what the game
// calls those places on screen. Anything unlisted is split on its capitals,
// which turns IslandSouthEast into "Island South East" — imperfect, but
// readable, and correct for the modded locations we cannot know about.
var locationLabels = map[string]string{
	"Farm":            "Farm",
	"Greenhouse":      "Greenhouse",
	"IslandWest":      "Ginger Island Farm",
	"IslandNorth":     "Ginger Island North",
	"IslandSouth":     "Ginger Island South",
	"IslandEast":      "Ginger Island East",
	"IslandSouthEast": "Ginger Island Southeast",
	"FarmHouse":       "Farmhouse",
	"Cellar":          "Cellar",
	"Shed":            "Shed",
	"Big Shed":        "Big Shed",
	"Sunroom":         "Sunroom",
	"Town":            "Pelican Town",
	"Forest":          "Cindersap Forest",
	"Mountain":        "The Mountain",
	"Beach":           "The Beach",
	"Desert":          "The Desert",
	"BusStop":         "Bus Stop",
	"Backwoods":       "Backwoods",
	"Railroad":        "Railroad",
}

func locationLabel(name string) string {
	if name == "" {
		return "Unknown location"
	}
	if label, ok := locationLabels[name]; ok {
		return label
	}
	var out strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}

// seasonExempt reports whether the season simply does not apply where this crop
// is planted: the greenhouse, the Ginger Island farm, and any garden pot that
// is indoors all keep crops alive across a season change.
func seasonExempt(p parser.CropPlant) bool {
	return p.Greenhouse || p.Location == "Greenhouse" ||
		strings.HasPrefix(p.Location, "Island") ||
		(p.InPot && !p.Outdoors)
}

func bucketLabel(days int) string {
	switch days {
	case 0:
		return "Ready now"
	case 1:
		return "1 day"
	default:
		return strconv.Itoa(days) + " days"
	}
}

// BuildCrops turns the planted crops in a save into the crops view: every
// instance, the same instances grouped by location and crop, and a harvest
// timeline over all of them.
func BuildCrops(snap *parser.Snapshot, crops map[string]CropData) CropsView {
	today := newGameDate(snap.Date.Season, snap.Date.Day, snap.Date.Year)
	view := CropsView{Date: today, Crops: []CropInstance{}, Groups: []CropGroup{}, Timeline: []CropBucket{}, Locations: []CropLocation{}}

	type groupState struct {
		group   *CropGroup
		buckets map[int]int
	}
	groups := map[string]*groupState{}
	var groupOrder []string
	timeline := map[int]map[string]int{}
	timelineCounts := map[int]int{}
	locationCounts := map[string]int{}

	for _, p := range snap.Crops {
		var meta *CropData
		if d, ok := crops[p.HarvestID]; ok {
			meta = &d
		}
		instance := buildCropInstance(p, meta, today.Season)
		instance.Accessible = locationAccessible(p.Location, snap.MailReceived)
		view.Crops = append(view.Crops, instance)
		locationCounts[p.Location]++

		view.Summary.TotalGrowing++
		if instance.Ready {
			view.Summary.ReadyNow++
		}
		if !instance.Watered {
			view.Summary.Unwatered++
		}
		if instance.Dead {
			view.Summary.Dead++
		}
		if instance.InSeason != nil && !*instance.InSeason {
			view.Summary.OutOfSeason++
		}
		if !instance.Accessible {
			view.Summary.Locked++
		}

		key := p.Location + "\x00" + instance.CropID
		state, ok := groups[key]
		if !ok {
			state = &groupState{
				group: &CropGroup{
					Key: key, CropID: instance.CropID, Name: instance.Name,
					Location: instance.Location, LocationLabel: instance.LocationLabel,
					Regrows: instance.Regrows, RegrowDays: instance.RegrowDays,
					Seasons: instance.Seasons, WikiURL: instance.WikiURL,
					Accessible: instance.Accessible,
				},
				buckets: map[int]int{},
			}
			groups[key] = state
			groupOrder = append(groupOrder, key)
		}
		g := state.group
		g.Count++
		if instance.Ready {
			g.Ready++
		}
		if instance.Watered {
			g.Watered++
		} else {
			g.Unwatered++
		}
		if instance.Dead {
			g.Dead++
		}
		if instance.InSeason != nil && !*instance.InSeason {
			g.OutOfSeason++
		}
		// A crop with no reachable harvest belongs in no bucket: it is dead.
		days, harvestable := instanceDays(instance)
		if harvestable {
			state.buckets[days]++
			if g.EarliestDays == nil || days < *g.EarliestDays {
				d := days
				g.EarliestDays = &d
			}
			timelineCounts[days]++
			if timeline[days] == nil {
				timeline[days] = map[string]int{}
			}
			timeline[days][instance.Name]++
			if days > 0 && (view.Summary.NextHarvestDays == nil || days < *view.Summary.NextHarvestDays) {
				d := days
				view.Summary.NextHarvestDays = &d
			}
		}
	}

	if view.Summary.NextHarvestDays != nil {
		d := today.AddDays(*view.Summary.NextHarvestDays)
		view.Summary.NextHarvestDate = &d
	}

	for _, key := range groupOrder {
		state := groups[key]
		g := *state.group
		g.Buckets = sortedBuckets(state.buckets, today)
		if g.EarliestDays != nil && *g.EarliestDays > 0 {
			d := today.AddDays(*g.EarliestDays)
			g.EarliestDate = &d
		}
		view.Groups = append(view.Groups, g)
	}
	sort.SliceStable(view.Groups, func(i, j int) bool {
		a, b := view.Groups[i], view.Groups[j]
		if a.LocationLabel != b.LocationLabel {
			return a.LocationLabel < b.LocationLabel
		}
		return a.Name < b.Name
	})

	view.Timeline = sortedBuckets(timelineCounts, today)
	for i := range view.Timeline {
		for name, count := range timeline[view.Timeline[i].Days] {
			view.Timeline[i].Crops = append(view.Timeline[i].Crops, CropCount{Name: name, Count: count})
		}
		sort.Slice(view.Timeline[i].Crops, func(a, b int) bool {
			x, y := view.Timeline[i].Crops[a], view.Timeline[i].Crops[b]
			if x.Count != y.Count {
				return x.Count > y.Count
			}
			return x.Name < y.Name
		})
	}

	for name, count := range locationCounts {
		view.Locations = append(view.Locations, CropLocation{
			Name: name, Label: locationLabel(name), Count: count,
			Accessible: locationAccessible(name, snap.MailReceived),
		})
	}
	// Reachable places first, then by how much is growing there: a locked
	// location is never the one a player is looking for.
	sort.Slice(view.Locations, func(i, j int) bool {
		a, b := view.Locations[i], view.Locations[j]
		if a.Accessible != b.Accessible {
			return a.Accessible
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Label < b.Label
	})
	return view
}

// instanceDays flattens a crop to the bucket it belongs in. A dead crop is
// never harvestable, so it is left out of every countdown.
func instanceDays(c CropInstance) (int, bool) {
	if c.Dead {
		return 0, false
	}
	if c.Ready {
		return 0, true
	}
	if c.DaysUntilHarvest == nil {
		return 0, false
	}
	return *c.DaysUntilHarvest, true
}

func sortedBuckets(counts map[int]int, today GameDate) []CropBucket {
	buckets := make([]CropBucket, 0, len(counts))
	for days, count := range counts {
		bucket := CropBucket{Days: days, Label: bucketLabel(days), Count: count}
		if days > 0 {
			d := today.AddDays(days)
			bucket.Date = &d
		}
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Days < buckets[j].Days })
	return buckets
}

func buildCropInstance(p parser.CropPlant, meta *CropData, season string) CropInstance {
	days, ready := daysUntilHarvest(p)
	c := CropInstance{
		CropID:        p.HarvestID,
		SeedID:        p.SeedID,
		Name:          cropName(p, meta),
		Location:      p.Location,
		LocationLabel: locationLabel(p.Location),
		X:             p.X,
		Y:             p.Y,
		InPot:         p.InPot,
		Phase:         p.CurrentPhase,
		PhaseCount:    len(p.PhaseDays),
		DayInPhase:    p.DayOfCurrentPhase,
		PhaseSchedule: p.PhaseDays,
		Watered:       p.Watered,
		Dead:          p.Dead,
		Ready:         ready && !p.Dead,
		FullyGrown:    p.FullyGrown,
	}
	if !c.Ready && !p.Dead {
		d := days
		c.DaysUntilHarvest = &d
		if total := totalGrowthDays(p, meta); total > 0 {
			done := float64(total-d) / float64(total)
			if done < 0 {
				done = 0
			}
			c.Progress = &done
		}
	} else if c.Ready {
		full := 1.0
		c.Progress = &full
	}

	// The save proves a crop regrows the moment it has regrown once; the
	// dataset answers for everything that has not yet been harvested.
	switch {
	case meta != nil && meta.RegrowDays != nil:
		yes := true
		c.Regrows, c.RegrowDays = &yes, meta.RegrowDays
	case p.FullyGrown && !p.Forage:
		yes := true
		c.Regrows = &yes
	case meta != nil:
		no := false
		c.Regrows = &no
	}

	if meta != nil {
		c.Seasons = meta.Seasons
		c.WikiURL = meta.WikiURL
	}
	c.SeasonExempt = seasonExempt(p)
	switch {
	case c.SeasonExempt:
		yes := true
		c.InSeason = &yes
	case meta != nil && len(meta.Seasons) > 0 && season != "":
		in := false
		for _, s := range meta.Seasons {
			if s == season {
				in = true
				break
			}
		}
		c.InSeason = &in
	}
	return c
}

func cropName(p parser.CropPlant, meta *CropData) string {
	if meta != nil && meta.Name != "" {
		return meta.Name
	}
	if p.Forage {
		return "Wild crop"
	}
	if p.HarvestID != "" {
		return "Crop " + p.HarvestID
	}
	return "Unknown crop"
}
