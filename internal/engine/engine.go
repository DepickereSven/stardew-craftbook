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

// MachineAvailability is a machine conversion the current inventory can run.
// MaxRuns is constrained by every input, including category inputs such as
// any fruit or any fish.
type MachineAvailability struct {
	Machine
	MaxRuns int `json:"max_runs"`
	// Missing is what the inventory is short of, and is only ever populated for
	// a conversion that cannot run: a machine listed so it can be found by
	// search has to say what it is waiting for.
	Missing    []MissingItem   `json:"missing,omitempty"`
	Profits    []QualityProfit `json:"profits,omitempty"`
	Throughput *Throughput     `json:"throughput,omitempty"`
}

// Throughput is the whole-stack economics of one machine conversion for the
// selected item: every quality the player holds, converted at its own rate and
// summed. Like QualityProfit it is attached only to an item-detail response,
// since only there is there a selected input to convert.
//
// Nil means the arithmetic could not be trusted — an unpriced quality, no full
// run, or a machine with no recorded time — and the UI says so rather than
// printing a confident zero.
type Throughput struct {
	// Input names the ingredient entry the selected item matched, so a card can
	// say whose runs these are when the machine accepts a whole category.
	Input    string `json:"input"`
	Runs     int    `json:"runs"`
	Consumed int    `json:"consumed"`
	// Collected is what the finished output sells for and RawValue is what the
	// consumed input would have sold for untouched. TotalProfit is the
	// difference. All three are reported because the difference alone reads as
	// the total takings and makes a real gain look like a shortfall.
	Collected    int     `json:"collected"`
	RawValue     int     `json:"raw_value"`
	TotalProfit  int     `json:"total_profit"`
	ProfitPerRun int     `json:"profit_per_run"`
	GPerMachMin  float64 `json:"g_per_machine_min"`
	GPerInputMin float64 `json:"g_per_input_min"`
}

// QualityProfit is a processing gain for one quality of the selected input.
// It is attached only to an item-detail response; the recipes overview has no
// selected input item from which to calculate an honest value.
type QualityProfit struct {
	Quality int  `json:"quality"`
	Delta   *int `json:"delta,omitempty"`
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
	return recipes, dropAlternativeVariants(machines), nil
}

// dropAlternativeVariants removes the machine entries that list a conversion's
// *alternative* inputs as if one run consumed them all — the wiki writes "milk
// or large milk" as two ingredient rows, which decodes as a cheese press
// demanding both. Such an entry is always a superset of a real one for the same
// machine and output, so it is recognised by that and dropped: left in, it
// prices a run against ingredients no run consumes.
func dropAlternativeVariants(machines []Machine) []Machine {
	names := func(m Machine) map[string]bool {
		set := make(map[string]bool, len(m.Inputs))
		for _, in := range m.Inputs {
			set[in.Name] = true
		}
		return set
	}
	out := make([]Machine, 0, len(machines))
	for i, machine := range machines {
		superset := false
		for j, other := range machines {
			if i == j || other.Machine != machine.Machine || other.Output.ID != machine.Output.ID {
				continue
			}
			if len(other.Inputs) >= len(machine.Inputs) {
				continue
			}
			mine, theirs := names(machine), names(other)
			covered := true
			for name := range theirs {
				if !mine[name] {
					covered = false
					break
				}
			}
			if covered && len(theirs) < len(mine) {
				superset = true
				break
			}
		}
		if !superset {
			out = append(out, machine)
		}
	}
	return out
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
		if n, ok := snap.Items[ing.ID]; ok && ing.ID != "" {
			return n
		}
		// Ids are learned from recipe data, so an item no recipe mentions —
		// wheat, milk, wool — reaches a machine input by name alone. Without
		// this the keg could never see the wheat that makes beer.
		return countByName(snap, ing.Name)
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

// countByName sums every owned id the save calls by this name.
func countByName(snap *parser.Snapshot, name string) int {
	if name == "" {
		return 0
	}
	total := 0
	for id, owned := range snap.Names {
		if strings.EqualFold(owned, name) {
			total += snap.Items[id]
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

// AvailableMachines lists the individual processing jobs the inventory can
// currently supply. Unlike recipe planning, these are useful on their own:
// wine, dried fruit, and smoked fish are not ingredients in a craft/cook
// recipe, so they would otherwise have no route into the UI.
func AvailableMachines(snap *parser.Snapshot, machines []Machine) []MachineAvailability {
	out := make([]MachineAvailability, 0, len(machines))
	for _, machine := range AllMachines(snap, machines) {
		if machine.MaxRuns > 0 {
			out = append(out, machine)
		}
	}
	return out
}

// AllMachines is every conversion in the dataset with the runs the inventory
// could supply, zero included. A machine the player cannot feed yet still has
// to be reachable — beer is a keg conversion whether or not there is wheat in
// the chest — so search covers this list while the ready-to-run view filters it.
func AllMachines(snap *parser.Snapshot, machines []Machine) []MachineAvailability {
	out := make([]MachineAvailability, 0, len(machines))
	for _, machine := range machines {
		av := MachineAvailability{Machine: machine, MaxRuns: -1}
		for _, in := range machine.Inputs {
			if in.Qty <= 0 {
				continue
			}
			have := countOf(snap, in)
			if runs := have / in.Qty; av.MaxRuns == -1 || runs < av.MaxRuns {
				av.MaxRuns = runs
			}
			if have < in.Qty {
				av.Missing = append(av.Missing, MissingItem{
					ID: in.ID, Name: in.Name, Need: in.Qty, Have: have, WikiURL: wikiURL(in.Name),
				})
			}
		}
		if av.MaxRuns < 0 {
			av.MaxRuns = 0
		}
		if av.MaxRuns > 0 {
			// A short input and a runnable machine cannot both be true; the
			// list is only a to-do for a conversion that cannot run.
			av.Missing = nil
		}
		out = append(out, av)
	}
	return out
}
