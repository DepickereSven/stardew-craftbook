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

	logUnknown sync.Once

	mu       sync.RWMutex
	version  int
	snap     *parser.Snapshot
	lastMod  time.Time
	parseErr string
}

// unknownItemIDs lists item ids held in the save that no recipe or machine in
// the dataset refers to — mod leftovers, or items added by a newer game
// version. They are counted under their raw id and simply never match.
func unknownItemIDs(snap *parser.Snapshot, recipes []engine.Recipe, machines []engine.Machine) []string {
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
		if !known[id] {
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
	s := &Server{savePath: savePath, recipes: recipes, machines: machines, parseErr: detectErr}
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
		if unknown := unknownItemIDs(snap, s.recipes, s.machines); len(unknown) > 0 {
			shown := unknown
			if len(shown) > 10 {
				shown = shown[:10]
			}
			log.Printf("%d item ids in the save are not referenced by any recipe or machine "+
				"(counted, but never matched): %v...", len(unknown), shown)
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
	body := map[string]any{"version": version, "save_path": s.savePath, "recipes": avs}
	if parseErr != "" {
		// Serving the last good snapshot, but the newest read failed.
		body["error"] = parseErr
	}
	writeJSON(w, 200, body)
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
