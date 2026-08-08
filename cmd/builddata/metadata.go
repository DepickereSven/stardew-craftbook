package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Buff struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

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

var fieldStartRe = regexp.MustCompile(`(?m)^\|\s*([[:alnum:]_]+)\s*=`)
var leadingNumberRe = regexp.MustCompile(`^\s*([0-9][0-9,]*(?:\.[0-9]+)?)\s*(min(?:ute)?s?|m|h(?:our)?s?|d(?:ay)?s?)\b`)
var trailingQuantityRe = regexp.MustCompile(`\s*\((\d+)\)\s*$`)
var priceTemplateRe = regexp.MustCompile(`^\s*\{\{[Pp]rice\|([0-9][0-9,]*)\}\}\s*$`)
var anyCategoryQtyRe = regexp.MustCompile(`(?i)any\s+\[\[[^\]]+\]\][^(]*\((\d+)\)`)

var idlessItemInfoboxes = map[string]bool{
	"big craftable": true,
	"clothing":      true,
	"fish":          true,
	"furniture":     true,
	"ring":          true,
	"tool":          true,
	"weapon":        true,
}

func infoboxField(page, want string) string {
	matches := fieldStartRe.FindAllStringSubmatchIndex(page, -1)
	for i, m := range matches {
		if !strings.EqualFold(page[m[2]:m[3]], want) {
			continue
		}
		end := len(page)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		if close := strings.Index(page[m[1]:], "\n}}"); close >= 0 && m[1]+close < end {
			end = m[1] + close
		}
		return strings.TrimSpace(page[m[1]:end])
	}
	return ""
}

func parseCraftMinutes(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.EqualFold(trimmed, "Ready the next morning") {
		return 1440, nil
	}
	m := leadingNumberRe.FindStringSubmatch(trimmed)
	if m == nil {
		// Tapper pages use prose such as "5 or 2 days". The leading value is
		// the normal processing time; the shorter value is a profession bonus.
		if first := regexp.MustCompile(`^\s*([0-9][0-9,]*(?:\.[0-9]+)?)\b`).FindStringSubmatch(trimmed); first != nil {
			unit := ""
			switch {
			case strings.Contains(trimmed, "day"):
				unit = "d"
			case strings.Contains(trimmed, "hour"):
				unit = "h"
			case strings.Contains(trimmed, "minute"):
				unit = "min"
			}
			if unit != "" {
				m = []string{"", first[1], unit}
			}
		}
	}
	if m == nil {
		return 0, fmt.Errorf("unrecognized processing time %q", raw)
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil {
		return 0, err
	}
	multiplier := 1.0
	switch m[2] {
	case "h", "hour", "hours":
		multiplier = 60
	case "d", "day", "days":
		multiplier = 1440
	}
	return int(n*multiplier + 0.5), nil
}

func parseItemPage(title, page string) (Item, error) {
	id := infoboxField(page, "id")
	if id == "" {
		id = infoboxField(page, "objectid")
	}
	item := Item{ID: strings.TrimSpace(id), Name: title, WikiURL: wikiURL(title)}
	priceField := infoboxField(page, "sellprice")
	if priceField == "" {
		priceField = infoboxField(page, "price")
	}
	if priceField == "" {
		priceField = infoboxField(page, "value")
	}
	if raw := priceField; raw != "" {
		price := strings.TrimSpace(raw)
		if template := priceTemplateRe.FindStringSubmatch(price); template != nil {
			price = template[1]
		}
		if n, err := strconv.Atoi(strings.ReplaceAll(price, ",", "")); err == nil {
			item.SellPrice = &n
		} else {
			item.SellPriceNote = raw
		}
	}
	if raw := infoboxField(page, "edibility"); raw != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			item.Edibility = &n
		}
	}
	item.Buffs = parseBuffs(infoboxField(page, "buff"))
	item.BuffDuration = infoboxField(page, "buffduration")
	if item.BuffDuration == "" {
		item.BuffDuration = infoboxField(page, "duration")
	}
	if raw := infoboxField(page, "crafttime"); raw != "" {
		minutes, err := parseCraftMinutes(raw)
		if err == nil {
			item.ProcessingMinutes = &minutes
		}
	}
	return item, nil
}

func idlessItemKey(title, page string) string {
	trimmed := strings.TrimSpace(page)
	if !strings.HasPrefix(trimmed, "{{Infobox ") {
		return ""
	}
	name := strings.ToLower(strings.TrimSpace(strings.SplitN(strings.TrimPrefix(trimmed, "{{Infobox "), "\n", 2)[0]))
	name = strings.TrimSpace(strings.SplitN(name, "|", 2)[0])
	if !idlessItemInfoboxes[name] {
		return ""
	}
	return "name:" + title
}

