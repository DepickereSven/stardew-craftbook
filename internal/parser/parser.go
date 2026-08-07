// Package parser turns a Stardew Valley 1.6 save file into an inventory
// snapshot. It never writes to the save.
package parser

import (
	"bufio"
	"encoding/xml"
	"io"
	"os"
	"strings"
)

// Snapshot is everything the engine needs to know about a save: how much of
// each item the player owns anywhere, what each of those items is called, what
// category it is in (so category ingredients like "any milk" can be resolved),
// and which recipes have been learned.
//
// Items is keyed by qualified id — see qualifyID. Names is keyed the same way
// and is populated for every entry in Items; the save names every item it
// holds, so nothing here needs a fallback to the item dataset.
type Snapshot struct {
	Items           map[string]int
	Names           map[string]string
	Categories      map[string]int
	CraftingLearned map[string]bool
	CookingLearned  map[string]bool
}

type saveGame struct {
	Player    playerXML     `xml:"player"`
	Locations []locationXML `xml:"locations>GameLocation"`
	Team      teamXML       `xml:"team"`
}

type playerXML struct {
	Items           []itemXML `xml:"items>Item"`
	CraftingRecipes []kvXML   `xml:"craftingRecipes>item"`
	CookingRecipes  []kvXML   `xml:"cookingRecipes>item"`
}

type kvXML struct {
	Key string `xml:"key>string"`
}

type itemXML struct {
	Nil      bool   `xml:"http://www.w3.org/2001/XMLSchema-instance nil,attr"`
	Type     string `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Name     string `xml:"name"`
	ItemID   string `xml:"itemId"`
	Stack    int    `xml:"stack"`
	Category int    `xml:"category"`
	// BigCraftable is a string rather than a bool because it is absent on tools,
	// weapons and clothing, where "" and "false" must stay distinguishable from
	// each other only in intent — both mean "not a big craftable".
	BigCraftable string    `xml:"bigCraftable"`
	Items        []itemXML `xml:"items>Item"` // chest contents
	// HeldObject is a machine's finished output sitting in it, waiting to be
	// collected. Deliberately NOT lastInputItem, which merely records what the
	// machine last consumed and is no longer owned.
	HeldObject *itemXML `xml:"heldObject"`
}

// A location holds placed objects (chests among them), a farmhouse fridge,
// and buildings whose interiors are locations in their own right.
type locationXML struct {
	Objects   []objEntryXML `xml:"objects>item"`
	Fridge    *itemXML      `xml:"fridge"`
	Buildings []buildingXML `xml:"buildings>Building"`
}

type objEntryXML struct {
	Object itemXML `xml:"value>Object"`
}

type buildingXML struct {
	Indoors *locationXML `xml:"indoors"`
}

type teamXML struct {
	GlobalInventories []globalInvXML `xml:"globalInventories>item"`
}

// globalInvXML covers Junimo chests, whose contents live once on the team
// rather than in any one location.
type globalInvXML struct {
	Key   string    `xml:"key>string"`
	Items []itemXML `xml:"value>Inventory>Item"`
}

func Parse(r io.Reader) (*Snapshot, error) {
	br := bufio.NewReader(r)
	// Save files start with a UTF-8 BOM, which encoding/xml will not accept
	// ahead of the declaration.
	if b, err := br.Peek(3); err == nil && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		br.Discard(3)
	}
	var sg saveGame
	if err := xml.NewDecoder(br).Decode(&sg); err != nil {
		return nil, err
	}
	snap := &Snapshot{
		Items:           map[string]int{},
		Names:           map[string]string{},
		Categories:      map[string]int{},
		CraftingLearned: map[string]bool{},
		CookingLearned:  map[string]bool{},
	}
	addItems(snap, sg.Player.Items)
	for i := range sg.Locations {
		addLocation(snap, &sg.Locations[i])
	}
	for _, gi := range sg.Team.GlobalInventories {
		addItems(snap, gi.Items)
	}
	for _, kv := range sg.Player.CraftingRecipes {
		snap.CraftingLearned[kv.Key] = true
	}
	for _, kv := range sg.Player.CookingRecipes {
		snap.CookingLearned[kv.Key] = true
	}
	return snap, nil
}

// qualifyID returns the key an item is counted under.
//
// The game namespaces item ids by type, but saves overwhelmingly store the bare
// number and the id spaces overlap: 131 is Sardine as an object and Crystal
// Chair as furniture, 246 is Wheat Flour as an object and Coffee Maker as a big
// craftable. Counting all of them under the bare number makes a held chair
// satisfy a recipe calling for sardines.
//
// Only true objects keep the bare id, because that is the space recipe and
// machine ingredients are written in. Everything else is prefixed, which means
// it can never match an ingredient — which is correct, since no recipe asks for
// a Gold Axe.
func qualifyID(it itemXML) string {
	id := strings.TrimPrefix(it.ItemID, "(O)")
	if id == "" || strings.HasPrefix(id, "(") {
		return id // already qualified by the save
	}
	if it.BigCraftable == "true" {
		return "(BC)" + id
	}
	switch it.Type {
	case "Furniture", "BedFurniture":
		return "(F)" + id
	case "MeleeWeapon", "Slingshot":
		return "(W)" + id
	case "Hat":
		return "(H)" + id
	case "Boots":
		return "(B)" + id
	case "Clothing":
		return "(C)" + id
	case "Wallpaper":
		return "(WP)" + id
	case "Axe", "Hoe", "Pickaxe", "WateringCan", "Pan", "MilkPail", "FishingRod", "Wand", "Shears":
		return "(T)" + id
	}
	return id
}

func addItems(snap *Snapshot, items []itemXML) {
	for _, it := range items {
		if it.Nil || it.Stack <= 0 {
			continue
		}
		id := qualifyID(it)
		if id == "" {
			continue
		}
		snap.Items[id] += it.Stack
		snap.Categories[id] = it.Category
		if it.Name != "" {
			// First name wins. One id can cover several differently-named items
			// (DriedFruit is Dried Blueberries and Dried Strawberries both), so
			// this is one truthful answer among several, not the only one.
			if _, seen := snap.Names[id]; !seen {
				snap.Names[id] = it.Name
			}
		}
		addItems(snap, it.Items) // a container is itself an owned item
		if it.HeldObject != nil {
			addItems(snap, []itemXML{*it.HeldObject})
		}
	}
}

func addLocation(snap *Snapshot, loc *locationXML) {
	for i := range loc.Objects {
		addItems(snap, []itemXML{loc.Objects[i].Object})
	}
	// Chest objects carry their own boolean <fridge> field, which decodes to
	// an empty item and is skipped; only a location's fridge holds contents.
	if loc.Fridge != nil {
		addItems(snap, []itemXML{*loc.Fridge})
	}
	for _, b := range loc.Buildings {
		if b.Indoors != nil {
			addLocation(snap, b.Indoors)
		}
	}
}

func ParseFile(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}
