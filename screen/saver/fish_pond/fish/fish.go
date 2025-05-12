package fish

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	segmentRadii = []float32{25, 29, 33, 35.5, 38, 40.5, 43, 44, 44, 43.5, 42, 40, 38, 36.5, 35, 32.5, 30, 27.5, 25, 23, 21, 19, 17}
)

type Fish struct {
	segments    []*Segment
	segmentDist float32
	ai          FishAI
}

func NewFish(width, height int) *Fish {
	fish := &Fish{
		segments:    make([]*Segment, 0),
		segmentDist: 30,
		ai:          NewFishAI(width, height),
	}

	for i := 0; i < len(segmentRadii); i++ {
		fish.segments = append(fish.segments, &Segment{position: rl.NewVector2(0.0, 0.0), size: segmentRadii[i]})
	}

	return fish
}

func (f *Fish) calculateSegPos() {
	const (
		screenW = 2560.0
		screenH = 1440.0
	)

	prev := f.segments[0]
	for _, next := range f.segments[1:] {
		dir := rl.Vector2Subtract(next.position, prev.position)

		if dir.X > screenW/2 {
			dir.X -= screenW
		} else if dir.X < -screenW/2 {
			dir.X += screenW
		}
		if dir.Y > screenH/2 {
			dir.Y -= screenH
		} else if dir.Y < -screenH/2 {
			dir.Y += screenH
		}

		dir = rl.Vector2Normalize(dir)
		dir = rl.Vector2Scale(dir, prev.size/2.5)

		pos := rl.Vector2Add(prev.position, dir)

		if pos.X < 0 {
			pos.X += screenW
		} else if pos.X > screenW {
			pos.X -= screenW
		}
		if pos.Y < 0 {
			pos.Y += screenH
		} else if pos.Y > screenH {
			pos.Y -= screenH
		}

		next.SetPosition(pos)
		prev = next
	}
}

// func (f *Fish) calculateSegPos() {
// 	prev := f.segments[0]
// 	for _, next := range f.segments[1:] {
// 		dir := rl.Vector2Subtract(next.position, prev.position)
// 		dir = rl.Vector2Normalize(dir)
// 		dir = rl.Vector2Scale(dir, prev.size/1.25)
// 		pos := rl.Vector2Add(prev.position, dir)
// 		// pos = rl.Vector2Lerp(next.position, pos, 0.75) // Smoothing.
// 		next.SetPosition(pos)
// 		prev = next
// 	}
// }

func (f *Fish) SetPosition(pos rl.Vector2) {
	f.segments[0].position = pos
	f.calculateSegPos()
}

func (f *Fish) Update() {
	f.ai.Update()
	f.segments[0].position = f.ai.Pos
	f.calculateSegPos()

	for i := len(f.segments) - 1; i >= 0; i-- {
		seg := f.segments[i]
		rl.DrawCircleV(seg.position, seg.size, rl.White)
	}
}
