package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/nomad-software/screensaver/screen/saver/night_sky/convertor/bayer"
	"github.com/nomad-software/screensaver/screen/saver/night_sky/convertor/constellation"
	"github.com/nomad-software/screensaver/screen/saver/night_sky/sky"
)

func calcTemp(colorIndex float64) float64 {
	// Calculate the temperature using the Ballesteros formula
	temperature := 4600 * (1/(0.92*colorIndex+1.7) + 1/(0.92*colorIndex+0.62))
	return math.Round(temperature)
}

func convertTempToRGB(tmpKelvin float64) []uint8 {
	var tmpCalc float64

	// Temperature must fall between 1000 and 40000 degrees
	if tmpKelvin < 1000 {
		tmpKelvin = 1000
	}
	if tmpKelvin > 40000 {
		tmpKelvin = 40000
	}

	// All calculations require tmpKelvin / 100, so only do the conversion once
	tmpKelvin /= 100

	// Calculate each color in turn

	// First: red
	var r uint8
	if tmpKelvin <= 66 {
		r = 255
	} else {
		// Note: the R-squared value for this approximation is .988
		tmpCalc = tmpKelvin - 60
		tmpCalc = 329.698727446 * math.Pow(tmpCalc, -0.1332047592)
		r = uint8(tmpCalc)
		if r < 0 {
			r = 0
		}
		if r > 255 {
			r = 255
		}
	}

	// Second: green
	var g uint8
	if tmpKelvin <= 66 {
		// Note: the R-squared value for this approximation is .996
		tmpCalc = tmpKelvin
		tmpCalc = 99.4708025861*math.Log(tmpCalc) - 161.1195681661
		g = uint8(tmpCalc)
		if g < 0 {
			g = 0
		}
		if g > 255 {
			g = 255
		}
	} else {
		// Note: the R-squared value for this approximation is .987
		tmpCalc = tmpKelvin - 60
		tmpCalc = 288.1221695283 * math.Pow(tmpCalc, -0.0755148492)
		g = uint8(tmpCalc)
		if g < 0 {
			g = 0
		}
		if g > 255 {
			g = 255
		}
	}

	// Third: blue
	var b uint8
	if tmpKelvin >= 66 {
		b = 255
	} else if tmpKelvin <= 19 {
		b = 0
	} else {
		// Note: the R-squared value for this approximation is .998
		tmpCalc = tmpKelvin - 10
		tmpCalc = 138.5177312231*math.Log(tmpCalc) - 305.0447927307
		b = uint8(tmpCalc)
		if b < 0 {
			b = 0
		}
		if b > 255 {
			b = 255
		}
	}

	return []uint8{r, g, b}
}

func apparentMagnitudeToScale(magnitude float64) float64 {
	flipped := -magnitude
	linear := math.Pow(2.512, flipped)

	minValue := 2.086775569953444e-11
	maxValue := 2.5142724484057912e+08
	rangeValue := maxValue - minValue
	normalized := (linear - minValue) / rangeValue

	return normalized
}

type Vector struct {
	X float64
	Y float64
	Z float64
}

func lerp(v1, v2 Vector, amount float64) Vector {
	result := Vector{}

	result.X = v1.X + amount*(v2.X-v1.X)
	result.Y = v1.Y + amount*(v2.Y-v1.Y)
	result.Z = v1.Z + amount*(v2.Z-v1.Z)

	return result
}

func distance(v1, v2 Vector) float64 {
	dx := v2.X - v1.X
	dy := v2.Y - v1.Y
	dz := v2.Z - v1.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func main() {
	f, err := os.Open("hyg_v35.csv")
	if err != nil {
		log.Fatalf("file cannot be opened: %s\n", err)
	}
	defer f.Close()

	lines, err := csv.NewReader(f).ReadAll()
	if err != nil {
		log.Fatalf("file cannot be read: %s\n", err)
	}

	stars := make([]sky.Star, 0)

	for i, line := range lines {
		// fmt.Printf("line: %d\n", i)
		if i == 0 {
			continue
		}

		properName := ""
		constellationName := ""
		bayerLetter := ""
		rgb := make([]uint8, 0)
		magnitude := 0.0

		properName = line[6]
		if properName == "Sol" {
			continue
		}

		ci := line[16]
		if ci != "" {
			cifloat, err := strconv.ParseFloat(ci, 64)
			if err != nil {
				log.Fatalf("couldn't parse colour index: %s\n", ci)
			}
			temp := calcTemp(cifloat)
			rgb = convertTempToRGB(temp)
			// fmt.Printf("ci: %s, temp: %f, rgb: %v\n", ci, temp, rgb)
		} else {
			rgb = []uint8{128, 128, 128}
		}

		mag := line[13]
		if mag != "" {
			magfloat, err := strconv.ParseFloat(mag, 64)
			if err != nil {
				log.Fatalf("couldn't parse magnitude: %s\n", ci)
			}
			// magnitude = apparentMagnitudeToScale(magfloat)
			// magnitude = math.Pow(2.512, magfloat)
			magnitude = magfloat
		}

		// Convert all b names.
		b := line[27]
		if b != "" {
			found := false
			for k, v := range bayer.Letters {
				if strings.HasPrefix(b, k) {
					bayerLetter = v
					// fmt.Printf("found: %s = %s\n", bayer, v)
					found = true
					break
				}
			}
			if !found {
				log.Fatalf("couldn't find bayer name: %s\n", b)
			}
		}

		// Convert all constellation names.
		con := line[29]
		if con != "" {
			c, ok := constellation.Names[con]
			constellationName = c
			if !ok {
				log.Fatalf("couldn't find constellation: %s\n", con)
			}
			// fmt.Printf("found: %s = %s\n", con, constellationName)
		}

		x := line[17]
		if x == "" {
			continue
		}
		xfloat, err := strconv.ParseFloat(x, 64)
		if err != nil {
			log.Fatalf("couldn't parse x coord: %s\n", ci)
		}

		y := line[18]
		if y == "" {
			continue
		}
		yfloat, err := strconv.ParseFloat(y, 64)
		if err != nil {
			log.Fatalf("couldn't parse y coord: %s\n", ci)
		}

		z := line[19]
		if z == "" {
			continue
		}
		zfloat, err := strconv.ParseFloat(z, 64)
		if err != nil {
			log.Fatalf("couldn't parse z coord: %s\n", ci)
		}

		center := Vector{X: 0, Y: 0, Z: 0}
		coords := Vector{X: xfloat, Y: yfloat, Z: zfloat}
		dist := distance(center, coords)
		l := lerp(center, coords, 100/dist)

		star := sky.Star{
			ProperName:        properName,
			ConstellationName: constellationName,
			BayerLetter:       bayerLetter,
			Coords: sky.Coords{
				X: l.X,
				Y: l.Z,
				Z: -l.Y,
			},
			ColorRGB: sky.Color{
				R: rgb[0],
				G: rgb[1],
				B: rgb[2],
			},
			Magnitude: magnitude,
		}

		// if star.Magnitude < 0.3 {
		// fmt.Printf("%v\n", star)
		// }

		stars = append(stars, star)
	}

	bytes, err := json.MarshalIndent(stars, "", " ")
	if err != nil {
		log.Fatalf("error marshalling json: %s\n", err)
	}

	err = os.WriteFile("../assets/json/stars.json", bytes, 0644)
	if err != nil {
		log.Fatalf("error writing json file: %s\n", err)
	}
}
