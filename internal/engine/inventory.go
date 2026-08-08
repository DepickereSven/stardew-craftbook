package engine

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/svendep/stardew-craftbook/internal/parser"
)

// InventoryItem is one owned item, aggregated across every container in the
// save. SellPrice and StackValue are pointers because an item with no recorded
// price must not render as free — see ui-spec-items.md §4.2.
type InventoryItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
	SellPrice   *int   `json:"sell_price,omitempty"`
	StackValue  *int   `json:"stack_value,omitempty"`
	Category    int    `json:"category"`
	Quality     int    `json:"quality"`
	RecipeCount int    `json:"recipe_count"`
	WikiURL     string `json:"wiki_url,omitempty"`
}

// Verdict answers "is making this worth it?" for one recipe. The four states
// exist because two would lie: a Bee House is not a loss-making trade, it is
// something you build to use, and a recipe whose output price is simply
// unrecorded must not be reported as breaking even.
type Verdict string

const (
	// VerdictProfit means the output sells for more than the ingredients.
	VerdictProfit Verdict = "profit"
	// VerdictLoss means it sells for less. Meaningful mostly for cooking.
	VerdictLoss Verdict = "loss"
	// VerdictNotForSale means the output cannot be sold at all.
	VerdictNotForSale Verdict = "not_for_sale"
	// VerdictUnknown means a price is missing somewhere. Show no number.
	VerdictUnknown Verdict = "unknown"
)

// Economics is the sale arithmetic of one crafting of a recipe: what the
// output fetches against what the ingredients would have sold for raw. The
// pointer fields stay absent when a price is unknown — an absent price and a
// zero price are different facts, and only the verdict may paper over it.
type Economics struct {
	OutputValue *int    `json:"output_value,omitempty"`
	InputCost   *int    `json:"input_cost,omitempty"`
	Delta       *int    `json:"delta,omitempty"`
	Verdict     Verdict `json:"verdict"`
}

