package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// getRaw is the array-friendly sibling of get: /api/item/{id} returns an object
// but its used_in is a list, and decoding straight into a typed shape keeps the
// assertions readable.
func getRaw(t *testing.T, s *Server, path string, into any) int {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if into != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
			t.Fatalf("%s: non-JSON body: %s", path, rec.Body.String())
		}
	}
	return rec.Code
}

type invResponse struct {
	Version int    `json:"version"`
	Error   string `json:"error"`
	Items   []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Count       int    `json:"count"`
		SellPrice   *int   `json:"sell_price"`
		StackValue  *int   `json:"stack_value"`
		RecipeCount int    `json:"recipe_count"`
	} `json:"items"`
}

func TestInventoryEndpoint(t *testing.T) {
	s := testServer(t)
	var body invResponse
	if code := getRaw(t, s, "/api/inventory", &body); code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body.Version == 0 {
		t.Error("version not reported")
	}
	if len(body.Items) == 0 {
		t.Fatal("no items returned")
	}
	for _, it := range body.Items {
		if it.Name == "" {
			t.Errorf("item %s has no name", it.ID)
		}
		if it.Count <= 0 {
			t.Errorf("item %s has count %d", it.ID, it.Count)
		}
		// Absent and zero are different facts; a priceless item must carry
		// neither number rather than a zero.
		if it.SellPrice == nil && it.StackValue != nil {
			t.Errorf("item %s has a stack value but no price", it.ID)
		}
		if it.SellPrice != nil && (it.StackValue == nil || *it.StackValue != *it.SellPrice*it.Count) {
			t.Errorf("item %s stack value does not match price × count", it.ID)
		}
	}
}

func TestInventorySortedByValueDescending(t *testing.T) {
	s := testServer(t)
	var body invResponse
	getRaw(t, s, "/api/inventory", &body)
	prev := -1
	for _, it := range body.Items {
		v := 0
		if it.StackValue != nil {
			v = *it.StackValue
		}
		if prev >= 0 && v > prev {
			t.Fatalf("item %s (%d g) sorts after a %d g stack", it.ID, v, prev)
		}
		prev = v
	}
}

type detailResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Count  int    `json:"count"`
	UsedIn []struct {
		RecipeKey   string `json:"recipe_key"`
		Verdict     string `json:"verdict"`
		Delta       *int   `json:"delta"`
		InputCost   *int   `json:"input_cost"`
		MaxMakeable int    `json:"max_makeable"`
	} `json:"used_in"`
}

func TestItemDetailEndpoint(t *testing.T) {
	s := testServer(t)
	var body detailResponse
	if code := getRaw(t, s, "/api/item/388", &body); code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body.Name != "Wood" || body.Count != 75 {
		t.Errorf("got %s ×%d, want Wood ×75", body.Name, body.Count)
	}
	if len(body.UsedIn) == 0 {
		t.Fatal("Wood feeds no recipes")
	}
	valid := map[string]bool{"profit": true, "loss": true, "not_for_sale": true, "unknown": true}
	for _, u := range body.UsedIn {
		if !valid[u.Verdict] {
			t.Errorf("%s: unknown verdict %q", u.RecipeKey, u.Verdict)
		}
		// A delta is only meaningful when the cost behind it is known.
		if u.Delta != nil && u.InputCost == nil {
			t.Errorf("%s: delta without an input cost", u.RecipeKey)
		}
		if u.Verdict == "unknown" && u.Delta != nil {
			t.Errorf("%s: unknown verdict still carries a delta", u.RecipeKey)
		}
		if u.Verdict == "not_for_sale" && (u.Delta != nil || u.InputCost != nil) {
			t.Errorf("%s: unsellable output carries numbers", u.RecipeKey)
		}
	}
}

func TestItemDetailUnknownID(t *testing.T) {
	s := testServer(t)
	if code := getRaw(t, s, "/api/item/does-not-exist", nil); code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

func TestInventoryAndItemWithoutSave(t *testing.T) {
	s, err := New("", "no save found")
	if err != nil {
		t.Fatal(err)
	}
	var body invResponse
	if code := getRaw(t, s, "/api/inventory", &body); code != 200 {
		t.Errorf("inventory status = %d, want 200", code)
	}
	if body.Error == "" {
		t.Error("inventory did not report the detection error")
	}
	if len(body.Items) != 0 {
		t.Errorf("inventory returned %d items without a save", len(body.Items))
	}
	if code := getRaw(t, s, "/api/item/388", nil); code != 503 {
		t.Errorf("item status = %d, want 503", code)
	}
}

// /api/items (the static dataset) and /api/item/{id} must not shadow one another.
func TestItemsAndItemRoutesCoexist(t *testing.T) {
	s := testServer(t)
	var static map[string]any
	if code := getRaw(t, s, "/api/items", &static); code != 200 {
		t.Fatalf("/api/items status = %d", code)
	}
	if len(static) == 0 {
		t.Error("/api/items returned nothing")
	}
	if _, isDetail := static["used_in"]; isDetail {
		t.Error("/api/items was served by the item-detail handler")
	}
}
