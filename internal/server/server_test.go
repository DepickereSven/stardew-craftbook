package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/svendep/stardew-craftbook/internal/engine"
	"github.com/svendep/stardew-craftbook/internal/parser"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	src, err := os.ReadFile("../parser/testdata/player_only.xml")
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

func get(t *testing.T, s *Server, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: non-JSON body: %s", path, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("%s: Content-Type = %q", path, ct)
	}
	return rec.Code, body
}

func TestVersionEndpoint(t *testing.T) {
	code, body := get(t, testServer(t), "/api/version")
	if code != 200 || body["version"].(float64) != 1 {
		t.Errorf("code=%d body=%v", code, body)
	}
}

func TestStateEndpoint(t *testing.T) {
	code, body := get(t, testServer(t), "/api/state")
	if code != 200 {
		t.Errorf("code=%d", code)
	}
	if _, ok := body["recipes"].([]any); !ok {
		t.Errorf("no recipes array: %v", body)
	}
}

func TestPlanEndpointUnknownKey(t *testing.T) {
	code, _ := get(t, testServer(t), "/api/plan/NotARecipe")
	if code != 404 {
		t.Errorf("code=%d, want 404", code)
	}
}

func TestPlanEndpointKnownKey(t *testing.T) {
	code, body := get(t, testServer(t), "/api/plan/Gate")
	if code != 200 || body["recipe_key"] != "Gate" {
		t.Errorf("code=%d body=%v", code, body)
	}
}

// Recipe keys contain spaces and punctuation, so the path segment arrives
// percent-encoded and must be decoded before lookup.
func TestPlanEndpointEncodedKey(t *testing.T) {
	code, body := get(t, testServer(t), "/api/plan/Wood%20Fence")
	if code != 200 || body["recipe_key"] != "Wood Fence" {
		t.Errorf("code=%d body=%v", code, body)
	}
}

func TestIndexServed(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	testServer(t).Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("index code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Stardew Craftbook") {
		t.Error("placeholder page not served")
	}
}

func TestUnknownPathIs404(t *testing.T) {
	req := httptest.NewRequest("GET", "/nope", nil)
	rec := httptest.NewRecorder()
	testServer(t).Handler().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("code=%d, want 404", rec.Code)
	}
}

func TestNoSaveFoundStillServes(t *testing.T) {
	s, err := New("", "no save found; tried: /a, /b")
	if err != nil {
		t.Fatal(err)
	}
	code, body := get(t, s, "/api/state")
	if code != 200 {
		t.Errorf("code=%d", code)
	}
	if body["error"] != "no save found; tried: /a, /b" {
		t.Errorf("error payload missing: %v", body)
	}
}

// With no snapshot there is nothing to plan against, but the server must say
// so rather than pretend the recipe is unknown.
func TestPlanWithoutSnapshot(t *testing.T) {
	s, err := New("", "no save")
	if err != nil {
		t.Fatal(err)
	}
	code, _ := get(t, s, "/api/plan/Gate")
	if code != 503 {
		t.Errorf("code=%d, want 503", code)
	}
}

// A save that fails to parse must not destroy the last good snapshot.
func TestBadParseKeepsLastGoodSnapshot(t *testing.T) {
	src, err := os.ReadFile("../parser/testdata/player_only.xml")
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
	_, before := get(t, s, "/api/state")
	countBefore := len(before["recipes"].([]any))

	// simulate the game writing a half-flushed save
	if err := os.WriteFile(path, []byte("<SaveGame><trunc"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.refresh()

	code, after := get(t, s, "/api/state")
	if code != 200 {
		t.Errorf("code=%d", code)
	}
	if got := len(after["recipes"].([]any)); got != countBefore {
		t.Errorf("recipes went from %d to %d — last good snapshot lost", countBefore, got)
	}
	if after["error"] == "" || after["error"] == nil {
		t.Error("parse error not reported")
	}
}

// Spec §8: items the dataset never mentions are still counted, but reported so
// a modded or newer save is visible.
func TestUnknownItemIDsReported(t *testing.T) {
	recipes := []engine.Recipe{{Key: "R", Ingredients: []engine.Ingredient{{ID: "388"}}}}
	machines := []engine.Machine{{Machine: "M",
		Inputs: []engine.Ingredient{{ID: "380"}}, Output: engine.Ingredient{ID: "335"}}}
	snap := &parser.Snapshot{Items: map[string]int{
		"388": 1, "380": 1, "335": 1, "SomeModItem": 1, "9999": 1,
	}}
	got := unknownItemIDs(snap, recipes, machines)
	if len(got) != 2 || got[0] != "9999" || got[1] != "SomeModItem" {
		t.Errorf("unknown ids = %v, want [9999 SomeModItem]", got)
	}
}

func TestUnknownItemIDsEmptyWhenAllKnown(t *testing.T) {
	recipes := []engine.Recipe{{Key: "R", Ingredients: []engine.Ingredient{{ID: "388"}}}}
	snap := &parser.Snapshot{Items: map[string]int{"388": 1}}
	if got := unknownItemIDs(snap, recipes, nil); len(got) != 0 {
		t.Errorf("unknown ids = %v, want none", got)
	}
}

// Item metadata is static reference data, served on its own endpoint so the
// already-large /api/state does not have to carry it.
func TestItemsEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/items", nil)
	rec := httptest.NewRecorder()
	testServer(t).Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var items map[string]engine.Item
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("non-JSON body: %v", err)
	}
	if len(items) < 300 {
		t.Errorf("only %d items served", len(items))
	}
	tea, ok := items["614"]
	if !ok {
		t.Fatal("Green Tea (614) not served")
	}
	if tea.ProcessingMinutes == nil || *tea.ProcessingMinutes != 180 {
		t.Errorf("processing_minutes = %v", tea.ProcessingMinutes)
	}
	if tea.SellPrice == nil || *tea.SellPrice != 100 {
		t.Errorf("sell_price = %v", tea.SellPrice)
	}
	if len(tea.Buffs) == 0 {
		t.Error("buffs not served")
	}
}

// Reference data does not depend on the save, so it must answer even when no
// save was found.
func TestItemsEndpointWorksWithoutSave(t *testing.T) {
	s, err := New("", "no save")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/items", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code=%d, want 200", rec.Code)
	}
}
