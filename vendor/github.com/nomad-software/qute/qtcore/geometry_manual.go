// Hand-written; qutegen never overwrites *_manual.go files.

package qtcore

import "fmt"

// Small Qt value types become plain Go values at the boundary, so they never
// need freeing.

// QPoint is Qt's QPoint: an integer point.
type QPoint struct{ X, Y int }

// QPointF is Qt's QPointF: a floating-point point.
type QPointF struct{ X, Y float64 }

// QSize is Qt's QSize. Qt's invalid size is {-1, -1}.
type QSize struct{ Width, Height int }

// QSizeF is Qt's QSizeF.
type QSizeF struct{ Width, Height float64 }

// QRect is Qt's QRect, stored as origin plus size.
type QRect struct{ X, Y, Width, Height int }

// QRectF is Qt's QRectF, stored as origin plus size.
type QRectF struct{ X, Y, Width, Height float64 }

// QMargins is Qt's QMargins.
type QMargins struct{ Left, Top, Right, Bottom int }

func (p QPoint) String() string  { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }
func (s QSize) String() string   { return fmt.Sprintf("%dx%d", s.Width, s.Height) }
func (r QRect) String() string   { return fmt.Sprintf("%dx%d+%d+%d", r.Width, r.Height, r.X, r.Y) }
func (p QPointF) String() string { return fmt.Sprintf("(%g,%g)", p.X, p.Y) }

// Add returns p+q.
func (p QPoint) Add(q QPoint) QPoint { return QPoint{p.X + q.X, p.Y + q.Y} }

// Sub returns p-q.
func (p QPoint) Sub(q QPoint) QPoint { return QPoint{p.X - q.X, p.Y - q.Y} }

// ToF converts to a QPointF.
func (p QPoint) ToF() QPointF { return QPointF{float64(p.X), float64(p.Y)} }

// ToPoint rounds to a QPoint.
func (p QPointF) ToPoint() QPoint { return QPoint{roundInt(p.X), roundInt(p.Y)} }

// IsValid reports whether both dimensions are non-negative.
func (s QSize) IsValid() bool { return s.Width >= 0 && s.Height >= 0 }

// IsEmpty reports whether either dimension is zero or negative.
func (s QSize) IsEmpty() bool { return s.Width <= 0 || s.Height <= 0 }

// IsEmpty reports whether the rectangle has no area.
func (r QRect) IsEmpty() bool { return r.Width <= 0 || r.Height <= 0 }

// Left, Top, Right and Bottom follow Qt: Right is X+Width-1.
func (r QRect) Left() int   { return r.X }
func (r QRect) Top() int    { return r.Y }
func (r QRect) Right() int  { return r.X + r.Width - 1 }
func (r QRect) Bottom() int { return r.Y + r.Height - 1 }

// Size returns the rectangle's size.
func (r QRect) Size() QSize { return QSize{r.Width, r.Height} }

// TopLeft returns the origin.
func (r QRect) TopLeft() QPoint { return QPoint{r.X, r.Y} }

// Center returns the centre point (rounded down, as Qt does).
func (r QRect) Center() QPoint { return QPoint{r.X + (r.Width-1)/2, r.Y + (r.Height-1)/2} }

// Contains reports whether p lies inside the rectangle.
func (r QRect) Contains(p QPoint) bool {
	return !r.IsEmpty() && p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// Intersects reports whether the rectangles overlap.
func (r QRect) Intersects(o QRect) bool {
	return !r.IsEmpty() && !o.IsEmpty() && r.X < o.X+o.Width && o.X < r.X+r.Width && r.Y < o.Y+o.Height && o.Y < r.Y+r.Height
}

// Translated returns the rectangle moved by d.
func (r QRect) Translated(d QPoint) QRect { return QRect{r.X + d.X, r.Y + d.Y, r.Width, r.Height} }

// Adjusted returns the rectangle with dx1/dy1 added to the top-left and
// dx2/dy2 added to the bottom-right corner.
func (r QRect) Adjusted(dx1, dy1, dx2, dy2 int) QRect {
	return QRect{r.X + dx1, r.Y + dy1, r.Width - dx1 + dx2, r.Height - dy1 + dy2}
}

// ToF converts to a QRectF.
func (r QRect) ToF() QRectF {
	return QRectF{float64(r.X), float64(r.Y), float64(r.Width), float64(r.Height)}
}

// Contains reports whether p lies inside the rectangle.
func (r QRectF) Contains(p QPointF) bool {
	return p.X >= r.X && p.X <= r.X+r.Width && p.Y >= r.Y && p.Y <= r.Y+r.Height
}

// ToRect rounds to a QRect.
func (r QRectF) ToRect() QRect {
	return QRect{roundInt(r.X), roundInt(r.Y), roundInt(r.Width), roundInt(r.Height)}
}

func roundInt(f float64) int {
	if f < 0 {
		return int(f - 0.5)
	}
	return int(f + 0.5)
}

// QMarginsF is Qt's QMarginsF.
type QMarginsF struct{ Left, Top, Right, Bottom float64 }

// Pair is a std::pair from Qt: gradient stops, URL query items, and other
// lists of two-part entries.
type Pair[A, B any] struct {
	First  A
	Second B
}
