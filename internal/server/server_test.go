package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
