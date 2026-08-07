// Package engine decides what a save can craft right now, and how to get
// closer to what it cannot.
package engine

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/svendep/stardew-craftbook/data"
	"github.com/svendep/stardew-craftbook/internal/parser"
)

// These mirror the JSON written by cmd/builddata. The tags must match exactly:
// a mismatch decodes to a zero value rather than an error.
type Ingredient struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Qty      int    `json:"qty"`
	Category bool   `json:"category,omitempty"`
}

type Recipe struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Ingredients []Ingredient `json:"ingredients"`
	OutputQty   int          `json:"output_qty"`
	Unlock      string       `json:"unlock"`
	WikiURL     string       `json:"wiki_url"`
}

type Machine struct {
	Machine string       `json:"machine"`
	Inputs  []Ingredient `json:"inputs"`
	Output  Ingredient   `json:"output"`
	Minutes int          `json:"minutes"`
}

// Buff is one effect an item grants when eaten or drunk. Value is a signed
// string as the wiki writes it ("+30", "-1") and may be empty for effects with
// no magnitude, such as Tipsy.
type Buff struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// Item is the per-item metadata: what it sells for, what it does when eaten,
// and how long a machine takes to make it. Every numeric field is a pointer
// because "unknown" and "zero" mean different things — an item with no
// recorded price must not look free.
type Item struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	SellPrice         *int   `json:"sell_price,omitempty"`
	SellPriceNote     string `json:"sell_price_note,omitempty"`
	Edibility         *int   `json:"edibility,omitempty"`
	Buffs             []Buff `json:"buffs,omitempty"`
	BuffDuration      string `json:"buff_duration,omitempty"`
	ProcessingMinutes *int   `json:"processing_minutes,omitempty"`
	WikiURL           string `json:"wiki_url"`
}

type State string

const (
	Craftable State = "craftable"
	Partial   State = "partial"
	FarOff    State = "far_off"
)

type MissingItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Need    int    `json:"need"`
	Have    int    `json:"have"`
	WikiURL string `json:"wiki_url"`
}

type Availability struct {
	Recipe  Recipe        `json:"recipe"`
	Learned bool          `json:"learned"`
	State   State         `json:"state"`
	Missing []MissingItem `json:"missing"`
}

func LoadData() ([]Recipe, []Machine, error) {
	var recipes []Recipe
	if err := json.Unmarshal(data.RecipesJSON, &recipes); err != nil {
		return nil, nil, err
	}
	var machines []Machine
	if err := json.Unmarshal(data.MachinesJSON, &machines); err != nil {
		return nil, nil, err
	}
	return recipes, machines, nil
}

// LoadItems decodes the embedded item metadata, keyed by item id.
func LoadItems() (map[string]Item, error) {
	var items map[string]Item
	if err := json.Unmarshal(data.ItemsJSON, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// countOf reports how much of an ingredient the save holds. A category
// ingredient is satisfied by any owned item in that category.
func countOf(snap *parser.Snapshot, ing Ingredient) int {
	if !ing.Category {
		if ing.ID == "" {
			return 0
		}
		return snap.Items[ing.ID]
	}
	// Some generated machine inputs describe their category in prose and carry
	// no id; nothing in the save can match those.
	cat, err := strconv.Atoi(ing.ID)
	if err != nil {
		return 0
	}
	total := 0
	for id, n := range snap.Items {
		if snap.Categories[id] == cat {
			total += n
		}
	}
	return total
}

func wikiURL(name string) string {
	return "https://stardewvalleywiki.com/" + strings.ReplaceAll(name, " ", "_")
}

// Evaluate reports, for each recipe, whether the save can make it now.
func Evaluate(snap *parser.Snapshot, recipes []Recipe) []Availability {
	out := make([]Availability, 0, len(recipes))
	for _, r := range recipes {
		av := Availability{Recipe: r, Missing: []MissingItem{}}
		if r.Type == "cooking" {
			av.Learned = snap.CookingLearned[r.Key]
		} else {
			av.Learned = snap.CraftingLearned[r.Key]
		}
		satisfied := 0
		for _, ing := range r.Ingredients {
			have := countOf(snap, ing)
			if have >= ing.Qty {
				satisfied++
			} else {
				av.Missing = append(av.Missing, MissingItem{
					ID: ing.ID, Name: ing.Name, Need: ing.Qty, Have: have, WikiURL: wikiURL(ing.Name),
				})
			}
		}
		switch {
		case len(av.Missing) == 0:
			av.State = Craftable
		case satisfied > 0:
			av.State = Partial
		default:
			av.State = FarOff
		}
		out = append(out, av)
	}
	return out
}
