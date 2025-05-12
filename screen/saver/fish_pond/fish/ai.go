package fish

import (
	"math"
	"math/rand/v2"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type FishAI struct {
	constraint rl.Vector2

	Pos       rl.Vector2
	Angle     float32
	AngleTime float32
	Speed     float32
	TurnRate  float32
	TurnFreq  float32
}

func NewFishAI(width, height int) FishAI {
	return FishAI{
		constraint: rl.NewVector2(float32(width), float32(height)),
		Pos:        rl.NewVector2(rand.Float32()*float32(width), rand.Float32()*float32(height)),
		Angle:      rand.Float32() * 2 * math.Pi,
		AngleTime:  rand.Float32() * 100,
		Speed:      200,
		TurnRate:   1.0,
		TurnFreq:   0.5,
	}
}

func (f *FishAI) Update() rl.Vector2 {
	dt := float32(1.0 / 60.0)
	f.AngleTime += dt

	drift := float32(math.Sin(float64(f.AngleTime*f.TurnFreq))) * f.TurnRate
	f.Angle += drift * dt

	vel := rl.NewVector2(
		float32(math.Cos(float64(f.Angle))),
		float32(math.Sin(float64(f.Angle))),
	)
	vel = rl.Vector2Scale(vel, f.Speed*dt)
	f.Pos = rl.Vector2Add(f.Pos, vel)

	if f.Pos.X < 0 {
		f.Pos.X += f.constraint.X
	} else if f.Pos.X > f.constraint.X {
		f.Pos.X -= f.constraint.X
	}
	if f.Pos.Y < 0 {
		f.Pos.Y += f.constraint.Y
	} else if f.Pos.Y > f.constraint.Y {
		f.Pos.Y -= f.constraint.Y
	}

	return f.Pos
}
