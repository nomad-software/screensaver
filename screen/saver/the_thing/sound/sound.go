package sound

import (
	"encoding/binary"
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	sampleRate = 22050 // Samples per second of the generated sounds.
	tickLength = 0.04  // Length of the tick in seconds.
	tickVolume = 0.18  // Volume of the tick from 0 to 1.
)

var (
	tick  rl.Sound
	ready bool
)

// Init opens the audio device and creates the sounds. Without an audio device
// the sounds are silently skipped.
func Init() {
	rl.InitAudioDevice()
	if !rl.IsAudioDeviceReady() {
		return
	}

	samples := tickSamples()
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(s))
	}

	wave := rl.NewWave(uint32(len(samples)), sampleRate, 16, 1, data)
	tick = rl.LoadSoundFromWave(wave)
	rl.SetSoundVolume(tick, tickVolume)

	ready = true
}

// Close releases the sounds and closes the audio device.
func Close() {
	if !ready {
		return
	}
	rl.UnloadSound(tick)
	rl.CloseAudioDevice()
}

// Tick plays the dull tick of a character being typed, varying the pitch a
// little each time so it doesn't sound mechanical.
func Tick() {
	if !ready {
		return
	}
	rl.SetSoundPitch(tick, 0.92+rand.Float32()*0.16)
	rl.PlaySound(tick)
}

// tickSamples generates the tick. It's mostly a soft low thump with a
// muffled burst of noise and only a hint of a click, so it sounds dull.
func tickSamples() []int16 {
	noise := rand.New(rand.NewSource(1))
	samples := make([]float64, int(math.Round(sampleRate*tickLength+0.5)))

	low, lower := 0.0, 0.0
	peak := 0.0
	for i := range samples {
		t := float64(i) / sampleRate

		thump := math.Sin(2*math.Pi*220*t) * math.Exp(-t/0.007) * 0.4
		click := math.Sin(2*math.Pi*1000*t) * math.Exp(-t/0.002) * 0.2

		// Low pass the noise twice so it's a muffled knock rather than a
		// hiss.
		low += (noise.Float64()*2 - 1 - low) * 0.3
		lower += (low - lower) * 0.3
		knock := lower * math.Exp(-t/0.0025) * 2.5

		// Ease in over half a millisecond to avoid a pop.
		attack := math.Min(1, t/0.0005)

		samples[i] = (thump + click + knock) * attack
		peak = math.Max(peak, math.Abs(samples[i]))
	}

	out := make([]int16, len(samples))
	for i, s := range samples {
		out[i] = int16(s / peak * math.MaxInt16 * 0.9)
	}
	return out
}