// UsedIn is one recipe that consumes a given item, with the economics of
// making it once.
type UsedIn struct {
	RecipeKey   string `json:"recipe_key"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	State       State  `json:"state"`
	Learned     bool   `json:"learned"`
	QtyHere     int    `json:"qty_here"`
	MaxMakeable int    `json:"max_makeable"`
	OutputQty   int    `json:"output_qty"`
	Economics
	WikiURL string `json:"wiki_url,omitempty"`
}

// ItemDetail is the payload behind GET /api/item/{id}.
type ItemDetail struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Count      int      `json:"count"`
	SellPrice  *int     `json:"sell_price,omitempty"`
	StackValue *int     `json:"stack_value,omitempty"`
	Category   int      `json:"category"`
	Quality    int      `json:"quality"`
	WikiURL    string   `json:"wiki_url,omitempty"`
	UsedIn     []UsedIn `json:"used_in"`
}

// priceTemplate preserves compatibility with older generated datasets whose
// sell_price_note contains raw wiki markup ("{{Price|50}}").
var priceTemplate = regexp.MustCompile(`^\{\{Price\|(\d+)\}\}$`)

// ItemIndex resolves item metadata for ids taken from a save. It exists because
// a bare id alone is not a safe key: the dataset holds 131 as Sardine while a
// save may hold furniture under the same number. Every lookup is therefore
// checked against the name the save reports.
type ItemIndex struct {
	byID   map[string]Item
	byName map[string]Item
}

// NewItemIndex normalises the raw dataset and indexes it by id and by name.
func NewItemIndex(items map[string]Item) *ItemIndex {
	idx := &ItemIndex{byID: make(map[string]Item, len(items)), byName: make(map[string]Item, len(items))}
	for id, it := range items {
		if it.SellPrice == nil && it.SellPriceNote != "" {
			if m := priceTemplate.FindStringSubmatch(strings.TrimSpace(it.SellPriceNote)); m != nil {
				if n, err := strconv.Atoi(m[1]); err == nil {
					it.SellPrice = &n
					it.SellPriceNote = ""
				}
			}
		}
		idx.byID[id] = it
		if it.Name != "" {
			if _, seen := idx.byName[it.Name]; !seen {
				idx.byName[it.Name] = it
			}
		}
	}
	return idx
}

// Lookup resolves an owned item. name is what the save calls it; when the
// dataset disagrees about that id the entry is rejected rather than trusted,
// which is what stops a held Coffee Maker inheriting Wheat Flour's price.
func (idx *ItemIndex) Lookup(id, name string) (Item, bool) {
	it, ok := idx.byID[id]
	if ok && (name == "" || strings.EqualFold(it.Name, name)) {
		return it, true
	}
	// A qualified id ((BC)146) is not in the dataset, which keys big craftables
	// bare; fall back to the bare number, still under the name guard.
	if bare := stripQualifier(id); bare != id {
		if it, ok := idx.byID[bare]; ok && (name == "" || strings.EqualFold(it.Name, name)) {
			return it, true
		}
	}
	if name != "" {
		if it, ok := idx.byName[name]; ok {
			return it, true
		}
		for _, generic := range variableItemNames(name) {
			if it, ok := idx.byName[generic]; ok {
				return it, true
			}
		}
	}
	return Item{}, false
}

func variableItemNames(name string) []string {
	if strings.HasPrefix(name, "Dried ") && name != "Dried Fruit" && name != "Dried Mushrooms" {
		return []string{"Dried Fruit"}
	}
	if strings.HasSuffix(name, " Roe") {
		if strings.HasPrefix(name, "Aged ") {
			return []string{"Aged Roe"}
		}
		return []string{"Roe"}
	}
	if strings.HasPrefix(name, "Smoked ") {
		return []string{"Smoked Fish"}
	}
	return nil
}

// ByName resolves a recipe's output, which the recipe dataset identifies by
// name only — it carries no output item id.
func (idx *ItemIndex) ByName(name string) (Item, bool) {
	it, ok := idx.byName[name]
	return it, ok
}

func stripQualifier(id string) string {
	if i := strings.IndexByte(id, ')'); strings.HasPrefix(id, "(") && i > 0 {
		return id[i+1:]
	}
	return id
}

// notForSale reports whether the dataset states outright that an item cannot be
// sold. The wiki writes this two ways; "N/A" is deliberately not included,
// since it records absence of information rather than a rule.
func notForSale(it Item) bool {
	return it.SellPrice == nil && strings.EqualFold(strings.TrimSpace(it.SellPriceNote), "cannot be sold")
}

func savedSellPrice(stack parser.ItemStack, fallback Item) *int {
	base := fallback.SellPrice
	if stack.Price != nil {
		base = stack.Price
	}
	if base == nil {
		return nil
	}
	price := *base
	switch stack.Quality {
	case 1:
		price = price * 5 / 4
	case 2:
		price = price * 3 / 2
	case 4:
		price *= 2
	}
	return &price
}

// recipeIndex maps an ingredient id to the recipes that consume it.
func recipeIndex(recipes []Recipe) map[string][]Recipe {
	idx := map[string][]Recipe{}
	for _, r := range recipes {
		for _, ing := range r.Ingredients {
			if ing.ID == "" {
				continue
			}
			idx[ing.ID] = append(idx[ing.ID], r)
		}
	}
	return idx
}

// BuildInventory lists everything the save holds, most valuable stack first.
// Items with no known price sort last but keep their place in the list — absent
// and zero are different facts.
func BuildInventory(snap *parser.Snapshot, idx *ItemIndex, recipes []Recipe) []InventoryItem {
	rev := recipeIndex(recipes)
	stacks := snap.Stacks
	if len(stacks) == 0 {
		stacks = make(map[string]parser.ItemStack, len(snap.Items))
		for id, count := range snap.Items {
			stacks[id] = parser.ItemStack{Key: id, ID: id, Name: snap.Names[id], Count: count, Category: snap.Categories[id]}
		}
	}
	out := make([]InventoryItem, 0, len(stacks))
	for key, stack := range stacks {
		inv := InventoryItem{
			ID:          key,
			Name:        stack.Name,
			Count:       stack.Count,
			Category:    stack.Category,
			Quality:     stack.Quality,
			RecipeCount: len(rev[stack.ID]),
		}
		metadata, known := idx.Lookup(stack.ID, stack.Name)
		if known {
			inv.WikiURL = metadata.WikiURL
			if inv.Name == "" {
				inv.Name = metadata.Name
			}
		}
		if price := savedSellPrice(stack, metadata); price != nil {
			value := *price * stack.Count
			inv.SellPrice = price
			inv.StackValue = &value
		}
		if inv.Name == "" {
			inv.Name = stack.ID
		}
		out = append(out, inv)
	}
	sort.Slice(out, func(i, j int) bool {
		vi, vj := 0, 0
		if out[i].StackValue != nil {
			vi = *out[i].StackValue
		}
		if out[j].StackValue != nil {
			vj = *out[j].StackValue
		}
		if vi != vj {
			return vi > vj
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// maxMakeable reports how many times a recipe can be made from what is owned,
// every ingredient considered.
func maxMakeable(snap *parser.Snapshot, r Recipe) int {
	best := -1
	for _, ing := range r.Ingredients {
		if ing.Qty <= 0 {
			continue
		}
		n := countOf(snap, ing) / ing.Qty
		if best < 0 || n < best {
			best = n
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

// inputCost sums the sell value of one crafting's ingredients, and reports
// false unless every one of them is priced — a partial total is worse than
// none, because it reads as a real number.
func inputCost(idx *ItemIndex, r Recipe) (int, bool) {
	total := 0
	for _, ing := range r.Ingredients {
		it, ok := idx.Lookup(ing.ID, ing.Name)
		if !ok || it.SellPrice == nil {
			return 0, false
		}
		total += *it.SellPrice * ing.Qty
	}
	return total, true
}

// RecipeEconomics prices one crafting of a recipe. Where no honest number
// exists — an unsellable output, or a missing price somewhere — the verdict
// carries the answer and the number fields stay empty.
func RecipeEconomics(idx *ItemIndex, r Recipe) Economics {
	ec := Economics{Verdict: VerdictUnknown}
	cost, costKnown := inputCost(idx, r)
	if costKnown {
		ec.InputCost = &cost
	}
	out, outKnown := idx.ByName(r.Name)
	switch {
	case outKnown && notForSale(out):
		ec.Verdict = VerdictNotForSale
		// An input cost is real but meaningless next to an unsellable
		// output; leaving it visible invites a subtraction that has no
		// answer.
		ec.InputCost = nil
	case outKnown && out.SellPrice != nil:
		v := *out.SellPrice
		ec.OutputValue = &v
		if costKnown {
			d := v*r.OutputQty - cost
			ec.Delta = &d
			if d >= 0 {
				ec.Verdict = VerdictProfit
			} else {
				ec.Verdict = VerdictLoss
			}
		}
	}
	return ec
}

// BuildItemDetail answers GET /api/item/{id}. avail supplies each recipe's
// state and learned flag so this view never recomputes them.
func BuildItemDetail(snap *parser.Snapshot, idx *ItemIndex, recipes []Recipe, avail []Availability, id string) (ItemDetail, bool) {
	stack, owned := snap.Stacks[id]
	if !owned {
		count, legacy := snap.Items[id]
		if !legacy {
			return ItemDetail{}, false
		}
		stack = parser.ItemStack{Key: id, ID: id, Name: snap.Names[id], Count: count, Category: snap.Categories[id]}
	}
	detail := ItemDetail{ID: id, Name: stack.Name, Count: stack.Count, Category: stack.Category, Quality: stack.Quality, UsedIn: []UsedIn{}}
	metadata, known := idx.Lookup(stack.ID, stack.Name)
	if known {
		detail.WikiURL = metadata.WikiURL
		if detail.Name == "" {
			detail.Name = metadata.Name
		}
	}
	if price := savedSellPrice(stack, metadata); price != nil {
		value := *price * stack.Count
		detail.SellPrice = price
		detail.StackValue = &value
	}
	if detail.Name == "" {
		detail.Name = id
	}

	byKey := make(map[string]Availability, len(avail))
	for _, av := range avail {
		byKey[av.Recipe.Key] = av
	}

	for _, r := range recipeIndex(recipes)[stack.ID] {
		qty := 0
		for _, ing := range r.Ingredients {
			if ing.ID == stack.ID {
				qty = ing.Qty
				break
			}
		}
		u := UsedIn{
			RecipeKey:   r.Key,
			Name:        r.Name,
			Type:        r.Type,
			QtyHere:     qty,
			MaxMakeable: maxMakeable(snap, r),
			OutputQty:   r.OutputQty,
			Economics:   RecipeEconomics(idx, r),
			WikiURL:     r.WikiURL,
		}
		if av, ok := byKey[r.Key]; ok {
			u.State, u.Learned = av.State, av.Learned
		}
		detail.UsedIn = append(detail.UsedIn, u)
	}
	sort.Slice(detail.UsedIn, func(i, j int) bool {
		a, b := detail.UsedIn[i], detail.UsedIn[j]
		// Actionable first, then by how much it earns, then by name.
		if (a.MaxMakeable > 0) != (b.MaxMakeable > 0) {
			return a.MaxMakeable > 0
		}
		da, db := 0, 0
		if a.Delta != nil {
			da = *a.Delta
		}
		if b.Delta != nil {
			db = *b.Delta
		}
		if da != db {
			return da > db
		}
		return a.Name < b.Name
	})
	return detail, true
}
