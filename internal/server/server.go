// Package server exposes the engine over HTTP and keeps its snapshot in step
// with the save file on disk.
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/svendep/stardew-craftbook/internal/engine"
	"github.com/svendep/stardew-craftbook/internal/parser"
	"github.com/svendep/stardew-craftbook/web"
)

type Server struct {
	savePath string
	recipes  []engine.Recipe
	machines []engine.Machine
	items    map[string]engine.Item
	itemIdx  *engine.ItemIndex
	crops    map[string]engine.CropData

	logUnknown sync.Once

	mu       sync.RWMutex
	version  int
	snap     *parser.Snapshot
	lastMod  time.Time
	parseErr string
}

// unknownItemIDs lists held items that are neither relevant to planning nor
// represented by item metadata. This keeps decorative items out of the warning
// while surfacing modded or newer-game items the app cannot describe.
func unknownItemIDs(snap *parser.Snapshot, recipes []engine.Recipe, machines []engine.Machine, itemIdx *engine.ItemIndex) []string {
	known := map[string]bool{}
	for _, r := range recipes {
		for _, ing := range r.Ingredients {
			known[ing.ID] = true
		}
	}
	for _, m := range machines {
		for _, in := range m.Inputs {
			known[in.ID] = true
		}
		known[m.Output.ID] = true
	}
	var out []string
	for id := range snap.Items {
		if known[id] {
			continue
		}
		if _, ok := itemIdx.Lookup(id, snap.Names[id]); !ok {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// New builds a server. savePath may be empty when detection failed; detectErr
// then carries the human-readable explanation (paths tried plus the
// --save-path hint) that /api/state exposes, per spec §8.
func New(savePath, detectErr string) (*Server, error) {
	recipes, machines, err := engine.LoadData()
	if err != nil {
		return nil, err
	}
	items, err := engine.LoadItems()
	if err != nil {
		return nil, err
	}
	crops, err := engine.LoadCrops()
	if err != nil {
		return nil, err
	}
	s := &Server{
		savePath: savePath,
		recipes:  recipes,
		machines: machines,
		items:    items,
		itemIdx:  engine.NewItemIndex(items),
		crops:    crops,
		parseErr: detectErr,
	}
	if savePath != "" {
		s.refresh()
	}
	return s, nil
}

// refresh re-reads the save. A failure (typically the game mid-write) keeps
// the last good snapshot and is reported through /api/state.
func (s *Server) refresh() {
	snap, err := parser.ParseFile(s.savePath)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.parseErr = err.Error()
		return
	}
	s.parseErr = ""
	s.snap = snap
	s.version++

	// Spec §8: report items the dataset does not know about, once, so a game
	// update or a modded save is visible without spamming every poll.
	s.logUnknown.Do(func() {
		if unknown := unknownItemIDs(snap, s.recipes, s.machines, s.itemIdx); len(unknown) > 0 {
			shown := unknown
			if len(shown) > 10 {
				shown = shown[:10]
			}
			log.Printf("%d item ids in the save are neither described by item metadata nor used in planning: %v...", len(unknown), shown)
		}
	})
}

// StartPolling re-reads the save whenever its modification time changes.
func (s *Server) StartPolling(interval time.Duration) {
	go func() {
		for range time.Tick(interval) {
			fi, err := os.Stat(s.savePath)
			if err != nil {
				continue
			}
			s.mu.RLock()
			changed := fi.ModTime().After(s.lastMod)
			s.mu.RUnlock()
			if changed {
				s.mu.Lock()
				s.lastMod = fi.ModTime()
				s.mu.Unlock()
				s.refresh()
			}
		}
	}()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/items", s.handleItems)
	mux.HandleFunc("GET /api/inventory", s.handleInventory)
	mux.HandleFunc("GET /api/crops", s.handleCrops)
	mux.HandleFunc("GET /api/item/", s.handleItemDetail)
	mux.HandleFunc("GET /api/plan/", s.handlePlan)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(web.IndexHTML)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, 200, map[string]int{"version": s.version})
}

// handleItems serves the static item reference: sell price, edibility, buffs
// and machine processing time, keyed by item id. It is baked into the binary
// and never changes at runtime, so it is safe to fetch once and cache.
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	writeJSON(w, 200, s.items)
}

