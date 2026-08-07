package engine

import (
	"fmt"
	"strconv"

	"github.com/svendep/stardew-craftbook/internal/parser"
)

type PlanStep struct {
	Machine string       `json:"machine"`
	Inputs  []Ingredient `json:"inputs"` // quantities are per run
	Output  Ingredient   `json:"output"`
	Runs    int          `json:"runs"`
	Minutes int          `json:"minutes"`
}

type PlanResult struct {
	RecipeKey    string        `json:"recipe_key"`
	Feasible     bool          `json:"feasible"`
	Steps        []PlanStep    `json:"steps"`
	StillMissing []MissingItem `json:"still_missing"`
}

// maxDepth caps how many machine conversions may be chained. Three covers the
// real chains (ore -> bar, wood -> coal -> bar) without letting a pathological
// dataset explore forever.
const maxDepth = 3

// PlanRecipe works out how to get from what the save holds to one crafting of
// the named recipe, producing missing intermediates through machines where it
// can. Returns an error only when the recipe key is unknown.
//
// Everything is spent from a single budget, so the same coal cannot satisfy
// two different steps.
func PlanRecipe(snap *parser.Snapshot, recipes []Recipe, machines []Machine, key string) (PlanResult, error) {
	var recipe *Recipe
	for i := range recipes {
		if recipes[i].Key == key {
			recipe = &recipes[i]
			break
		}
	}
	if recipe == nil {
		return PlanResult{}, fmt.Errorf("unknown recipe %q", key)
	}

	budget := make(map[string]int, len(snap.Items))
	for id, n := range snap.Items {
		budget[id] = n
	}
	res := PlanResult{RecipeKey: key, Feasible: true, Steps: []PlanStep{}, StillMissing: []MissingItem{}}
	for _, ing := range recipe.Ingredients {
		need := ing.Qty
		if ing.Category {
			need -= consumeCategory(snap, budget, ing)
		} else if ing.ID != "" {
			take := min(budget[ing.ID], need)
			budget[ing.ID] -= take
			need -= take
		}
		if need <= 0 {
			continue
		}
		// A category ingredient names a class of items, not something a
		// machine can be asked to output, so there is nothing to produce.
		if !ing.Category && produce(machines, budget, ing.ID, need, maxDepth, map[string]bool{}, &res) {
			continue
		}
		res.Feasible = false
		res.StillMissing = append(res.StillMissing, MissingItem{
			ID: ing.ID, Name: ing.Name, Need: ing.Qty, Have: ing.Qty - need, WikiURL: wikiURL(ing.Name),
		})
	}
	return res, nil
}

// produce tries to make `need` more of item `id` using machines, spending from
// budget. Steps are appended dependency-first. visited guards against machine
// cycles; depth bounds the chain length.
//
// Each candidate machine is tried against a copy of the budget, so a partial
// attempt that fails leaves nothing behind.
func produce(machines []Machine, budget map[string]int, id string, need, depth int, visited map[string]bool, res *PlanResult) bool {
	if depth == 0 || need <= 0 || id == "" || visited[id] {
		return false
	}
	visited[id] = true
	defer delete(visited, id)

	for _, m := range machines {
		if m.Output.ID != id || m.Output.Qty < 1 {
			continue
		}
		runs := (need + m.Output.Qty - 1) / m.Output.Qty

		trial := make(map[string]int, len(budget))
		for k, v := range budget {
			trial[k] = v
		}
		trialRes := PlanResult{Steps: []PlanStep{}}
		ok := true
		for _, in := range m.Inputs {
			totalNeed := in.Qty * runs
			if in.ID != "" && !in.Category {
				take := min(trial[in.ID], totalNeed)
				trial[in.ID] -= take
				totalNeed -= take
			}
			if totalNeed > 0 {
				if in.Category || !produce(machines, trial, in.ID, totalNeed, depth-1, visited, &trialRes) {
					ok = false
					break
				}
			}
		}
		if !ok {
			continue
		}
		// Commit: the trial budget becomes real, and any overproduction is
		// left available for later ingredients.
		for k, v := range trial {
			budget[k] = v
		}
		budget[id] += runs*m.Output.Qty - need
		res.Steps = append(res.Steps, trialRes.Steps...)
		res.Steps = append(res.Steps, PlanStep{
			Machine: m.Machine, Inputs: m.Inputs, Output: m.Output, Runs: runs, Minutes: m.Minutes,
		})
		return true
	}
	return false
}

// consumeCategory spends owned items belonging to an ingredient's category and
// reports how many were found.
func consumeCategory(snap *parser.Snapshot, budget map[string]int, ing Ingredient) int {
	cat, err := strconv.Atoi(ing.ID)
	if err != nil {
		return 0 // prose-described category; nothing in the save can match it
	}
	consumed := 0
	for id, n := range budget {
		if consumed >= ing.Qty {
			break
		}
		if n > 0 && snap.Categories[id] == cat {
			take := min(n, ing.Qty-consumed)
			budget[id] -= take
			consumed += take
		}
	}
	return consumed
}

// EvaluateWithPlanner is Evaluate, with far-off recipes promoted to partial
// when the planner can actually produce what they are missing.
func EvaluateWithPlanner(snap *parser.Snapshot, recipes []Recipe, machines []Machine) []Availability {
	avs := Evaluate(snap, recipes)
	for i := range avs {
		if avs[i].State != FarOff {
			continue
		}
		if res, err := PlanRecipe(snap, recipes, machines, avs[i].Recipe.Key); err == nil && (res.Feasible || len(res.Steps) > 0) {
			avs[i].State = Partial
		}
	}
	return avs
}
