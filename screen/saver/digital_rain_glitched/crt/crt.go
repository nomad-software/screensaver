// Package crt makes the picture look like it is being shown on an old tube
// monitor that is on its way out.
package crt

import (
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// stutterChance is the chance of a fault flaring up again straight after it
// has died away.
const stutterChance = 0.45

// fault is one way the monitor is failing. It lies dormant, flares up for a
// moment, then lies dormant again. Flare ups are short and violent and often
// come in stutters, one straight after another.
type fault struct {
	minLength, maxLength float64 // How long a flare up lasts in seconds.
	minGap, maxGap       float64 // How long the fault lies dormant in seconds.

	start float64 // When the current flare up started.
	end   float64 // When the current flare up ends.
	next  float64 // When the next flare up starts.

	level float32 // The current strength, from 0 to 1.
}

// newFault creates a fault that first flares up after one of its gaps.
func newFault(minLength, maxLength, minGap, maxGap float64) *fault {
	f := &fault{
		minLength: minLength,
		maxLength: maxLength,
		minGap:    minGap,
		maxGap:    maxGap,
	}
	f.next = between(minGap, maxGap)

	return f
}

// update moves the fault along to the passed time.
func (f *fault) update(now float64) {
	if now >= f.next {
		f.start = now
		f.end = now + between(f.minLength, f.maxLength)
		if rand.Float64() < stutterChance {
			f.next = f.end + between(0.03, 0.2)
		} else {
			f.next = f.end + between(f.minGap, f.maxGap)
		}
	}

	if now < f.end {
		// Snap straight to full strength, crackle erratically, then drop
		// away right at the end.
		progress := (now - f.start) / (f.end - f.start)
		f.level = float32((1 - math.Pow(progress, 4)) * between(0.55, 1))
	} else {
		f.level = 0
	}
}

// Monitor is a failing tube monitor.
type Monitor struct {
	shader rl.Shader

	ghost *fault // Signal reflections.

	timeLoc  int32
	ghostLoc int32
}

// New creates a monitor that shows its picture through the passed shader on a
// screen of the passed size.
func New(shader rl.Shader, width, height int) *Monitor {
	m := &Monitor{
		shader: shader,

		ghost: newFault(0.06, 0.3, 3, 10),

		timeLoc:  rl.GetShaderLocation(shader, "time"),
		ghostLoc: rl.GetShaderLocation(shader, "ghost"),
	}

	resolution := []float32{float32(width), float32(height)}
	rl.SetShaderValue(shader, rl.GetShaderLocation(shader, "resolution"), resolution, rl.ShaderUniformVec2)

	return m
}

// Shader returns the shader to draw the picture with.
func (m *Monitor) Shader() rl.Shader {
	return m.shader
}

// Update moves the monitor's faults along to the passed time in seconds.
func (m *Monitor) Update(now float64) {
	m.ghost.update(now)

	m.set(m.timeLoc, float32(now))
	m.set(m.ghostLoc, m.ghost.level)
}

// set passes a value to the shader.
func (m *Monitor) set(loc int32, value float32) {
	rl.SetShaderValue(m.shader, loc, []float32{value}, rl.ShaderUniformFloat)
}

// between returns a random number between the passed limits.
func between(low, high float64) float64 {
	return low + rand.Float64()*(high-low)
}