func parseBuffs(raw string) []Buff {
	var buffs []Buff
	for _, fields := range nameTemplateFields(raw) {
		if len(fields) == 0 || strings.TrimSpace(fields[0]) == "" {
			continue
		}
		buff := Buff{Name: strings.TrimSpace(fields[0])}
		if len(fields) > 1 && !strings.Contains(fields[1], "=") {
			buff.Value = strings.ReplaceAll(strings.TrimSpace(fields[1]), "−", "-")
		}
		buffs = append(buffs, buff)
	}
	return buffs
}

// nameTemplateFields scans balanced {{Name|...}} templates so nested {{!}}
// link separators cannot terminate a match early.
func nameTemplateFields(raw string) [][]string {
	var result [][]string
	for start := 0; start < len(raw); {
		i := strings.Index(raw[start:], "{{Name|")
		if i < 0 {
			break
		}
		i += start
		depth, end := 1, i+len("{{Name|")
		for end < len(raw) && depth > 0 {
			switch {
			case strings.HasPrefix(raw[end:], "{{"):
				depth++
				end += 2
			case strings.HasPrefix(raw[end:], "}}"):
				depth--
				end += 2
			default:
				end++
			}
		}
		if depth != 0 {
			break
		}
		result = append(result, splitTemplateFields(raw[i+len("{{Name|"):end-2]))
		start = end
	}
	return result
}

func splitTemplateFields(raw string) []string {
	var fields []string
	start, depth := 0, 0
	for i := 0; i < len(raw); i++ {
		if strings.HasPrefix(raw[i:], "{{") {
			depth++
			i++
		} else if strings.HasPrefix(raw[i:], "}}") {
			depth--
			i++
		} else if raw[i] == '|' && depth == 0 {
			fields = append(fields, raw[start:i])
			start = i + 1
		}
	}
	return append(fields, raw[start:])
}

func machineFromPage(output Item, page string, idsByName map[string]string) (Machine, bool, error) {
	station := nameTemplateFields(infoboxField(page, "craftingstation"))
	if len(station) == 0 || len(station[0]) == 0 || output.ID == "" || output.ProcessingMinutes == nil {
		return Machine{}, false, nil
	}
	inputs := parseMachineIngredients(infoboxField(page, "ingredients"), idsByName)
	if len(inputs) == 0 {
		return Machine{}, false, nil
	}
	return Machine{Machine: strings.TrimSpace(station[0][0]), Inputs: inputs, Output: Ingredient{ID: output.ID, Name: output.Name, Qty: 1}, Minutes: *output.ProcessingMinutes}, true, nil
}

func parseMachineIngredients(raw string, idsByName map[string]string) []Ingredient {
	var out []Ingredient
	for _, fields := range nameTemplateFields(raw) {
		if len(fields) < 2 {
			continue
		}
		qty, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil || qty < 1 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		out = append(out, Ingredient{ID: idsByName[name], Name: name, Qty: qty})
	}
	if category, ok := machineCategoryInput(raw); ok {
		out = append([]Ingredient{category}, out...)
	}
	if len(out) > 0 {
		return out
	}
	if strings.Contains(strings.ToLower(raw), "fruit") {
		match := trailingQuantityRe.FindStringSubmatch(raw)
		if match != nil {
			qty, _ := strconv.Atoi(match[1])
			return []Ingredient{{ID: "-79", Name: "Fruit (Any)", Qty: qty, Category: true}}
		}
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "[[") {
		name := strings.TrimSpace(raw)
		name = strings.TrimPrefix(name, "[[")
		name = strings.TrimSuffix(name, "]]")
		if pipe := strings.LastIndex(name, "|"); pipe >= 0 {
			name = name[pipe+1:]
		}
		return []Ingredient{{ID: idsByName[name], Name: name, Qty: 1}}
	}
	match := trailingQuantityRe.FindStringSubmatch(raw)
	if match == nil || !strings.Contains(strings.ToLower(raw), "any ") {
		return nil
	}
	qty, _ := strconv.Atoi(match[1])
	name := raw[:len(raw)-len(match[0])]
	name = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), "Any"))
	if pipe := strings.LastIndex(name, "|"); pipe >= 0 {
		name = strings.TrimSuffix(name[pipe+1:], "]]")
	}
	name = strings.Trim(name, "[] ") + " (Any)"
	return []Ingredient{{Name: name, Qty: qty, Category: true}}
}

