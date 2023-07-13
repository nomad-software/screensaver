package sky

import rl "github.com/gen2brain/raylib-go/raylib"

// Coords holds the coordinates of the star.
type Coords struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Colour holds the RGB colour of the star.
type Color struct {
	R byte `json:"r"`
	G byte `json:"g"`
	B byte `json:"b"`
}

// Star hold information about the star.
type Star struct {
	ProperName        string  `json:"proper_name"`
	ConstellationName string  `json:"constellation_name"`
	BayerLetter       string  `json:"bayer_letter"`
	Coords            *Coords `json:"coords"`
	ColorRGB          *Color  `json:"color"`
	Magnitude         float64 `json:"magnitude"`

	CoordsV rl.Vector3 `json:"-"`
	Color   rl.Color   `json:"-"`
	Size    float32    `json:"-"`
}
