package server

import (
	"os"
	"path/filepath"
	"testing"
)

type cropsResponse struct {
	Version int    `json:"version"`
	Error   string `json:"error"`
	Date    struct {
		Season string `json:"season"`
		Day    int    `json:"day"`
		Year   int    `json:"year"`
		Label  string `json:"label"`
	} `json:"date"`
	Summary struct {
		TotalGrowing    int  `json:"total_growing"`
		ReadyNow        int  `json:"ready_now"`
		Unwatered       int  `json:"unwatered"`
		NextHarvestDays *int `json:"next_harvest_days"`
	} `json:"summary"`
	Locations []struct {
		Name  string `json:"name"`
		Label string `json:"label"`
		Count int    `json:"count"`
	} `json:"locations"`
	Crops []struct {
		CropID           string   `json:"crop_id"`
		Name             string   `json:"name"`
		Location         string   `json:"location"`
		LocationLabel    string   `json:"location_label"`
		X                int      `json:"x"`
		Y                int      `json:"y"`
		Phase            int      `json:"phase"`
		DayInPhase       int      `json:"day_in_phase"`
		PhaseSchedule    []int    `json:"phase_schedule"`
		Watered          bool     `json:"watered"`
		Ready            bool     `json:"ready"`
		DaysUntilHarvest *int     `json:"days_until_harvest"`
		Regrows          *bool    `json:"regrows"`
		InSeason         *bool    `json:"in_season"`
		Seasons          []string `json:"seasons"`
	} `json:"crops"`
	Groups []struct {
		Name         string `json:"name"`
		Location     string `json:"location"`
		Count        int    `json:"count"`
		Ready        int    `json:"ready"`
		EarliestDays *int   `json:"earliest_days"`
		Buckets      []struct {
			Days  int    `json:"days"`
			Label string `json:"label"`
			Count int    `json:"count"`
		} `json:"buckets"`
	} `json:"groups"`
	Timeline []struct {
		Days  int    `json:"days"`
		Label string `json:"label"`
		Count int    `json:"count"`
		Crops []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"crops"`
	} `json:"timeline"`
}

func cropServer(t *testing.T) *Server {
	t.Helper()
	src, err := os.ReadFile("../parser/testdata/crops.xml")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "TestSave")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(path, "")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCropsEndpoint(t *testing.T) {
	s := cropServer(t)
	var body cropsResponse
	if code := getRaw(t, s, "/api/crops", &body); code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body.Version == 0 {
		t.Error("version not reported")
	}
	if body.Error != "" {
		t.Errorf("error = %q", body.Error)
	}
	if body.Date.Label != "Fall 7, Year 3" {
		t.Errorf("date = %+v", body.Date)
	}
	if body.Summary.TotalGrowing != 5 {
		t.Fatalf("total growing = %d, want 5", body.Summary.TotalGrowing)
	}
	if body.Summary.ReadyNow != 1 { // only the pumpkin; the regrowing corn is two days out
		t.Errorf("ready now = %d, want 1", body.Summary.ReadyNow)
	}
	if body.Summary.Unwatered != 1 {
		t.Errorf("unwatered = %d, want 1", body.Summary.Unwatered)
	}

	byName := map[string]int{}
	for _, c := range body.Crops {
		byName[c.Name]++
	}
	for _, want := range []string{"Cranberries", "Corn", "Parsnip", "Strawberry", "Pumpkin"} {
		if byName[want] != 1 {
			t.Errorf("crop %q appears %d times; have %v", want, byName[want], byName)
		}
	}

	for _, c := range body.Crops {
		if c.Name != "Cranberries" {
			continue
		}
		if c.LocationLabel != "Farm" || c.X != 50 || c.Y != 31 {
			t.Errorf("cranberry placement = %+v", c)
		}
		if c.DaysUntilHarvest == nil || *c.DaysUntilHarvest != 1 {
			t.Errorf("cranberry days = %v, want 1", c.DaysUntilHarvest)
		}
		if c.Regrows == nil || !*c.Regrows {
			t.Error("cranberries regrow")
		}
		if len(c.PhaseSchedule) != 6 || c.Phase != 4 || c.DayInPhase != 1 {
			t.Errorf("cranberry schedule = %+v", c)
		}
	}

	if len(body.Locations) != 2 {
		t.Errorf("locations = %+v, want Farm and Greenhouse", body.Locations)
	}
	if len(body.Groups) == 0 {
		t.Fatal("no groups")
	}
	for _, g := range body.Groups {
		if g.Count == 0 || len(g.Buckets) == 0 {
			t.Errorf("group without contents: %+v", g)
		}
	}
	if len(body.Timeline) == 0 || body.Timeline[0].Label != "Ready now" {
		t.Errorf("timeline = %+v", body.Timeline)
	}
}

// A greenhouse strawberry in fall is in season; the same crop outdoors would
// not be. The endpoint has to carry that distinction through.
func TestCropsEndpointReportsSeasonExemption(t *testing.T) {
	s := cropServer(t)
	var body cropsResponse
	getRaw(t, s, "/api/crops", &body)
	for _, c := range body.Crops {
		if c.Name != "Strawberry" {
			continue
		}
		if c.Location != "Greenhouse" {
			t.Fatalf("strawberry location = %q", c.Location)
		}
		if c.InSeason == nil || !*c.InSeason {
			t.Errorf("greenhouse strawberry marked out of season: %v", c.InSeason)
		}
	}
}

// With no save at all the endpoint still answers, the same way /api/inventory
// does, so the UI has one shape to render.
func TestCropsEndpointWithoutSave(t *testing.T) {
	s, err := New("", "no save found")
	if err != nil {
		t.Fatal(err)
	}
	var body cropsResponse
	if code := getRaw(t, s, "/api/crops", &body); code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body.Error == "" {
		t.Error("missing save not reported")
	}
	if len(body.Crops) != 0 {
		t.Errorf("crops = %+v, want none", body.Crops)
	}
}