func machineCategoryInput(raw string) (Ingredient, bool) {
	match := anyCategoryQtyRe.FindStringSubmatch(raw)
	if match == nil {
		return Ingredient{}, false
	}
	qty, err := strconv.Atoi(match[1])
	if err != nil || qty < 1 {
		return Ingredient{}, false
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "fish"):
		return Ingredient{ID: "-4", Name: "Fish (Any)", Qty: qty, Category: true}, true
	case strings.Contains(lower, "fruit"):
		return Ingredient{ID: "-79", Name: "Fruit (Any)", Qty: qty, Category: true}, true
	case strings.Contains(lower, "vegetable"):
		return Ingredient{ID: "-75", Name: "Vegetable (Any)", Qty: qty, Category: true}, true
	}
	return Ingredient{}, false
}

func wikiURL(title string) string {
	return "https://stardewvalleywiki.com/" + strings.ReplaceAll(title, " ", "_")
}

func collectMetadata(pages map[string]string, recipeNames map[string]string) ([]Machine, map[string]Item, int, error) {
	idsByName := map[string]string{}
	for id, name := range recipeNames {
		idsByName[name] = id
	}
	for name, id := range machineItemIDs {
		idsByName[name] = id
	}
	items := map[string]Item{}
	for title, page := range pages {
		item, err := parseItemPage(title, page)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("%s: %w", title, err)
		}
		if item.ID == "" {
			item.ID = idsByName[item.Name]
		}
		if item.ID == "" {
			item.ID = idlessItemKey(item.Name, page)
		}
		if item.ID != "" {
			if !strings.HasPrefix(item.ID, "name:") {
				idsByName[item.Name] = item.ID
			}
			items[item.ID] = item
		}
	}
	machines := append([]Machine(nil), machineConversions()...)
	unresolved := 0
	for title, page := range pages {
		item, _ := parseItemPage(title, page)
		if item.ID == "" {
			item.ID = idsByName[item.Name]
		}
		m, ok, err := machineFromPage(item, page, idsByName)
		if err != nil {
			return nil, nil, 0, err
		}
		if !ok {
			continue
		}
		for _, input := range m.Inputs {
			if input.ID == "" && !input.Category {
				unresolved++
			}
		}
		machines = appendMachineUnique(machines, m)
		for _, station := range nameTemplateFields(infoboxField(page, "craftingstation"))[1:] {
			if len(station) == 0 || station[0] == "" {
				continue
			}
			variant := m
			variant.Machine = strings.TrimSpace(station[0])
			machines = appendMachineUnique(machines, variant)
		}
	}
	sort.Slice(machines, func(i, j int) bool { return machineKey(machines[i]) < machineKey(machines[j]) })
	return machines, items, unresolved, nil
}

// These are object names not present in any crafting or cooking recipe, so
// resolveNames cannot learn their IDs. They are kept narrowly to preserve
// machine-only production chains such as Tea Leaves → Green Tea.
var machineItemIDs = map[string]string{
	"Beer": "346", "Dried Fruit": "DriedFruit", "Dried Mushrooms": "DriedMushrooms", "Flour": "246",
	"Green Tea": "614", "Honey": "340", "Hops": "304", "Jelly": "344", "Juice": "350",
	"Maple Syrup": "724", "Mead": "459", "Oak Resin": "725", "Pale Ale": "303",
	"Pickles": "342", "Pine Tar": "726", "Raisins": "733", "Rice": "423", "Sugar": "245",
	"Smoked Fish": "SmokedFish", "Tea Leaves": "815", "Wine": "348",
}

func appendMachineUnique(machines []Machine, candidate Machine) []Machine {
	key := machineSemanticKey(candidate)
	for _, existing := range machines {
		if machineSemanticKey(existing) == key {
			return machines
		}
	}
	return append(machines, candidate)
}

// machineSemanticKey deliberately excludes item IDs. Scraped infobox data
// sometimes lacks an ID that the hand-checked baseline has; that must update
// neither the baseline row nor the planner's ability to use it.
func machineSemanticKey(m Machine) string {
	var inputs []string
	for _, input := range m.Inputs {
		inputs = append(inputs, input.Name+":"+strconv.Itoa(input.Qty))
	}
	return m.Machine + "|" + strings.Join(inputs, ",") + "|" + m.Output.Name + ":" + strconv.Itoa(m.Output.Qty) + "|" + strconv.Itoa(m.Minutes)
}

func machineKey(m Machine) string {
	var inputs []string
	for _, input := range m.Inputs {
		inputs = append(inputs, input.ID+":"+input.Name+":"+strconv.Itoa(input.Qty))
	}
	return m.Machine + "|" + strings.Join(inputs, ",") + "|" + m.Output.ID + ":" + strconv.Itoa(m.Output.Qty) + "|" + strconv.Itoa(m.Minutes)
}

func isNonEmptyName(s string) bool { return strings.IndexFunc(s, unicode.IsLetter) >= 0 }
