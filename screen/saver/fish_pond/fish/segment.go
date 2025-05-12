package fish

import rl "github.com/gen2brain/raylib-go/raylib"

type Segment struct {
	position rl.Vector2
	size     float32
}

func (s *Segment) SetPosition(pos rl.Vector2) {
	s.position = pos
}
