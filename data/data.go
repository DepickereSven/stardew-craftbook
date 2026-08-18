package data

import _ "embed"

//go:embed recipes.json
var RecipesJSON []byte

//go:embed machines.json
var MachinesJSON []byte

//go:embed items.json
var ItemsJSON []byte

//go:embed crops.json
var CropsJSON []byte