// stateRecipe is one /api/state entry: availability plus the sale economics
// of one crafting, so the recipe view can answer "is making this worth it"
// without a second request.
type stateRecipe struct {
	engine.Availability
	Economics engine.Economics `json:"economics"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	snap, version, parseErr := s.snap, s.version, s.parseErr
	s.mu.RUnlock()
	if snap == nil {
		writeJSON(w, 200, map[string]any{
			"version": version, "save_path": s.savePath, "error": parseErr, "recipes": []any{},
		})
		return
	}
	avs := engine.EvaluateWithPlanner(snap, s.recipes, s.machines)
	recipes := make([]stateRecipe, len(avs))
	for i, av := range avs {
		recipes[i] = stateRecipe{av, engine.RecipeEconomics(s.itemIdx, av.Recipe)}
	}
	body := map[string]any{
		"version": version, "save_path": s.savePath, "recipes": recipes,
		"machines": engine.AvailableMachines(snap, s.machines),
		// Every conversion, runnable or not, so a search for "beer" finds the
		// keg that makes it before there is any wheat to put in.
		"all_machines": engine.AllMachines(snap, s.machines),
	}
	if parseErr != "" {
		// Serving the last good snapshot, but the newest read failed.
		body["error"] = parseErr
	}
	writeJSON(w, 200, body)
}

// handleInventory serves everything the save holds, most valuable stack first.
// It carries the same error semantics as /api/state: an error with an empty
// list means no save, an error alongside a populated one means this is the last
// good snapshot.
func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	snap, version, parseErr := s.snap, s.version, s.parseErr
	s.mu.RUnlock()
	if snap == nil {
		writeJSON(w, 200, map[string]any{"version": version, "error": parseErr, "items": []any{}})
		return
	}
	body := map[string]any{"version": version, "items": engine.BuildInventory(snap, s.itemIdx, s.recipes)}
	if parseErr != "" {
		body["error"] = parseErr
	}
	writeJSON(w, 200, body)
}

// handleCrops serves every crop planted in the save, grouped by location and
// crop type and laid out on a harvest timeline. Error semantics match
// /api/inventory: an error with no crops means no save could be read, an error
// alongside crops means these came from the last good snapshot.
func (s *Server) handleCrops(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	snap, version, parseErr := s.snap, s.version, s.parseErr
	s.mu.RUnlock()
	if snap == nil {
		writeJSON(w, 200, map[string]any{"version": version, "error": parseErr, "crops": []any{}})
		return
	}
	view := engine.BuildCrops(snap, s.crops)
	body := map[string]any{
		"version": version, "date": view.Date, "summary": view.Summary,
		"locations": view.Locations, "crops": view.Crops,
		"groups": view.Groups, "timeline": view.Timeline,
	}
	if parseErr != "" {
		body["error"] = parseErr
	}
	writeJSON(w, 200, body)
}

// handleItemDetail serves one owned item and the recipes it feeds, with the
// economics of each. Item ids can contain characters needing encoding
// ("Dish o' The Sea" style keys do; ids such as MoreWalls:11 do too).
func (s *Server) handleItemDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/item/")
	if decoded, err := url.PathUnescape(id); err == nil {
		id = decoded
	}
	s.mu.RLock()
	snap := s.snap
	s.mu.RUnlock()
	if snap == nil {
		writeJSON(w, 503, map[string]string{"error": "no save loaded"})
		return
	}
	avail := engine.EvaluateWithPlanner(snap, s.recipes, s.machines)
	detail, ok := engine.BuildItemDetail(snap, s.itemIdx, s.recipes, s.machines, avail, id)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "unknown item"})
		return
	}
	writeJSON(w, 200, detail)
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/plan/")
	// Recipe keys contain spaces and apostrophes, so the segment is encoded.
	if decoded, err := url.PathUnescape(key); err == nil {
		key = decoded
	}
	s.mu.RLock()
	snap := s.snap
	s.mu.RUnlock()
	if snap == nil {
		writeJSON(w, 503, map[string]string{"error": "no save loaded"})
		return
	}
	res, err := engine.PlanRecipe(snap, s.recipes, s.machines, key)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "unknown recipe"})
		return
	}
	writeJSON(w, 200, res)
}
