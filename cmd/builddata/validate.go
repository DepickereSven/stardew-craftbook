package main

import "fmt"

func validate(recipes []Recipe, machines []Machine) []string {
	var problems []string
	seen := map[string]bool{}
	for _, r := range recipes {
		if seen[r.Key] {
			problems = append(problems, fmt.Sprintf("duplicate key %q", r.Key))
		}
		seen[r.Key] = true
		if r.Name == "" {
			problems = append(problems, fmt.Sprintf("%q: empty display name", r.Key))
		}
		if len(r.Ingredients) == 0 {
			problems = append(problems, fmt.Sprintf("%q: no ingredients", r.Key))
		}
		if r.OutputQty < 1 {
			problems = append(problems, fmt.Sprintf("%q: output_qty %d", r.Key, r.OutputQty))
		}
		for _, ing := range r.Ingredients {
			if ing.Qty < 1 {
				problems = append(problems, fmt.Sprintf("%q: ingredient %s qty %d", r.Key, ing.ID, ing.Qty))
			}
			if ing.Name == "" {
				problems = append(problems, fmt.Sprintf("%q: ingredient %s has no name", r.Key, ing.ID))
			}
		}
	}
	for _, m := range machines {
		if len(m.Inputs) == 0 || m.Output.ID == "" || m.Output.Qty < 1 || m.Minutes < 1 {
			problems = append(problems, fmt.Sprintf("machine %s -> %s malformed", m.Machine, m.Output.Name))
		}
	}
	return problems
}
