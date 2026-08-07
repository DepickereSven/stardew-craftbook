package main

type Machine struct {
	Machine string       `json:"machine"`
	Inputs  []Ingredient `json:"inputs"`
	Output  Ingredient   `json:"output"`
	Minutes int          `json:"minutes"`
}

// machineConversions is a curated list, hand-checked against the wiki. IDs are
// 1.6 object IDs.
func machineConversions() []Machine {
	coal := Ingredient{ID: "382", Name: "Coal", Qty: 1}
	return []Machine{
		{"Furnace", []Ingredient{{ID: "378", Name: "Copper Ore", Qty: 5}, coal}, Ingredient{ID: "334", Name: "Copper Bar", Qty: 1}, 30},
		{"Furnace", []Ingredient{{ID: "380", Name: "Iron Ore", Qty: 5}, coal}, Ingredient{ID: "335", Name: "Iron Bar", Qty: 1}, 120},
		{"Furnace", []Ingredient{{ID: "384", Name: "Gold Ore", Qty: 5}, coal}, Ingredient{ID: "336", Name: "Gold Bar", Qty: 1}, 300},
		{"Furnace", []Ingredient{{ID: "386", Name: "Iridium Ore", Qty: 5}, coal}, Ingredient{ID: "337", Name: "Iridium Bar", Qty: 1}, 480},
		{"Furnace", []Ingredient{{ID: "80", Name: "Quartz", Qty: 1}, coal}, Ingredient{ID: "338", Name: "Refined Quartz", Qty: 1}, 90},
		{"Furnace", []Ingredient{{ID: "82", Name: "Fire Quartz", Qty: 1}, coal}, Ingredient{ID: "338", Name: "Refined Quartz", Qty: 3}, 90},
		{"Cheese Press", []Ingredient{{ID: "184", Name: "Milk", Qty: 1}}, Ingredient{ID: "424", Name: "Cheese", Qty: 1}, 200},
		{"Cheese Press", []Ingredient{{ID: "436", Name: "Goat Milk", Qty: 1}}, Ingredient{ID: "426", Name: "Goat Cheese", Qty: 1}, 200},
		{"Mayonnaise Machine", []Ingredient{{ID: "176", Name: "Egg", Qty: 1}}, Ingredient{ID: "306", Name: "Mayonnaise", Qty: 1}, 180},
		{"Mayonnaise Machine", []Ingredient{{ID: "442", Name: "Duck Egg", Qty: 1}}, Ingredient{ID: "307", Name: "Duck Mayonnaise", Qty: 1}, 180},
		{"Loom", []Ingredient{{ID: "440", Name: "Wool", Qty: 1}}, Ingredient{ID: "428", Name: "Cloth", Qty: 1}, 240},
		{"Oil Maker", []Ingredient{{ID: "270", Name: "Corn", Qty: 1}}, Ingredient{ID: "247", Name: "Oil", Qty: 1}, 1000},
		{"Oil Maker", []Ingredient{{ID: "421", Name: "Sunflower", Qty: 1}}, Ingredient{ID: "247", Name: "Oil", Qty: 1}, 60},
		{"Oil Maker", []Ingredient{{ID: "431", Name: "Sunflower Seeds", Qty: 1}}, Ingredient{ID: "247", Name: "Oil", Qty: 1}, 3200},
		{"Oil Maker", []Ingredient{{ID: "430", Name: "Truffle", Qty: 1}}, Ingredient{ID: "432", Name: "Truffle Oil", Qty: 1}, 360},
		{"Keg", []Ingredient{{ID: "433", Name: "Coffee Bean", Qty: 5}}, Ingredient{ID: "395", Name: "Coffee", Qty: 1}, 120},
		{"Charcoal Kiln", []Ingredient{{ID: "388", Name: "Wood", Qty: 10}}, Ingredient{ID: "382", Name: "Coal", Qty: 1}, 30},
		{"Recycling Machine", []Ingredient{{ID: "168", Name: "Trash", Qty: 1}}, Ingredient{ID: "338", Name: "Refined Quartz", Qty: 1}, 60},
		// The wiki describes these inputs as variable prose rather than {{Name}}
		// templates, so they are represented as category inputs for the planner.
		{"Bee House", []Ingredient{{ID: "-80", Name: "Flower (Any)", Qty: 1, Category: true}}, Ingredient{ID: "340", Name: "Honey", Qty: 1}, 6100},
		{"Preserves Jar", []Ingredient{{ID: "-79", Name: "Fruit (Any)", Qty: 1, Category: true}}, Ingredient{ID: "344", Name: "Jelly", Qty: 1}, 4000},
		{"Preserves Jar", []Ingredient{{ID: "-75", Name: "Vegetable (Any)", Qty: 1, Category: true}}, Ingredient{ID: "342", Name: "Pickles", Qty: 1}, 4000},
	}
}
